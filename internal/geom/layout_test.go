package geom

import "testing"

func TestReadingOrderTwoColumns(t *testing.T) {
	// left column x~50, right column x~350
	lines := []Line{
		{Text: "左1", Page: 1, X0: 40, Y0: 40, X1: 120, Y1: 55, PageWidth: 600, PageHeight: 800},
		{Text: "右1", Page: 1, X0: 340, Y0: 40, X1: 420, Y1: 55, PageWidth: 600, PageHeight: 800},
		{Text: "左2", Page: 1, X0: 40, Y0: 70, X1: 120, Y1: 85, PageWidth: 600, PageHeight: 800},
		{Text: "右2", Page: 1, X0: 340, Y0: 70, X1: 420, Y1: 85, PageWidth: 600, PageHeight: 800},
		{Text: "左3", Page: 1, X0: 40, Y0: 100, X1: 120, Y1: 115, PageWidth: 600, PageHeight: 800},
		{Text: "右3", Page: 1, X0: 340, Y0: 100, X1: 420, Y1: 115, PageWidth: 600, PageHeight: 800},
		{Text: "左4", Page: 1, X0: 40, Y0: 130, X1: 120, Y1: 145, PageWidth: 600, PageHeight: 800},
		{Text: "右4", Page: 1, X0: 340, Y0: 130, X1: 420, Y1: 145, PageWidth: 600, PageHeight: 800},
		{Text: "左5", Page: 1, X0: 40, Y0: 160, X1: 120, Y1: 175, PageWidth: 600, PageHeight: 800},
		{Text: "右5", Page: 1, X0: 340, Y0: 160, X1: 420, Y1: 175, PageWidth: 600, PageHeight: 800},
	}
	ordered := ReadingOrder(lines)
	got := Texts(ordered)
	wantPrefix := []string{"左1", "左2", "左3", "左4", "左5", "右1"}
	for i, w := range wantPrefix {
		if got[i] != w {
			t.Fatalf("order[%d]=%q want %q full=%v", i, got[i], w, got)
		}
	}
}

func TestIsDiagram(t *testing.T) {
	// Four x-bands spaced > 20% of page width; each band has 2 short labels.
	xs := []float64{20, 35, 200, 215, 380, 395, 560, 575}
	var lines []Line
	for i, x := range xs {
		y := float64(40 + (i%2)*100)
		lines = append(lines, Line{
			Text: "标签", Page: 1,
			X0: x, Y0: y, X1: x + 24, Y1: y + 14,
			PageWidth: 600, PageHeight: 800,
		})
	}
	if !IsDiagram(lines) {
		sizes := []int{}
		for _, g := range Clusters(lines) {
			sizes = append(sizes, len(g))
		}
		t.Fatalf("expected diagram clusters=%d sizes=%v", len(Clusters(lines)), sizes)
	}
	body := []Line{
		{Text: "这是一段比较长的正文内容用于排除导图", Page: 1, X0: 40, Y0: 40, X1: 400, Y1: 60, PageWidth: 600, PageHeight: 800},
		{Text: "继续很长的第二行正文内容仍然很长", Page: 1, X0: 40, Y0: 70, X1: 400, Y1: 90, PageWidth: 600, PageHeight: 800},
	}
	if IsDiagram(body) {
		t.Fatal("body should not be diagram")
	}
}
