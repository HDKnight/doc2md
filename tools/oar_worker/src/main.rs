//! Warm OCR worker for doc2md — same stdin protocol as tools/ocr_cli.py.
//!
//!   oar_worker --worker
//!   <image-path>           → {"data":[...],"ms":N,"model":"..."}
//!   BATCH ["a.jpg","b.jpg"] → {"pages":[{"data":[...]},...],...}
//!   EXIT
//!
//! Env:
//!   DOC2MD_OCR_MODEL / DOC2MD_OCR_OAR_SIZE = tiny|small|medium
//!   DOC2MD_OCR_THREADS                    = ORT intra threads (default: min(8, cpus))
//!   DOC2MD_OCR_IMAGE_BATCH                = det image batch (default: 8; CPU unset was 1)
//!   DOC2MD_OCR_REGION_BATCH               = rec region batch (default: 32)
//!   DOC2MD_OCR_DET_LIMIT                  = det limit_side_len (default: 736; 0=library default)

use oar_ocr::core::config::{OrtGraphOptimizationLevel, OrtSessionConfig};
use oar_ocr::core::OrtGlobalThreadPoolOptions;
use oar_ocr::domain::tasks::TextDetectionConfig;
use oar_ocr::prelude::*;
use oar_ocr::processors::LimitType;
use serde::Serialize;
use serde_json::{json, Value};
use std::env;
use std::io::{self, BufRead, Write};
use std::path::Path;
use std::time::Instant;

#[derive(Serialize)]
struct DataItem {
    text: String,
    x0: f64,
    y0: f64,
    x1: f64,
    y1: f64,
}

fn model_size() -> PpOcrV6Size {
    let raw = env::var("DOC2MD_OCR_OAR_SIZE")
        .or_else(|_| env::var("DOC2MD_OCR_MODEL"))
        .unwrap_or_else(|_| "tiny".into());
    match raw.trim().to_ascii_lowercase().as_str() {
        "small" | "accurate" | "quality" => PpOcrV6Size::Small,
        "medium" => PpOcrV6Size::Medium,
        _ => PpOcrV6Size::Tiny,
    }
}

fn model_label(size: PpOcrV6Size) -> &'static str {
    match size {
        PpOcrV6Size::Tiny => "tiny",
        PpOcrV6Size::Small => "small",
        PpOcrV6Size::Medium => "medium",
        _ => "tiny",
    }
}

fn env_usize(key: &str, default: usize, max: usize) -> usize {
    env::var(key)
        .ok()
        .and_then(|s| s.trim().parse().ok())
        .filter(|n| *n >= 1 && *n <= max)
        .unwrap_or(default)
}

fn threads() -> usize {
    let cpus = std::thread::available_parallelism()
        .map(|n| n.get())
        .unwrap_or(4);
    env_usize("DOC2MD_OCR_THREADS", cpus.min(8).max(2), 32)
}

fn image_batch() -> usize {
    // CPU library default is 1; 8 worked best in local benches for same-size PDF pages.
    env_usize("DOC2MD_OCR_IMAGE_BATCH", 8, 64)
}

fn region_batch(size: PpOcrV6Size) -> usize {
    let default = match size {
        PpOcrV6Size::Tiny => 32,
        _ => 16,
    };
    env_usize("DOC2MD_OCR_REGION_BATCH", default, 256)
}

fn det_limit() -> Option<u32> {
    match env::var("DOC2MD_OCR_DET_LIMIT") {
        Ok(s) => {
            let s = s.trim();
            if s.is_empty() || s == "0" || s.eq_ignore_ascii_case("default") {
                return None;
            }
            s.parse().ok().filter(|n| *n >= 32)
        }
        // 72 DPI quiz pages are usually <960; 736 cuts work on denser pages without a big quality hit.
        Err(_) => Some(736),
    }
}

fn build_engine(size: PpOcrV6Size) -> Result<OAROCR, String> {
    let nthreads = threads();
    let img_batch = image_batch();
    let reg_batch = region_batch(size);

    // Share one ORT pool across det + rec (official example recommendation).
    let mut pool = OrtGlobalThreadPoolOptions::new();
    pool = pool.with_intra_threads(nthreads);
    let _ = pool.commit();

    let ort = OrtSessionConfig::auto()
        .with_optimization_level(OrtGraphOptimizationLevel::All)
        .with_parallel_execution(true);

    let box_thresh = match size {
        PpOcrV6Size::Tiny => 0.4,
        _ => 0.45,
    };
    let mut det = TextDetectionConfig {
        score_threshold: 0.2,
        box_threshold: box_thresh,
        unclip_ratio: 1.4,
        max_candidates: 3000,
        ..TextDetectionConfig::default()
    };
    if let Some(limit) = det_limit() {
        det.limit_side_len = Some(limit);
        det.limit_type = Some(LimitType::Max);
        det.max_side_len = Some(4000);
    }

    OAROCRBuilder::pp_ocrv6(size)
        .ort_session(ort)
        .image_batch_size(img_batch)
        .region_batch_size(reg_batch)
        .text_detection_config(det)
        .build()
        .map_err(|e| format!("load oar-ocr models: {e}"))
}

fn region_item(region: &TextRegion) -> Option<DataItem> {
    let text = region.text.as_ref()?.trim();
    if text.is_empty() {
        return None;
    }
    let bb = &region.bounding_box;
    Some(DataItem {
        text: text.to_string(),
        x0: bb.x_min() as f64,
        y0: bb.y_min() as f64,
        x1: bb.x_max() as f64,
        y1: bb.y_max() as f64,
    })
}

fn page_data(result: &OAROCRResult) -> Vec<DataItem> {
    let mut items: Vec<DataItem> = result.text_regions.iter().filter_map(region_item).collect();
    items.sort_by(|a, b| {
        a.y0
            .partial_cmp(&b.y0)
            .unwrap_or(std::cmp::Ordering::Equal)
            .then_with(|| a.x0.partial_cmp(&b.x0).unwrap_or(std::cmp::Ordering::Equal))
    });
    items
}

fn run_paths(ocr: &OAROCR, paths: &[String]) -> Result<Vec<Vec<DataItem>>, String> {
    let images = load_images(paths).map_err(|e| format!("load images: {e}"))?;
    if images.len() != paths.len() {
        return Err(format!(
            "loaded {} images, want {}",
            images.len(),
            paths.len()
        ));
    }
    let results = ocr.predict(images).map_err(|e| format!("ocr predict: {e}"))?;
    Ok(results.iter().map(page_data).collect())
}

fn write_json(obj: &Value) -> io::Result<()> {
    let mut out = io::stdout().lock();
    serde_json::to_writer(&mut out, obj)?;
    out.write_all(b"\n")?;
    out.flush()
}

fn main() {
    let args: Vec<String> = env::args().skip(1).collect();
    let worker = args
        .first()
        .map(|a| a == "--worker" || a == "-w")
        .unwrap_or(false);
    if !worker && (args.is_empty() || args[0].starts_with('-')) {
        eprintln!("usage: oar_worker <image> | oar_worker --worker");
        std::process::exit(2);
    }

    let size = model_size();
    let label = model_label(size);
    let ocr = match build_engine(size) {
        Ok(e) => e,
        Err(e) => {
            eprintln!("oar_worker: {e}");
            std::process::exit(1);
        }
    };

    if worker {
        eprintln!(
            "[oar_worker] ready backend=oar-ocr model={label} threads={} img_batch={} region_batch={} det_limit={:?}",
            threads(),
            image_batch(),
            region_batch(size),
            det_limit()
        );
        let stdin = io::stdin();
        for line in stdin.lock().lines() {
            let Ok(line) = line else { break };
            let raw = line.trim();
            if raw.is_empty() || raw.eq_ignore_ascii_case("EXIT") || raw.eq_ignore_ascii_case("QUIT")
            {
                break;
            }
            let t0 = Instant::now();
            if let Some(rest) = raw
                .strip_prefix("BATCH ")
                .or_else(|| raw.strip_prefix("batch "))
            {
                match serde_json::from_str::<Vec<String>>(rest) {
                    Ok(paths) if !paths.is_empty() => match run_paths(&ocr, &paths) {
                        Ok(pages) => {
                            let pages_json: Vec<Value> = pages
                                .into_iter()
                                .map(|data| json!({ "data": data }))
                                .collect();
                            let _ = write_json(&json!({
                                "pages": pages_json,
                                "ms": t0.elapsed().as_millis() as u64,
                                "model": label,
                            }));
                        }
                        Err(e) => {
                            let _ = write_json(&json!({ "pages": [], "error": e }));
                        }
                    },
                    _ => {
                        let _ = write_json(&json!({ "pages": [], "error": "batch paths empty" }));
                    }
                }
            } else {
                match run_paths(&ocr, &[raw.to_string()]) {
                    Ok(pages) => {
                        let data = pages.into_iter().next().unwrap_or_default();
                        let _ = write_json(&json!({
                            "data": data,
                            "ms": t0.elapsed().as_millis() as u64,
                            "model": label,
                        }));
                    }
                    Err(e) => {
                        let _ = write_json(&json!({ "data": [], "error": e }));
                    }
                }
            }
        }
        return;
    }

    let path = &args[0];
    if !Path::new(path).is_file() {
        eprintln!("oar_worker: not a file: {path}");
        std::process::exit(1);
    }
    let t0 = Instant::now();
    match run_paths(&ocr, &[path.clone()]) {
        Ok(pages) => {
            let data = pages.into_iter().next().unwrap_or_default();
            let _ = write_json(&json!({
                "data": data,
                "ms": t0.elapsed().as_millis() as u64,
                "model": label,
            }));
        }
        Err(e) => {
            eprintln!("oar_worker: {e}");
            std::process::exit(1);
        }
    }
}
