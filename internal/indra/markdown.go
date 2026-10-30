package indra

import (
	"strings"
	"unicode/utf8"

	"doc2md/internal/feat"
)

// MarkdownFromPages 保守渲染：不跑 feat 标题启发式。
// 只有显式标题类型出 #/##；其余一律段落。避免「1、题干」「A、选项」被抬成标题。
func MarkdownFromPages(pages []feat.Page) string {
	var b strings.Builder
	for _, pg := range pages {
		for _, bl := range pg.Blocks {
			t := strings.TrimSpace(bl.Content)
			if t == "" {
				continue
			}
			typ := strings.ToLower(strings.TrimSpace(bl.Type))
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			switch {
			case typ == "doc_title" && looksLikeRealTitle(t):
				b.WriteString("# ")
				b.WriteString(t)
			case (typ == "paragraph_title" || typ == "section_title") && looksLikeRealTitle(t):
				b.WriteString("## ")
				b.WriteString(t)
			default:
				b.WriteString(t)
			}
		}
	}
	if b.Len() > 0 {
		b.WriteByte('\n')
	}
	return b.String()
}

// looksLikeRealTitle：拦掉题号/选项/过长正文被标成 Section-Header 的情况。
func looksLikeRealTitle(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if utf8.RuneCountInString(s) > 40 {
		return false
	}
	if isQuizStemOrOption(s) {
		return false
	}
	return true
}

func isQuizStemOrOption(s string) bool {
	r, size := utf8.DecodeRuneInString(s)
	// A、 B. 选项头
	if r >= 'A' && r <= 'Z' {
		rest := s[size:]
		if strings.HasPrefix(rest, "、") || strings.HasPrefix(rest, ".") || strings.HasPrefix(rest, "．") {
			return true
		}
	}
	// 1、 12. 数字题号（含全角数字）
	i := 0
	for i < len(s) {
		ch, n := utf8.DecodeRuneInString(s[i:])
		if ch >= '0' && ch <= '9' {
			i += n
			continue
		}
		if ch >= '０' && ch <= '９' {
			i += n
			continue
		}
		break
	}
	if i == 0 {
		return false
	}
	rest := s[i:]
	return strings.HasPrefix(rest, "、") ||
		strings.HasPrefix(rest, ".") ||
		strings.HasPrefix(rest, "．")
}
