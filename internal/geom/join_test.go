package geom

import (
	"fmt"
	"strings"
	"testing"

	"doc2md/internal/textnorm"
)

// line mirrors pdf-md tests.conftest.line defaults.
func line(text string, x0, y0, x1, y1 float64) Line {
	if x1 == 0 {
		x1 = 200
	}
	if y1 == 0 {
		y1 = y0 + 14
	}
	return Line{
		Text: textnorm.Normalize(text), Page: 1,
		X0: x0, Y0: y0, X1: x1, Y1: y1,
		PageWidth: 595, PageHeight: 842, Source: "text", FontSize: 12,
	}
}

func textsOf(lines []Line) []string { return Texts(lines) }

func TestJoinWrappedChinese(t *testing.T) {
	rows := []Line{
		line("矛盾的同一性使事物", 72, 80, 200, 94),
		line("保持相对稳定。", 72, 98, 200, 112),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 1 || got[0] != "矛盾的同一性使事物保持相对稳定。" {
		t.Fatalf("%v", got)
	}
}

func TestJoinDateWrap(t *testing.T) {
	rows := []Line{
		line("是1927年", 72, 80, 200, 96),
		line("1月，根据毛泽东的倡议在武昌创办的。", 73, 100, 200, 116),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 1 || !strings.Contains(got[0], "是1927年1月") || !strings.Contains(got[0], "武昌") {
		t.Fatalf("%v", got)
	}
}

func TestJoinFullDateWrap(t *testing.T) {
	rows := []Line{
		line("1926年4月", 72, 80, 200, 96),
		line("15日，在当时黄埔军校校长蒋介石强行勒令下。", 73, 100, 200, 116),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 1 || !strings.HasPrefix(got[0], "1926年4月15日") {
		t.Fatalf("%v", got)
	}
}

func TestJoinMonthAloneNewParagraph(t *testing.T) {
	rows := []Line{
		line("上一段没有句号", 72, 80, 200, 96),
		line("1月，会议决定继续办所。", 72, 100, 200, 116),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func TestJoinNumberedItemNewParagraph(t *testing.T) {
	rows := []Line{
		line("上一段没有句号", 72, 80, 200, 96),
		line("1、下一题", 72, 100, 200, 116),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 2 || got[1] != "1、下一题" {
		t.Fatalf("%v", got)
	}
}

func TestJoinShortHeadingDoesNotJoinWiderPara(t *testing.T) {
	rows := []Line{
		{Text: "(2)“三查三整”的方法步骤", Page: 1, X0: 359, Y0: 242, X1: 827, Y1: 284, PageWidth: 1600, PageHeight: 842, FontSize: 12},
		{Text: "这次整党是结合土地改革运动进行的，", Page: 1, X0: 286, Y0: 336, X1: 1508, Y1: 377, PageWidth: 1600, PageHeight: 842, FontSize: 12},
		{Text: "整顿的重点对象是农村基层党组织。", Page: 1, X0: 272, Y0: 390, X1: 1513, Y1: 431, PageWidth: 1600, PageHeight: 842, FontSize: 12},
	}
	got := textsOf(JoinLines(rows))
	if got[0] != "(2)“三查三整”的方法步骤" {
		t.Fatalf("heading joined: %v", got)
	}
	if !strings.HasPrefix(got[1], "这次整党是结合土地改革运动进行的，") {
		t.Fatalf("%v", got)
	}
}

func TestJoinReachingRightEdge(t *testing.T) {
	rows := []Line{
		line("它以共产党", 133, 80, 700, 96),
		line("员为骨干。", 134, 100, 698, 116),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 1 || got[0] != "它以共产党员为骨干。" {
		t.Fatalf("%v", got)
	}
}

func TestJoinIndentedFirstLineContinuation(t *testing.T) {
	rows := []Line{
		line("这是首行缩进但右边顶到边的一行", 110, 80, 700, 96),
		line("第二行回到左端并且同样顶到右边。", 72, 100, 700, 116),
	}
	if len(JoinLines(rows)) != 1 {
		t.Fatalf("%v", textsOf(JoinLines(rows)))
	}
}

func TestJoinLongLatinToken(t *testing.T) {
	rows := []Line{
		line("参见配置项", 72, 80, 560, 96),
		line("InternationalizationConfiguration的说明文字补足行宽。", 72, 100, 700, 116),
	}
	if len(JoinLines(rows)) != 1 {
		t.Fatalf("%v", textsOf(JoinLines(rows)))
	}
}

func TestJoinShortLatinDoesNotPull(t *testing.T) {
	rows := []Line{
		line("参见", 72, 80, 200, 96),
		line("OK后面是中文说明并且这一行很长。", 72, 100, 700, 116),
	}
	if len(JoinLines(rows)) != 2 {
		t.Fatalf("%v", textsOf(JoinLines(rows)))
	}
}

func TestJoinPunctuationNewParagraph(t *testing.T) {
	rows := []Line{
		line("这是一句完整的话。", 72, 80, 200, 94),
		line("下一句另起。", 72, 98, 200, 112),
	}
	if len(JoinLines(rows)) != 2 {
		t.Fatal(textsOf(JoinLines(rows)))
	}
}

func TestJoinIndentNewParagraph(t *testing.T) {
	rows := []Line{
		line("上一行没有收句号", 72, 80, 200, 94),
		line("下一行缩进了", 90, 98, 200, 112),
	}
	if len(JoinLines(rows)) != 2 {
		t.Fatal(textsOf(JoinLines(rows)))
	}
}

func TestJoinMarkerOnlyPeriod(t *testing.T) {
	rows := []Line{
		line("2。", 72, 80, 90, 94),
		line("什么是马克思主义？", 72, 98, 200, 112),
	}
	got := JoinLines(rows)
	if !strings.HasPrefix(got[0].Text, "2。什么是马克思主义") {
		t.Fatalf("%q", got[0].Text)
	}
}

func TestJoinSeamNoVerticalGap(t *testing.T) {
	left := line("左栏最后没有句号", 72, 700, 250, 714)
	right := line("右栏从上面接上。", 340, 80, 520, 94)
	got := JoinSeam(left, right)
	if got == nil || got.Text != "左栏最后没有句号右栏从上面接上。" {
		t.Fatalf("%v", got)
	}
}

func TestJoinTightOptionsDoNotSplitWrap(t *testing.T) {
	var rows []Line
	y := 100.0
	for i := 0; i < 8; i++ {
		rows = append(rows, line(fmt.Sprintf("A、选项%d。", i), 72, y, 200, y+16))
		y += 28
	}
	rows = append(rows, line("4、它以共产党", 72, y, 200, y+16))
	y = rows[len(rows)-1].Y1 + 19
	rows = append(rows, line("员为骨干的革", 73, y, 200, y+16))
	y = rows[len(rows)-1].Y1 + 19
	rows = append(rows, line("命团体。", 73, y, 200, y+16))
	got := JoinLines(rows)
	last := got[len(got)-1].Text
	if last != "4、它以共产党员为骨干的革命团体。" {
		t.Fatalf("%q", last)
	}
}

func TestJoinWideGapDoesNotJoin(t *testing.T) {
	rows := []Line{
		line("上一行", 72, 80, 200, 94),
		line("紧接着", 72, 98, 200, 112),
		line("离得很远", 72, 200, 200, 214),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 2 || got[0] != "上一行紧接着" || got[1] != "离得很远" {
		t.Fatalf("%v", got)
	}
}

func TestJoinSameRowFragmentsWithSpace(t *testing.T) {
	rows := []Line{
		line("导", 95, 286, 107, 303),
		line("论", 119, 286, 131, 303),
		line("第一章", 95, 309, 131, 326),
		line("世界的物质性及发展规律", 143, 309, 275, 326),
	}
	got := textsOf(JoinLines(rows))
	want := []string{"导 论", "第一章 世界的物质性及发展规律"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestJoinSeamFinishedMarker(t *testing.T) {
	left := Line{Text: "1.3.8", Page: 1, X0: 532, Y0: 448, X1: 556, Y1: 460, PageWidth: 595, PageHeight: 842, FontSize: 12}
	right := Line{Text: "简介", Page: 2, X0: 35, Y0: 55, X1: 200, Y1: 88, PageWidth: 595, PageHeight: 842, FontSize: 12}
	if JoinSeam(left, right) != nil {
		t.Fatal("finished marker should not seam-join")
	}
}

func TestJoinAnswerLabelWithBody(t *testing.T) {
	rows := []Line{
		line("5. 生产力", 35, 80, 180, 94),
		line("【答案】", 35, 98, 83, 112),
		line("生产力是指人们改造自然的能力。", 35, 116, 400, 130),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 2 || got[1] != "【答案】生产力是指人们改造自然的能力。" {
		t.Fatalf("%v", got)
	}
}

func TestJoinAnswerLabelIndentedBody(t *testing.T) {
	rows := []Line{
		line("1. 简述意义", 35, 80, 200, 94),
		line("【答案】", 35, 120, 83, 134),
		line("习近平新时代中国特色社会主义思想。", 59, 138, 400, 152),
	}
	got := textsOf(JoinLines(rows))
	if len(got) != 2 || got[1] != "【答案】习近平新时代中国特色社会主义思想。" {
		t.Fatalf("%v", got)
	}
}

func TestJoinSeamNotAnswerLabel(t *testing.T) {
	left := line("5. 生产力[中央财经大学2018年研]", 35, 750, 211, 764)
	right := Line{Text: "【答案】", Page: 2, X0: 35, Y0: 30, X1: 83, Y1: 44, PageWidth: 595, PageHeight: 842, FontSize: 12}
	if JoinSeam(left, right) != nil {
		t.Fatal("should not seam-join into 【答案】")
	}
}

func TestJoinSeamBareOCRNumber(t *testing.T) {
	left := line("2。", 72, 800, 90, 814)
	right := Line{Text: "什么是马克思主义？", Page: 2, X0: 72, Y0: 80, X1: 200, Y1: 94, PageWidth: 595, PageHeight: 842, FontSize: 12}
	got := JoinSeam(left, right)
	if got == nil || !strings.HasPrefix(got.Text, "2。什么是马克思主义") {
		t.Fatalf("%v", got)
	}
}
