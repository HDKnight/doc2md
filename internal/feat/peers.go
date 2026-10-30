package feat

import (
	"strings"
)

// seriesOf classifies a line into a numbered peer series.
// kind examples: arab_dun (1、), arab_dot (1.), cn_dun (一、), paren ((1)/(一))
// 行首有题号时，即使含「。」或较长也识别（便于拆段后的标题对齐）。
func seriesOf(text string) (kind string, num int) {
	t := strings.TrimSpace(text)
	if t == "" {
		return "", 0
	}
	r := []rune(t)

	// (1) / （1） / (一) / （一）——含 OCR 混用括号
	if r[0] == '(' || r[0] == '（' {
		j := 1
		n, ok := parseArabOrCN(r, &j)
		if ok && j < len(r) && (r[j] == ')' || r[j] == '）') {
			return "paren", n
		}
	}

	// 1、 / 1. / 12、
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i > 0 && i <= 3 && i < len(r) {
		if r[i] == '、' && isLikelyNumberRangeAfterDun(r, i) {
			return "", 0 // 16、17、… 连写区间
		}
		switch r[i] {
		case '、':
			return "arab_dun", atoiRunes(r[:i])
		case '.', '．':
			return "arab_dot", atoiRunes(r[:i])
		}
	}

	// 一、 二、
	cn := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
	}
	if n, ok := cn[r[0]]; ok && len(r) > 1 {
		switch r[1] {
		case '、', '.', '．':
			return "cn_dun", n
		}
	}
	return "", 0
}

func parseArabOrCN(r []rune, j *int) (int, bool) {
	if *j >= len(r) {
		return 0, false
	}
	if r[*j] >= '0' && r[*j] <= '9' {
		start := *j
		for *j < len(r) && r[*j] >= '0' && r[*j] <= '9' {
			*j++
		}
		return atoiRunes(r[start:*j]), true
	}
	cn := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
	}
	if n, ok := cn[r[*j]]; ok {
		*j++
		return n, true
	}
	return 0, false
}

func atoiRunes(r []rune) int {
	n := 0
	for _, c := range r {
		n = n*10 + int(c-'0')
	}
	return n
}

// splitLongSeriesStems：同型题号（1、/1.）若与长正文粘在同一块，拆成「短题干标题 + 正文」，
// 以便与短题干行走同一套 peer 对齐（避免 10、整段正文 vs 17、短标题 不一致）。
func splitLongSeriesStems(segs []seg) []seg {
	if len(segs) == 0 {
		return segs
	}
	out := make([]seg, 0, len(segs)+4)
	for _, s := range segs {
		head, body, ok := splitSeriesStemBody(s.text)
		if !ok {
			out = append(out, s)
			continue
		}
		h := s
		h.text = head
		h.kind = "heading"
		h.lvl = 0 // 层级由 normalizeHeadingLevels 统一分配
		h.series, h.num = seriesOf(head)
		out = append(out, h)

		b := s
		b.text = body
		b.kind = "para"
		b.lvl = 0
		b.series, b.num = "", 0
		b.typ = "text"
		out = append(out, b)
	}
	return out
}

func splitSeriesStemBody(text string) (head, body string, ok bool) {
	t := strings.TrimSpace(text)
	ser, _ := seriesOf(t)
	if ser != "arab_dun" && ser != "arab_dot" {
		return "", "", false
	}
	r := []rune(t)
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(r) {
		return "", "", false
	}
	switch r[i] {
	case '、', '.', '．':
		i++
	default:
		return "", "", false
	}
	// 只在「问号/冒号/分号」处切开题干与后文；绝不在「。」处拆——
	// 选择题干常含「……方法。它是指（A）」整句原文，按句号切开会误伤。
	cut := -1
	for _, sep := range []rune{'？', '?', '：', ':', '；', ';'} {
		for j := i; j < len(r); j++ {
			if r[j] != sep {
				continue
			}
			cand := j + 1
			if cand < len(r) {
				cut = cand
				break
			}
		}
		if cut >= 0 {
			break
		}
	}
	if cut < 0 {
		return "", "", false
	}
	head = strings.TrimSpace(string(r[:cut]))
	body = strings.TrimSpace(string(r[cut:]))
	if head == "" || body == "" {
		return "", "", false
	}
	return head, body, true
}

// alignPeerHeadings：同型连续编号应对齐为同级；漏抬的中间项补抬。
// 例如已有 ##1、 与 ##3、，则中间「2、」正文抬为同级；级别不齐则取该串目标级。
func alignPeerHeadings(segs []seg) {
	if len(segs) == 0 {
		return
	}
	// 刷新 series（软拼后可能丢过）
	for i := range segs {
		if segs[i].series == "" {
			segs[i].series, segs[i].num = seriesOf(segs[i].text)
		}
	}

	// 按 series 收集候选（已是标题，或可补抬的同型题号行）
	byKind := map[string][]int{}
	for i, s := range segs {
		if s.series == "" || s.num <= 0 {
			continue
		}
		if isOptionLine(s.text) {
			continue
		}
		if s.kind != "heading" && !peerCandidate(s) {
			continue
		}
		byKind[s.series] = append(byKind[s.series], i)
	}

	for kind, idxs := range byKind {
		if len(idxs) < 2 {
			continue
		}
		runs := splitPeerRuns(segs, idxs)
		for _, run := range runs {
			if len(run) < 2 {
				continue
			}
			alignOneRun(segs, run, kind)
		}
	}
}

func peerCandidate(s seg) bool {
	if s.kind == "heading" {
		return true
	}
	if strings.TrimSpace(s.text) == "" || s.series == "" {
		return false
	}
	// 阿拉伯题号：不按字数过滤；其它系列排除「句号论述句」
	if s.series == "arab_dun" || s.series == "arab_dot" {
		return true
	}
	if strings.Contains(s.text, "。") && !hasQuestionMark(s.text) {
		return false
	}
	return true
}

func splitPeerRuns(segs []seg, idxs []int) [][]int {
	if len(idxs) == 0 {
		return nil
	}
	var runs [][]int
	cur := []int{idxs[0]}
	for k := 1; k < len(idxs); k++ {
		prev, now := idxs[k-1], idxs[k]
		dn := segs[now].num - segs[prev].num
		// 编号递增（允许漏 1 号）；中间可夹其它系列标题/正文
		contiguous := dn >= 1 && dn <= 2
		// 编号回绕或倒退 → 新开一串
		restart := segs[now].num <= segs[prev].num
		if contiguous && !restart {
			cur = append(cur, now)
			continue
		}
		runs = append(runs, cur)
		cur = []int{now}
	}
	runs = append(runs, cur)
	return runs
}

func alignOneRun(segs []seg, run []int, kind string) {
	// 阶段 A：只补抬为标题；层级留给 normalizeHeadingLevels
	lo, hi := run[0], run[len(run)-1]
	inRun := map[int]bool{}
	for _, i := range run {
		inRun[i] = true
	}
	for i := lo; i <= hi; i++ {
		if inRun[i] {
			continue
		}
		if segs[i].series != kind || segs[i].num <= 0 {
			continue
		}
		if !peerCandidate(segs[i]) {
			continue
		}
		if segs[i].num < segs[lo].num || segs[i].num > segs[hi].num {
			continue
		}
		run = append(run, i)
		inRun[i] = true
	}

	for _, i := range run {
		segs[i].kind = "heading"
		segs[i].lvl = 0
	}
}
