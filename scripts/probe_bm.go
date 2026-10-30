package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

func walk(bms []pdfcpu.Bookmark, depth int) {
	shown := 0
	var rec func([]pdfcpu.Bookmark, int)
	rec = func(xs []pdfcpu.Bookmark, d int) {
		for _, b := range xs {
			if shown < 15 {
				fmt.Printf("%sL%d p%d %q\n", strings.Repeat("  ", d), d+1, b.PageFrom, b.Title)
				shown++
			}
			rec(b.Kids, d+1)
		}
	}
	rec(bms, depth)
}

func main() {
	path := `C:\Users\96511\Documents\党政学习\马克思主义基本原理\《马克思主义基本原理》配套题库（考研真题精选＋章节题库）.pdf`
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	bms, err := api.Bookmarks(f, nil)
	fmt.Println("err:", err, "top:", len(bms))
	n := 0
	var count func([]pdfcpu.Bookmark)
	count = func(xs []pdfcpu.Bookmark) {
		for _, b := range xs {
			n++
			count(b.Kids)
		}
	}
	count(bms)
	fmt.Println("total:", n)
	walk(bms, 0)

	path2 := `C:\Users\96511\Documents\党政学习\马克思主义基本原理\《马克思主义基本原理》期末章节笔记.pdf`
	f2, err := os.Open(path2)
	if err != nil {
		panic(err)
	}
	defer f2.Close()
	bms2, err := api.Bookmarks(f2, nil)
	n2 := 0
	count = func(xs []pdfcpu.Bookmark) {
		for _, b := range xs {
			n2++
			count(b.Kids)
		}
	}
	count(bms2)
	fmt.Println("notes err:", err, "total:", n2)
}
