package textnorm

import (
	"strings"
	"testing"
)

func TestJoinBrokenHalfWord(t *testing.T) {
	md := "调动了积\n极性，推动了发展。"
	out := JoinBrokenMarkdown(md)
	if !strings.Contains(out, "调动了积极性") {
		t.Fatalf("%q", out)
	}
}

func TestJoinBrokenDoesNotEatNumbered(t *testing.T) {
	md := "上一段没有句号\n1. 下一题干"
	out := JoinBrokenMarkdown(md)
	if !strings.Contains(out, "\n1.") && !strings.Contains(out, "\n1. ") {
		t.Fatalf("should keep break before numbered:\n%s", out)
	}
}

func TestJoinBrokenSkipsShortTitle(t *testing.T) {
	md := "简介\n本书是辅导资料。"
	out := JoinBrokenMarkdown(md)
	if strings.Contains(out, "简介本书") {
		t.Fatalf("简介 joined:\n%s", out)
	}
}

func TestJoinBrokenSkipsParenItem(t *testing.T) {
	md := "（1） 辩证否定观的特点\n②辩证的否定是事物发展的环节。"
	out := JoinBrokenMarkdown(md)
	if strings.Contains(strings.ReplaceAll(out, " ", ""), "特点②") || strings.Contains(out, "特点②") {
		t.Fatalf("paren title joined:\n%s", out)
	}
}

func TestJoinBrokenClosingParenAfterBlankFill(t *testing.T) {
	md := "34、1937年8月,毛泽东写的(B\n),阐释了唯物辩证法的最根本的法则,"
	out := JoinBrokenMarkdown(md)
	if !strings.Contains(out, "(B),阐释") {
		t.Fatalf("expected (B) rejoined:\n%s", out)
	}
	if strings.Contains(out, "(B\n)") {
		t.Fatalf("paren still split:\n%s", out)
	}
}

func TestJoinBrokenDiyiNotChapter(t *testing.T) {
	md := "《伟大的历史文件》，\n第一次明确提出“(    )”这一概念。"
	out := JoinBrokenMarkdown(md)
	if !strings.Contains(out, "文件》，第一次明确提出") {
		t.Fatalf("第一次 blocked as chapter:\n%s", out)
	}
	// 真章节仍拒接
	kept := JoinBrokenMarkdown("上文未完，\n第一章 绪论")
	if strings.Contains(kept, "未完，第一章") {
		t.Fatalf("real chapter joined:\n%s", kept)
	}
	// 连续多条「第X…」成族 → 拒接
	family := JoinBrokenMarkdown("第一阶段完成任务，\n第二阶段开始实施。\n第三阶段总结验收。")
	if strings.Contains(family, "任务，第二阶段") || strings.Contains(family, "实施。第三阶段") {
		t.Fatalf("di-prefix family should not join:\n%s", family)
	}
}
