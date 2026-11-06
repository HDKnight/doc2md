package pipeline

import (
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

// colEdge is the page's text-column right edge.
// ok is false when no two lines arrive at the same right edge.
type colEdge struct {
	x   float64
	tol float64
	ok  bool
}

func softWrapItems(items []item, edge colEdge) []item {
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
		if !wrapJoin(*prev, it, edge) {
			out = append(out, it)
			continue
		}
		mergeItem(prev, it)
	}
	return out
}

// columnRight is where body lines share a right edge.
// A lone longest line does not define the column.
func columnRight(items []item) colEdge {
	var xs, hs []float64
	for _, it := range items {
		h := it.y1 - it.y0
		if h < 1 || it.w < 1 {
			continue
		}
		xs = append(xs, it.x0+it.w)
		hs = append(hs, h)
	}
	if len(xs) < 2 {
		return colEdge{}
	}
	tol := medianFloat(hs)
	if tol < 1 {
		tol = 12
	}
	sort.Float64s(xs)
	// 只认最靠右的那一组。更短的一簇短行不能充当栏右缘。
	if xs[len(xs)-1]-xs[len(xs)-2] <= tol {
		return colEdge{x: xs[len(xs)-1], tol: tol, ok: true}
	}
	return colEdge{tol: tol}
}

// wrapJoin is true only for a layout wrap:
// the previous line reaches the column edge, the next line is the immediate
// next line (or the same baseline split by extraction), and the previous
// line does not already end a sentence. Same-baseline fragments are one line.
func wrapJoin(prev, next item, edge colEdge) bool {
	if !hasLineBox(prev) || !hasLineBox(next) {
		return false
	}
	// 下一行已是新题号或新选项：宁可放过，也不把题干接到上一行。
	if joinsAcrossItem(prev, next) {
		return false
	}
	if sameBaseline(prev, next) {
		return true
	}
	if !reachesColumn(prev, edge) || endsHard(prev.text) {
		return false
	}
	return tightVertical(prev, next)
}

// wrapAcrossPage joins a full line cut by a page break onto the next page's first line.
func wrapAcrossPage(prev, next item, edge colEdge) bool {
	if next.page != prev.page+1 || !hasLineBox(prev) || !hasLineBox(next) {
		return false
	}
	if joinsAcrossItem(prev, next) {
		return false
	}
	if !reachesColumn(prev, edge) || endsHard(prev.text) {
		return false
	}
	lh := prev.y1 - prev.y0
	return math.Abs(next.x0-prev.x0) <= lh*2
}

// joinsAcrossItem：下一行自己就是题号、选项，或选项右侧多出来的纯数字。
func joinsAcrossItem(prev, next item) bool {
	if freshStart(next.text) {
		return true
	}
	return isOptionLine(prev.text) && bareNumber(next.text)
}

func freshStart(text string) bool {
	t := strings.TrimSpace(text)
	if isOptionLine(t) {
		return true
	}
	kind, _ := seriesOf(t)
	return kind != ""
}

func bareNumber(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	n := 0
	for _, r := range t {
		if !isDigit(r) {
			return false
		}
		n++
		if n > 3 {
			return false
		}
	}
	return true
}

func hasLineBox(it item) bool {
	return it.y1 > it.y0 && it.w > 0
}

func reachesColumn(it item, edge colEdge) bool {
	if !edge.ok {
		return false
	}
	return edge.x-(it.x0+it.w) <= edge.tol
}

func sameBaseline(prev, next item) bool {
	lh := prev.y1 - prev.y0
	if lh < 1 || math.Abs(next.y0-prev.y0) > lh*0.35 {
		return false
	}
	hgap := next.x0 - (prev.x0 + prev.w)
	return hgap >= -lh*0.3 && hgap <= lh*2.5
}

func tightVertical(prev, next item) bool {
	lh := prev.y1 - prev.y0
	if lh < 1 {
		return false
	}
	gap := next.y0 - prev.y1
	return gap >= -lh*0.2 && gap <= lh*0.45
}

func mergeItem(dst *item, src item) {
	same := sameBaseline(*dst, src)
	dst.text = joinSoft(dst.text, src.text)
	if same {
		right := dst.x0 + dst.w
		if src.x0+src.w > right {
			right = src.x0 + src.w
		}
		dst.w = right - dst.x0
		if src.y1 > dst.y1 {
			dst.y1 = src.y1
		}
	} else {
		// 下一次判断看最后这一行自己的行框，不沿用上面满行的右缘。
		dst.x0 = src.x0
		dst.w = src.w
		dst.y0 = src.y0
		dst.y1 = src.y1
	}
	if src.em > dst.em {
		dst.em = src.em
	}
}

func endsHard(text string) bool {
	t := strings.TrimSpace(text)
	for {
		if t == "" {
			return true
		}
		r, size := utf8.DecodeLastRuneInString(t)
		// 句号后面的收引号不另起一句，跳过去再看。
		// 引号前不是句末标点时仍视为折行，例如“批评”后接“，说服”。
		if isClosingQuote(r) {
			t = strings.TrimSpace(t[:len(t)-size])
			continue
		}
		switch r {
		case '。', '！', '？', '!', '?', '；', ';', '…':
			return true
		}
		return false
	}
}

func isClosingQuote(r rune) bool {
	switch r {
	case '"', '\'', '”', '’', '」', '』', '＂':
		return true
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
