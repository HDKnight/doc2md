package pipeline

import (
	"strings"
)

// MarkdownFromPages：oar structure JSON 页 → Markdown。
func MarkdownFromPages(pages []Page) string {
	var all []item
	for pi, p := range pages {
		all = append(all, pageItems(p.Blocks, pi+1)...)
	}
	if len(all) == 0 {
		return ""
	}
	return renderItems(all)
}

// MarkdownFromLines：docx/txt 等无 bbox 行 → Markdown。
func MarkdownFromLines(lines []string) string {
	items := make([]item, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		items = append(items, item{text: t, typ: "text", em: 12, w: 400, page: 1})
	}
	if len(items) == 0 {
		return ""
	}
	return renderItems(items)
}

func renderItems(all []item) string {
	// 按页 soft-wrap
	var items []item
	pageStart := 0
	for pageStart < len(all) {
		page := all[pageStart].page
		end := pageStart + 1
		for end < len(all) && all[end].page == page {
			end++
		}
		// 页内序：与旧 feat 一致，仅 y0/x0（pageItems 已排）。
		// 不对 structure 块做双栏 ReadingOrder——块级 x0 参差易误判成栏，打乱题序。
		chunk := all[pageStart:end]
		bodyEm := medianBodyEm(chunk)
		items = append(items, softWrapItems(chunk, bodyEm)...)
		pageStart = end
	}

	bodyEm := medianBodyEm(items)
	segs := make([]seg, 0, len(items))
	for i, it := range items {
		ser, num := seriesOf(it.text)
		s := seg{
			text: it.text, em: it.em, w: it.w, x0: it.x0, y0: it.y0, y1: it.y1,
			series: ser, num: num, typ: it.typ,
		}
		if markIsHeading(items, i, bodyEm) {
			s.kind = "heading"
		} else {
			s.kind = "para"
		}
		segs = append(segs, s)
	}

	segs = splitGluedOptions(segs)
	segs = splitShortLabelBodies(segs)
	// 定标题后再拼半截标题折行（旧 textnorm.headingLineContinues）
	segs = joinHeadingFragments(segs)
	alignPeerHeadings(segs)
	assignLevels(segs)

	var parts []string
	for _, s := range segs {
		t := strings.TrimSpace(s.text)
		if t == "" {
			continue
		}
		if s.kind == "heading" {
			lvl := s.lvl
			if lvl < 1 {
				lvl = 2
			}
			if lvl > 6 {
				lvl = 6
			}
			parts = append(parts, strings.Repeat("#", lvl)+" "+t)
			continue
		}
		parts = append(parts, t)
	}
	md := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if md == "" {
		return ""
	}
	return md + "\n"
}
