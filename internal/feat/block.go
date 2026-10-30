// Package feat：OCR 表征 JSON → Markdown。三阶段：断行 → 标题识别 → 层级。
// 与 textnorm/geom 独立；版式 type/字号仅作标题信号，不用字数定义标题。
package feat

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Block is one structure region from oar structure --format json.
type Block struct {
	Type    string    `json:"type"`
	Content string    `json:"content"`
	Order   int       `json:"order"`
	BBox    []float64 `json:"bbox"` // [x0,y0,x1,y1]
}

// Page is one page of structure blocks.
type Page struct {
	Blocks []Block
}

type pageJSON struct {
	Blocks []Block `json:"blocks"`
}

// ParsePageJSON parses one oar structure JSON page file.
func ParsePageJSON(raw []byte) (Page, error) {
	var p pageJSON
	if err := json.Unmarshal(raw, &p); err != nil {
		return Page{}, err
	}
	return Page{Blocks: p.Blocks}, nil
}

func (b Block) X0() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[0]
}
func (b Block) Y0() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[1]
}
func (b Block) X1() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[2]
}
func (b Block) Y1() float64 {
	if len(b.BBox) < 4 {
		return 0
	}
	return b.BBox[3]
}
func (b Block) Height() float64 {
	h := b.Y1() - b.Y0()
	if h < 1 {
		return 1
	}
	return h
}
func (b Block) Width() float64 {
	w := b.X1() - b.X0()
	if w < 1 {
		return 1
	}
	return w
}

type item struct {
	text string
	typ  string // oar type, hint only
	em   float64
	boxH float64
	w    float64
	x0   float64
	y0   float64
	y1   float64
	page int
}

type seg struct {
	kind   string // heading | para
	lvl    int
	text   string
	em     float64
	w      float64 // bbox 宽度，用于「是否抵满行宽」
	x0     float64
	y0     float64
	y1     float64
	series string // peer series key, e.g. arab_dun
	num    int    // series ordinal if any
	typ    string
}

// MarkdownFromPages turns structure JSON pages into Markdown.
func MarkdownFromPages(pages []Page) string {
	var all []item
	for pi, p := range pages {
		pageItems := pageItems(p.Blocks, pi+1)
		all = append(all, pageItems...)
	}
	if len(all) == 0 {
		return ""
	}

	// ① 断行：页内 soft-wrap → 粘连切开 → 断章合并
	var segs []seg
	pageStart := 0
	for pageStart < len(all) {
		page := all[pageStart].page
		end := pageStart + 1
		for end < len(all) && all[end].page == page {
			end++
		}
		segs = append(segs, buildPageSegs(all[pageStart:end])...)
		pageStart = end
	}
	segs = splitGluedQuizSegs(segs)
	segs = splitEmbeddedParenMarkers(segs)
	segs = splitLongSeriesStems(segs)
	segs = mergeBrokenParas(segs)

	// ② 标题识别：结构修正（后接选项降正文、同族补抬）；不用字数定义标题
	alignPeerHeadings(segs)

	// ③ 层级：同族对齐后的栈/防跳级
	normalizeHeadingLevels(segs)

	var parts []string
	for _, s := range segs {
		t := strings.TrimSpace(s.text)
		if t == "" {
			continue
		}
		if s.kind == "heading" {
			lvl := s.lvl
			if lvl < 1 {
				lvl = 2
			}
			if lvl > 6 {
				lvl = 6
			}
			parts = append(parts, strings.Repeat("#", lvl)+" "+t)
			continue
		}
		parts = append(parts, t)
	}
	md := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if md == "" {
		return ""
	}
	return md + "\n"
}

func pageItems(blocks []Block, page int) []item {
	items := make([]item, 0, len(blocks))
	for _, b := range blocks {
		t := strings.TrimSpace(b.Content)
		if t == "" {
			continue
		}
		h, w := b.Height(), b.Width()
		items = append(items, item{
			text: t, typ: strings.ToLower(strings.TrimSpace(b.Type)),
			em: estimateLineEm(h, w, t), boxH: h, w: w,
			x0: b.X0(), y0: b.Y0(), y1: b.Y1(), page: page,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y0 == items[j].y0 {
			return items[i].x0 < items[j].x0
		}
		return items[i].y0 < items[j].y0
	})
	return items
}

func buildPageSegs(items []item) []seg {
	if len(items) == 0 {
		return nil
	}
	bodyEm := medianBodyEm(items)
	medW := medianWidth(items)

	// ① 断行优先：几何 soft-wrap，尚未定标题（避免半截题干被标成 heading 后无法拼接）
	items = softWrapItems(items, bodyEm)

	out := make([]seg, 0, len(items))
	for i, it := range items {
		ser, num := seriesOf(it.text)
		s := seg{
			text: it.text, em: it.em, w: it.w, x0: it.x0, y0: it.y0, y1: it.y1,
			series: ser, num: num, typ: it.typ,
		}
		// ② 标题：特殊序号 / 版式信号
		if markIsHeading(items, i, bodyEm, medW) {
			s.kind = "heading"
			s.lvl = 0
		} else {
			s.kind = "para"
		}
		out = append(out, s)
	}
	return out
}

// softWrapItems：同页近距、同缩进、非新段起行 → 拼成逻辑行。
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
		gap := it.y0 - prev.y1
		em := math.Max(prev.em, it.em)
		if em < 1 {
			em = bodyEm
		}
		sameIndent := math.Abs(it.x0-prev.x0) < em*0.9
		soft := gap >= -em*0.3 && gap < em*1.15 && sameIndent && !looksNewPara(it.text)
		if soft && !endsWithSentencePunct(prev.text) {
			// 已是完整序号行：不与下行正文粘；仅「未写完」的半截序号行才续拼
			if (isArabicStem(prev.text) || isStructuralLabel(prev.text)) && !lineNeedsContinuation(prev.text) {
				out = append(out, it)
				continue
			}
			prev.text = joinSoft(prev.text, it.text)
			prev.y1 = it.y1
			prev.w = it.w
			if it.em > prev.em {
				prev.em = it.em
			}
			continue
		}
		out = append(out, it)
	}
	return out
}

// lineNeedsContinuation：行末像「没写完」——软标点/虚词尾/未闭合括号，或「是党」这类虚词+汉字。
func lineNeedsContinuation(text string) bool {
	t := strings.TrimRight(strings.TrimSpace(text), " \t")
	if t == "" {
		return false
	}
	if unbalancedOpen(t) {
		return true
	}
	r, size := utf8.DecodeLastRuneInString(t)
	if r == utf8.RuneError && size == 0 {
		return false
	}
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
	// 「…是党」「…的开」：倒数第二字虚词/介词，末字汉字 → 半截
	if strings.ContainsRune(openTail, r2) && unicode.Is(unicode.Han, r) {
		return true
	}
	return false
}

func unbalancedOpen(s string) bool {
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

func estimateLineEm(boxH, boxW float64, text string) float64 {
	n := float64(utf8.RuneCountInString(strings.ReplaceAll(text, "\n", "")))
	if boxH < 1 {
		return 1
	}
	if n < 1 {
		return boxH
	}
	const pitch = 1.25
	const aspect = 0.88
	if n*aspect*boxH <= boxW*1.25 {
		return boxH
	}
	best := boxH / pitch
	for lines := 2; lines <= 60; lines++ {
		em := boxH / (pitch * float64(lines))
		if em < 4 {
			break
		}
		cpl := math.Max(1, boxW/(aspect*em))
		need := int(math.Ceil(n / cpl))
		if need <= lines {
			return em
		}
		best = em
	}
	return best
}

func typeIsTitle(typ string) bool {
	switch typ {
	case "doc_title", "title", "paragraph_title", "section_title", "aside_title":
		return true
	default:
		return false
	}
}

// markIsHeading：断行之后判定是否标题（不赋层级）。
// 优先特殊序号（1、/一、/（一）等）；选项行永不抬；字号/type 作补充。不用字数。
func markIsHeading(items []item, i int, bodyEm, medW float64) bool {
	it := items[i]
	if strings.TrimSpace(it.text) == "" {
		return false
	}
	if isOptionLine(it.text) {
		return false
	}
	// 特殊序号 → 标题（含后接选项的选择题干）
	if isArabicStem(it.text) || isStructuralLabel(it.text) {
		return true
	}
	if typeIsTitle(it.typ) {
		return true
	}
	if bodyEm < 1 {
		bodyEm = it.em
	}
	ratio := it.em / bodyEm
	if !looksProse(it.text) && ratio >= 1.14 {
		return true
	}
	_ = medW
	return false
}

func hasQuestionMark(s string) bool {
	return strings.Contains(s, "？") || strings.Contains(s, "?")
}

// looksProse：含句号的论述句。题号/结构标签仅末尾一个「。」不当散文；「答：…。」仍按散文。
func looksProse(text string) bool {
	t := strings.TrimSpace(text)
	if !strings.Contains(t, "。") {
		return false
	}
	if strings.Count(t, "。") == 1 {
		trimmed := strings.TrimRight(t, "\"'”’）)」』")
		if strings.HasSuffix(trimmed, "。") {
			if strings.HasPrefix(t, "答：") || strings.HasPrefix(t, "答:") {
				return true
			}
			core := strings.TrimSuffix(trimmed, "。")
			if isArabicStem(t) || isStructuralLabel(core) {
				return false
			}
		}
	}
	return true
}


func isArabicStem(text string) bool {
	r := []rune(strings.TrimSpace(text))
	if len(r) < 2 {
		return false
	}
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i == 0 || i > 3 || i >= len(r) {
		return false
	}
	if i < len(r) && r[i] == '、' && isLikelyNumberRangeAfterDun(r, i) {
		return false // 16、17 区间，非题号
	}
	switch r[i] {
	case '、', '.', '．':
		return true
	default:
		return false
	}
}

// isLikelyNumberRangeAfterDun：仅当「数字、数字、」才视为连写区间（如 16、17、…）。
// 「27、1936年」「5、12项」第二段后无顿号 → 仍是题干。dunIdx 指向首个「、」。
func isLikelyNumberRangeAfterDun(r []rune, dunIdx int) bool {
	if dunIdx+1 >= len(r) || r[dunIdx+1] < '0' || r[dunIdx+1] > '9' {
		return false
	}
	j := dunIdx + 1
	for j < len(r) && r[j] >= '0' && r[j] <= '9' {
		j++
	}
	return j < len(r) && r[j] == '、'
}

// isStructuralLabel / isShortLabel：结构标签（题号、一、、（一）等），不用字数卡标题。
func isStructuralLabel(text string) bool {
	return isShortLabel(text)
}

func isShortLabel(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" || strings.HasPrefix(t, "答") {
		return false
	}
	if strings.ContainsAny(t, "。！？") {
		return false
	}
	r := []rune(t)
	if isArabicStem(t) {
		return true
	}
	cn := "一二三四五六七八九十"
	if strings.ContainsRune(cn, r[0]) && len(r) > 1 && (r[1] == '、' || r[1] == '.' || r[1] == '．' || r[1] == ' ') {
		return true
	}
	if r[0] == '（' || r[0] == '(' {
		return true
	}
	return false
}

func looksNewPara(text string) bool {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "答：") || strings.HasPrefix(t, "答:") {
		return true
	}
	if isOptionLine(t) {
		return true
	}
	r := []rune(t)
	if len(r) == 0 {
		return false
	}
	if r[0] == '第' {
		return true
	}
	if isArabicStem(t) {
		return true
	}
	cn := "一二三四五六七八九十"
	if strings.ContainsRune(cn, r[0]) && len(r) > 1 && (r[1] == '、' || r[1] == '.' || r[1] == '．') {
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
	last, _ := utf8.DecodeLastRuneInString(a)
	first, _ := utf8.DecodeRuneInString(b)
	if isASCIIWord(last) && isASCIIWord(first) {
		return a + " " + b
	}
	return a + b
}

func isASCIIWord(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func medianBodyEm(items []item) float64 {
	var ems []float64
	for _, it := range items {
		t := strings.TrimSpace(it.text)
		if t == "" || isOptionLine(t) || isArabicStem(t) || isStructuralLabel(t) || hasQuestionMark(t) {
			continue
		}
		ems = append(ems, it.em)
	}
	if len(ems) == 0 {
		for _, it := range items {
			ems = append(ems, it.em)
		}
	}
	return medianOf(ems)
}

func medianWidth(items []item) float64 {
	var ws []float64
	for _, it := range items {
		ws = append(ws, it.w)
	}
	return medianOf(ws)
}

func medianOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 12
	}
	cp := append([]float64(nil), vals...)
	sort.Float64s(cp)
	return cp[len(cp)/2]
}
