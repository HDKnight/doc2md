package convert

import "testing"

func TestRecognizeOfOCRRate(t *testing.T) {
	got := recognizeOf(12, 12, 8000)
	if got.Pages != 12 || got.OCRPages != 12 {
		t.Fatalf("pages: %+v", got)
	}
	if got.PagesPerSec < 1.49 || got.PagesPerSec > 1.51 {
		t.Fatalf("ocr rate: %v", got.PagesPerSec)
	}
}

func TestRecognizeOfTextUsesAllPages(t *testing.T) {
	got := recognizeOf(4, 0, 2000)
	if got.OCRPages != 0 {
		t.Fatalf("ocr pages: %+v", got)
	}
	if got.PagesPerSec < 1.99 || got.PagesPerSec > 2.01 {
		t.Fatalf("text rate: %v", got.PagesPerSec)
	}
}

func TestRecognizeOfNoPages(t *testing.T) {
	got := recognizeOf(0, 0, 30)
	if got.PagesPerSec != 0 {
		t.Fatalf("rate should be zero: %+v", got)
	}
}
