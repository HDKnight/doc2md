package textnorm

import (
	"strings"
	"testing"
)

func TestExpandGluedChapterIntro(t *testing.T) {
	lines := expandGluedLines([]string{
		"第七章 共产主义崇高理想及其最终实现简介马克思主义理论研究",
	})
	if len(lines) < 3 || lines[1] != "简介" {
		t.Fatalf("got %#v", lines)
	}
}

func TestStripTOCLines(t *testing.T) {
	in := []string{
		"目录",
		"简介 ........................ 1",
		"第一章 ...................... 3",
		"这是足够长的正文起始行，用来结束目录识别并且保留下来作为正文内容，字数必须超过四十个汉字。",
		"一、概念",
	}
	out := stripTOCLines(in)
	joined := strings.Join(out, "\n")
	if strings.HasPrefix(joined, "目录") || strings.Contains(joined, "........") {
		t.Fatalf("toc not stripped: %q", joined)
	}
	if !strings.Contains(joined, "这是足够长的正文起始行") {
		t.Fatalf("body lost: %q", joined)
	}
	if !strings.Contains(joined, "一、概念") {
		t.Fatalf("section lost: %q", joined)
	}
}
