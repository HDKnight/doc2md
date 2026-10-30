package feat

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// OCR 常把括号写乱：（1） (1) （1) (1）
var reParenMark = regexp.MustCompile(`[（(]\s*[0-9一二三四五六七八九十]{1,3}\s*[）)]`)

// splitEmbeddedParenMarkers：正文里粘着的 (1)(2) 拆成标题；
// 标题取到该条首句「。」（含），句号后另起一段。
func splitEmbeddedParenMarkers(segs []seg) []seg {
	if len(segs) == 0 {
		return segs
	}
	out := make([]seg, 0, len(segs)+4)
	for _, s := range segs {
		if s.kind == "heading" {
			out = append(out, s)
			continue
		}
		parts := splitParenParts(s)
		if len(parts) == 0 {
			out = append(out, s)
			continue
		}
		out = append(out, parts...)
	}
	return out
}

func splitParenParts(s seg) []seg {
	text := s.text
	idxs := reParenMark.FindAllStringIndex(text, -1)
	if len(idxs) == 0 {
		return nil
	}
	first := idxs[0][0]
	prefix := strings.TrimSpace(text[:first])
	if prefix != "" && !looksAnswerPrefix(prefix) && utf8.RuneCountInString(prefix) > 6 {
		return nil
	}

	var out []seg
	cursor := 0
	for mi, loc := range idxs {
		start, markEnd := loc[0], loc[1]
		if start > cursor {
			pre := strings.TrimSpace(text[cursor:start])
			if pre != "" {
				out = append(out, paraSeg(s, pre))
			}
		}
		// 本条范围：当前题号 → 下一题号或文末
		limit := len(text)
		if mi+1 < len(idxs) {
			limit = idxs[mi+1][0]
		}
		chunk := text[start:limit]
		title, body := splitTitleAtPeriod(chunk)
		if title == "" {
			cursor = markEnd
			continue
		}
		ser, num := seriesOf(title)
		if ser == "" {
			ser, num = "paren", parenNumFromMark(text[start:markEnd])
		}
		h := s
		h.kind = "heading"
		h.lvl = 0
		h.text = title
		h.series = ser
		h.num = num
		out = append(out, h)
		if body != "" {
			out = append(out, paraSeg(s, body))
		}
		cursor = limit
	}
	if cursor < len(text) {
		if tail := strings.TrimSpace(text[cursor:]); tail != "" {
			out = append(out, paraSeg(s, tail))
		}
	}
	if len(out) == 0 {
		return nil
	}
	// 未真正拆出标题则放弃
	hasHead := false
	for _, p := range out {
		if p.kind == "heading" {
			hasHead = true
			break
		}
	}
	if !hasHead {
		return nil
	}
	return out
}

func looksAnswerPrefix(prefix string) bool {
	p := strings.TrimSpace(prefix)
	return p == "答：" || p == "答:" || strings.HasSuffix(p, "答：") || strings.HasSuffix(p, "答:")
}

func paraSeg(tmpl seg, text string) seg {
	s := tmpl
	s.kind = "para"
	s.lvl = 0
	s.text = text
	s.series, s.num = seriesOf(text)
	return s
}

// splitTitleAtPeriod：只按结构标点切开（。」或 ：/:）；不用字数硬切标题。
func splitTitleAtPeriod(chunk string) (title, body string) {
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return "", ""
	}
	period := indexFirstPeriod(chunk)
	if period >= 0 {
		title = strings.TrimSpace(chunk[:period+len("。")])
		body = strings.TrimSpace(chunk[period+len("。"):])
		return title, body
	}
	if i := indexColon(chunk); i >= 0 {
		r := []rune(chunk)
		return strings.TrimSpace(string(r[:i+1])), strings.TrimSpace(string(r[i+1:]))
	}
	return chunk, ""
}

func indexFirstPeriod(s string) int {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == '。' {
			return i
		}
		i += size
	}
	return -1
}

func indexColon(s string) int {
	r := []rune(s)
	for i, c := range r {
		if c == '：' || c == ':' {
			return i
		}
	}
	return -1
}

func parenNumFromMark(mark string) int {
	r := []rune(strings.TrimSpace(mark))
	if len(r) < 3 {
		return 0
	}
	j := 1
	n, ok := parseArabOrCN(r, &j)
	if !ok {
		return 0
	}
	return n
}
