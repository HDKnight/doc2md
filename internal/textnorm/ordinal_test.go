package textnorm

import (
	"strings"
	"testing"
)

func TestFormatMarkdownOrdinals_KeepsHeadingHashes(t *testing.T) {
	in := "## 标题\n\n### 1、两个时期各五个阶段:开端\n\n### 2、中共思想政治教育史的研究对象:党的思想政治教育产生、发展及其规律。\n\n### 3、中共对农民思政教育的主要目的:宣传\n"
	out := FormatMarkdownOrdinals(in)
	for _, ln := range strings.Split(out, "\n") {
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "#") && strings.Trim(trim, "# ") == "" {
			t.Fatalf("empty heading hashes:\n%s", out)
		}
	}
	if !strings.Contains(out, "### 1、") && !strings.Contains(out, "### 1、 ") {
		t.Fatalf("missing headed 1、:\n%s", out)
	}
	if !strings.Contains(out, "党的思想政治教育产生") {
		t.Fatalf("思想政治 should stay joined:\n%s", out)
	}
}

func TestToBlocks_DoesNotSplitSixiangZhengzhi(t *testing.T) {
	text := "中国共产党思想政治教育史-知识点总结\n" +
		"1、两个时期各五个阶段:开端\n" +
		"2、中共思想政治教育史的研究对象:党的思想政治教育产生、发展及其规律。\n" +
		"3、中共对农民思政教育的主要目的:宣传\n"
	blocks := ToBlocks(text)
	for _, b := range blocks {
		if strings.HasPrefix(strings.TrimSpace(b.Text), "政治教育") {
			t.Fatalf("split 思想|政治: %+v", blocks)
		}
		if b.Type == "heading" && strings.HasSuffix(strings.TrimSpace(b.Text), "党的思想") {
			t.Fatalf("heading cut before 政治: %q", b.Text)
		}
	}
}


func TestFormatOrdinalSpaceAndParen(t *testing.T) {
	in := "(1)什么是辩证的否定观？\n（2）简述辩证否定观的基本内涵。"
	out := FormatMarkdownOrdinals(in)
	if !strings.Contains(out, "（1） 什么是") {
		t.Fatalf("halfwidth paren+space:\n%s", out)
	}
	if !strings.Contains(out, "（2） 简述") {
		t.Fatalf("fullwidth space:\n%s", out)
	}
}

func TestFormatSubDigitGlue(t *testing.T) {
	in := "（1） 辩证否定观的特点2辩证的否定是事物发展的环节。"
	out := FormatMarkdownOrdinals(in)
	if strings.Contains(out, "特点2辩证") {
		t.Fatalf("sub digit still glued:\n%s", out)
	}
	if !strings.Contains(out, "特点") || !strings.Contains(out, "2 辩证") {
		t.Fatalf("expected split:\n%s", out)
	}
}

func TestFormatKeepsCentury(t *testing.T) {
	in := "在21世纪的发展做出了重大贡献。"
	out := FormatMarkdownOrdinals(in)
	if strings.Contains(out, "在\n\n2") || strings.Contains(out, "在\n\n21") {
		t.Fatalf("broke century:\n%s", out)
	}
}

func TestSplitAllChoiceOptions(t *testing.T) {
	in := "A、新文化运动\n\nB、五四运动\n\nC、马克思主义的传播D、中国共产党正式成立"
	out := FormatMarkdownOrdinals(in)
	if strings.Contains(out, "传播D") {
		t.Fatalf("C/D still glued:\n%s", out)
	}
	if !strings.Contains(out, "传播\n\nD、") {
		t.Fatalf("D not its own paragraph:\n%s", out)
	}
	blocks := ToBlocks("A、新文化运动\nB、五四运动\nC、马克思主义的传播 D、中国共产党正式成立")
	var texts []string
	for _, b := range blocks {
		texts = append(texts, b.Text)
	}
	joined := strings.Join(texts, "\n")
	for _, want := range []string{"A、", "B、", "C、", "D、"} {
		if strings.Count(joined, want) != 1 {
			t.Fatalf("option %s count in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "传播 D") || strings.Contains(joined, "传播D") {
		t.Fatalf("ToBlocks left C/D together:\n%s", joined)
	}
}

func TestFormatOptionGlue(t *testing.T) {
	in := "著作是（ ）。[考研]A. 《德意志意识形态》B. 《神圣家族》"
	out := FormatMarkdownOrdinals(in)
	if !strings.Contains(out, "\nA.") && !strings.Contains(out, "\nA、") && !strings.Contains(out, "\nA. ") {
		// after space fix may be A.
		if !strings.Contains(out, "A.") {
			t.Fatalf("option not split:\n%s", out)
		}
	}
}

func TestPrepOrdinalOCRPeriod(t *testing.T) {
	got := PrepOrdinalLine("1。什么是实践？")
	if !strings.HasPrefix(got, "1.") {
		t.Fatalf("got %q", got)
	}
}

func TestFormatAnswerGlue(t *testing.T) {
	in := "（2） 简述辩证否定观的基本内涵。[南京航空航天大学2018研]【答案】辩证的否定"
	out := FormatMarkdownOrdinals(in)
	if strings.Contains(out, "研]【答案】") {
		t.Fatalf("answer still glued:\n%s", out)
	}
	if !strings.Contains(out, "【答案】") {
		t.Fatalf("missing answer mark:\n%s", out)
	}
}
