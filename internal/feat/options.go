package feat

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// 字母选项（大小写 / 全角）
const optLetterClass = `A-Za-zＡ-Ｚａ-ｚ`

var (
	reLetterOrd = regexp.MustCompile(`[A-Za-zＡ-Ｚａ-ｚ][、.．,，]`)
	reArabDun   = regexp.MustCompile(`\d{1,3}、`)
	reArabDot   = regexp.MustCompile(`\d{1,3}[.．]`)
	reParenOrd  = regexp.MustCompile(`[（(](?:\d{1,3}|[一二三四五六七八九十]+)[）)]`)
	reCnDun     = regexp.MustCompile(`[一二三四五六七八九十百千]+、`)
	reQGlueDun  = regexp.MustCompile(`([\x{4e00}-\x{9fff}` + optLetterClass + `）)。；;!！？》\"”])\s*(\d{1,3}、\s*[\x{4e00}-\x{9fff}（(` + optLetterClass + `“\"《\d])`)
	reQGlueDot  = regexp.MustCompile(`([\x{4e00}-\x{9fff}` + optLetterClass + `）)。；;!！？》\"”])\s*(\d{1,3}[.．]\s*[\x{4e00}-\x{9fff}（(` + optLetterClass + `“\"《])`)
)

type ordMark struct {
	start, end int
	kind       string // letter | arab_dun | arab_dot | paren | cn_dun
	num        int
}

type ordHit struct {
	segIdx     int
	start, end int
	kind       string
	num        int
}

// isOptionLine：行首为字母选项标记，永不做标题。
func isOptionLine(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	r0, n0 := utf8.DecodeRuneInString(t)
	if !isOptLetter(r0) {
		return false
	}
	r1, _ := utf8.DecodeRuneInString(t[n0:])
	switch r1 {
	case '、', '.', '．', ',', '，':
		return true
	default:
		return false
	}
}

func isOptLetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
		(r >= 'Ａ' && r <= 'Ｚ') || (r >= 'ａ' && r <= 'ｚ')
}

// splitGluedQuizSegs：
// 1) 行内同类序号 ≥2 → 切开
// 2) 跨段：认出 A、 再找到相邻 B、（连续序号串 ≥2）→ 串内全部换行（含粘在题干后的）
// 3) 单题号粘连（整风27、1936）
func splitGluedQuizSegs(segs []seg) []seg {
	if len(segs) == 0 {
		return segs
	}
	flat := make([]seg, 0, len(segs)+8)
	for _, s := range segs {
		parts := splitGluedQuizText(s.text)
		for i, p := range parts {
			ns := s
			ns.text = p
			ns.series, ns.num = seriesOf(p)
			if i > 0 || isOptionLine(p) {
				ns.kind = "para"
				ns.lvl = 0
			}
			if isOptionLine(p) {
				ns.series, ns.num = "", 0
			}
			flat = append(flat, ns)
		}
	}
	return splitAcrossConfirmedRuns(flat)
}

func splitGluedQuizText(text string) []string {
	s := strings.TrimSpace(text)
	if s == "" {
		return []string{text}
	}
	for round := 0; round < 8; round++ {
		n := reQGlueDun.ReplaceAllString(s, "$1\n$2")
		n = reQGlueDot.ReplaceAllString(n, "$1\n$2")
		if n == s {
			break
		}
		s = n
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, splitByRepeatedOrdinals(line)...)
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(text)}
	}
	return out
}

// splitByRepeatedOrdinals：同行内同类序号 ≥2 才切开（不因单个 A、 误切）。
func splitByRepeatedOrdinals(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	marks := findOrdinalMarks(s)
	if len(marks) == 0 {
		return []string{s}
	}
	count := map[string]int{}
	for _, m := range marks {
		count[m.kind]++
	}
	var cuts []ordMark
	for _, m := range marks {
		if count[m.kind] >= 2 {
			cuts = append(cuts, m)
		}
	}
	return cutAtMarks(s, cuts)
}

// splitAcrossConfirmedRuns：跨段找连续序号（A→B→C… / 1→2→…）；串长 ≥2 则全部切开。
func splitAcrossConfirmedRuns(segs []seg) []seg {
	hits := collectOrdHits(segs)
	confirmed := confirmedHitKeys(hits)
	if len(confirmed) == 0 {
		return segs
	}
	out := make([]seg, 0, len(segs)+8)
	for i, s := range segs {
		marks := findOrdinalMarks(s.text)
		var cuts []ordMark
		for _, m := range marks {
			key := hitKey{i, m.start, m.kind, m.num}
			if confirmed[key] {
				cuts = append(cuts, m)
			}
		}
		parts := cutAtMarks(s.text, cuts)
		for j, p := range parts {
			ns := s
			ns.text = p
			ns.series, ns.num = seriesOf(p)
			if j > 0 || isOptionLine(p) {
				ns.kind = "para"
				ns.lvl = 0
			}
			if isOptionLine(p) {
				ns.series, ns.num = "", 0
			}
			out = append(out, ns)
		}
	}
	return out
}

type hitKey struct {
	seg, start int
	kind       string
	num        int
}

func collectOrdHits(segs []seg) []ordHit {
	var hits []ordHit
	for i, s := range segs {
		for _, m := range findOrdinalMarks(s.text) {
			hits = append(hits, ordHit{
				segIdx: i, start: m.start, end: m.end,
				kind: m.kind, num: m.num,
			})
		}
	}
	return hits
}

func confirmedHitKeys(hits []ordHit) map[hitKey]bool {
	out := map[hitKey]bool{}
	for i := 0; i < len(hits); {
		j := i + 1
		for j < len(hits) &&
			hits[j].kind == hits[i].kind &&
			hits[j].num == hits[j-1].num+1 {
			j++
		}
		if j-i >= 2 {
			for _, h := range hits[i:j] {
				out[hitKey{h.segIdx, h.start, h.kind, h.num}] = true
			}
			i = j
			continue
		}
		i++
	}
	return out
}

func cutAtMarks(s string, cuts []ordMark) []string {
	if len(cuts) == 0 {
		return []string{s}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
	// unique starts
	uniq := cuts[:0]
	last := -1
	for _, c := range cuts {
		if c.start == last {
			continue
		}
		uniq = append(uniq, c)
		last = c.start
	}
	cuts = uniq

	var parts []string
	if cuts[0].start > 0 {
		pre := strings.TrimSpace(s[:cuts[0].start])
		if pre != "" {
			parts = append(parts, pre)
		}
	}
	for i, c := range cuts {
		end := len(s)
		if i+1 < len(cuts) {
			end = cuts[i+1].start
		}
		chunk := strings.TrimSpace(s[c.start:end])
		if chunk != "" {
			parts = append(parts, chunk)
		}
	}
	if len(parts) == 0 {
		return []string{s}
	}
	return parts
}

func findOrdinalMarks(s string) []ordMark {
	var raw []ordMark
	add := func(re *regexp.Regexp, kind string) {
		for _, loc := range re.FindAllStringIndex(s, -1) {
			raw = append(raw, ordMark{
				start: loc[0], end: loc[1], kind: kind,
				num: ordinalNum(kind, s[loc[0]:loc[1]]),
			})
		}
	}
	add(reLetterOrd, "letter")
	add(reArabDun, "arab_dun")
	add(reParenOrd, "paren")
	add(reCnDun, "cn_dun")
	for _, loc := range reArabDot.FindAllStringIndex(s, -1) {
		if loc[1] < len(s) && s[loc[1]] >= '0' && s[loc[1]] <= '9' {
			continue
		}
		raw = append(raw, ordMark{
			start: loc[0], end: loc[1], kind: "arab_dot",
			num: ordinalNum("arab_dot", s[loc[0]:loc[1]]),
		})
	}
	if len(raw) == 0 {
		return nil
	}
	sort.Slice(raw, func(i, j int) bool {
		if raw[i].start != raw[j].start {
			return raw[i].start < raw[j].start
		}
		return raw[i].end > raw[j].end
	})
	out := make([]ordMark, 0, len(raw))
	lastEnd := 0
	for _, m := range raw {
		if m.start < lastEnd || m.num <= 0 {
			continue
		}
		if m.kind == "cn_dun" && m.start > 0 {
			prev, _ := utf8.DecodeLastRuneInString(s[:m.start])
			if prev >= 0x4e00 && prev <= 0x9fff {
				continue
			}
		}
		out = append(out, m)
		lastEnd = m.end
	}
	return out
}

func ordinalNum(kind, tok string) int {
	tok = strings.TrimSpace(tok)
	switch kind {
	case "letter":
		r, _ := utf8.DecodeRuneInString(tok)
		return letterOrdNum(r)
	case "arab_dun", "arab_dot":
		n := 0
		for _, r := range tok {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		return n
	case "paren":
		inner := strings.TrimPrefix(tok, "（")
		inner = strings.TrimPrefix(inner, "(")
		inner = strings.TrimSuffix(inner, "）")
		inner = strings.TrimSuffix(inner, ")")
		if inner == "" {
			return 0
		}
		if inner[0] >= '0' && inner[0] <= '9' {
			return ordinalNum("arab_dun", inner)
		}
		return cnOrdNum([]rune(inner))
	case "cn_dun":
		r := []rune(tok)
		if len(r) > 0 && r[len(r)-1] == '、' {
			r = r[:len(r)-1]
		}
		return cnOrdNum(r)
	default:
		return 0
	}
}

func letterOrdNum(r rune) int {
	switch {
	case r >= 'A' && r <= 'Z':
		return int(r - 'A' + 1)
	case r >= 'a' && r <= 'z':
		return int(r - 'a' + 1)
	case r >= 'Ａ' && r <= 'Ｚ':
		return int(r - 'Ａ' + 1)
	case r >= 'ａ' && r <= 'ｚ':
		return int(r - 'ａ' + 1)
	default:
		return 0
	}
}

func cnOrdNum(r []rune) int {
	if len(r) == 0 {
		return 0
	}
	cn := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
	}
	if len(r) == 1 {
		return cn[r[0]]
	}
	if r[0] == '十' {
		if len(r) == 1 {
			return 10
		}
		return 10 + cn[r[1]]
	}
	if len(r) == 2 && r[1] == '十' {
		return cn[r[0]] * 10
	}
	if len(r) == 3 && r[1] == '十' {
		return cn[r[0]]*10 + cn[r[2]]
	}
	return cn[r[0]]
}
