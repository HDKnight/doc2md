package textnorm

import (
	"strings"
	"testing"
)

func TestBoldAnswerLabelOnly(t *testing.T) {
	got := boldAnswerLabel("【答案】 A")
	if got != "**【答案】** A" {
		t.Fatalf("got %q", got)
	}
	got = boldAnswerLabel("【答案】A")
	if got != "**【答案】** A" {
		t.Fatalf("need space after bold close: %q", got)
	}
	got = boldAnswerLabel("【解析】说明文字。")
	if got != "**【解析】** 说明文字。" {
		t.Fatalf("need space after bold close: %q", got)
	}
}

func TestAnsGlueDoesNotBreakBoldLabel(t *testing.T) {
	in := "**【答案】** A\n\n正文"
	out := FormatMarkdownOrdinals(in)
	if strings.Contains(out, "**\n") || strings.HasPrefix(strings.TrimSpace(out), "*\n") {
		t.Fatalf("bold label split:\n%q", out)
	}
	if !strings.Contains(out, "**【答案】**") {
		t.Fatalf("missing bold label:\n%q", out)
	}
}

func TestJoinBrokenAcrossSanBiao(t *testing.T) {
	// 「三个代」+「表”重要」不得因 StartsWithMarker(表) 拒接
	if StartsWithMarker("表”重要思想、科学发展观") {
		t.Fatal("正文续行 表” 不应视为图表题注")
	}
	if !ShouldJoinBrokenLines(
		"【答案】习近平新时代中国特色社会主义思想是对马克思列宁主义、毛泽东思想、邓小平理论、“三个代",
		"表”重要思想、科学发展观的继承和发展,是马克思主义中国化最新成果。",
	) {
		t.Fatal("should join 三个代|表")
	}
}

func TestOutlineAnswerNotWholeLineBold(t *testing.T) {
	blocks := BuildOutline([]string{
		"【答案】习近平新时代中国特色社会主义思想是最新成果。",
	})
	if len(blocks) == 0 {
		t.Fatal("no blocks")
	}
	text := blocks[0].Text
	if strings.HasPrefix(text, "**【答案】习近平") || strings.HasSuffix(text, "成果。**") {
		t.Fatalf("whole line should not be bold-wrapped: %q", text)
	}
	if !strings.HasPrefix(text, "**【答案】**") {
		t.Fatalf("want label-only bold: %q", text)
	}
}
