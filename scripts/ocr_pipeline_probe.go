//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"doc2md/internal/convert"
	"doc2md/internal/pdf"
)

func main() {
	pdfPath := `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》简答题 --【16页】.pdf`
	if len(os.Args) > 1 {
		pdfPath = os.Args[1]
	}
	root, _ := os.Getwd()
	if filepath.Base(root) == "scripts" {
		root = filepath.Dir(root)
	}
	outDir := filepath.Join(root, "testdata", "out", "ocr_probe")
	_ = os.MkdirAll(outDir, 0o755)

	data, err := os.ReadFile(pdfPath)
	if err != nil {
		panic(err)
	}
	for _, pipe := range []convert.Pipeline{convert.PipelineRules, convert.PipelineFeat, convert.PipelineStructure} {
		t0 := time.Now()
		res, err := convert.Convert(context.Background(), filepath.Base(pdfPath), data, convert.Options{
			OCRMode:  pdf.OCROn,
			OCRModel: "tiny",
			DPI:      72,
			Pipeline: pipe,
		})
		elapsed := time.Since(t0).Round(time.Millisecond)
		name := string(pipe)
		if err != nil {
			fmt.Printf("%s FAIL %v\n", name, err)
			continue
		}
		out := filepath.Join(outDir, "jianda_pipe_"+name+".md")
		_ = os.WriteFile(out, []byte(res.Markdown), 0o644)
		stats := ""
		if res.OCRStats != nil {
			stats = fmt.Sprintf(" ocr=%dms/%dp", res.OCRStats.TotalMs, res.OCRStats.Pages)
		}
		fmt.Printf("%s ok engine=%s wall=%s%s md=%d -> %s\n",
			name, res.Engine, elapsed, stats, len(res.Markdown), out)
	}
}
