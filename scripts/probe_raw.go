package main

import (
	"fmt"
	"os"

	"doc2md/internal/pdf"
)

func main() {
	path := `C:\Users\96511\Documents\党政学习\马克思主义基本原理\《马克思主义基本原理》配套题库（考研真题精选＋章节题库）.pdf`
	data, _ := os.ReadFile(path)
	pages, err := pdf.ExtractLinesFromBytes(data)
	if err != nil {
		panic(err)
	}
	n, chars := 0, 0
	for _, p := range pages {
		for _, ln := range p.Lines {
			n++
			chars += len([]rune(ln.Text))
		}
	}
	fmt.Println("pages", len(pages), "lines", n, "runes", chars)
}
