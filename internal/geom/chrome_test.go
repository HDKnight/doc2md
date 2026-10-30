package geom

import (
	"strings"
	"testing"
)

func TestDropChromeTOCAndHeader(t *testing.T) {
	lines := []Line{
		{Text: "思想政治教育", Page: 1, X0: 100, Y0: 20, X1: 200, Y1: 35, PageWidth: 600, PageHeight: 800},
		{Text: "思想政治教育", Page: 2, X0: 100, Y0: 20, X1: 200, Y1: 35, PageWidth: 600, PageHeight: 800},
		{Text: "思想政治教育", Page: 3, X0: 100, Y0: 20, X1: 200, Y1: 35, PageWidth: 600, PageHeight: 800},
		{Text: "目录", Page: 1, X0: 40, Y0: 100, X1: 80, Y1: 120, PageWidth: 600, PageHeight: 800},
		{Text: "简介 ........................ 1", Page: 1, X0: 40, Y0: 130, X1: 400, Y1: 150, PageWidth: 600, PageHeight: 800},
		{Text: "第一章 ...................... 3", Page: 1, X0: 40, Y0: 160, X1: 400, Y1: 180, PageWidth: 600, PageHeight: 800},
		{Text: "这是足够长的正文起始行，用来结束目录识别并且保留下来作为正文内容，字数必须超过四十个汉字。", Page: 1, X0: 40, Y0: 200, X1: 500, Y1: 220, PageWidth: 600, PageHeight: 800},
		{Text: "一、概念", Page: 1, X0: 40, Y0: 240, X1: 120, Y1: 260, PageWidth: 600, PageHeight: 800},
		{Text: "12", Page: 1, X0: 280, Y0: 760, X1: 300, Y1: 780, PageWidth: 600, PageHeight: 800},
	}
	out := DropChrome(lines, 3)
	joined := strings.Join(Texts(out), "\n")
	for _, l := range out {
		if l.Text == "目录" || strings.Contains(l.Text, "........") {
			t.Fatalf("toc left: %s", joined)
		}
	}
	if strings.Contains(joined, "思想政治教育") {
		t.Fatalf("header left: %s", joined)
	}
	for _, l := range out {
		if l.Text == "12" {
			t.Fatalf("page number left: %s", joined)
		}
	}
	if !strings.Contains(joined, "这是足够长的正文起始行") || !strings.Contains(joined, "一、概念") {
		t.Fatalf("body lost: %s", joined)
	}
}
