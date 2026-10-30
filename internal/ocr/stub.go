package ocr

import (
	"context"
	"fmt"
)

// Stub is a no-op engine used when OCR is not configured.
type Stub struct{}

func (Stub) OCRImage(ctx context.Context, page int, image []byte) ([]Line, error) {
	_ = ctx
	_ = page
	_ = image
	return nil, fmt.Errorf("ocr not configured: pip install faster-paddle (or set DOC2MD_OCR_CMD / DOC2MD_OCR_PYTHON)")
}

func (Stub) Close() error { return nil }

// Configured reports whether a usable OCR engine is available.
func Configured(e Engine) bool {
	if e == nil {
		return false
	}
	if _, ok := e.(Stub); ok {
		return false
	}
	if _, ok := e.(*Stub); ok {
		return false
	}
	return true
}
