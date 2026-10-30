package textnorm

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Normalize applies NFKC (fixes Kangxi radicals like ⽬→目) and cleans PDF artifacts.
func Normalize(s string) string {
	s = norm.NFKC.String(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.ReplaceAll(s, "\u3000", " ") // ideographic space → normal for MD readability
	// Remove soft hyphens / BOM leftovers
	s = strings.ReplaceAll(s, "\u00ad", "")
	s = strings.ReplaceAll(s, "\ufeff", "")
	return s
}

// JoinWrappedLines merges PDF hard-wraps inside Chinese paragraphs.
func JoinWrappedLines(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return text
	}
	var out []string
	var buf strings.Builder
	flush := func() {
		t := strings.TrimSpace(buf.String())
		buf.Reset()
		if t != "" {
			out = append(out, t)
		}
	}
	blankPending := false
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" {
			// Page breaks often insert blank lines mid-sentence; defer the break.
			blankPending = true
			continue
		}
		if buf.Len() == 0 {
			buf.WriteString(trim)
			blankPending = false
			continue
		}
		prev := buf.String()
		if shouldJoin(prev, trim) {
			if needsASCIISpace(prev, trim) {
				buf.WriteByte(' ')
			}
			buf.WriteString(trim)
			blankPending = false
			continue
		}
		if blankPending {
			flush()
			out = append(out, "")
			blankPending = false
		} else {
			flush()
		}
		buf.WriteString(trim)
	}
	flush()
	return strings.Join(out, "\n")
}

func shouldJoin(prev, next string) bool {
	if prev == "" || next == "" {
		return false
	}
	// 未闭合的填空括号优先于题号、选项、标题：在( / (B / 日( 都要接下行直到括号合上
	if LineContinuesUnclosed(prev, next) {
		return true
	}
	// New structural line → don't join onto previous buffer
	if looksStructural(next) || startsWithMarker(next) || isAnswerLabel(next) {
		return false
	}
	rPrev, _ := lastRune(prev)
	rNext, _ := firstRune(next)
	// Short numbered titles like "1、什么是马克思主义" stay alone
	if IsShortStandaloneTitle(prev) {
		return false
	}
	// Ends with sentence punctuation → paragraph break
	if strings.ContainsRune("。！？；：.!?;:", rPrev) {
		return false
	}
	// Next starts with list/option marker already handled by looksStructural
	// Don't join if previous ends with closing paren of blank answer like （）
	if strings.HasSuffix(prev, "）") || strings.HasSuffix(prev, ")") {
		if looksStructural(next) {
			return false
		}
	}
	// Prefer joining CJK runs broken mid-phrase
	if isCJK(rPrev) && (isCJK(rNext) || unicode.IsLetter(rNext) || unicode.IsNumber(rNext) || rNext == '《' || rNext == '“' || rNext == '"' || rNext == '\'') {
		return true
	}
	if (unicode.IsLetter(rPrev) || unicode.IsNumber(rPrev)) && (unicode.IsLetter(rNext) || unicode.IsNumber(rNext)) {
		return true
	}
	return false
}

// LineContinuesUnclosed reports that next still belongs to prev because a
// bracket opened on prev and closes near the start of next.
// Headings are included: "#### 28、……日(" + "B)的爆发……".
// A real new item (A、 / 29、 / 【答案】) is not absorbed.
func LineContinuesUnclosed(prev, next string) bool {
	prev = stripMdHeading(prev)
	next = strings.TrimSpace(next)
	if prev == "" || next == "" || strings.HasPrefix(next, "#") {
		return false
	}
	if !hasUnclosedOpenBracket(prev) {
		return false
	}
	if reJoinNumItem.MatchString(next) || reJoinCnDun.MatchString(next) ||
		reJoinAnsMark.MatchString(next) {
		return false
	}
	// 孤立「第X…」不挡括号续接；真·章节/部分或成对第X前缀才拒
	if m := ClassifyMarker(next); m != nil && (m.Family == "chapter" || m.Family == "part") {
		return false
	}
	if reJoinOpt.MatchString(next) && !reFilledBlankCont.MatchString(next) {
		return false
	}
	head := []rune(next)
	if len(head) > 16 {
		head = head[:16]
	}
	return strings.ContainsAny(string(head), ")）]】》”\"")
}

func stripMdHeading(line string) string {
	s := strings.TrimSpace(line)
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	if i == 0 || i >= len(s) || s[i] != ' ' {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(s[i+1:])
}

func isClosingBracket(r rune) bool {
	switch r {
	case ')', '）', ']', '】', '』', '”', '"', '》':
		return true
	}
	return false
}

func hasUnclosedOpenBracket(s string) bool {
	for _, p := range []struct{ open, close string }{
		{"(", ")"}, {"（", "）"}, {"[", "]"}, {"【", "】"}, {"“", "”"}, {"《", "》"},
	} {
		openI := strings.LastIndex(s, p.open)
		closeI := strings.LastIndex(s, p.close)
		if openI >= 0 && openI > closeI {
			return true
		}
	}
	return false
}

func looksStructural(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	key := strings.ReplaceAll(line, " ", "")
	if key == "目录" || key == "简介" || key == "导论" ||
		strings.HasPrefix(key, "目录") || strings.HasPrefix(key, "简介") || strings.HasPrefix(key, "导论") {
		return true
	}
	runes := []rune(line)
	// Only treat 第X部分/章 as structural when the line is short (a heading, not a sentence).
	if strings.HasPrefix(line, "第") && utf8.RuneCountInString(line) <= 40 {
		if strings.Contains(line, "章") || strings.Contains(line, "部分") || strings.Contains(line, "节") {
			return true
		}
	}
	if strings.HasPrefix(line, "【") {
		return true
	}
	if len(runes) >= 2 {
		if (runes[1] == '、' || runes[1] == '.') && isChineseNumeral(runes[0]) && utf8.RuneCountInString(line) <= 40 {
			return true
		}
		i := 0
		for i < len(runes) && unicode.IsNumber(runes[i]) {
			i++
		}
		if i > 0 && i < len(runes) && (runes[i] == '、' || runes[i] == '.' || runes[i] == '．') {
			return true
		}
		if runes[0] >= 'A' && runes[0] <= 'Z' && (runes[1] == '.' || runes[1] == '、' || runes[1] == '．') {
			return true
		}
		if runes[0] >= 'Ａ' && runes[0] <= 'Ｚ' && (runes[1] == '.' || runes[1] == '、' || runes[1] == '．') {
			return true
		}
		if (runes[0] == '（' || runes[0] == '(') && len(runes) > 1 && unicode.IsNumber(runes[1]) {
			return true
		}
	}
	return false
}

func isChineseNumeral(r rune) bool {
	return strings.ContainsRune("一二三四五六七八九十百千零〇", r)
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r)
}

func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

func lastRune(s string) (rune, bool) {
	var last rune
	ok := false
	for _, r := range s {
		last = r
		ok = true
	}
	return last, ok
}

func needsASCIISpace(prev, next string) bool {
	rPrev, ok1 := lastRune(prev)
	rNext, ok2 := firstRune(next)
	if !ok1 || !ok2 {
		return false
	}
	return isASCIIWordChar(rPrev) && isASCIIWordChar(rNext)
}

func isASCIIWordChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}
