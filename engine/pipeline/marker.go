package pipeline

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func isOptionLine(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	r0, n0 := utf8.DecodeRuneInString(t)
	if !isOptLetter(r0) {
		return false
	}
	rest := t[n0:]
	if rest == "" {
		return false
	}
	r1, _ := utf8.DecodeRuneInString(rest)
	return r1 == '、' || r1 == '.' || r1 == '．' || r1 == ')' || r1 == '）' || unicode.IsSpace(r1)
}

func isOptLetter(r rune) bool {
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'Ａ' && r <= 'Ｚ' {
		return true
	}
	return false
}

// seriesOf：特殊序号族。选项不算标题序号族。
func seriesOf(text string) (kind string, num int) {
	t := strings.TrimSpace(text)
	if t == "" || isOptionLine(t) {
		return "", 0
	}
	r := []rune(t)
	// 1、 / 1.
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i > 0 && i < len(r) {
		switch r[i] {
		case '、':
			return "arab_dun", atoiRunes(r[:i])
		case '.', '．':
			// 1932.7.21 / 1949.3 是日期，不是 1. 这种序号
			if looksLikeDottedDate(r) {
				return "", 0
			}
			return "arab_dot", atoiRunes(r[:i])
		}
	}
	// （1） / (1)
	if r[0] == '（' || r[0] == '(' {
		j := 1
		for j < len(r) && (r[j] >= '0' && r[j] <= '9' || isCNDigit(r[j])) {
			j++
		}
		if j > 1 && j < len(r) && (r[j] == '）' || r[j] == ')') {
			return "paren", parseParenNum(r[1:j])
		}
	}
	// 一、
	cn := "一二三四五六七八九十"
	if strings.ContainsRune(cn, r[0]) && len(r) > 1 && (r[1] == '、' || r[1] == '.' || r[1] == '．') {
		return "cn_dun", cnNum(r[0])
	}
	return "", 0
}

// looksLikeDottedDate：行首点分日期（1932.7.21、1949.3召开），不当序号。
func looksLikeDottedDate(r []rune) bool {
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i != 4 {
		return false
	}
	year := atoiRunes(r[:i])
	if year < 1000 || year > 2999 {
		return false
	}
	if i >= len(r) || (r[i] != '.' && r[i] != '．') {
		return false
	}
	i++
	mStart := i
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i == mStart || i-mStart > 2 {
		return false
	}
	month := atoiRunes(r[mStart:i])
	if month < 1 || month > 12 {
		return false
	}
	if i >= len(r) || (r[i] != '.' && r[i] != '．') {
		return i >= len(r) || r[i] < '0' || r[i] > '9'
	}
	i++
	dStart := i
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i == dStart || i-dStart > 2 {
		return false
	}
	day := atoiRunes(r[dStart:i])
	if day < 1 || day > 31 {
		return false
	}
	return i >= len(r) || r[i] < '0' || r[i] > '9'
}

func isArabicStem(text string) bool {
	k, _ := seriesOf(text)
	return k == "arab_dun" || k == "arab_dot"
}

func isStructuralLabel(text string) bool {
	k, _ := seriesOf(text)
	return k == "cn_dun" || k == "paren" || k == "arab_dun" || k == "arab_dot"
}

func atoiRunes(r []rune) int {
	n := 0
	for _, c := range r {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 9999 {
			return n
		}
	}
	return n
}

func isCNDigit(r rune) bool {
	return strings.ContainsRune("一二三四五六七八九十百零〇两", r)
}

func parseParenNum(r []rune) int {
	if n := atoiRunes(r); n > 0 {
		return n
	}
	if len(r) == 1 {
		return cnNum(r[0])
	}
	return 0
}

func cnNum(r rune) int {
	m := map[rune]int{'一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9, '十': 10}
	return m[r]
}

func typeIsTitle(typ string) bool {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "doc_title", "title", "paragraph_title", "section_title", "aside_title":
		return true
	default:
		return false
	}
}
