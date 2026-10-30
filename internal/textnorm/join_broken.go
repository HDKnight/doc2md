package textnorm

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Port of Python doc2md _should_join_broken_lines / _join_broken：
// 后处理跨页半截词、误拆的续行。

var (
	reJoinAnsMark     = regexp.MustCompile(`^(?:答|答案|解答)\s*[:：]|^【\s*答|^【\s*答案`)
	reJoinNumItem     = regexp.MustCompile(`^\d{1,3}[、.．)]\s*`)
	reJoinCnDun       = regexp.MustCompile(`^[一二三四五六七八九十两]+[、.．，,]\s*`)
	// 「第X…」前缀：孤立匹配（如「第一次明确提出」）不成结构；连续成族（≥2）才拒接断章
	reJoinZhang       = regexp.MustCompile(`^第[一二三四五六七八九十百零〇两\d]`)
	reJoinParenItem   = regexp.MustCompile(`^[（(]\s*[一二三四五六七八九十\d]`)
	reJoinOpt         = regexp.MustCompile(`^[A-ZＡ-Ｚ][、.．,，]`)
	reFilledBlankCont = regexp.MustCompile(`^[A-ZＡ-Ｚ]\s*[)）]`)
	reJoinShortNum    = regexp.MustCompile(`^\d{1,3}[.．、)]\s*\S`)
	reJoinNewPara     = regexp.MustCompile(`^(如何|怎么|为什么|但是|然而|因此|所以|总之|综上|另外|此外|同时|首先|其次|最后|第一|第二|第三)`)
	// 硬停：真句末。软停（逗号/闭引号）由 lineSoftContinues 决定是否续接。
	// 行末冒号 ：/: 允许直接换行（小标题另起论述），不纳入软停拼接。
	joinStopEnd   = "。！？!?；;…"
	joinSoftEnd   = "，,、”\"』」"
	joinTruncTail = "的一了是不和与及以把被在为有其并从到按"
	// 行末虚词/介词：后接汉字或年份数字仍属同一句（…现代化和中|华… / …阶段,从|2020年）
	joinOpenTail  = "的一了是不和与及以把被在为有其并从到按向对给让使"
	reStemCont    = regexp.MustCompile(`^(和|与|及|并|而|但|为|以|的|了|在|对|把|被|从|向|给|让|使|“|"|\d)`)
)

// ShouldJoinBrokenLines reports whether two markdown body lines are one broken sentence.
func ShouldJoinBrokenLines(prev, nxt string) bool {
	return shouldJoinBrokenLines(prev, nxt, false)
}

func shouldJoinBrokenLines(prev, nxt string, nxtDiFamily bool) bool {
	prev = strings.TrimRight(prev, " \t")
	nxt = strings.TrimLeft(nxt, " \t")
	if prev == "" || nxt == "" {
		return false
	}
	if strings.HasPrefix(nxt, "#") || strings.HasPrefix(prev, "|") || strings.HasPrefix(nxt, "|") || strings.HasPrefix(prev, "---") {
		return false
	}
	// 括号未合 / 软停续句 / 长标题半截折行：标题行同样适用
	if LineContinuesUnclosed(prev, nxt) || lineSoftContinues(prev, nxt, nxtDiFamily) || headingLineContinues(prev, nxt, nxtDiFamily) {
		return true
	}
	if strings.HasPrefix(prev, "#") {
		return false
	}
	rPrev, _ := utf8.DecodeLastRuneInString(prev)
	if strings.ContainsRune(joinStopEnd, rPrev) {
		return false
	}
	if reJoinAnsMark.MatchString(nxt) {
		return false
	}
	rNext, _ := utf8.DecodeRuneInString(nxt)
	// 「第X…」仅成族（≥2 连续）或与上一行同为第X前缀时拒接；真·章节由 StartsWithMarker 覆盖
	if StartsWithMarker(nxt) || reJoinNumItem.MatchString(nxt) || reJoinCnDun.MatchString(nxt) ||
		nxtDiFamily || diPrefixPair(prev, nxt) || reJoinParenItem.MatchString(nxt) ||
		reJoinOpt.MatchString(nxt) {
		return false
	}
	// 短编号小标题不与后文粘连
	bodyPrev := stripMdHeading(prev)
	if reJoinShortNum.MatchString(bodyPrev) && utf8.RuneCountInString(bodyPrev) <= 60 &&
		!strings.Contains(bodyPrev, "。") && !strings.Contains(bodyPrev, "？") && !strings.Contains(bodyPrev, "?") {
		return false
	}
	if IsShortStandaloneTitle(bodyPrev) {
		return false
	}
	if !reSearchCJKAlnumEnd(prev) {
		return false
	}
	if !isJoinStartOK(rNext) {
		return false
	}
	if reJoinNewPara.MatchString(nxt) {
		return false
	}
	if utf8.RuneCountInString(prev) <= 60 {
		return true
	}
	if utf8.RuneCountInString(nxt) <= 40 {
		return true
	}
	return strings.ContainsRune(joinTruncTail, rPrev)
}

// lineSoftContinues：上一行停在逗号/闭引号等软停点，下一行仍是同一句。
// 与 finishLayout 共用，避免中间层拒接、终层再拼的双重规则。
func lineSoftContinues(prev, next string, nextDiFamily bool) bool {
	body := stripMdHeading(prev)
	next = strings.TrimSpace(next)
	if body == "" || next == "" || strings.HasPrefix(next, "#") || isStructuralNextLine(next, nextDiFamily) {
		return false
	}
	body = strings.TrimRight(body, " \t")
	// 短小节标题后的引语/正文另起一段，勿因下行以「“」命中 reStemCont 而粘回
	if IsShortStandaloneTitle(body) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(body)
	if strings.ContainsRune(joinStopEnd, r) {
		return false
	}
	// 冒号后换行合法：历史意义：\n原理…… → 不拼
	if r == '：' || r == ':' {
		return false
	}
	nr, _ := utf8.DecodeRuneInString(next)
	if !joinContStartOK(nr) {
		return false
	}
	if strings.ContainsRune(joinSoftEnd, r) {
		return true
	}
	if strings.ContainsRune(joinOpenTail, r) && (unicode.Is(unicode.Han, nr) || unicode.IsNumber(nr)) {
		return true
	}
	return reStemCont.MatchString(next)
}

// headingLineContinues：标题行已写入正文后又在词中被折行。
// 用结构信号判断，不用字数：正文已展开（含 ：，、“《 等）才接下行；
// 纯小标题「（一）历史意义」不含这些，不吞正文。
func headingLineContinues(prev, next string, nextDiFamily bool) bool {
	trim := strings.TrimLeft(prev, " \t")
	if !strings.HasPrefix(trim, "#") {
		return false
	}
	body := stripMdHeading(prev)
	next = strings.TrimSpace(next)
	if body == "" || next == "" || strings.HasPrefix(next, "#") || isStructuralNextLine(next, nextDiFamily) {
		return false
	}
	// 短小节「（2）“三查三整”的方法步骤」含引号也会触发 prose 信号，但绝不能吞下行正文
	if IsShortStandaloneTitle(body) {
		return false
	}
	if !headingBodyHasProse(body) {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(body, " \t"))
	if strings.ContainsRune(joinStopEnd, r) {
		return false
	}
	// 软停后若下行以年份/数字续句（…从\n2020年），仍拼；纯汉字新段不拼
	nr, _ := utf8.DecodeRuneInString(next)
	if strings.ContainsRune(joinSoftEnd, r) {
		return unicode.IsNumber(nr)
	}
	if !joinContStartOK(nr) {
		return false
	}
	return unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsNumber(r) ||
		strings.ContainsRune(joinOpenTail, r)
}

// joinContStartOK：续行可以汉字、字母、数字或引号/书名号起头。
func joinContStartOK(r rune) bool {
	if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsNumber(r) {
		return true
	}
	switch r {
	case '“', '"', '《', '「':
		return true
	}
	return false
}

func headingBodyHasProse(body string) bool {
	// 去掉行首序号后再看，避免「(一)」「（1）」里的括号被当成正文信号
	if m := ClassifyMarker(body); m != nil {
		body = strings.TrimSpace(m.Body)
	}
	// 标题后已跟论述：意义:…… / 含逗号、顿号、引号、书名号（不含序号括号）
	for _, c := range []string{"：", ":", "，", ",", "、", "；", ";", "“", "\"", "《"} {
		if i := strings.Index(body, c); i >= 0 {
			rest := strings.TrimSpace(body[i+len(c):])
			if rest != "" {
				return true
			}
		}
	}
	return false
}

func isStructuralNextLine(s string, diFamily bool) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "#") {
		return true
	}
	if reJoinOpt.MatchString(s) || reJoinNumItem.MatchString(s) ||
		reJoinCnDun.MatchString(s) || reJoinAnsMark.MatchString(s) {
		return true
	}
	// 成族「第X…」拒接；孤立「第一次…」放行。真·第X章/节/篇/部分始终结构。
	if diFamily {
		return true
	}
	body := stripMdHeading(s)
	if m := ClassifyMarker(body); m != nil && (m.Family == "chapter" || m.Family == "part") {
		return true
	}
	return false
}

func isDiPrefixLine(s string) bool {
	return reJoinZhang.MatchString(stripMdHeading(strings.TrimSpace(s)))
}

func diPrefixPair(prev, next string) bool {
	return isDiPrefixLine(prev) && isDiPrefixLine(next)
}

// markDiPrefixFamily：非空行流中连续 ≥2 条匹配 reJoinZhang 的行标成族（空白不打断）。
func markDiPrefixFamily(lines []string) []bool {
	out := make([]bool, len(lines))
	run := make([]int, 0, 4)
	flush := func() {
		if len(run) >= 2 {
			for _, i := range run {
				out[i] = true
			}
		}
		run = run[:0]
	}
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if isDiPrefixLine(t) {
			run = append(run, i)
			continue
		}
		flush()
	}
	flush()
	return out
}

func reSearchCJKAlnumEnd(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(s)
	if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsNumber(r) {
		return true
	}
	return false
}

func isJoinStartOK(r rune) bool {
	if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsNumber(r) {
		return true
	}
	switch r {
	case '「', '“', '\'', '"', '《',
		// OCR/排版常把 “(B” 与 “)” 拆到两行，闭括号须允许续接
		')', '）', ']', '】', '』', '”':
		return true
	}
	return false
}

// titleEndSuffixes：完整小节常见收尾（按字数长到短匹配）。
// 用结构收尾而非句子长度区分「标题 / 正文」。
var titleEndSuffixes = []string{
	"方法步骤", "基本方略", "核心要义", "丰富内涵", "党性修养",
	"步骤", "方法", "意义", "概述", "内容", "特点", "原则", "要求", "作用",
	"途径", "措施", "问题", "目的", "方针", "政策", "路线", "修养", "思想", "理论", "制度",
	"体系", "布局", "目标", "任务", "经验", "启示", "内涵", "要义", "基础", "条件",
	"保证", "动力", "本质", "特征", "优势", "阶段", "安排", "部署", "小结", "总结",
	"概念", "定义", "类型", "形式", "实质", "核心", "重点", "要点", "导论", "简介",
	"前言", "绪论", "结语", "题",
}

// IsShortStandaloneTitle reports a complete section label that must not swallow the next line.
// 判定靠结构信号（收尾词、引号闭合、虚词折行、冒号后是否仍有论述），不用字数阈值。
func IsShortStandaloneTitle(text string) bool {
	t := strings.TrimSpace(text)
	key := strings.ReplaceAll(t, " ", "")
	key = strings.ReplaceAll(key, "　", "")
	switch key {
	case "简介", "导论", "目录", "前言", "绪论", "结语", "参考文献", "附录":
		return true
	}
	if t == "" || strings.ContainsAny(t, "。！？") {
		return false
	}
	// 日期碎片（1926年4月）不是小节标题，允许与下行拼接
	if strings.ContainsAny(t, "年月日") {
		return false
	}
	m := ClassifyMarker(t)
	if m == nil {
		return false
	}
	switch markerFamily(m) {
	case "chapter", "part":
		return !hasUnclosedOpenBracket(t) && !endsWithOpenTail(t)
	case "cn_dun", "cn_paren", "num_paren", "circle", "integer_dot", "integer_dun", "latin":
		body := strings.TrimSpace(m.Body)
		if body == "" {
			return false
		}
		// 含句读逗号 → 已是叙述句，不是标签标题
		if strings.ContainsAny(body, "。！？；;，,") {
			return false
		}
		if (m.Family == "integer_dot" || m.Family == "integer_dun" || m.Family == "cn_dun") &&
			strings.Contains(body, "、") {
			return false
		}
		if hasUnclosedOpenBracket(body) || endsWithOpenTail(body) {
			return false
		}
		if hasExpandedColonProse(body) {
			return false
		}
		if m.Family == "integer_dot" || m.Family == "integer_dun" {
			if strings.ContainsAny(body, "以把被将向从比") {
				return false
			}
		}
		// 完整小节：收尾词 / 纯引号标语；排除「…方针政策党内不同思想」这类粘连假收尾
		if isClosedQuoteSlogan(body) {
			return true
		}
		return isCleanTitleSuffixBody(body)
	default:
		return false
	}
}

func endsWithOpenTail(s string) bool {
	s = strings.TrimRight(s, " \t")
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return strings.ContainsRune(joinOpenTail, r)
}

func endsWithTitleSuffix(body string) bool {
	body = strings.TrimSpace(body)
	for _, suf := range titleEndSuffixes {
		if strings.HasSuffix(body, suf) {
			return true
		}
	}
	return false
}

// isCleanTitleSuffixBody：以标题收尾词结束，且中间没有「收尾词+后续汉字」的粘连切口。
func isCleanTitleSuffixBody(body string) bool {
	body = strings.TrimSpace(body)
	if !endsWithTitleSuffix(body) {
		return false
	}
	// 探测内部切口：临时加续字，若切点落在正文前则非干净标题
	at := splitGluedTitleByStructure(body + "续")
	return at == 0 || at >= utf8.RuneCountInString(body)
}

// hasExpandedColonProse：冒号后引号已闭合却仍有论述（…意义:“…”论断符合…）。
func hasExpandedColonProse(body string) bool {
	for _, sep := range []string{"：", ":"} {
		i := strings.Index(body, sep)
		if i < 0 {
			continue
		}
		rest := strings.TrimSpace(body[i+len(sep):])
		if rest == "" {
			return false
		}
		qOpen, qClose := "", ""
		switch {
		case strings.HasPrefix(rest, "“"):
			qOpen, qClose = "“", "”"
		case strings.HasPrefix(rest, "\""):
			qOpen, qClose = "\"", "\""
		default:
			// 冒号后直接叙述且不以标题收尾词结束 → 展开正文
			return !endsWithTitleSuffix(rest)
		}
		inner := rest[len(qOpen):]
		j := strings.Index(inner, qClose)
		if j < 0 {
			return true
		}
		after := strings.TrimSpace(inner[j+len(qClose):])
		return after != ""
	}
	return false
}

func isClosedQuoteSlogan(body string) bool {
	rest := strings.TrimSpace(body)
	for _, sep := range []string{"：", ":"} {
		if i := strings.Index(body, sep); i >= 0 {
			rest = strings.TrimSpace(body[i+len(sep):])
			break
		}
	}
	if rest == "" {
		return false
	}
	if strings.HasPrefix(rest, "“") && strings.HasSuffix(rest, "”") {
		return strings.Count(rest, "“") == 1 && strings.Count(rest, "”") == 1
	}
	if strings.HasPrefix(rest, "\"") && strings.HasSuffix(rest, "\"") && utf8.RuneCountInString(rest) >= 2 {
		return strings.Count(rest, "\"") == 2
	}
	return false
}

// JoinBrokenMarkdown merges wrongly split body continuations (Python _join_broken).
func JoinBrokenMarkdown(md string) string {
	if md == "" {
		return md
	}
	lines := strings.Split(md, "\n")
	diFam := markDiPrefixFamily(lines)
	out := make([]string, 0, len(lines))
	inCode := false
	i := 0
	for i < len(lines) {
		ln := lines[i]
		trim := strings.TrimLeft(ln, " \t")
		if strings.HasPrefix(trim, "```") {
			inCode = !inCode
			out = append(out, ln)
			i++
			continue
		}
		if inCode || strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, "|") {
			out = append(out, ln)
			i++
			continue
		}
		if strings.TrimSpace(ln) == "" {
			// blank: peek next non-empty body and maybe join across blank（含未闭合括号的标题）
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
				j := i + 1
				for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
					j++
				}
				if j < len(lines) && shouldJoinBrokenLines(out[len(out)-1], lines[j], diFam[j]) {
					out[len(out)-1] = strings.TrimRight(out[len(out)-1], " \t") + strings.TrimLeft(lines[j], " \t")
					i = j + 1
					continue
				}
			}
			out = append(out, ln)
			i++
			continue
		}
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" && !strings.HasPrefix(out[len(out)-1], "#") &&
			shouldJoinBrokenLines(out[len(out)-1], ln, diFam[i]) {
			out[len(out)-1] = strings.TrimRight(out[len(out)-1], " \t") + strings.TrimLeft(ln, " \t")
			i++
			continue
		}
		out = append(out, ln)
		i++
	}
	return strings.Join(out, "\n")
}
