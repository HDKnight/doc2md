package ocr

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestDecodeOCRBytesUTF8(t *testing.T) {
	in := []byte(`{"data":[{"text":"中国共产党"}]}`)
	got := decodeOCRBytes(in)
	if !strings.Contains(got, "中国共产党") {
		t.Fatalf("utf8: %q", got)
	}
}

func TestDecodeOCRBytesGBKFallback(t *testing.T) {
	utf8s := `{"data":[{"text":"思想政治教育"}]}`
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(utf8s))
	if err != nil {
		t.Fatal(err)
	}
	got := decodeOCRBytes(gbk)
	if !strings.Contains(got, "思想政治教育") {
		t.Fatalf("gbk fallback: %q", got)
	}
}
