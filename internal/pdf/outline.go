package pdf

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

// OutlineEntry is one PDF bookmark (TOC) item.
type OutlineEntry struct {
	Level int    // 1-based hierarchy level
	Title string
	Page  int    // 1-based page number
}

var rePromo = regexp.MustCompile(`微信公众号|更多资料|免费分享|扫码关注|资料网|QQ群`)

// ExtractOutlineFromBytes reads PDF bookmarks via pdfcpu (pure Go).
func ExtractOutlineFromBytes(data []byte) ([]OutlineEntry, error) {
	rs := bytes.NewReader(data)
	bms, err := api.Bookmarks(rs, nil)
	if err != nil {
		// Missing / empty outlines are common; treat as no outline.
		return nil, nil
	}
	var out []OutlineEntry
	var walk func([]pdfcpu.Bookmark, int)
	walk = func(xs []pdfcpu.Bookmark, level int) {
		for _, b := range xs {
			title := strings.TrimSpace(b.Title)
			if title == "" || rePromo.MatchString(title) {
				walk(b.Kids, level+1)
				continue
			}
			page := b.PageFrom
			if page < 1 {
				page = 1
			}
			out = append(out, OutlineEntry{Level: level, Title: title, Page: page})
			walk(b.Kids, level+1)
		}
	}
	walk(bms, 1)
	return out, nil
}
