package geom

import (
	"strings"
	"testing"
)

func TestNumLevel(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"第一章 世界的物质性", 2, true},
		{"一、单项选择题", 3, true},
		{"（一）基本概念", 4, true},
		{"1.2 实践观", 4, true},
		{"这是普通段落。", 0, false},
		{"1. 题干很长不算章节编号层级", 0, false},
	}
	for _, c := range cases {
		got, ok := numLevel(c.in)
		if ok != c.ok || got != c.want {
			t.Fatalf("%q: got (%d,%v) want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestOutlineLocksH3AndQuestionsH4(t *testing.T) {
	long := "1. 1844年8月底，马克思和恩格斯在巴黎会面。这次会面使他们发现彼此的基本观点完全一致，于是开始了在科学理论研究和革命活动中的长期合作。马克思和恩格斯首次系统阐述历史唯物主义基本观点的著作是（ ）。"
	pages := []PageInput{
		{Page: 1, Width: 600, Height: 800, Lines: []Line{
			{Text: "简介", Page: 1, Y0: 40, Y1: 60, PageHeight: 800, FontSize: 16},
			{Text: "本书是辅导资料。", Page: 1, Y0: 80, Y1: 94, PageHeight: 800, FontSize: 12},
		}},
		{Page: 2, Width: 600, Height: 800, Lines: []Line{
			{Text: "一、单项选择题", Page: 2, Y0: 40, Y1: 60, PageHeight: 800, FontSize: 14},
			{Text: long, Page: 2, Y0: 80, Y1: 120, PageHeight: 800, FontSize: 12},
			{Text: "A. 选项甲", Page: 2, Y0: 130, Y1: 144, PageHeight: 800, FontSize: 12},
			{Text: "一、概念题 （）", Page: 2, Y0: 160, Y1: 174, PageHeight: 800, FontSize: 12},
			{Text: "1. 辩证思维方法[中央财经大学2018年研]", Page: 2, Y0: 190, Y1: 204, PageHeight: 800, FontSize: 12},
		}},
		{Page: 3, Width: 600, Height: 800, Lines: []Line{
			{Text: "二、概念题", Page: 3, Y0: 40, Y1: 60, PageHeight: 800, FontSize: 14},
			{Text: "1. 相对静止[南京航空航天大学2018研]", Page: 3, Y0: 80, Y1: 94, PageHeight: 800, FontSize: 12},
		}},
	}
	md := BuildHeadingMarkdown(pages, HeadingOptions{
		Enhance: true,
		Outline: []OutlineItem{
			{Level: 1, Title: "简介", Page: 1},
			{Level: 2, Title: "一、单项选择题", Page: 2},
			{Level: 2, Title: "二、概念题", Page: 3},
		},
		DocTitle: "题库",
	})
	if !strings.Contains(md, "# 题库") {
		t.Fatalf("missing doc title:\n%s", md)
	}
	if !strings.Contains(md, "## 简介") {
		t.Fatalf("简介 should be h2:\n%s", md)
	}
	if !strings.Contains(md, "### 一、") || !strings.Contains(md, "单项选择题") {
		t.Fatalf("bookmark section missing:\n%s", md)
	}
	// 章内题型标签不得抬成标题
	if strings.Contains(md, "### 一、概念题") || strings.Contains(md, "## 一、概念题") {
		t.Fatalf("inner 一、概念题 must stay body:\n%s", md)
	}
	if !strings.Contains(md, "概念题") || strings.Contains(md, "### 一、概念题") {
		t.Fatalf("inner label should remain as body text:\n%s", md)
	}
	// 金标：选择题题干完整进 ####，不在逗号处截断
	if !strings.Contains(md, "#### "+long) {
		t.Fatalf("full stem should be h4 heading:\n%s", md)
	}
	if !strings.Contains(md, "#### 1. 1844") {
		t.Fatalf("question should be h4:\n%s", md)
	}
	if strings.Count(md, "#####") > 0 {
		t.Fatalf("no h5 expected:\n%s", md)
	}
}

func TestSplitHeadingTitle(t *testing.T) {
	title, rest := splitHeadingTitle("1. "+strings.Repeat("字", 60), 48)
	if len([]rune(title)) > 48 {
		t.Fatalf("title too long: %d", len([]rune(title)))
	}
	if rest == "" {
		t.Fatal("expected remainder")
	}
}

func TestBuildHeadingMarkdownFontSize(t *testing.T) {
	pages := []PageInput{{
		Page: 1, Width: 600, Height: 800,
		Lines: []Line{
			{Text: "大标题", Page: 1, Y0: 40, Y1: 70, PageHeight: 800, FontSize: 22},
			{Text: "一、第一节", Page: 1, Y0: 90, Y1: 108, PageHeight: 800, FontSize: 12},
			{Text: "这是正文内容，字号较小。", Page: 1, Y0: 120, Y1: 134, PageHeight: 800, FontSize: 12},
		},
	}}
	md := BuildHeadingMarkdown(pages, HeadingOptions{})
	if !strings.Contains(md, "大标题") {
		t.Fatalf("missing title:\n%s", md)
	}
	if !strings.Contains(md, "一、") || !strings.Contains(md, "第一节") {
		t.Fatalf("missing numbered short head:\n%s", md)
	}
}

