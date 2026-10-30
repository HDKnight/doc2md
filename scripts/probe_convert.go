package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"doc2md/internal/convert"
	"doc2md/internal/pdf"
)

func main() {
	paths := []string{
		`C:\Users\96511\Documents\党政学习\马克思主义基本原理\《马克思主义基本原理》配套题库（考研真题精选＋章节题库）.pdf`,
		`C:\Users\96511\Documents\党政学习\马克思主义基本原理\《马克思主义基本原理》期末章节笔记.pdf`,
	}
	ids := []string{"tiku", "notes"}
	reHead := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Println(ids[i], "read err", err)
			continue
		}
		ol, _ := pdf.ExtractOutlineFromBytes(data)
		fmt.Printf("%s outline=%d\n", ids[i], len(ol))
		res, err := convert.Convert(nil, path, data, convert.Options{OCRMode: pdf.OCROff})
		if err != nil {
			fmt.Println(ids[i], "convert err", err)
			continue
		}
		heads := reHead.FindAllStringSubmatch(res.Markdown, -1)
		fmt.Printf("%s engine=%s md_chars=%d heads=%d text_chars=%d\n", ids[i], res.Engine, len(res.Markdown), len(heads), len(res.Text))
		for j, h := range heads {
			if j >= 12 {
				break
			}
			ttl := h[2]
			if len([]rune(ttl)) > 60 {
				ttl = string([]rune(ttl)[:60]) + "…"
			}
			fmt.Printf("  %s %s\n", h[1], ttl)
		}
		long := 0
		for _, h := range heads {
			if len([]rune(h[2])) > 80 {
				long++
			}
		}
		fmt.Printf("  long_heading_gt80=%d\n", long)
		out := fmt.Sprintf(`f:\code\java\workspace\aitest\doc2md\testdata\out\%s_v5.md`, ids[i])
		_ = os.WriteFile(out, []byte(res.Markdown), 0644)
		fmt.Println("  wrote", out)
		_ = strings.TrimSpace
	}
}
