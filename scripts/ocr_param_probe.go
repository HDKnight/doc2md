package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"doc2md/internal/convert"
	"doc2md/internal/ocr"
	"doc2md/internal/pdf"
)

func main() {
	root, _ := os.Getwd()
	_ = os.Setenv("DOC2MD_OCR_PYTHON", filepath.Join(root, ".venv-fpaddle", "Scripts", "python.exe"))
	_ = os.Setenv("DOC2MD_PDFTOPPM", filepath.Join(root, "tools", "poppler", "Library", "bin", "pdftoppm.exe"))
	_ = os.Setenv("DOC2MD_PDFTOTEXT", filepath.Join(root, "tools", "poppler", "Library", "bin", "pdftotext.exe"))

	pdfPath := `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》简答题 --【16页】.pdf`
	if len(os.Args) > 1 {
		pdfPath = os.Args[1]
	}
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		fmt.Println("read:", err)
		os.Exit(1)
	}

	outDir := filepath.Join(root, "testdata", "out", "ocr_probe")
	_ = os.MkdirAll(outDir, 0755)

	type cfg struct {
		name  string
		model string
		dpi   int
		cls   bool
	}
	cfgs := []cfg{
		{name: "tiny_dpi72", model: "tiny", dpi: 72},
		{name: "tiny_dpi120", model: "tiny", dpi: 120},
		{name: "small_dpi72", model: "small", dpi: 72},
	}

	markers := []string{
		"方针政策党内",
		"贯彻治病救人的方针政策",
		"加强党员干部的党性修养",
		"第一次明确提出",
		"伟大的历史文件",
	}

	for _, c := range cfgs {
		fmt.Printf("\n======== %s model=%s dpi=%d ========\n", c.name, c.model, c.dpi)
		_ = ocr.DefaultPool().Close()
		t0 := time.Now()
		res, err := convert.Convert(context.Background(), filepath.Base(pdfPath), data, convert.Options{
			OCRMode:   pdf.OCROn,
			OCRModel:  c.model,
			OCRUseCls: c.cls,
			DPI:       c.dpi,
			OCR:       ocr.NewFromEnv(),
		})
		elapsed := time.Since(t0)
		if err != nil {
			fmt.Println("ERR", err)
			continue
		}
		md := res.Markdown
		out := filepath.Join(outDir, c.name+".md")
		_ = os.WriteFile(out, []byte(md), 0644)
		fmt.Printf("engine=%s md_chars=%d elapsed=%s\n", res.Engine, utf8.RuneCountInString(md), elapsed.Round(time.Millisecond))
		if res.OCRStats != nil {
			fmt.Printf("ocr_stats model=%s pages=%d total_ms=%d avg_ms/page=%.0f\n",
				res.OCRStats.Model, res.OCRStats.Pages, res.OCRStats.TotalMs, res.OCRStats.AvgMsPerPage)
		}
		for _, m := range markers {
			fmt.Printf("  contains %q: %v\n", m, strings.Contains(md, m))
		}
		// 粘行：标题后直接接正文
		glued := strings.Contains(md, "方针政策党内")
		splitOK := strings.Contains(md, "贯彻治病救人的方针政策") && !glued
		fmt.Printf("  glue_fix(方针政策|党内)=%v  split_ok=%v\n", glued, splitOK)
		printContext(md, "贯彻治病救人", 120)
		printContext(md, "加强党员干部", 80)
		fmt.Println("  wrote", out)
	}
}

func printContext(md, needle string, width int) {
	i := strings.Index(md, needle)
	if i < 0 {
		fmt.Printf("  ctx %q: NOT FOUND\n", needle)
		return
	}
	start := i - 20
	if start < 0 {
		start = 0
	}
	end := i + width
	if end > len(md) {
		end = len(md)
	}
	snip := md[start:end]
	snip = strings.ReplaceAll(snip, "\n", "↵")
	fmt.Printf("  ctx %q: …%s…\n", needle, snip)
}
