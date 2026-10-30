package ingest

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"doc2md/engine/oarx"
	"doc2md/engine/pipeline"
	"doc2md/internal/docx"
	"doc2md/internal/pdf"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// Result is conversion output from engine path.
type Result struct {
	Filename  string
	Format    string
	Engine    string
	Text      string
	Markdown  string
	Pages     int
	OCRPages  int
	ElapsedMs int64
	OCRStats  *oarx.Stats
}

// Options for ingest.
type Options struct {
	DPI int
}

// Convert routes by extension into engine pipeline.
func Convert(ctx context.Context, filename string, data []byte, opt Options) (*Result, error) {
	if opt.DPI <= 0 {
		opt.DPI = 150
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return convertPDF(ctx, filename, data, opt.DPI)
	case ".docx":
		return convertDOCX(filename, data)
	case ".txt", ".md", ".text":
		return convertTXT(filename, data)
	case ".doc":
		return nil, fmt.Errorf("unsupported format .doc; convert to .docx first")
	default:
		return nil, fmt.Errorf("unsupported format %q; use .pdf, .docx or .txt", ext)
	}
}

func convertPDF(ctx context.Context, filename string, data []byte, dpi int) (*Result, error) {
	t0 := time.Now()
	// 先抽文字层；够用则用行框字号定标题，不走 OCR
	pageLines, err := pdf.ExtractLinesFromBytes(data)
	if err != nil {
		return nil, err
	}
	flat := pageLinesToPageText(pageLines)
	if !pdf.NeedsOCR(flat, pdf.OCRAuto, 50) {
		pages, plain := pagesFromPDFLines(pageLines)
		return &Result{
			Filename:  filename,
			Format:    "pdf",
			Engine:    "engine-pdf-text",
			Text:      plain,
			Markdown:  pipeline.MarkdownFromPages(pages),
			Pages:     len(pageLines),
			OCRPages:  0,
			ElapsedMs: time.Since(t0).Milliseconds(),
		}, nil
	}
	// 扫描件 / 少字 / 乱码 → oar structure JSON
	pages, plain, stats, err := oarx.RunStructureJSON(ctx, data, dpi)
	if err != nil {
		return nil, err
	}
	ocrPages := 0
	var elapsed int64
	if stats != nil {
		ocrPages = stats.Pages
		elapsed = stats.TotalMs
	}
	if elapsed <= 0 {
		elapsed = time.Since(t0).Milliseconds()
	}
	totalPages := len(pageLines)
	if totalPages < ocrPages {
		totalPages = ocrPages
	}
	if totalPages < len(pages) {
		totalPages = len(pages)
	}
	return &Result{
		Filename:  filename,
		Format:    "pdf",
		Engine:    "engine-oar-json",
		Text:      plain,
		Markdown:  pipeline.MarkdownFromPages(pages),
		Pages:     totalPages,
		OCRPages:  ocrPages,
		ElapsedMs: elapsed,
		OCRStats:  stats,
	}, nil
}

func pageLinesToPageText(pages []pdf.PageLines) []pdf.PageText {
	out := make([]pdf.PageText, len(pages))
	for i, p := range pages {
		var b strings.Builder
		for _, ln := range p.Lines {
			b.WriteString(ln.Text)
			b.WriteByte('\n')
		}
		out[i] = pdf.PageText{Page: p.Page, Text: b.String()}
	}
	return out
}

// pagesFromPDFLines：pdftotext 行框 → pipeline 页；字号用行高/FontSize 进 bbox，供 em 比定标题。
func pagesFromPDFLines(pageLines []pdf.PageLines) ([]pipeline.Page, string) {
	pages := make([]pipeline.Page, 0, len(pageLines))
	var plain strings.Builder
	for _, p := range pageLines {
		blocks := make([]pipeline.Block, 0, len(p.Lines))
		for i, ln := range p.Lines {
			t := strings.TrimSpace(ln.Text)
			if t == "" {
				continue
			}
			y0, y1 := ln.Y0, ln.Y1
			if y1 <= y0 {
				// 无有效高度时用 FontSize 估一行盒，便于 em 比较
				em := ln.Em()
				if em < 1 {
					em = 12
				}
				y1 = y0 + em
			}
			blocks = append(blocks, pipeline.Block{
				Type:    "text",
				Content: t,
				Order:   i,
				BBox:    []float64{ln.X0, y0, ln.X1, y1},
			})
			if plain.Len() > 0 {
				plain.WriteByte('\n')
			}
			plain.WriteString(t)
		}
		pages = append(pages, pipeline.Page{Blocks: blocks})
	}
	return pages, plain.String()
}

func convertDOCX(filename string, data []byte) (*Result, error) {
	blocks, err := docx.Parse(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(blocks))
	var plain strings.Builder
	for _, b := range blocks {
		t := strings.TrimSpace(b.Text)
		if t == "" {
			continue
		}
		lines = append(lines, t)
		plain.WriteString(t)
		plain.WriteByte('\n')
	}
	text := plain.String()
	if strings.TrimSpace(text) == "" {
		return &Result{
			Filename: filename,
			Format:   "docx",
			Engine:   "engine-docx-empty",
			Text:     "",
			Markdown: "<!-- docx has no extractable text (likely image-only pages) -->\n",
		}, nil
	}
	return &Result{
		Filename: filename,
		Format:   "docx",
		Engine:   "engine-docx",
		Text:     text,
		Markdown: pipeline.MarkdownFromLines(lines),
	}, nil
}

func convertTXT(filename string, data []byte) (*Result, error) {
	text := decodeText(data)
	raw := strings.ReplaceAll(text, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	var lines []string
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			lines = append(lines, ln)
		}
	}
	if len(lines) == 0 {
		return &Result{
			Filename: filename,
			Format:   "txt",
			Engine:   "engine-txt-empty",
			Text:     "",
			Markdown: "",
		}, nil
	}
	return &Result{
		Filename: filename,
		Format:   "txt",
		Engine:   "engine-txt",
		Text:     strings.Join(lines, "\n") + "\n",
		Markdown: pipeline.MarkdownFromLines(lines),
	}, nil
}

func decodeText(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(data)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	// try GB18030
	r := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GB18030.NewDecoder())
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r); err == nil && utf8.Valid(buf.Bytes()) {
		return buf.String()
	}
	return string(data)
}
