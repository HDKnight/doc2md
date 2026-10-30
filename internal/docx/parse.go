package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"doc2md/internal/markdown"
	"doc2md/internal/textnorm"
)

// Parse reads a DOCX file and returns markdown blocks.
func Parse(r io.ReaderAt, size int64) ([]markdown.Block, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return nil, fmt.Errorf("docx missing word/document.xml")
	}
	rc, err := docFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return parseDocumentXML(data)
}

func parseDocumentXML(data []byte) ([]markdown.Block, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var blocks []markdown.Block
	var para strings.Builder
	var inPara, inTable bool
	var tableRows [][]string
	var currentRow []string
	var cellText strings.Builder
	inCell := false

	flushPara := func() {
		text := strings.TrimSpace(para.String())
		para.Reset()
		if text == "" {
			return
		}
		blocks = append(blocks, markdown.Block{Type: "paragraph", Text: text})
	}

	flushTable := func() {
		if len(tableRows) == 0 {
			return
		}
		blocks = append(blocks, markdown.Block{Type: "table", Text: rowsToMarkdownTable(tableRows)})
		tableRows = nil
	}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				inPara = true
			case "tbl":
				inTable = true
				flushPara()
			case "tr":
				currentRow = nil
			case "tc":
				inCell = true
				cellText.Reset()
			case "t":
				// text follows as CharData; handled below via decoder — use CharData
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				if inTable && inCell {
					// paragraph inside cell already accumulated in cellText via t
				} else if inPara && !inTable {
					flushPara()
				}
				inPara = false
			case "tbl":
				flushTable()
				inTable = false
			case "tr":
				if inTable {
					tableRows = append(tableRows, currentRow)
					currentRow = nil
				}
			case "tc":
				if inCell {
					currentRow = append(currentRow, strings.TrimSpace(cellText.String()))
					inCell = false
				}
			}
		case xml.CharData:
			s := string(t)
			if inCell {
				cellText.WriteString(s)
			} else if inPara && !inTable {
				para.WriteString(s)
			}
		}
	}
	flushPara()
	for i := range blocks {
		blocks[i].Text = textnorm.Normalize(blocks[i].Text)
	}
	return blocks, nil
}

func rowsToMarkdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	pad := func(r []string) []string {
		out := make([]string, cols)
		copy(out, r)
		return out
	}
	var b strings.Builder
	header := pad(rows[0])
	b.WriteString("|")
	for _, c := range header {
		b.WriteString(" ")
		b.WriteString(escapeCell(c))
		b.WriteString(" |")
	}
	b.WriteByte('\n')
	b.WriteString("|")
	for range header {
		b.WriteString(" --- |")
	}
	b.WriteByte('\n')
	for _, row := range rows[1:] {
		r := pad(row)
		b.WriteString("|")
		for _, c := range r {
			b.WriteString(" ")
			b.WriteString(escapeCell(c))
			b.WriteString(" |")
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
