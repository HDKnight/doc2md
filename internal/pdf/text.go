package pdf

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"doc2md/internal/markdown"
	"doc2md/internal/textnorm"
)

// PageText holds extracted text for one page.
type PageText struct {
	Page int
	Text string
}

var formFeedSplit = regexp.MustCompile(`\f`)

// ExtractTextFromBytes extracts plain text using poppler `pdftotext -raw`
// (better reading order for multi-column Chinese PDFs than -layout).
func ExtractTextFromBytes(data []byte) ([]PageText, error) {
	pages, err := extractWithPDFToText(data)
	if err != nil {
		return nil, err
	}
	for i := range pages {
		pages[i].Text = textnorm.Normalize(pages[i].Text)
	}
	if IsGarbled(pages) {
		empty := make([]PageText, len(pages))
		for i := range pages {
			empty[i] = PageText{Page: pages[i].Page, Text: ""}
		}
		if len(empty) == 0 {
			empty = []PageText{{Page: 1, Text: ""}}
		}
		return empty, nil
	}
	return pages, nil
}

func extractWithPDFToText(data []byte) ([]PageText, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// -raw: content stream order; avoids broken two-column -layout for textbooks.
	cmd := exec.CommandContext(ctx, bin, "-raw", "-enc", "UTF-8", path, "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftotext: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	parts := formFeedSplit.Split(stdout.String(), -1)
	pages := make([]PageText, 0, len(parts))
	for i, p := range parts {
		pages = append(pages, PageText{Page: i + 1, Text: strings.TrimSpace(p)})
	}
	if len(pages) == 0 {
		pages = append(pages, PageText{Page: 1, Text: ""})
	}
	return pages, nil
}

// IsGarbled reports whether extracted text looks like PDF binary/metadata junk.
func IsGarbled(pages []PageText) bool {
	var b strings.Builder
	for _, p := range pages {
		b.WriteString(p.Text)
		b.WriteByte('\n')
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return false
	}
	if !utf8.ValidString(text) {
		return true
	}
	total := 0
	letters := 0
	junk := 0
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' {
			continue
		}
		total++
		switch {
		case r >= 0x4e00 && r <= 0x9fff:
			letters++
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			letters++
		case r < 32 || r == 0xfffd:
			junk++
		case strings.ContainsRune(`()*\\<>[]{}%~^`+"`", r):
			junk++
		}
	}
	if total < 20 {
		return false
	}
	lower := strings.ToLower(text)
	metaHits := 0
	for _, k := range []string{"endobj", "endstream", "/type", "/font", "xref"} {
		if strings.Contains(lower, k) {
			metaHits++
		}
	}
	if metaHits >= 2 {
		return true
	}
	if float64(letters)/float64(total) < 0.45 || float64(junk)/float64(total) > 0.25 {
		return true
	}
	return false
}

// PagesToBlocks turns page texts into structured markdown blocks.
func PagesToBlocks(pages []PageText) []markdown.Block {
	var b strings.Builder
	for i, p := range pages {
		if p.Text == "" {
			continue
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.Text)
		b.WriteByte('\n')
	}
	return textnorm.ToBlocks(b.String())
}

// PageCountHint returns number of pages if /Count is findable (best-effort).
func PageCountHint(data []byte) int {
	re := regexp.MustCompile(`/Count\s+(\d+)`)
	m := re.FindSubmatch(data)
	if len(m) == 2 {
		n, _ := strconv.Atoi(string(m[1]))
		if n > 0 {
			return n
		}
	}
	return 0
}
