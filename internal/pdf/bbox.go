package pdf

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"doc2md/internal/geom"
	"doc2md/internal/textnorm"
)

// PageLines is one PDF page of geometry-aware lines.
type PageLines struct {
	Page   int
	Width  float64
	Height float64
	Lines  []geom.Line
}

// ExtractLinesFromBytes uses poppler `pdftotext -bbox-layout` (XHTML with line boxes).
func ExtractLinesFromBytes(data []byte) ([]PageLines, error) {
	bin := popplerBin("DOC2MD_PDFTOTEXT", "pdftotext")
	if err := lookPoppler(bin); err != nil {
		return nil, fmt.Errorf("未找到 pdftotext：请运行 start.bat 自动安装到 tools/poppler，或设置 DOC2MD_PDFTOTEXT")
	}
	tmp, err := os.CreateTemp("", "doc2md-*.pdf")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-bbox-layout", "-enc", "UTF-8", path, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext -bbox-layout: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	pages, err := ParseBBoxLayoutHTML(stdout.Bytes())
	if err != nil {
		return nil, err
	}
	// Empty / garbled → treat as no text layer (caller may OCR).
	flat := make([]PageText, len(pages))
	for i, p := range pages {
		var b strings.Builder
		for _, ln := range p.Lines {
			b.WriteString(ln.Text)
			b.WriteByte('\n')
		}
		flat[i] = PageText{Page: p.Page, Text: b.String()}
	}
	if IsGarbled(flat) {
		out := make([]PageLines, len(pages))
		for i, p := range pages {
			out[i] = PageLines{Page: p.Page, Width: p.Width, Height: p.Height}
		}
		if len(out) == 0 {
			out = []PageLines{{Page: 1}}
		}
		return out, nil
	}
	return pages, nil
}

// ParseBBoxLayoutHTML parses pdftotext -bbox-layout XHTML into page lines.
func ParseBBoxLayoutHTML(data []byte) ([]PageLines, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	var pages []PageLines
	var cur *PageLines
	var lineBuf strings.Builder
	var lineX0, lineY0, lineX1, lineY1 float64
	var inLine bool
	pageNo := 0

	flushLine := func() {
		if cur == nil || !inLine {
			return
		}
		text := strings.TrimSpace(textnorm.Normalize(lineBuf.String()))
		inLine = false
		lineBuf.Reset()
		if text == "" {
			return
		}
		h := lineY1 - lineY0
		cur.Lines = append(cur.Lines, geom.Line{
			Text:       text,
			Page:       cur.Page,
			X0:         lineX0,
			Y0:         lineY0,
			X1:         lineX1,
			Y1:         lineY1,
			PageWidth:  cur.Width,
			PageHeight: cur.Height,
			Source:     "text",
			FontSize:   h,
		})
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("bbox xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "page":
				flushLine()
				pageNo++
				pages = append(pages, PageLines{
					Page:   pageNo,
					Width:  attrFloat(t, "width"),
					Height: attrFloat(t, "height"),
				})
				cur = &pages[len(pages)-1]
			case "line":
				flushLine()
				inLine = true
				lineX0 = attrFloat(t, "xMin")
				lineY0 = attrFloat(t, "yMin")
				lineX1 = attrFloat(t, "xMax")
				lineY1 = attrFloat(t, "yMax")
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "line":
				flushLine()
			case "page":
				flushLine()
				cur = nil
			}
		case xml.CharData:
			if !inLine {
				continue
			}
			trimmed := strings.TrimSpace(string(t))
			if trimmed == "" {
				continue
			}
			if lineBuf.Len() > 0 {
				lineBuf.WriteByte(' ')
			}
			lineBuf.WriteString(trimmed)
		}
	}
	flushLine()
	if len(pages) == 0 {
		pages = []PageLines{{Page: 1}}
	}
	return pages, nil
}

func attrFloat(el xml.StartElement, name string) float64 {
	for _, a := range el.Attr {
		if strings.EqualFold(a.Name.Local, name) {
			v, _ := strconv.ParseFloat(a.Value, 64)
			return v
		}
	}
	return 0
}
