package textnorm

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Marker is a classified line-leading outline token.
type Marker struct {
	Family string
	Token  string
	Body   string
	// Weak：无点/顿/括号等硬标点，仅「数字+空格」或裸数字粘连拆开。
	// 升标题须同族成族且数字大致连续，避免「3种」「7个」误升。
	Weak bool
}

var (
	reDotOCR    = regexp.MustCompile(`^(\d{1,3}(?:。\d{1,3})+)(。)?(.*)$`)
	reCnOCR     = regexp.MustCompile(`^([一二三四五六七八九十百千]+)。([^0-9].*)$`)
	reIntOCR    = regexp.MustCompile(`^(\d{1,3})。(.*)$`)
	reRefDun    = regexp.MustCompile(`^(\d{1,3})、`)
	reRefDot    = regexp.MustCompile(`^(\d{1,3})[\.．]`)
	reRefParen  = regexp.MustCompile(`^[（(](\d{1,3})[)）]`)
	reDotFamily = regexp.MustCompile(`^(\d{1,3}(?:\.\d{1,3})+)(.*)$`)

	reOpt         = regexp.MustCompile(`^([A-ZＡ-Ｚ][\.、．])(.*)$`)
	rePartMark    = regexp.MustCompile(`^(第[一二三四五六七八九十百千0-9]+部分)(.*)$`)
	reChapterMark = regexp.MustCompile(`^(第[一二三四五六七八九十百千0-9]+[章节篇])(.*)$`)
	reCnDun       = regexp.MustCompile(`^([一二三四五六七八九十百千]+、)(.*)$`)
	reCnParen     = regexp.MustCompile(`^([（(][一二三四五六七八九十百千]+[)）])(.*)$`)
	reNumParen    = regexp.MustCompile(`^([（(]\d{1,3}[)）])(.*)$`)
	reCircle      = regexp.MustCompile(`^([①②③④⑤⑥⑦⑧⑨⑩])(.*)$`)
	reOrdinal     = regexp.MustCompile(`^((?:第|其)[一二三四五六七八九十]+，)(.*)$`)
	reLatin       = regexp.MustCompile(`^([a-z][\.、．])(.*)$`)
	reIntDun      = regexp.MustCompile(`^(\d{1,3}、)(.*)$`)
	reIntDot      = regexp.MustCompile(`^(\d{1,3}[\.．])(.*)$`)
	reIntSpace    = regexp.MustCompile(`^(\d{1,3})\s+(\S.*)$`)
	reIntBare     = regexp.MustCompile(`^(\d{1,3})$`)

	// pdf-md _MARKER_BODY / MARKER_START / MARKER_ONLY
	markerBody = `[A-ZＡ-Ｚ][\.、．]` +
		`|第[一二三四五六七八九十百千0-9]+[章节篇]` +
		`|[一二三四五六七八九十百千]+[、。]` +
		`|[（(][一二三四五六七八九十0-9]+[)）]` +
		`|\d{1,3}(?:[\.。．、]\d{1,3})+` +
		`|\d{1,3}[\.。．、]` +
		`|\d{1,3}` +
		`|[①②③④⑤⑥⑦⑧⑨⑩]` +
		`|[a-z][\.、．]` +
		`|第[一二三四五六七八九十]+，` +
		`|其[一二三四五六七八九十]，`
	reMarkerStart = regexp.MustCompile(`^(?:` + markerBody + `)`)
	reMarkerOnly  = regexp.MustCompile(`^(?:` + markerBody + `)$`)
	// 年份/年代行不是大纲序号（避免 2020年… 被 \d{1,3} 误匹配）
	reYearLineStart = regexp.MustCompile(`^\d{4}|^\d{1,3}年|^\d{1,3}世纪`)
)

// NormalizeMarkers fixes OCR-confused leading outline punctuation (pdf-md rules).
func NormalizeMarkers(lines []string) []string {
	prepared := make([]string, len(lines))
	for i, text := range lines {
		prepared[i] = separateGluedInteger(text)
	}
	rewritten := make([]string, len(prepared))
	for i, text := range prepared {
		stripped := strings.TrimSpace(text)
		if m := reDotOCR.FindStringSubmatch(stripped); m != nil {
			num := strings.ReplaceAll(m[1], "。", ".")
			rewritten[i] = num + m[3]
			continue
		}
		if m := reCnOCR.FindStringSubmatch(stripped); m != nil {
			rewritten[i] = m[1] + "、" + m[2]
			continue
		}
		rewritten[i] = text
	}
	result := make([]string, len(rewritten))
	for i, text := range rewritten {
		m := reIntOCR.FindStringSubmatch(strings.TrimSpace(text))
		if m == nil || startsWithDigitDotNumber(strings.TrimSpace(text)) {
			result[i] = text
			continue
		}
		// Only rewrite when 。 is OCR marker (not mid decimal already handled).
		rest := m[2]
		if rest != "" && rest[0] >= '0' && rest[0] <= '9' {
			result[i] = text
			continue
		}
		number := m[1]
		prefix := ""
		for _, step := range []int{-1, 1} {
			sample := nearestMarker(rewritten, i, step)
			if sample == "" {
				continue
			}
			prefix = applyReferent(number, sample)
			if prefix != "" {
				break
			}
		}
		if prefix == "" {
			prefix = number + "."
		}
		result[i] = prefix + rest
	}
	return result
}

func separateGluedInteger(text string) string {
	// "2政治工作" → "2 政治工作"; keep "1927年" / "1月"
	runes := []rune(text)
	i := 0
	for i < len(runes) && unicode.IsSpace(runes[i]) {
		i++
	}
	start := i
	for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
		i++
	}
	digits := i - start
	if digits == 0 || digits > 3 || i >= len(runes) {
		return text
	}
	next := runes[i]
	if next == '.' || next == '。' || next == '．' || next == '、' {
		return text
	}
	if next == '年' || next == '月' || next == '日' {
		return text
	}
	// Keep "19世纪" / "20年代" intact.
	rest := string(runes[i:])
	if strings.HasPrefix(rest, "世纪") || strings.HasPrefix(rest, "年代") {
		return text
	}
	if unicode.Is(unicode.Han, next) {
		return string(runes[:i]) + " " + string(runes[i:])
	}
	return text
}

func startsWithDigitDotNumber(text string) bool {
	// 2。3 already handled by reDotOCR
	return reDotOCR.MatchString(text)
}

func nearestMarker(lines []string, index, step int) string {
	for cursor := index + step; cursor >= 0 && cursor < len(lines); cursor += step {
		text := strings.TrimSpace(lines[cursor])
		if reIntOCR.MatchString(text) && !reDotOCR.MatchString(text) {
			rest := ""
			if m := reIntOCR.FindStringSubmatch(text); m != nil {
				rest = m[2]
			}
			if rest == "" || (rest[0] < '0' || rest[0] > '9') {
				continue
			}
		}
		if startsWithMarker(text) {
			return text
		}
	}
	return ""
}

func applyReferent(number, sample string) string {
	if reRefDun.MatchString(sample) {
		return number + "、"
	}
	if reRefDot.MatchString(sample) {
		return number + "."
	}
	if reRefParen.MatchString(sample) {
		return "（" + number + "）"
	}
	return ""
}

func startsWithMarker(text string) bool {
	stripped := strings.TrimSpace(text)
	// 图1 / 表 2：图表题注。排除正文续行「表”重要思想」（三个代表折行）。
	if isFigureOrTableCaption(stripped) {
		return true
	}
	if reYearLineStart.MatchString(stripped) {
		return false
	}
	// Align with pdf-md starts_with_marker / MARKER_START (not only ClassifyMarker).
	return reMarkerStart.MatchString(stripped)
}

func isFigureOrTableCaption(text string) bool {
	if text == "图" || text == "表" {
		return true
	}
	var prefix string
	switch {
	case strings.HasPrefix(text, "图"):
		prefix = "图"
	case strings.HasPrefix(text, "表"):
		prefix = "表"
	default:
		return false
	}
	rest := strings.TrimLeft(text[len(prefix):], " \t　")
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return r >= '0' && r <= '9' || r >= '０' && r <= '９'
}

func isMarkerOnly(text string) bool {
	return IsMarkerOnly(text)
}

// IsMarkerOnly reports a line that is only an outline token (e.g. "2。" / "1.3.8").
// Aligns with pdf-md MARKER_ONLY (broader than ClassifyMarker-with-empty-body).
func IsMarkerOnly(text string) bool {
	stripped := strings.TrimSpace(text)
	if reMarkerOnly.MatchString(stripped) {
		return true
	}
	m := ClassifyMarker(stripped)
	return m != nil && strings.TrimSpace(m.Body) == ""
}

func isAnswerLabel(text string) bool {
	return IsAnswerLabel(text)
}

// IsAnswerLabel reports 【答案】/【解析】 lines.
func IsAnswerLabel(text string) bool {
	s := strings.TrimSpace(text)
	return strings.HasPrefix(s, "【答案】") || strings.HasPrefix(s, "【解析】")
}

// StartsWithMarker reports outline / figure / table markers at line start.
func StartsWithMarker(text string) bool {
	return startsWithMarker(text)
}

// ClassifyMarker classifies a line's leading outline token (pdf-md rules).
func ClassifyMarker(text string) *Marker {
	stripped := strings.TrimSpace(text)
	if m := reDotFamily.FindStringSubmatch(stripped); m != nil {
		token := m[1]
		family := fmt.Sprintf("dot-%d", strings.Count(token, ".")+1)
		return &Marker{Family: family, Token: token, Body: strings.TrimLeft(m[2], " \t")}
	}
	type pat struct {
		family string
		re     *regexp.Regexp
	}
	for _, p := range []pat{
		{"option", reOpt},
		{"part", rePartMark},
		{"chapter", reChapterMark},
		{"cn_dun", reCnDun},
		{"cn_paren", reCnParen},
		{"num_paren", reNumParen},
		{"circle", reCircle},
		{"ordinal", reOrdinal},
		{"latin", reLatin},
		{"integer_dun", reIntDun},
		{"integer_dot", reIntDot},
		{"integer_dot", reIntSpace},
		{"integer_dot", reIntBare},
	} {
		m := p.re.FindStringSubmatch(stripped)
		if m == nil {
			continue
		}
		// integer_dot must not swallow decimals like 1.2 (handled by reDotFamily)
		if p.family == "integer_dot" && p.re == reIntDot {
			if len(m[1]) > 0 {
				// "1.2 xxx" already matched dot-family; if token is "1." and body starts with digit, skip
				body := ""
				if len(m) >= 3 {
					body = m[2]
				}
				if body != "" && body[0] >= '0' && body[0] <= '9' {
					continue
				}
			}
		}
		body := ""
		if len(m) >= 3 {
			body = strings.TrimLeft(m[2], " \t")
		}
		token := m[1]
		weak := false
		if p.family == "integer_dot" && body != "" && !strings.ContainsAny(token, ".．、") {
			// reIntSpace / reIntBare（含 separateGluedInteger 后的「1 正文」）
			weak = p.re == reIntSpace || p.re == reIntBare
			token = m[1] + "."
		}
		// Prose that merely mentions 第X部分/章 should not become an outline marker.
		if (p.family == "part" || p.family == "chapter") && isProseMarkerBody(body) {
			continue
		}
		return &Marker{Family: p.family, Token: token, Body: body, Weak: weak}
	}
	return nil
}

func markerFamily(m *Marker) string {
	if m == nil {
		return ""
	}
	if m.Family == "option" || m.Family == "ordinal" {
		return ""
	}
	return m.Family
}

func isZhang(m *Marker) bool {
	return m != nil && m.Family == "chapter" && strings.HasSuffix(strings.TrimSpace(m.Token), "章")
}

func isOnceTitle(m *Marker) bool {
	if m == nil || (m.Family != "integer_dot" && m.Family != "integer_dun") {
		return false
	}
	body := strings.TrimSpace(m.Body)
	if body == "" {
		return false
	}
	// 不用字数：仅「…：」小节题可一次成题；「…。」论述句靠频次 ≥2
	return strings.HasSuffix(body, "：")
}

func isReplyLine(text string) bool {
	return strings.TrimSpace(text) == "答："
}

func isProseMarkerBody(body string) bool {
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}
	// 结构：含句读即论述体，不用字数阈值
	return strings.ContainsAny(body, "。！？；")
}

func hasExamBlank(body string) bool {
	return strings.Contains(body, "( )") || strings.Contains(body, "（ ）") ||
		strings.Contains(body, "（）") || strings.Contains(body, "()") ||
		strings.Contains(body, "（  ）") || strings.Contains(body, "(  )")
}

func hasExamCite(body string) bool {
	if !strings.Contains(body, "[") {
		return false
	}
	if strings.Contains(body, "研") || strings.Contains(body, "考研") || strings.Contains(body, "真题") {
		return true
	}
	// [学校名2018年] / [某某大学2018考研]
	return reExamYear.MatchString(body)
}

var reExamYear = regexp.MustCompile(`\[[^\]]*(?:19|20)\d{2}`)
