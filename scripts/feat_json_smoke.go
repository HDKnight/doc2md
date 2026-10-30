//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"doc2md/internal/feat"
)

func main() {
	dir := filepath.Join("testdata", "out", "oar_structure_json")
	var pages []feat.Page
	for i := 1; i <= 3; i++ {
		matches, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("*_p%d.json", i)))
		if len(matches) == 0 {
			continue
		}
		b, err := os.ReadFile(matches[0])
		if err != nil {
			panic(err)
		}
		p, err := feat.ParsePageJSON(b)
		if err != nil {
			panic(err)
		}
		pages = append(pages, p)
	}
	md := feat.MarkdownFromPages(pages)
	out := filepath.Join("testdata", "out", "ocr_probe", "jianda_feat_from_json.md")
	_ = os.WriteFile(out, []byte(md), 0o644)
	fmt.Printf("pages=%d md=%d headings=%v -> %s\n", len(pages), len(md), strings.Contains(md, "## "), out)
}
