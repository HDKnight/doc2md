package geom

import (
	"regexp"
	"strings"
)

var (
	reDots    = regexp.MustCompile(`[.…·]{3,}`)
	rePageEnd = regexp.MustCompile(`\s+\d{1,4}$`)
	reTOCTitle = regexp.MustCompile(`^目\s*录$`)
)

func normSpace(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func inMargin(item Line) bool {
	center := (item.Y0 + item.Y1) / 2
	return center <= item.PageHeight*0.08 || center >= item.PageHeight*0.92
}

func isTOCEntry(text string) bool {
	return reDots.MatchString(text) || rePageEnd.MatchString(text)
}

func endsTOC(text string) bool {
	return len([]rune(text)) > 40 && !rePageEnd.MatchString(text) && !reDots.MatchString(text)
}

// DropChrome removes TOC blocks, repeated headers/footers, and page numbers (pdf-md chrome).
func DropChrome(lines []Line, pageCount int) []Line {
	headerPages := map[string]map[int]struct{}{}
	for _, item := range lines {
		if inMargin(item) && !isDigits(strings.TrimSpace(item.Text)) {
			key := normSpace(item.Text)
			if headerPages[key] == nil {
				headerPages[key] = map[int]struct{}{}
			}
			headerPages[key][item.Page] = struct{}{}
		}
	}
	repeated := map[string]struct{}{}
	for text, pages := range headerPages {
		n := len(pages)
		if n >= 3 && float64(n) >= float64(pageCount)*0.3 {
			repeated[text] = struct{}{}
		}
	}

	kept := make([]Line, 0, len(lines))
	index := 0
	for index < len(lines) {
		item := lines[index]
		text := strings.TrimSpace(item.Text)
		if inMargin(item) {
			if isDigits(text) {
				index++
				continue
			}
			if _, ok := repeated[normSpace(item.Text)]; ok {
				index++
				continue
			}
		}
		if reTOCTitle.MatchString(text) {
			tocPage := item.Page
			if index+1 < len(lines) && endsTOC(strings.TrimSpace(lines[index+1].Text)) {
				index++
				continue
			}
			index++
			for index < len(lines) {
				nxt := lines[index]
				nxtText := strings.TrimSpace(nxt.Text)
				if endsTOC(nxtText) {
					break
				}
				if nxt.Page != tocPage && !isTOCEntry(nxtText) {
					break
				}
				index++
			}
			continue
		}
		run := index
		for run < len(lines) && isTOCEntry(strings.TrimSpace(lines[run].Text)) {
			run++
		}
		if run-index >= 5 {
			index = run
			continue
		}
		cleaned := strings.TrimSpace(reDots.ReplaceAllString(item.Text, ""))
		if cleaned != "" {
			item.Text = cleaned
			kept = append(kept, item)
		}
		index++
	}
	return kept
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
