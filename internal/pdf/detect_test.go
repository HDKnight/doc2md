package pdf

import "testing"

func TestNeedsOCRWhenEmpty(t *testing.T) {
	pages := []PageText{{Page: 1, Text: ""}}
	if !NeedsOCR(pages, OCRAuto, 50) {
		t.Fatal("empty pages should need OCR in auto mode")
	}
}
