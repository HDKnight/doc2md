package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"doc2md/internal/convert"
	"doc2md/internal/ocr"
	"doc2md/internal/pdf"
	"doc2md/internal/textnorm"
)

func main() {
	root, _ := os.Getwd()
	_ = os.Setenv("DOC2MD_OCR_PYTHON", filepath.Join(root, ".venv-fpaddle", "Scripts", "python.exe"))
	_ = os.Setenv("DOC2MD_PDFTOPPM", filepath.Join(root, "tools", "poppler", "Library", "bin", "pdftoppm.exe"))
	path := `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》简答题 --【16页】.pdf`
	data, _ := os.ReadFile(path)
	_ = ocr.DefaultPool().Close()
	res, err := convert.Convert(context.Background(), "x.pdf", data, convert.Options{
		OCRMode: pdf.OCROn, OCRModel: "tiny", DPI: 72, OCR: ocr.NewFromEnv(),
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("=== TEXT lines of interest ===")
	var interesting []string
	for _, ln := range strings.Split(res.Text, "\n") {
		if strings.Contains(ln, "贯彻治病救人") || strings.Contains(ln, "加强党员干部的党性") ||
			strings.Contains(ln, "坚持实事求是的思想") || strings.Contains(ln, "运用唯物辩证") {
			interesting = append(interesting, ln)
			rs := []rune(ln)
			head := ln
			if len(rs) > 60 {
				head = string(rs[:60]) + "…"
			}
			fmt.Printf("len=%d %q\n", len(rs), head)
		}
	}
	fmt.Println("\n=== ExpandGluedLines on those ===")
	for _, ln := range textnorm.ExpandGluedLines(interesting) {
		rs := []rune(ln)
		head := ln
		if len(rs) > 50 {
			head = string(rs[:50]) + "…"
		}
		fmt.Printf("=> %q\n", head)
	}
	// simulate ToBlocks path on a reconstructed snippet
	snippet := strings.Join(interesting, "\n")
	blocks := textnorm.ToBlocks(snippet)
	fmt.Println("\n=== ToBlocks headings ===")
	for _, b := range blocks {
		if b.Type == "heading" {
			fmt.Printf("H%d %q\n", b.Level, b.Text)
		}
	}
}
