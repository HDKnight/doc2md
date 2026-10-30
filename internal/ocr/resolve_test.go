package ocr

import (
	"strings"
	"testing"
)

func TestSplitCommandQuotes(t *testing.T) {
	got := splitCommand(`D:\Python314\python.exe "F:\code\tools\ocr_cli.py"`)
	if len(got) != 2 || !strings.HasSuffix(got[0], "python.exe") || !strings.Contains(got[1], "ocr_cli.py") {
		t.Fatalf("got %#v", got)
	}
}

func TestResolveFindsOcrCliWhenPythonHasFasterPaddle(t *testing.T) {
	cmd := resolveBundledOCR()
	if cmd == "" {
		t.Skip("no bundled OCR on this machine")
	}
	low := strings.ToLower(cmd)
	if !strings.Contains(cmd, "ocr_cli.py") && !strings.Contains(low, "rapidocr") {
		t.Fatalf("unexpected cmd %q", cmd)
	}
}
