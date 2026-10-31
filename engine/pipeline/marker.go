package pipeline

import (
	"strconv"
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
	if r >= 'A' && r <= 'Z' || r >= 'Ａ' && r <= 'Ｚ' {
		return true
	}
	return false
}

func isLatinLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'ａ' && r <= 'ｚ'
}

// seriesOf：特殊序号族。大写选项不算标题序号族。
// 同一种写法是同一族；点分序号按段数分族，1. 与 1.1 不是同一种。
func seriesOf(text string) (kind string, num int) {
	t := strings.TrimSpace(text)
	if t == "" || isOptionLine(t) {
		return "", 0
	}
	r := []rune(t)
	if kind, num, ok := parseChapter(r); ok {
		return kind, num
	}
	if kind, num, ok := parseWrappedNum(r); ok {
		return kind, num
	}
	if n, ok := circleNum(r[0]); ok {
		return "circle", n
	}
	if kind, num, ok := parseCNMarker(r); ok {
		return kind, num
	}
	if kind, num, ok := parseLatinMarker(r); ok {
		return kind, num
	}
	if kind, num, ok := parseArabicMarker(r); ok {
		return kind, num
	}
	return "", 0
}

func parseChapter(r []rune) (kind string, num int, ok bool) {
	if len(r) < 3 || r[0] != '第' || strings.ContainsRune(string(r), '。') {
		return "", 0, false
	}
	i := 1
	if isDigit(r[i]) {
		j := i
		for j < len(r) && isDigit(r[j]) {
			j++
		}
		num = atoiRunes(r[i:j])
		i = j
	} else {
		val, n := parseCNNum(r[i:])
		if n == 0 {
			return "", 0, false
		}
		num = val
		i += n
	}
	if num <= 0 || i >= len(r) {
		return "", 0, false
	}
	switch r[i] {
	case '篇':
		return "pian", num, true
	case '章':
		return "chapter", num, true
	case '节':
		return "section", num, true
	default:
		return "", 0, false
	}
}

func parseWrappedNum(r []rune) (kind string, num int, ok bool) {
	if r[0] != '（' && r[0] != '(' {
		return "", 0, false
	}
	j := 1
	for j < len(r) && (isDigit(r[j]) || isCNDigit(r[j])) {
		j++
	}
	if j == 1 || j >= len(r) || (r[j] != '）' && r[j] != ')') {
		return "", 0, false
	}
	inner := r[1:j]
	if n := atoiRunes(inner); n > 0 && isDigit(inner[0]) {
		return "paren", n, true
	}
	if n := parseParenNum(inner); n > 0 {
		return "cn_paren", n, true
	}
	return "", 0, false
}

func parseCNMarker(r []rune) (kind string, num int, ok bool) {
	val, n := parseCNNum(r)
	if n == 0 || n >= len(r) {
		return "", 0, false
	}
	switch r[n] {
	case '、':
		return "cn_dun", val, true
	case '.', '．':
		return "cn_dot", val, true
	default:
		if unicode.IsSpace(r[n]) && strings.TrimSpace(string(r[n:])) != "" {
			return "cn_sp", val, true
		}
	}
	return "", 0, false
}

func parseLatinMarker(r []rune) (kind string, num int, ok bool) {
	if !isLatinLetter(r[0]) || len(r) < 2 {
		return "", 0, false
	}
	switch r[1] {
	case '、':
		return "latin_dun", latinOrder(r[0]), true
	case '.', '．':
		return "latin_dot", latinOrder(r[0]), true
	case ')', '）':
		return "latin_rparen", latinOrder(r[0]), true
	default:
		return "", 0, false
	}
}

func parseArabicMarker(r []rune) (kind string, num int, ok bool) {
	i := 0
	for i < len(r) && isDigit(r[i]) {
		i++
	}
	if i == 0 || i >= len(r) {
		return "", 0, false
	}
	first := atoiRunes(r[:i])
	switch r[i] {
	case '、':
		return "arab_dun", first, true
	case ')', '）':
		return "arab_rparen", first, true
	case '.', '．':
		if looksLikeDottedDate(r) {
			return "", 0, false
		}
		segs := []int{first}
		for i < len(r) && isDot(r[i]) && i+1 < len(r) && isDigit(r[i+1]) {
			i++
			j := i
			for j < len(r) && isDigit(r[j]) {
				j++
			}
			segs = append(segs, atoiRunes(r[i:j]))
			i = j
		}
		if len(segs) == 1 {
			if first <= 0 {
				return "", 0, false
			}
			return "arab_dot", first, true
		}
		depth := len(segs)
		if depth > 4 {
			depth = 4
		}
		last := segs[len(segs)-1]
		if last <= 0 {
			last = 1
		}
		return "arab_dot" + strconv.Itoa(depth), last, true
	default:
		if unicode.IsSpace(r[i]) && i <= 3 && strings.TrimSpace(string(r[i:])) != "" {
			return "arab_sp", first, true
		}
	}
	return "", 0, false
}

func isWeakOrdinal(text string) bool {
	kind, _ := seriesOf(text)
	return kind == "arab_sp"
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
	return strings.HasPrefix(k, "arab_")
}

func isStructuralLabel(text string) bool {
	k, _ := seriesOf(text)
	return k != ""
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9' || r >= '０' && r <= '９'
}

func isDot(r rune) bool {
	return r == '.' || r == '．'
}

func latinOrder(r rune) int {
	switch {
	case r >= 'a' && r <= 'z':
		return int(r-'a') + 1
	case r >= 'ａ' && r <= 'ｚ':
		return int(r-'ａ') + 1
	default:
		return 0
	}
}

func circleNum(r rune) (int, bool) {
	const circles = "①②③④⑤⑥⑦⑧⑨⑩⑪⑫⑬⑭⑮⑯⑰⑱⑲⑳"
	for i, c := range []rune(circles) {
		if c == r {
			return i + 1, true
		}
	}
	return 0, false
}

func parseCNNum(r []rune) (val, n int) {
	if len(r) == 0 || !isCNDigit(r[0]) {
		return 0, 0
	}
	if r[0] == '十' {
		if len(r) > 1 && cnNum(r[1]) > 0 {
			return 10 + cnNum(r[1]), 2
		}
		return 10, 1
	}
	d := cnNum(r[0])
	if d == 0 {
		return 0, 0
	}
	if len(r) > 1 && r[1] == '十' {
		if len(r) > 2 && cnNum(r[2]) > 0 {
			return d*10 + cnNum(r[2]), 3
		}
		return d * 10, 2
	}
	return d, 1
}

func atoiRunes(r []rune) int {
	n := 0
	for _, c := range r {
		if c >= '０' && c <= '９' {
			c = '0' + (c - '０')
		}
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
