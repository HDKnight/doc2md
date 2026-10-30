package feat

import (
	"strings"
	"testing"
)

func TestMergeBrokenParas(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "首先，进行政治理论教育，解决思", w: 400},
		{kind: "para", text: "想认识问题。其次，建立请示报告制度。", w: 200},
		{kind: "heading", lvl: 4, text: "(3)下一节", series: "paren", num: 3},
		{kind: "para", text: "完整一句。", w: 100},
		{kind: "para", text: "不应合并。", w: 100},
	}
	segs = mergeBrokenParas(segs)
	if len(segs) < 3 {
		t.Fatalf("segs=%d", len(segs))
	}
	if !strings.Contains(segs[0].text, "解决思想认识问题") {
		t.Fatalf("should merge mid-word break: %q", segs[0].text)
	}
	if segs[1].kind != "heading" {
		t.Fatalf("heading stays: %+v", segs[1])
	}
	// 以。结尾的不与下一段合并
	if strings.Contains(segs[2].text, "不应合并") {
		// 完整一句。 与 不应合并。 应仍是两段
		found := false
		for _, s := range segs {
			if s.text == "不应合并。" {
				found = true
			}
			if strings.Contains(s.text, "完整一句。不应合并") {
				t.Fatalf("must not merge after sentence end: %q", s.text)
			}
		}
		if !found {
			t.Fatalf("expected separate 不应合并。 segs=%v", segs)
		}
	}
}

func TestMergeBrokenParas_NotAcrossAnswer(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "上一题未写完", w: 400},
		{kind: "para", text: "答：这是新题答案。", w: 200},
	}
	segs = mergeBrokenParas(segs)
	if len(segs) != 2 {
		t.Fatalf("must not merge into 答：, got %d: %+v", len(segs), segs)
	}
}

func TestMergeBrokenParas_RequiresFullWidth(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "短句未完", w: 80},
		{kind: "para", text: "下一段独立内容。", w: 400},
		{kind: "para", text: "这是很长很长抵满整页宽度的上一句碎片末", w: 390},
		{kind: "para", text: "接上句续写。", w: 120},
	}
	segs = mergeBrokenParas(segs)
	if len(segs) != 3 {
		t.Fatalf("short incomplete must stay separate; full-width mid-break merges: got %d %+v", len(segs), segs)
	}
	if segs[0].text != "短句未完" {
		t.Fatalf("short line must not merge: %q", segs[0].text)
	}
	if segs[1].text != "下一段独立内容。" {
		t.Fatalf("expected independent para, got %q", segs[1].text)
	}
	if !strings.Contains(segs[2].text, "接上句续写") {
		t.Fatalf("full-width prev should merge: %q", segs[2].text)
	}
}

func TestMergeBrokenParas_LargeVerticalGap(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "满行宽度的上一段还没写完", w: 400, y0: 100, y1: 130},
		{kind: "para", text: "间距过大应另起一段。", w: 200, y0: 220, y1: 250},
	}
	segs = mergeBrokenParas(segs)
	if len(segs) != 2 {
		t.Fatalf("large gap must not merge, got %d %+v", len(segs), segs)
	}
}

func TestMergeBrokenParas_EmMismatch(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "满行宽度的正文碎片接", w: 400, em: 12, y0: 100, y1: 120},
		{kind: "para", text: "大字标题感续行", w: 200, em: 22, y0: 125, y1: 155},
	}
	segs = mergeBrokenParas(segs)
	if len(segs) != 2 {
		t.Fatalf("em mismatch must not merge, got %d %+v", len(segs), segs)
	}
}

func TestMergeBrokenParas_SoftCommaFullWidth(t *testing.T) {
	segs := []seg{
		{kind: "para", text: "这是抵满行宽并以逗号收尾的上一行，", w: 400},
		{kind: "para", text: "应与下一行合并。", w: 150},
	}
	segs = mergeBrokenParas(segs)
	if len(segs) != 1 || !strings.Contains(segs[0].text, "应与下一行合并") {
		t.Fatalf("soft comma + full width should merge: %+v", segs)
	}
}
