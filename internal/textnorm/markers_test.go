package textnorm

import "testing"

func TestNormalizeMarkersOCRDot(t *testing.T) {
	in := []string{"1、概念", "2。定义", "3、说明"}
	got := NormalizeMarkers(in)
	if got[1] != "2、定义" {
		t.Fatalf("want 2、定义, got %q", got[1])
	}
}

func TestClassifyChapterAndOption(t *testing.T) {
	ch := ClassifyMarker("第三章 世界的物质性")
	if ch == nil || ch.Family != "chapter" || !isZhang(ch) {
		t.Fatalf("chapter: %+v", ch)
	}
	opt := ClassifyMarker("A. 《德意志意识形态》")
	if opt == nil || opt.Family != "option" || markerFamily(opt) != "" {
		t.Fatalf("option should not be outline family: %+v", opt)
	}
}

func TestClassifyDotFamily(t *testing.T) {
	m := ClassifyMarker("1.2.3 小节")
	if m == nil || m.Family != "dot-3" {
		t.Fatalf("dot family: %+v", m)
	}
}

func TestYearLineNotMarkerStart(t *testing.T) {
	if StartsWithMarker("2020年到2035年,基本实现社会主义现代化") {
		t.Fatal("year line must not be marker start")
	}
	if !StartsWithMarker("1. 下一题") || !StartsWithMarker("20、概念") {
		t.Fatal("real markers still match")
	}
}

func TestClassifyExamQuestionStillMarker(t *testing.T) {
	// Exam stems remain outline markers; nesting is decided by frequency/stack.
	m := ClassifyMarker("1. 实践是认识的基础 ( )")
	if m == nil || m.Family != "integer_dot" {
		t.Fatalf("mcq stem should classify: %+v", m)
	}
	m2 := ClassifyMarker("(1)什么是辩证的否定观?[武汉大学2018考研]")
	if m2 == nil || m2.Family != "num_paren" {
		t.Fatalf("brief stem should classify: %+v", m2)
	}
}
