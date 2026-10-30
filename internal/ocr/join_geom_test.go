package ocr

import (
	"strings"
	"testing"
)

func TestGeometryPlainSplitsTitleByGapAndWidth(t *testing.T) {
	// Mirrors 简答题 page4: short title then full-width quote body.
	lines := []Line{
		{Text: "2.加强党员干部的党性修养", Page: 1, X0: 340, Y0: 100, X1: 759, Y1: 134, HasBBox: true},
		{Text: "整风运动从解决党内路线问题的认识开始，教育训练了党内经过长期斗争保", Page: 1, X0: 344, Y0: 225, X1: 1509, Y1: 255, HasBBox: true},
		{Text: "留下来的一批老干部。", Page: 1, X0: 276, Y0: 318, X1: 800, Y1: 348, HasBBox: true},
		{Text: "3.贯彻治病救人的方针政策", Page: 1, X0: 342, Y0: 500, X1: 757, Y1: 533, HasBBox: true},
		{Text: "“党内不同思想的对立和斗争是经常发生的，这是社会的阶级矛盾和新旧事", Page: 1, X0: 366, Y0: 625, X1: 1509, Y1: 655, HasBBox: true},
		{Text: "物矛盾在党内的反映。", Page: 1, X0: 274, Y0: 719, X1: 900, Y1: 749, HasBBox: true},
	}
	out := GeometryPlain(lines)
	if strings.Contains(out, "方针政策“党内") || strings.Contains(out, "方针政策党内") {
		t.Fatalf("title glued to body:\n%s", out)
	}
	if !strings.Contains(out, "3.贯彻治病救人的方针政策\n“党内不同思想") {
		t.Fatalf("expected title/body break:\n%s", out)
	}
	if !strings.Contains(out, "斗争保留下来的一批老干部。") {
		t.Fatalf("body wrap should join:\n%s", out)
	}
}

func TestGeometryPlainFallsBackWithoutBoxes(t *testing.T) {
	lines := []Line{
		{Text: "第一行", Page: 1},
		{Text: "第二行", Page: 1},
	}
	out := GeometryPlain(lines)
	if out != "第一行\n第二行" {
		t.Fatalf("%q", out)
	}
}

func TestGeometryPlainJoinsCJKToYear(t *testing.T) {
	lines := []Line{
		{Text: "第一个阶段,从", Page: 1, X0: 100, Y0: 100, X1: 1400, Y1: 130, HasBBox: true},
		// 较大行距仍应因行末「从」续接年份
		{Text: "2020年到2035年,基本实现社会主义现代化", Page: 1, X0: 100, Y0: 220, X1: 1400, Y1: 250, HasBBox: true},
	}
	out := GeometryPlain(lines)
	if !strings.Contains(out, "从2020年到2035年") {
		t.Fatalf("从|2020 not joined:\n%s", out)
	}
}

func TestGeometryPlainJoinsUnclosedBookTitle(t *testing.T) {
	lines := []Line{
		{Text: "65、 1967年5月 18 日《", Page: 1, X0: 100, Y0: 100, X1: 600, Y1: 130, HasBBox: true},
		{Text: "人民日报》、《红旗》杂志发表文章《伟大的历史文件》,第一次明确提出", Page: 1, X0: 100, Y0: 145, X1: 1400, Y1: 175, HasBBox: true},
	}
	out := GeometryPlain(lines)
	if !strings.Contains(out, "日《人民日报》") {
		t.Fatalf("book title not joined:\n%s", out)
	}
}

func TestParseOCRJSONWithBBox(t *testing.T) {
	raw := `{"data":[{"text":"3.标题","x0":1,"y0":2,"x1":10,"y1":12},{"text":"正文","x0":1,"y0":20,"x1":100,"y1":30}]}`
	lines, ok := parseOCRJSON(raw, 3)
	if !ok || len(lines) != 2 {
		t.Fatalf("parse: ok=%v n=%d", ok, len(lines))
	}
	if !lines[0].HasBBox || lines[0].Page != 3 || lines[0].Y0 != 2 {
		t.Fatalf("%+v", lines[0])
	}
}
