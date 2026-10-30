package feat

import (
	"strings"
	"testing"
)

func TestSplitGluedQuizSegs_OptionsBeyondD(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "3、开端事件是（D)。A、新文化运动B、五四运动C、马克思主义的传播D、中国共产党正式成立E、党的二大"},
		{kind: "para", text: "A、陈独秀B、李大钊"},
		{kind: "para", text: "C、胡适D、钱玄同4、下一题开始。"},
	}
	segs = splitGluedQuizSegs(segs)
	joined := ""
	for _, s := range segs {
		joined += s.text + "\n"
		if isOptionLine(s.text) && s.kind == "heading" {
			t.Fatalf("option must not be heading: %+v", s)
		}
	}
	for _, want := range []string{"A、新文化运动", "B、五四运动", "C、马克思主义的传播", "D、中国共产党正式成立", "E、党的二大", "4、下一题开始。"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "成立E、") || strings.Contains(joined, "钱玄同4、") {
		t.Fatalf("glue not split:\n%s", joined)
	}
}

func TestSplitGluedQuiz_KeepNoiseBetweenOptions(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "C、作风整顿1D、思想整顿"},
	}
	segs = splitGluedQuizSegs(segs)
	if len(segs) != 2 {
		t.Fatalf("got %d segs: %+v", len(segs), segs)
	}
	if segs[0].text != "C、作风整顿1" || segs[1].text != "D、思想整顿" {
		t.Fatalf("must keep digits between options: %+v", segs)
	}
}

func TestSplitGluedQuizSegs_StemA_ThenBAcrossSegs(t *testing.T) {
	// 认出 A、 后找到相邻 B、 → 确认是选项串 → 题干与 A 也切开
	segs := []seg{
		{kind: "para", text: "67、“文化大革命”结束后的（D ）是继延安整风后的又一次思想解放运动，对广大干部和群众进行了一次比较普遍的辩证唯物主义的思想教育。A、反对“两个凡是”"},
		{kind: "para", text: "B、改革开放的大讨论"},
		{kind: "para", text: "C、思想路线的大讨论"},
		{kind: "para", text: "D、真理标准问题的大讨论"},
	}
	segs = splitGluedQuizSegs(segs)
	joined := ""
	for _, s := range segs {
		joined += s.text + "\n"
	}
	if strings.Contains(joined, "教育。A、") {
		t.Fatalf("stem still glued to A:\n%s", joined)
	}
	if !strings.Contains(joined, "思想教育。") || !strings.Contains(joined, "A、反对") {
		t.Fatalf("expected stem + A split:\n%s", joined)
	}
}

func TestSplitByRepeatedOrdinals_MultiKinds(t *testing.T) {
	got := splitByRepeatedOrdinals("（1）甲（2）乙（3）丙")
	if len(got) != 3 || got[0] != "（1）甲" || got[2] != "（3）丙" {
		t.Fatalf("paren: %q", got)
	}
	got = splitByRepeatedOrdinals("1、甲2、乙")
	if len(got) != 2 || got[0] != "1、甲" || got[1] != "2、乙" {
		t.Fatalf("arab: %q", got)
	}
	// 单独一个 A、 且无相邻 B、 → 不切
	got = splitByRepeatedOrdinals("只有A、一项")
	if len(got) != 1 {
		t.Fatalf("lonely A without B must not split: %q", got)
	}
	got = splitByRepeatedOrdinals("a、小b、写")
	if len(got) != 2 {
		t.Fatalf("lowercase letter: %q", got)
	}
}

func TestMergeBrokenParas_SkipsOptions(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "A、陈独秀", w: 400},
		{kind: "para", text: "B、李大钊", w: 400},
	}
	segs = mergeBrokenParas(segs)
	if len(segs) != 2 {
		t.Fatalf("options must not merge: %+v", segs)
	}
}

func TestMarkdownFromPages_OptionNotGlueYearStem(t *testing.T) {
	pages := []Page{{Blocks: []Block{
		{Type: "text", Content: "D、延安整风27、1936年10月9日，红一、二、四方面军在甘肃会师。", BBox: []float64{40, 100, 500, 120}},
		{Type: "text", Content: "A、瓦窑堡", BBox: []float64{40, 140, 200, 160}},
	}}}
	md := MarkdownFromPages(pages)
	if strings.Contains(md, "整风27、") {
		t.Fatalf("D option glued to next Q:\n%s", md)
	}
	if !strings.Contains(md, "D、延安整风") || !strings.Contains(md, "27、1936年") {
		t.Fatalf("expected split option / year-stem:\n%s", md)
	}
}

func TestIsArabicStem_YearQuestion(t *testing.T) {
	if !isArabicStem("27、1936年10月9日，会师。") {
		t.Fatal("year-leading stem must count as arabic stem")
	}
	if !isArabicStem("5、12项基本原则") {
		t.Fatal("two-digit after 、 without second 、 is still a stem")
	}
	if isArabicStem("16、17、18世纪的变化") {
		t.Fatal("数字、数字、 must not count as stem")
	}
	if !isArabicStem("16、17世纪的变化") {
		t.Fatal("without second 、, do not treat as range")
	}
}
