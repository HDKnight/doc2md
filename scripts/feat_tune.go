//go:build ignore

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"doc2md/internal/feat"
	"doc2md/internal/ocr"
)

func main() {
	root, _ := os.Getwd()
	if filepath.Base(root) == "scripts" {
		root = filepath.Dir(root)
	}
	pdf := `C:\Users\96511\Documents\党政学习\思想政治教育\《中国共产党思想政治教育史》简答题 --【16页】.pdf`
	if len(os.Args) > 1 {
		pdf = os.Args[1]
	}
	bin := ocr.ResolveOarBin()
	if bin == "" {
		panic("oar not found")
	}
	outDir := filepath.Join(root, "testdata", "out", "oar_structure_json")
	_ = os.RemoveAll(outDir)
	_ = os.MkdirAll(outDir, 0o755)

	dpi := 72
	cmd := exec.Command(bin, "structure", pdf, "--dpi", strconv.Itoa(dpi), "--format", "json", "-o", outDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic(err)
	}
	files, _ := filepath.Glob(filepath.Join(outDir, "*.json"))
	sort.Slice(files, func(i, j int) bool {
		return pageNum(files[i]) < pageNum(files[j])
	})
	var pages []feat.Page
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			panic(err)
		}
		p, err := feat.ParsePageJSON(b)
		if err != nil {
			panic(err)
		}
		pages = append(pages, p)
		fmt.Printf("--- %s blocks=%d ---\n", filepath.Base(f), len(p.Blocks))
	}
	md := feat.MarkdownFromPages(pages)
	outMD := filepath.Join(root, "testdata", "out", "ocr_probe", "jianda_feat_tuned.md")
	_ = os.WriteFile(outMD, []byte(md), 0o644)

	// stats
	heads, paras := 0, 0
	for _, ln := range strings.Split(md, "\n") {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			heads++
		} else {
			paras++
		}
	}
	fmt.Printf("pages=%d md=%d bytes headings=%d paras=%d -> %s\n", len(pages), len(md), heads, paras, outMD)
}

func pageNum(path string) int {
	base := filepath.Base(path)
	i := strings.LastIndex(base, "_p")
	if i < 0 {
		return 0
	}
	rest := strings.TrimSuffix(base[i+2:], filepath.Ext(base))
	n, _ := strconv.Atoi(rest)
	return n
}
