package ocr

import (
	"context"
	"strings"
)

// Line is one recognized text line.
// When HasBBox is true, coordinates are in the OCR page image pixel space.
type Line struct {
	Text           string
	Page           int
	X0, Y0, X1, Y1 float64
	HasBBox        bool
}

// Engine recognizes text from page images (PNG/JPEG bytes).
type Engine interface {
	// OCRImage runs OCR on a single page image.
	OCRImage(ctx context.Context, page int, image []byte) ([]Line, error)
	// Close releases resources (optional for subprocess engines).
	Close() error
}

// FileEngine can OCR an on-disk image path (avoids re-encoding for worker CLI).
type FileEngine interface {
	OCRFile(ctx context.Context, page int, path string) ([]Line, error)
}

// BatchFileEngine OCRs many on-disk images in one call (faster-paddle ocr_batch).
type BatchFileEngine interface {
	OCRFiles(ctx context.Context, paths []string) ([][]Line, error)
}

// ModelConfigurer optionally accepts per-request model settings.
type ModelConfigurer interface {
	ConfigureOCR(model string, useCls bool)
}

// NormalizeModel returns small|tiny (default tiny，偏速度).
func NormalizeModel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "small", "accurate", "quality":
		return "small"
	default:
		return "tiny"
	}
}
