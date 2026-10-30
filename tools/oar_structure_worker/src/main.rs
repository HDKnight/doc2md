//! Warm structure worker for doc2md.
//!
//!   oar_structure_worker --worker
//!   {"op":"structure","pdf":"...","out":"...","dpi":96,"format":"json","pages":[1,2]}
//!     → {"ok":true,"pages":N,"ms":…}  (writes {stem}_p{N}.json|.md under out)
//!   pages: optional 1-based page list; omit = all pages (prefer small batches from Go)
//!   EXIT
//!
//! Models stay loaded until the process is killed (Go idle timer).

use anyhow::{Context, Result, anyhow, ensure};
use hayro::{PixmapSettings, RenderCache, hayro_syntax::Pdf};
use image::RgbImage;
use oar_ocr::core::config::{OrtGraphOptimizationLevel, OrtSessionConfig};
use oar_ocr::prelude::*;
use serde::Deserialize;
use serde_json::{Value, json};
use std::env;
use std::fs::{self, File};
use std::io::{self, BufRead, Read, Write};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Instant;

#[derive(Debug, Deserialize)]
struct Req {
    op: String,
    #[serde(default)]
    pdf: String,
    #[serde(default)]
    out: String,
    #[serde(default = "default_dpi")]
    dpi: f32,
    #[serde(default = "default_format")]
    format: String,
    /// Optional 1-based page numbers; empty = all pages.
    #[serde(default)]
    pages: Vec<usize>,
}

fn default_dpi() -> f32 {
    96.0
}
fn default_format() -> String {
    "json".into()
}

struct PdfInput {
    path: PathBuf,
    pdf: Pdf,
    pages: Vec<usize>,
}

impl PdfInput {
    fn open(path: &Path, want: &[usize]) -> Result<Self> {
        ensure!(path.is_file(), "not a file: {}", path.display());
        let mut file = File::open(path)?;
        let mut data = Vec::new();
        file.read_to_end(&mut data)?;
        ensure!(
            data.starts_with(b"%PDF-"),
            "not a PDF: {}",
            path.display()
        );
        let pdf = Pdf::new(Arc::new(data))
            .map_err(|e| anyhow!("parse PDF {}: {e:?}", path.display()))?;
        let count = pdf.pages().len();
        ensure!(count > 0, "PDF has no pages: {}", path.display());
        let pages = if want.is_empty() {
            (1..=count).collect()
        } else {
            let mut v = Vec::with_capacity(want.len());
            for &n in want {
                ensure!(n >= 1 && n <= count, "page {n} out of range 1..{count}");
                v.push(n);
            }
            v
        };
        Ok(Self {
            path: path.to_path_buf(),
            pdf,
            pages,
        })
    }

    fn render(&self, number: usize, dpi: f32) -> Result<RgbImage> {
        let page = &self.pdf.pages()[number - 1];
        let scale = dpi / 72.0;
        let (width, height) = page.render_dimensions();
        ensure!(
            [width * scale, height * scale]
                .iter()
                .all(|s| s.is_finite() && *s >= 1.0 && *s < 65536.0),
            "page {number} render size out of range; adjust dpi"
        );
        let pixmap_settings = PixmapSettings {
            x_scale: scale,
            y_scale: scale,
            bg_color: hayro::vello_cpu::color::palette::css::WHITE,
        };
        let cache = RenderCache::new();
        let pixmap = hayro::render(
            page,
            &cache,
            &Default::default(),
            &Default::default(),
            &pixmap_settings,
        );
        let data = pixmap
            .data_as_u8_slice()
            .as_chunks::<4>()
            .0
            .iter()
            .flat_map(|rgba| rgba[..3].iter().copied())
            .collect();
        RgbImage::from_raw(u32::from(pixmap.width()), u32::from(pixmap.height()), data)
            .context("RGB convert failed")
    }
}

fn write_line(obj: &Value) -> io::Result<()> {
    let mut out = io::stdout().lock();
    serde_json::to_writer(&mut out, obj)?;
    out.write_all(b"\n")?;
    out.flush()
}

fn build_structure() -> Result<OARStructure> {
    let ort = OrtSessionConfig::auto()
        .with_optimization_level(OrtGraphOptimizationLevel::All)
        .with_parallel_execution(true);
    OARStructureBuilder::pp_structurev3()
        .ort_session(ort)
        .build()
        .map_err(|e| anyhow!("load structure models: {e}"))
}

fn run_structure(model: &OARStructure, req: &Req) -> Result<usize> {
    let pdf_path = PathBuf::from(&req.pdf);
    let out_dir = PathBuf::from(&req.out);
    let dpi = if req.dpi.is_finite() && req.dpi > 0.0 {
        req.dpi
    } else {
        96.0
    };
    let as_json = !req.format.eq_ignore_ascii_case("markdown")
        && !req.format.eq_ignore_ascii_case("md");

    fs::create_dir_all(&out_dir)
        .with_context(|| format!("create out dir {}", out_dir.display()))?;

    let input = PdfInput::open(&pdf_path, &req.pages)?;
    let stem = pdf_path
        .file_stem()
        .map(|s| s.to_string_lossy().into_owned())
        .unwrap_or_else(|| "page".into());
    let ext = if as_json { "json" } else { "md" };

    // Infer one page at a time when caller already batches; keep small if many pages still sent.
    const BATCH: usize = 1;
    let mut done = 0usize;
    for chunk in input.pages.chunks(BATCH) {
        let images = chunk
            .iter()
            .map(|n| input.render(*n, dpi))
            .collect::<Result<Vec<_>>>()?;
        let dims: Vec<_> = images.iter().map(RgbImage::dimensions).collect();
        let results = model.predict_images(images);
        for (i, (page_no, res)) in chunk.iter().zip(results.into_iter()).enumerate() {
            let mut page = res.map_err(|e| anyhow!("structure page {page_no}: {e}"))?;
            page.input_path = input.path.to_string_lossy().into_owned().into();
            page.index = *page_no - 1;
            let (w, h) = dims[i];
            let mut json = page.to_json(w, h);
            if let Some(p) = json.get_mut("page") {
                p["index"] = (*page_no - 1).into();
            }
            json["source"] = input.path.to_string_lossy().into_owned().into();

            let out_path = out_dir.join(format!("{stem}_p{page_no}.{ext}"));
            let body = if as_json {
                serde_json::to_string_pretty(&json)?
            } else {
                page.to_markdown()
            };
            fs::write(&out_path, body)
                .with_context(|| format!("write {}", out_path.display()))?;
            done += 1;
        }
    }
    Ok(done)
}

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    let worker = args
        .first()
        .map(|a| a == "--worker" || a == "-w")
        .unwrap_or(false);
    if !worker {
        eprintln!("usage: oar_structure_worker --worker");
        std::process::exit(2);
    }

    eprintln!("[oar_structure_worker] loading pp_structurev3 …");
    let model = match build_structure() {
        Ok(m) => m,
        Err(e) => {
            eprintln!("oar_structure_worker: {e:#}");
            std::process::exit(1);
        }
    };
    eprintln!("[oar_structure_worker] ready");

    let stdin = io::stdin();
    for line in stdin.lock().lines() {
        let Ok(line) = line else { break };
        let raw = line.trim();
        if raw.is_empty() {
            continue;
        }
        if raw.eq_ignore_ascii_case("EXIT") || raw.eq_ignore_ascii_case("QUIT") {
            break;
        }
        let req: Req = match serde_json::from_str(raw) {
            Ok(r) => r,
            Err(e) => {
                let _ = write_line(&json!({"ok": false, "error": format!("bad request: {e}")}));
                continue;
            }
        };
        if !req.op.eq_ignore_ascii_case("structure") {
            let _ = write_line(&json!({"ok": false, "error": format!("unknown op {}", req.op)}));
            continue;
        }
        if req.pdf.is_empty() || req.out.is_empty() {
            let _ = write_line(&json!({"ok": false, "error": "pdf and out required"}));
            continue;
        }
        let t0 = Instant::now();
        match run_structure(&model, &req) {
            Ok(pages) => {
                let _ = write_line(&json!({
                    "ok": true,
                    "pages": pages,
                    "ms": t0.elapsed().as_millis() as u64,
                }));
            }
            Err(e) => {
                let _ = write_line(&json!({
                    "ok": false,
                    "error": format!("{e:#}"),
                }));
            }
        }
    }
}
