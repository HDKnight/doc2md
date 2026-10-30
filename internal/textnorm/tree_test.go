package textnorm

import (
	"strings"
	"testing"
)

func TestBuildOutlineFrequencyAndStack(t *testing.T) {
	lines := []string{
		"导论",
		"一、单项选择题",
		"1. 题目甲",
		"A. 选项一",
		"B. 选项二",
		"【答案】 A",
		"二、简答题",
		"1. 题目乙",
		"第三章 实践与认识",
		"一、基本概念",
		"1. 什么是实践？",
		"2. 什么是认识？",
	}
	blocks := BuildOutline(lines)
	var md strings.Builder
	for _, b := range blocks {
		if b.Type == "heading" {
			md.WriteString(strings.Repeat("#", b.Level))
			md.WriteByte(' ')
			md.WriteString(b.Text)
			md.WriteByte('\n')
		}
	}
	out := md.String()
	if !strings.Contains(out, "## 导论") && !strings.Contains(out, "# 导论") {
		// pdf-md: short before markers → level 0 → ##
		if !strings.Contains(out, "导论") {
			t.Fatalf("missing 导论 heading:\n%s", out)
		}
	}
	if strings.Count(out, "一、") < 1 {
		t.Fatalf("expected section headings:\n%s", out)
	}
	// options must not be headings
	for _, b := range blocks {
		if b.Type == "heading" && strings.HasPrefix(b.Text, "A.") {
			t.Fatalf("option became heading: %q", b.Text)
		}
	}
}

func TestBuildOutlineSingletonNotHeading(t *testing.T) {
	lines := []string{
		"正文开头一段话。",
		"1、只出现一次的序号不应成为标题，因为它在上下文中不足两次",
		"后面继续正文。",
	}
	blocks := BuildOutline(lines)
	for _, b := range blocks {
		if b.Type == "heading" && strings.HasPrefix(b.Text, "1、") {
			t.Fatalf("singleton marker became heading: %+v", b)
		}
	}
}

func TestToBlocksDropsTOC(t *testing.T) {
	text := "目录\n简介 ........................ 1\n第一章 ...................... 2\n\n简介\n这是正文简介，内容足够长，用来结束目录块并保留正文，字数必须超过四十个汉字才行。\n一、概念\n二、方法\n"
	blocks := ToBlocks(text)
	joined := ""
	for _, b := range blocks {
		joined += b.Text + "\n"
	}
	if strings.Contains(joined, "........") {
		t.Fatalf("TOC leaders should be removed:\n%s", joined)
	}
}

func TestSeparateGluedKeepsCentury(t *testing.T) {
	got := NormalizeMarkers([]string{"19世纪的三大科学发现"})
	if got[0] != "19世纪的三大科学发现" {
		t.Fatalf("century glued: %q", got[0])
	}
}

func TestBuildOutlineNestsExamQuestions(t *testing.T) {
	// 特殊序号题干嵌套为标题；选项不成标题
	lines := []string{
		"简介",
		"一、单项选择题",
		"1. 实践是认识的基础 ( )",
		"A. 正确",
		"B. 错误",
		"2. 真理是客观的 ( )",
		"二、简答题",
		"(1)什么是辩证的否定观?[武汉大学2018考研]",
		"(2)辩证否定观的特点",
		"(3)辩证否定观的实质",
	}
	blocks := BuildOutline(lines)
	out := renderHeads(blocks)
	if !strings.Contains(out, "### 一、单项选择题") {
		t.Fatalf("missing section:\n%s", out)
	}
	if !strings.Contains(out, "#### 1. 实践是认识的基础 ( )") {
		t.Fatalf("MCQ ordinal should nest:\n%s", out)
	}
	if !strings.Contains(out, "#### (1)什么是辩证的否定观?[武汉大学2018考研]") {
		t.Fatalf("brief Q should nest:\n%s", out)
	}
	for _, b := range blocks {
		if b.Type == "heading" && strings.HasPrefix(b.Text, "A.") {
			t.Fatalf("option became heading: %q", b.Text)
		}
	}
}
