package pipeline

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

func softWrapItems(items []item, bodyEm float64) []item {
	if len(items) == 0 {
		return nil
	}
	out := make([]item, 0, len(items))
	for _, it := range items {
		if len(out) == 0 {
			out = append(out, it)
			continue
		}
		prev := &out[len(out)-1]
		// 置信 OCR 标题：title 块不与邻行 soft-wrap（正文另段）
		if typeIsTitle(prev.typ) || typeIsTitle(it.typ) {
			out = append(out, it)
			continue
		}
		// 下一行是新序号/选项 → 绝不拼
		if looksNewBlock(it.text) {
			out = append(out, it)
			continue
		}
		gap := it.y0 - prev.y1
		em := math.Max(prev.em, it.em)
		if em < 1 {
			em = bodyEm
		}
		if em < 1 {
			em = 12
		}
		cont := needsContinuation(prev.text)
		stemCont := cont && (isArabicStem(prev.text) || isStructuralLabel(prev.text))
		// 无征兆中间断行：放宽几何，同一序号辖域内尽量拼
		unheralded := !endsHard(prev.text) || cont
		gapMax := em * 1.2
		indentMax := em * 0.9
		if stemCont || unheralded {
			gapMax = em * 2.8
			indentMax = em * 1.6
		}
		sameIndent := math.Abs(it.x0-prev.x0) < indentMax
		near := gap >= -em*0.3 && gap < gapMax
		if !near || !sameIndent {
			out = append(out, it)
			continue
		}
		// 完整序号行（不像半截）不与下行正文粘
		if (isArabicStem(prev.text) || isStructuralLabel(prev.text)) && !cont {
			out = append(out, it)
			continue
		}
		// 普通行以句号硬收尾且非半截 → 不拼
		if endsHard(prev.text) && !cont {
			out = append(out, it)
			continue
		}
		prev.text = joinSoft(prev.text, it.text)
		prev.y1 = it.y1
		prev.w = it.w
		if it.em > prev.em {
			prev.em = it.em
		}
	}
	return out
}

func needsContinuation(text string) bool {
	t := strings.TrimRight(strings.TrimSpace(text), " \t")
	if t == "" {
		return false
	}
	if unbalancedParen(t) {
		return true
	}
	r, size := utf8.DecodeLastRuneInString(t)
	const soft = "，,、"
	const openTail = "的一了是不和与及以把被在为有其并从到按向对给让使"
	if strings.ContainsRune(soft, r) || strings.ContainsRune(openTail, r) {
		return true
	}
	rest := strings.TrimRight(t[:len(t)-size], " \t")
	if rest == "" {
		return false
	}
	r2, _ := utf8.DecodeLastRuneInString(rest)
	return strings.ContainsRune(openTail, r2) && unicode.Is(unicode.Han, r)
}

func unbalancedParen(s string) bool {
	bal := 0
	for _, r := range s {
		switch r {
		case '（', '(':
			bal++
		case '）', ')':
			bal--
		}
	}
	return bal > 0
}

func endsHard(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(t)
	switch r {
	case '。', '！', '？', '!', '?', '；', ';', '…':
		return true
	}
	return false
}

func looksNewBlock(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	if isOptionLine(t) || isArabicStem(t) || isStructuralLabel(t) {
		return true
	}
	if strings.HasPrefix(t, "答：") || strings.HasPrefix(t, "答:") {
		return true
	}
	// 有征兆的新段：第一，/首先/第X…
	if looksEnumeratedClause(t) {
		return true
	}
	return false
}

// looksEnumeratedClause：「第一，」「首先」等枚举起句（同一序号辖域内的真·新段征兆）。
func looksEnumeratedClause(text string) bool {
	t := strings.TrimSpace(text)
	prefixes := []string{
		"第一，", "第二，", "第三，", "第四，", "第五，",
		"第一、", "第二、", "第三、",
		"首先，", "其次，", "再次，", "最后，", "总之，", "综上，",
		"首先、", "其次、", "再次、", "最后、",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	r := []rune(t)
	if len(r) >= 2 && r[0] == '第' {
		// 第X章/节/部分 / 第一段…；排除「第一次明确」孤立词——用后面是否像序数结构
		if strings.HasPrefix(t, "第") && (strings.Contains(t, "章") || strings.Contains(t, "节") ||
			strings.Contains(t, "部分") || strings.Contains(t, "条")) {
			return true
		}
	}
	return false
}

func joinSoft(a, b string) string {
	a = strings.TrimRight(a, " \t")
	b = strings.TrimLeft(b, " \t")
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + b
}

// splitGluedOptions：只切开同行粘连的 A、xxB、yy；不拆题干句号。
func splitGluedOptions(segs []seg) []seg {
	out := make([]seg, 0, len(segs)+4)
	for _, s := range segs {
		parts := splitOptionGlues(s.text)
		if len(parts) <= 1 {
			out = append(out, s)
			continue
		}
		for _, p := range parts {
			ns := s
			ns.text = p
			ns.series, ns.num = seriesOf(p)
			if isOptionLine(p) {
				ns.kind = "para"
				ns.lvl = 0
			}
			out = append(out, ns)
		}
	}
	return out
}

func splitOptionGlues(text string) []string {
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	// 反复在「选项标记」前切开
	var cuts []int
	r := []rune(t)
	for i := 1; i < len(r)-1; i++ {
		if !isOptLetter(r[i]) {
			continue
		}
		sep := r[i+1]
		if sep != '、' && sep != '.' && sep != '．' {
			continue
		}
		// 前一字符非字母开头的选项串中部
		prev := r[i-1]
		if isOptLetter(prev) {
			continue
		}
		cuts = append(cuts, i)
	}
	if len(cuts) == 0 {
		return []string{t}
	}
	var parts []string
	start := 0
	for _, c := range cuts {
		chunk := strings.TrimSpace(string(r[start:c]))
		if chunk != "" {
			parts = append(parts, chunk)
		}
		start = c
	}
	tail := strings.TrimSpace(string(r[start:]))
	if tail != "" {
		parts = append(parts, tail)
	}
	if len(parts) < 2 {
		return []string{t}
	}
	return parts
}
