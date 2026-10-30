package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"doc2md/internal/ocr"
	"doc2md/internal/pdf"
	"doc2md/internal/textnorm"
)

func main() {
	root, _ := os.Getwd()
	_ = os.Setenv("DOC2MD_OCR_PYTHON", filepath.Join(root, ".venv-fpaddle", "Scripts", "python.exe"))
	_ = os.Setenv("DOC2MD_PDFTOPPM", filepath.Join(root, "tools", "poppler", "Library", "bin", "pdftoppm.exe"))
	path := `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》复习题.pdf`
	data, _ := os.ReadFile(path)
	tmp, _ := os.CreateTemp("", "doc2md-*.pdf")
	pdfPath := tmp.Name()
	_, _ = tmp.Write(data)
	_ = tmp.Close()
	defer os.Remove(pdfPath)

	workDir, imgs, err := pdf.RenderPagesForOCR(context.Background(), pdfPath, 72)
	if err != nil {
		fmt.Println("render", err)
		return
	}
	defer os.RemoveAll(workDir)
	_ = ocr.DefaultPool().Close()
	pages, err := ocr.OCRPagesParallel(context.Background(), imgs, "tiny", false, 1, ocr.NewFromEnv())
	if err != nil {
		fmt.Println("ocr", err)
		return
	}
	var lines []ocr.Line
	boxed := 0
	for _, pl := range pages {
		for _, ln := range pl {
			lines = append(lines, ln)
			if ln.HasBBox {
				boxed++
			}
		}
	}
	fmt.Printf("lines=%d boxed=%d\n", len(lines), boxed)
	geom := ocr.GeometryPlain(lines)
	prep := textnorm.PreparePlain(geom)
	fmt.Printf("geom_chars=%d prep_chars=%d geom_lines=%d prep_lines=%d\n",
		utf8.RuneCountInString(geom), utf8.RuneCountInString(prep),
		countNL(geom)+1, countNL(prep)+1)
	// count markers at line start
	fmt.Println("geom marker lines:", countMarkers(geom))
	fmt.Println("prep marker lines:", countMarkers(prep))
	// show if 59/60 still separate in geom
	for _, needle := range []string{"59、", "60、", "65、"} {
		fmt.Printf("geom has \\n%s = %v\n", needle, strings.Contains(geom, "\n"+needle) || strings.HasPrefix(geom, needle))
	}
	_ = os.WriteFile(filepath.Join(root, "testdata", "out", "ocr_probe", "fuxi_geom_only.txt"), []byte(geom), 0644)
}

func countNL(s string) int { return strings.Count(s, "\n") }

func countMarkers(s string) int {
	n := 0
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if textnorm.StartsWithMarker(ln) {
			n++
		}
	}
	return n
}
