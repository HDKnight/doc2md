package indra

import (
	"strings"
	"testing"

	"doc2md/internal/feat"
)

func TestMarkdownFromPages_NoFalseHeadings(t *testing.T) {
	pages := []feat.Page{{
		Blocks: []feat.Block{
			{Type: "paragraph_title", Content: "《中国共产党思想政治教育史》复习题"},
			{Type: "text", Content: "一、单项选择题"},
			{Type: "paragraph_title", Content: "2、（ A）是比较系统地、正确地传播马克思主义"},
			{Type: "text", Content: "A、李大钊 B、陈独秀"},
			{Type: "paragraph_title", Content: "这是一段很长很长很长很长很长很长很长很长很长很长很长很长很长很长的被误标成标题的正文内容"},
		},
	}}
	md := MarkdownFromPages(pages)
	if strings.Contains(md, "###") || strings.Count(md, "## ") > 1 {
		t.Fatalf("unexpected headings:\n%s", md)
	}
	if !strings.Contains(md, "## 《中国共产党") {
		t.Fatalf("real title missing:\n%s", md)
	}
	if strings.Contains(md, "## 2、") {
		t.Fatalf("quiz stem must not be heading:\n%s", md)
	}
	if strings.Contains(md, "## 这是一段很长") {
		t.Fatalf("long body must not be heading:\n%s", md)
	}
	if !strings.Contains(md, "2、（ A）是比较系统") {
		t.Fatalf("stem as para missing:\n%s", md)
	}
}
