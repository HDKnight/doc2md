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
	outDir := filepath.Join(root, "testdata", "out", "ocr_probe")
	_ = os.MkdirAll(outDir, 0755)

	jobs := []struct {
		name, path, model string
		dpi               int
	}{
		{"jianda_tiny72", `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》简答题 --【16页】.pdf`, "tiny", 72},
		{"fuxi_tiny72", `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》复习题.pdf`, "tiny", 72},
	}
	for _, j := range jobs {
		data, err := os.ReadFile(j.path)
		if err != nil {
			fmt.Println(j.name, err)
			continue
		}
		_ = ocr.DefaultPool().Close()
		t0 := time.Now()
		res, err := convert.Convert(context.Background(), filepath.Base(j.path), data, convert.Options{
			OCRMode: pdf.OCROn, OCRModel: j.model, DPI: j.dpi, OCR: ocr.NewFromEnv(),
		})
		if err != nil {
			fmt.Println(j.name, err)
			continue
		}
		_ = os.WriteFile(filepath.Join(outDir, j.name+"_fixed.md"), []byte(res.Markdown), 0644)
		fmt.Printf("\n%s elapsed=%s md=%d\n", j.name, time.Since(t0).Round(time.Millisecond), utf8.RuneCountInString(res.Markdown))
		if res.OCRStats != nil {
			fmt.Printf("  ocr pages=%d total_ms=%d avg=%.0f\n", res.OCRStats.Pages, res.OCRStats.TotalMs, res.OCRStats.AvgMsPerPage)
		}
		i := strings.Index(res.Markdown, "贯彻治病救人的方针政策")
		if i >= 0 {
			lineEnd := strings.Index(res.Markdown[i:], "\n")
			line := res.Markdown[i : i+lineEnd]
			fmt.Printf("  title_line=%q\n", line)
			fmt.Printf("  split_ok=%v\n", strings.Contains(res.Markdown[i:], "方针政策\n“党内") || strings.Contains(res.Markdown[i:], "方针政策\n党内"))
		} else {
			fmt.Println("  (no 贯彻治病救人)")
		}
		for _, m := range []string{"第一次明确提出", "伟大的历史文件", "文件》，第一次"} {
			fmt.Printf("  %s=%v\n", m, strings.Contains(res.Markdown, m))
		}
	}
}
