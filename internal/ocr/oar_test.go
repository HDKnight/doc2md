package ocr

import (
	"testing"
)

func TestParseOarOCRJSON_RegionsWithPoints(t *testing.T) {
	raw := `{
  "input_path": "page.jpg",
  "index": 0,
  "text_regions": [
    {
      "bounding_box": {"points": [{"x": 10, "y": 20}, {"x": 100, "y": 20}, {"x": 100, "y": 40}, {"x": 10, "y": 40}]},
      "text": "3.标题",
      "confidence": 0.97
    },
    {
      "bounding_box": {"points": [[1, 50], [200, 50], [200, 70], [1, 70]]},
      "text": "正文内容",
      "confidence": 0.91
    },
    {
      "bounding_box": {"points": []},
      "text": null,
      "confidence": null
    }
  ],
  "orientation_angle": null
}`
	lines, err := parseOarOCRJSON(raw, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines=%d want 2", len(lines))
	}
	if lines[0].Text != "3.标题" || !lines[0].HasBBox {
		t.Fatalf("line0=%+v", lines[0])
	}
	if lines[0].X0 != 10 || lines[0].Y0 != 20 || lines[0].X1 != 100 || lines[0].Y1 != 40 {
		t.Fatalf("bbox0=%v,%v,%v,%v", lines[0].X0, lines[0].Y0, lines[0].X1, lines[0].Y1)
	}
	if lines[1].Text != "正文内容" || lines[1].Y0 != 50 {
		t.Fatalf("line1=%+v", lines[1])
	}
}

func TestParseOarOCRJSON_Array(t *testing.T) {
	raw := `[{"text_regions":[{"bounding_box":{"points":[{"x":0,"y":0},{"x":1,"y":1}]},"text":"A"}]},{"text_regions":[{"bounding_box":{"points":[{"x":0,"y":0},{"x":2,"y":2}]},"text":"B"}]}]`
	pages, err := parseOarOCRJSONMulti(raw, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 || pages[0][0].Text != "A" || pages[1][0].Text != "B" {
		t.Fatalf("%+v", pages)
	}
	lines, err := parseOarOCRJSON(raw, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Text != "A" || lines[0].Page != 2 {
		t.Fatalf("%+v", lines)
	}
}

func TestNormalizeOarSize(t *testing.T) {
	t.Setenv("DOC2MD_OCR_OAR_SIZE", "")
	if got := normalizeOarSize("tiny"); got != "tiny" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeOarSize("small"); got != "small" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("DOC2MD_OCR_OAR_SIZE", "medium")
	if got := normalizeOarSize("tiny"); got != "medium" {
		t.Fatalf("env override got %q", got)
	}
}
