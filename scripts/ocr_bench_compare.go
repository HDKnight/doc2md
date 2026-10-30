// Compare faster-paddle vs oar-ocr CLI on the same rendered PDF pages.
//
//	go run ./scripts/ocr_bench_compare.go
//
// Env:
//
//	DOC2MD_OCR_PYTHON / .venv-fpaddle — faster-paddle
//	DOC2MD_OCR_OAR or PATH oar — oar-ocr CLI (`cargo install oar-ocr-cli`)
//	DOC2MD_OCR_MODEL — tiny|small (default tiny)
//	DOC2MD_OCR_DPI — render DPI (default 72)
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"doc2md/internal/ocr"
	"doc2md/internal/pdf"
	"doc2md/internal/textnorm"
)

func main() {
	root, _ := os.Getwd()
	_ = os.Setenv("DOC2MD_OCR_PYTHON", filepath.Join(root, ".venv-fpaddle", "Scripts", "python.exe"))
	_ = os.Setenv("DOC2MD_PDFTOPPM", filepath.Join(root, "tools", "poppler", "Library", "bin", "pdftoppm.exe"))

	model := ocr.NormalizeModel(os.Getenv("DOC2MD_OCR_MODEL"))
	dpi := ocr.DefaultOCRDPI()
	outDir := filepath.Join(root, "testdata", "out", "ocr_bench")
	_ = os.MkdirAll(outDir, 0755)

	jobs := []struct {
		name, path string
	}{
		{"jianda", `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》简答题 --【16页】.pdf`},
		{"fuxi", `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》复习题.pdf`},
	}

	anchors := []string{
		"贯彻治病救人的方针政策",
		"第一次明确提出",
		"伟大的历史文件",
		"三查三整",
	}

	engines := []struct {
		name string
		make func() (ocr.Engine, error)
	}{
		{"fpaddle", func() (ocr.Engine, error) {
			eng := ocr.NewFasterPaddleFromEnv()
			if _, ok := eng.(ocr.Stub); ok {
				return nil, fmt.Errorf("faster-paddle not available (tools/ocr_cli.py + .venv-fpaddle)")
			}
			return eng, nil
		}},
		{"oar", func() (ocr.Engine, error) {
			return ocr.NewOarFromEnv()
		}},
	}

	for _, j := range jobs {
		data, err := os.ReadFile(j.path)
		if err != nil {
			fmt.Printf("\n== %s SKIP read: %v\n", j.name, err)
			continue
		}
		tmp, err := os.CreateTemp("", "doc2md-bench-*.pdf")
		if err != nil {
			fmt.Println(j.name, err)
			continue
		}
		pdfPath := tmp.Name()
		_, _ = tmp.Write(data)
		_ = tmp.Close()

		fmt.Printf("\n======== %s model=%s dpi=%d ========\n", j.name, model, dpi)
		workDir, imgs, err := pdf.RenderPagesForOCR(context.Background(), pdfPath, dpi)
		_ = os.Remove(pdfPath)
		if err != nil {
			fmt.Println("render", err)
			continue
		}

		for _, e := range engines {
			eng, err := e.make()
			if err != nil {
				fmt.Printf("[%s] SKIP: %v\n", e.name, err)
				continue
			}
			if cfg, ok := eng.(ocr.ModelConfigurer); ok {
				cfg.ConfigureOCR(model, false)
			}

			// Warm-up (first page) — excluded from timed totals.
			warmCtx, warmCancel := context.WithTimeout(context.Background(), 5*time.Minute)
			warmOK := false
			if fe, ok := eng.(ocr.FileEngine); ok {
				if _, err := fe.OCRFile(warmCtx, 1, imgs[0]); err == nil {
					warmOK = true
				}
			}
			if !warmOK {
				if _, err := eng.OCRImage(warmCtx, 1, ocrWarmPNG()); err != nil {
					fmt.Printf("[%s] warm failed: %v\n", e.name, err)
					warmCancel()
					_ = eng.Close()
					continue
				}
			}
			warmCancel()

			t0 := time.Now()
			var all []ocr.Line
			pageCount := len(imgs)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			var ocrErr error
			if batch, ok := eng.(ocr.BatchFileEngine); ok {
				var pages [][]ocr.Line
				pages, ocrErr = batch.OCRFiles(ctx, imgs)
				for _, pl := range pages {
					all = append(all, pl...)
				}
			} else if fe, ok := eng.(ocr.FileEngine); ok {
				for i, p := range imgs {
					var lines []ocr.Line
					lines, ocrErr = fe.OCRFile(ctx, i+1, p)
					if ocrErr != nil {
						ocrErr = fmt.Errorf("page %d: %w", i+1, ocrErr)
						break
					}
					all = append(all, lines...)
				}
			} else {
				ocrErr = fmt.Errorf("engine lacks FileEngine")
			}
			cancel()
			if ocrErr != nil {
				fmt.Printf("[%s] OCR failed: %v\n", e.name, ocrErr)
				_ = eng.Close()
				continue
			}
			elapsed := time.Since(t0)
			boxed := 0
			for _, ln := range all {
				if ln.HasBBox {
					boxed++
				}
			}
			geom := ocr.GeometryPlain(all)
			prep := textnorm.PreparePlain(geom)
			avg := float64(elapsed.Milliseconds()) / float64(pageCount)

			base := filepath.Join(outDir, fmt.Sprintf("%s_%s_%s", j.name, e.name, model))
			_ = os.WriteFile(base+"_geom.txt", []byte(geom), 0644)
			_ = os.WriteFile(base+"_prep.md", []byte(prep), 0644)

			fmt.Printf("[%s] pages=%d lines=%d boxed=%d elapsed=%s avg=%.0fms/page geom_chars=%d prep_chars=%d\n",
				e.name, pageCount, len(all), boxed, elapsed.Round(time.Millisecond), avg,
				utf8.RuneCountInString(geom), utf8.RuneCountInString(prep))
			for _, a := range anchors {
				fmt.Printf("  anchor %q=%v\n", a, strings.Contains(geom, a) || strings.Contains(prep, a))
			}
			if i := strings.Index(prep, "贯彻治病救人的方针政策"); i >= 0 {
				rest := prep[i:]
				lineEnd := strings.Index(rest, "\n")
				if lineEnd < 0 {
					lineEnd = len(rest)
				}
				line := rest[:lineEnd]
				splitOK := strings.Contains(rest, "方针政策\n“党内") || strings.Contains(rest, "方针政策\n党内")
				fmt.Printf("  title_line=%q split_ok=%v\n", trimRunes(line, 80), splitOK)
			}
			_ = eng.Close()
		}
		_ = os.RemoveAll(workDir)
	}
	fmt.Println("\nartifacts:", outDir)
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func ocrWarmPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0,
		0x00, 0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xfe, 0xd4, 0xef, 0x00, 0x00, 0x00, 0x00,
		0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}
