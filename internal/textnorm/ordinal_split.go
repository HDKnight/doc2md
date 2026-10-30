package textnorm

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	reLetterOrd = regexp.MustCompile(`[A-Za-zＡ-Ｚａ-ｚ][、.．,，]`)
	reArabDunM  = regexp.MustCompile(`\d{1,3}、`)
	reArabDotM  = regexp.MustCompile(`\d{1,3}[.．]`)
	reParenOrd  = regexp.MustCompile(`[（(](?:\d{1,3}|[一二三四五六七八九十]+)[）)]`)
	reCnDunM    = regexp.MustCompile(`[一二三四五六七八九十百千]+、`)
)

type ordMark struct {
	start, end int
	kind       string
	num        int
}

type ordHit struct {
	lineIdx    int
	start, end int
	kind       string
	num        int
}

type hitKey struct {
	line, start int
	kind        string
	num         int
}

// splitByRepeatedOrdinals：同行同类 ≥2 切开。
func splitByRepeatedOrdinals(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	marks := findOrdinalMarks(s)
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

// splitAcrossConfirmedOrdinalLines：跨行连续序号（A→B…）≥2 则全部换行切开。
func splitAcrossConfirmedOrdinalLines(lines []string) []string {
	// 先丢掉纯空行参与命中，但保留输出空行结构较麻烦；对非空行做 hit
	type idxLine struct {
		orig int
		text string
	}
	var nonEmpty []idxLine
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		nonEmpty = append(nonEmpty, idxLine{i, t})
	}
	texts := make([]string, len(nonEmpty))
	for i, x := range nonEmpty {
		texts[i] = x.text
	}
	hits := collectLineHits(texts)
	confirmed := confirmedLineHits(hits)
	if len(confirmed) == 0 {
		return lines
	}

	// 对每个非空行切开，重建 lines（空行原样穿插：按 orig 下标重写）
	splitAt := map[int][]string{} // orig line idx → parts
	for i, x := range nonEmpty {
		// 标题行：在去掉 # 前缀后的正文上切，避免把 `### 1、…` 切成空 `###` + 正文
		prefix, body := mdHeadingPrefix(x.text)
		var cuts []ordMark
		for _, m := range findOrdinalMarks(x.text) {
			if !confirmed[hitKey{i, m.start, m.kind, m.num}] {
				continue
			}
			if prefix != "" && m.start < len(prefix) {
				continue
			}
			if prefix != "" {
				m.start -= len(prefix)
				m.end -= len(prefix)
			}
			cuts = append(cuts, m)
		}
		parts := cutAtMarks(body, cuts)
		if prefix != "" && len(parts) > 0 {
			parts[0] = prefix + parts[0]
		}
		if len(parts) > 1 || (len(parts) == 1 && parts[0] != x.text) {
			splitAt[x.orig] = parts
		}
	}
	if len(splitAt) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines)+8)
	for i, ln := range lines {
		parts, ok := splitAt[i]
		if !ok {
			out = append(out, ln)
			continue
		}
		for j, p := range parts {
			if j > 0 {
				out = append(out, "")
			}
			out = append(out, p)
		}
	}
	return out
}

func collectLineHits(lines []string) []ordHit {
	var hits []ordHit
	for i, s := range lines {
		for _, m := range findOrdinalMarks(s) {
			hits = append(hits, ordHit{i, m.start, m.end, m.kind, m.num})
		}
	}
	return hits
}

func confirmedLineHits(hits []ordHit) map[hitKey]bool {
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
				out[hitKey{h.lineIdx, h.start, h.kind, h.num}] = true
			}
			i = j
			continue
		}
		i++
	}
	return out
}

// mdHeadingPrefix 拆出 CommonMark AT前缀（如 "### "），正文不含前缀。
func mdHeadingPrefix(s string) (prefix, body string) {
	if m := reHeadPrefix.FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[1]):]
	}
	return "", s
}

func cutAtMarks(s string, cuts []ordMark) []string {
	if len(cuts) == 0 {
		return []string{s}
	}
	sort.Slice(cuts, func(i, j int) bool { return cuts[i].start < cuts[j].start })
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
	add(reArabDunM, "arab_dun")
	add(reParenOrd, "paren")
	add(reCnDunM, "cn_dun")
	for _, loc := range reArabDotM.FindAllStringIndex(s, -1) {
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
