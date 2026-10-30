package geom

import (
	"fmt"
	"strings"

	"doc2md/internal/markdown"
	"doc2md/internal/textnorm"
)

// PageInput is one page of raw geometry lines (before chrome/join).
type PageInput struct {
	Page   int
	Width  float64
	Height float64
	Lines  []Line
	// Diagram forces this page to be skipped from the text tree.
	// When true and AssetMarkdown is set, that markdown snippet is inserted.
	Diagram       bool
	AssetMarkdown string // e.g. ![](assets/x-p3.png) or <!-- page N: no text -->
}

// BuildMarkdown runs the default markdown pipeline.
func BuildMarkdown(pages []PageInput) string {
	return BuildMarkdownOpt(pages, HeadingOptions{})
}

// BuildMarkdownOpt chooses heading strategy:
//   - outline ≥ 3 → Python/doc2md 金标路径（书签锁 ### + 题号 ####）
//   - otherwise → pdf-md frequency+stack（无书签笔记等）
func BuildMarkdownOpt(pages []PageInput, opt HeadingOptions) string {
	if len(opt.Outline) >= 3 {
		opt.Enhance = true // 题库金标：题号一律 ####（不依赖 bold）
		return BuildHeadingMarkdown(pages, opt)
	}
	return buildMarkdownStack(pages, opt)
}

func buildMarkdownStack(pages []PageInput, opt HeadingOptions) string {
	pageCount := len(pages)
	var body []Line
	for _, p := range pages {
		if p.Diagram {
			continue
		}
		body = append(body, p.Lines...)
	}
	cleaned := DropChrome(body, pageCount)

	byPage := map[int][]Line{}
	for _, item := range cleaned {
		byPage[item.Page] = append(byPage[item.Page], item)
	}

	var pieces []string
	var pending [][]Line

	flush := func() {
		merged := joinPages(pending)
		pending = nil
		if len(merged) == 0 {
			return
		}
		texts := textnorm.ExpandGluedLines(Texts(merged))
		blocks := textnorm.BuildOutline(texts)
		md := textnorm.FinishMarkdown(markdown.Render(blocks))
		if strings.TrimSpace(md) != "" {
			pieces = append(pieces, md)
		}
	}

	for _, p := range pages {
		if p.Diagram {
			flush()
			if p.AssetMarkdown != "" {
				pieces = append(pieces, p.AssetMarkdown)
			}
			continue
		}
		pageLines := byPage[p.Page]
		if len(pageLines) == 0 {
			flush()
			if len(p.Lines) == 0 && p.AssetMarkdown != "" {
				pieces = append(pieces, p.AssetMarkdown)
			} else if len(p.Lines) == 0 {
				pieces = append(pieces, fmt.Sprintf("<!-- page %d: no text -->", p.Page))
			}
			continue
		}
		pending = append(pending, joinGroups(columnGroupsOrReading(pageLines)))
	}
	flush()
	md := strings.Join(pieces, "\n\n")
	if t := strings.TrimSpace(opt.DocTitle); t != "" && !strings.HasPrefix(strings.TrimSpace(md), "# ") {
		md = "# " + t + "\n\n" + md
	}
	return md
}

func columnGroupsOrReading(lines []Line) [][]Line {
	cols := ColumnGroups(lines)
	if cols != nil {
		return cols
	}
	return [][]Line{ReadingOrder(lines)}
}

func joinGroups(groups [][]Line) []Line {
	parts := make([][]Line, 0, len(groups))
	for _, g := range groups {
		parts = append(parts, JoinLines(g))
	}
	if len(parts) == 2 && len(parts[0]) > 0 && len(parts[1]) > 0 {
		if seam := JoinSeam(parts[0][len(parts[0])-1], parts[1][0]); seam != nil {
			out := append([]Line{}, parts[0][:len(parts[0])-1]...)
			out = append(out, *seam)
			out = append(out, parts[1][1:]...)
			return out
		}
	}
	var merged []Line
	for _, part := range parts {
		merged = append(merged, part...)
	}
	return merged
}

func joinPages(pages [][]Line) []Line {
	var merged []Line
	for _, page := range pages {
		if len(page) == 0 {
			continue
		}
		if len(merged) > 0 {
			if seam := JoinSeam(merged[len(merged)-1], page[0]); seam != nil {
				merged = append(merged[:len(merged)-1], *seam)
				merged = append(merged, page[1:]...)
				continue
			}
		}
		merged = append(merged, page...)
	}
	return merged
}

// PlainText joins line texts for the "原文" view after chrome+join.
func PlainText(pages []PageInput) string {
	pageCount := len(pages)
	var body []Line
	for _, p := range pages {
		if p.Diagram {
			continue
		}
		body = append(body, p.Lines...)
	}
	cleaned := DropChrome(body, pageCount)
	byPage := map[int][]Line{}
	for _, item := range cleaned {
		byPage[item.Page] = append(byPage[item.Page], item)
	}
	var pending [][]Line
	for _, p := range pages {
		if p.Diagram {
			continue
		}
		pageLines := byPage[p.Page]
		if len(pageLines) == 0 {
			continue
		}
		pending = append(pending, joinGroups(columnGroupsOrReading(pageLines)))
	}
	merged := joinPages(pending)
	var b strings.Builder
	for _, l := range merged {
		t := strings.TrimSpace(l.Text)
		if t == "" {
			continue
		}
		b.WriteString(t)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(textnorm.Normalize(b.String()))
}
