package pipeline

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// joinHeadingFragments：对齐旧 textnorm.headingLineContinues ——
// 标题已抬出后，半截折行（…是党 / 在军队中…）按文本结构拼回，不依赖 bbox 间距。
func joinHeadingFragments(segs []seg) []seg {
	if len(segs) < 2 {
		return segs
	}
	out := make([]seg, 0, len(segs))
	for i := 0; i < len(segs); i++ {
		s := segs[i]
		for i+1 < len(segs) && s.kind == "heading" && headingContinues(s, segs[i+1]) {
			nxt := segs[i+1]
			s.text = joinSoft(s.text, nxt.text)
			s.y1 = nxt.y1
			s.w = nxt.w
			if nxt.em > s.em {
				s.em = nxt.em
			}
			i++
		}
		out = append(out, s)
	}
	out = demoteExpandedHeadings(out)
	return joinUnheraldedParas(out)
}

// demoteExpandedHeadings：半截序号折行拼完后若已成多句论述 → 降为正文。
// 避免页末「一、…」先抬标题，再跨页吞掉整段「八个明确」。
func demoteExpandedHeadings(segs []seg) []seg {
	for i := range segs {
		if segs[i].kind != "heading" {
			continue
		}
		if expandedProseBlock(segs[i].text) {
			segs[i].kind = "para"
			segs[i].lvl = 0
		}
	}
	return segs
}

// joinUnheraldedParas：同一序号辖域内，无新段征兆的中间断行一律续拼（可跨页）。
// 有征兆（新序号/选项/第一，/OCR title）则硬断；完整标题不吞正文。
func joinUnheraldedParas(segs []seg) []seg {
	if len(segs) < 2 {
		return segs
	}
	out := make([]seg, 0, len(segs))
	for i := 0; i < len(segs); i++ {
		s := segs[i]
		for i+1 < len(segs) && canJoinUnheralded(s, segs[i+1]) {
			nxt := segs[i+1]
			s.text = joinSoft(s.text, nxt.text)
			s.y1 = nxt.y1
			s.w = nxt.w
			if nxt.em > s.em {
				s.em = nxt.em
			}
			i++
		}
		out = append(out, s)
	}
	return out
}

func canJoinUnheralded(prev, next seg) bool {
	if next.kind == "heading" || typeIsTitle(next.typ) {
		return false
	}
	if looksNewBlock(next.text) {
		return false
	}
	// 上行已句终（。！？）→ 合法段界，即使下行无序号也不拼
	// （只拼半截折行：如「新时代党」+「的建设」）
	if endsHard(prev.text) && !needsContinuation(prev.text) {
		return false
	}
	// 完整标题 / OCR 短标题不吞正文
	if prev.kind == "heading" && !needsContinuation(prev.text) {
		return false
	}
	if typeIsTitle(prev.typ) && !needsContinuation(prev.text) {
		return false
	}
	return true
}

func headingContinues(head, next seg) bool {
	if next.kind == "heading" {
		return false
	}
	body := strings.TrimSpace(head.text)
	nxt := strings.TrimSpace(next.text)
	if body == "" || nxt == "" {
		return false
	}
	if looksNewBlock(nxt) {
		return false
	}
	// 置信 OCR 标题：完整 title 不吞下行正文（半截折行仍续拼）
	if typeIsTitle(head.typ) && !needsContinuation(body) {
		return false
	}
	// 短标签标题不吞正文
	if isShortLabelOnly(body) {
		return false
	}
	if !headingHasProse(body) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(body, " \t"))
	if strings.ContainsRune("。！？!?；;…", r) {
		return false
	}
	nr, _ := utf8.DecodeRuneInString(nxt)
	if strings.ContainsRune("，,、", r) {
		return unicode.IsNumber(nr)
	}
	if !(unicode.Is(unicode.Han, nr) || unicode.IsLetter(nr) || unicode.IsNumber(nr)) {
		switch nr {
		case '“', '"', '《', '「':
		default:
			return false
		}
	}
	return unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsNumber(r) ||
		strings.ContainsRune("的一了是不和与及以把被在为有其并从到按向对给让使", r)
}

func headingHasProse(body string) bool {
	// 去掉序号后再看
	rest := markerBody(body)
	if rest == "" {
		rest = body
	}
	for _, c := range []string{"：", ":", "，", ",", "、", "；", ";", "“", "\"", "《", "（", "("} {
		if i := strings.Index(rest, c); i >= 0 {
			if strings.TrimSpace(rest[i+len(c):]) != "" {
				return true
			}
		}
	}
	return false
}

func isShortLabelOnly(body string) bool {
	rest := strings.TrimSpace(markerBody(body))
	if rest == "" {
		return true
	}
	// 仅短标签+冒号，无展开论述
	rest = strings.TrimSuffix(strings.TrimSuffix(rest, "："), ":")
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return true
	}
	if strings.ContainsAny(rest, "。！？；，,") {
		return false
	}
	return false
}
