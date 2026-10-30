package convert

import (
	"context"

	"doc2md/engine/ingest"
	"doc2md/internal/ocr"
	"doc2md/internal/pdf"
)

// Result is the conversion output.
type Result struct {
	Filename  string     `json:"filename"`
	Format    string     `json:"format"`
	Engine    string     `json:"engine"`
	Text      string     `json:"text"`
	Markdown  string     `json:"markdown"`
	Recognize *Recognize `json:"recognize,omitempty"`
	OCRStats  *OCRStats  `json:"ocr_stats,omitempty"`
}

// Recognize is shown in the UI: page count, OCR page count, and pages per second.
type Recognize struct {
	Pages       int     `json:"pages"`
	OCRPages    int     `json:"ocr_pages"`
	TotalMs     int64   `json:"total_ms"`
	PagesPerSec float64 `json:"pages_per_sec"`
}

// OCRStats reports recognition timing when structure/OCR ran.
type OCRStats struct {
	Used         bool    `json:"used"`
	Model        string  `json:"model,omitempty"`
	Pages        int     `json:"pages"`
	TotalMs      int64   `json:"total_ms"`
	AvgMsPerPage float64 `json:"avg_ms_per_page"`
}

// Options controls conversion behavior.
// OCRMode / OCR / Pipeline 等为 API 兼容保留；主路径固定 engine（oar structure JSON）。
type Options struct {
	OCRMode    pdf.OCRMode
	OCR        ocr.Engine
	OCRModel   string
	OCRUseCls  bool
	DPI        int
	Enhance    bool
	Pipeline   Pipeline
	FeatEngine string
}

// Convert converts file bytes to Markdown via engine/.
func Convert(ctx context.Context, filename string, data []byte, opt Options) (*Result, error) {
	if opt.DPI <= 0 {
		opt.DPI = 150
	}
	er, err := ingest.Convert(ctx, filename, data, ingest.Options{DPI: opt.DPI})
	if err != nil {
		return nil, err
	}
	out := &Result{
		Filename:  er.Filename,
		Format:    er.Format,
		Engine:    er.Engine,
		Text:      er.Text,
		Markdown:  er.Markdown,
		Recognize: recognizeOf(er.Pages, er.OCRPages, er.ElapsedMs),
	}
	if er.OCRStats != nil {
		out.OCRStats = &OCRStats{
			Used:         er.OCRStats.Used,
			Model:        er.OCRStats.Model,
			Pages:        er.OCRStats.Pages,
			TotalMs:      er.OCRStats.TotalMs,
			AvgMsPerPage: er.OCRStats.AvgMsPerPage,
		}
	}
	return out, nil
}

func recognizeOf(pages, ocrPages int, elapsedMs int64) *Recognize {
	basis := pages
	if ocrPages > 0 {
		basis = ocrPages
	}
	var rate float64
	if basis > 0 && elapsedMs > 0 {
		rate = float64(basis) / (float64(elapsedMs) / 1000)
	}
	return &Recognize{
		Pages:       pages,
		OCRPages:    ocrPages,
		TotalMs:     elapsedMs,
		PagesPerSec: rate,
	}
}
