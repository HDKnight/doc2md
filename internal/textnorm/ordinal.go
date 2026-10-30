package textnorm

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reHalfParenItem = regexp.MustCompile(`^\((\d{1,3})\)\s*`)
	reHeadPrefix    = regexp.MustCompile(`^(#{1,6}\s+)`)
	reOCRPeriodItem = regexp.MustCompile(`^(\d{1,3})。(.*)$`)
	reOCRCnPeriod   = regexp.MustCompile(`^([一二三四五六七八九十百千]+)。([^0-9].*)$`)
	reQGlueDot = regexp.MustCompile(`([\x{4e00}-\x{9fff}A-Za-zＡ-Ｚａ-ｚ）)。；;!！？》\"”])\s*(\d{1,3}[.．]\s*[\x{4e00}-\x{9fff}（(A-Za-zＡ-Ｚａ-ｚ“\"《])`)
	reQGlueDun = regexp.MustCompile(`([\x{4e00}-\x{9fff}A-Za-zＡ-Ｚａ-ｚ）)。；;!！？》\"”])\s*(\d{1,3}、\s*[\x{4e00}-\x{9fff}（(A-Za-zＡ-Ｚａ-ｚ“\"《\d])`)
	reSubDigitGlue = regexp.MustCompile(`([\x{4e00}-\x{9fff}])([1-9])([\x{4e00}-\x{9fff}]{2,})`)
	reYearLike     = regexp.MustCompile(`世纪|年代|年月|年研|考研|^[月日时分秒点％%]`)
	// 排除 * ：避免把 **【答案】** 拆成 * + 【答案】**
	reAnsGlue     = regexp.MustCompile(`([^\n【*])(【\s*(?:答案|解析)\s*】)`)
	reRelatedGlue = regexp.MustCompile(`([。？！；）\)\]』”"])(相关试题\s*[:：]?)`)
)

// PrepOrdinalLine normalizes a single line's leading ordinal before heading decisions.
// Safe to call on every body line (idempotent for already-clean text).
func PrepOrdinalLine(line string) string {
	s := strings.TrimSpace(line)
	if s == "" {
		return s
	}
	s = separateGluedInteger(s)
	s = fixOCRPeriodMarker(s)
	s = normalizeLeadingParen(s)
	s = spaceAfterOrdinal(s)
	return s
}

func fixOCRPeriodMarker(s string) string {
	if m := reOCRCnPeriod.FindStringSubmatch(s); m != nil {
		return m[1] + "、" + m[2]
	}
	if m := reOCRPeriodItem.FindStringSubmatch(s); m != nil {
		rest := m[2]
		// 1。2.3 多级 / 小数：不动
		if rest != "" && rest[0] >= '0' && rest[0] <= '9' {
			return s
		}
		if rest == "" {
			return m[1] + "."
		}
		return m[1] + ". " + strings.TrimLeft(rest, " \t")
	}
	return s
}

// FormatMarkdownOrdinals 处理「尚未定级」的扁平试题串：拆粘连序号、再无害排版。
// 已 BuildOutline + Render 的 Markdown 不要走这里——定级后只用 FinishMarkdown。
func FormatMarkdownOrdinals(md string) string {
	if md == "" {
		return md
	}
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines)+8)
	inCode := false
	for _, line := range lines {
		trimLeft := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimLeft, "```") {
			inCode = !inCode
			out = append(out, line)
			continue
		}
		if inCode || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "---") {
			out = append(out, line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			out = append(out, line)
			continue
		}
		// 已是标题行：定级产物，禁止再拆序号（避免 ### → 空 # + 正文）
		if reHeadPrefix.MatchString(strings.TrimLeft(line, " \t")) {
			out = append(out, PrepOrdinalLine(line))
			continue
		}
		formatted := formatOrdinalLine(line)
		parts := strings.Split(formatted, "\n")
		out = append(out, parts...)
	}
	// 跨行连续序号：只切非标题行（标题行已在上面跳过拆分）
	out = splitAcrossConfirmedOrdinalLines(out)
	joined := strings.Join(out, "\n")
	joined = regexp.MustCompile(`\n{3,}`).ReplaceAllString(joined, "\n\n")
	joined = JoinBrokenMarkdown(joined)
	joined = regexp.MustCompile(`\n{3,}`).ReplaceAllString(joined, "\n\n")
	return FinishMarkdown(joined)
}

// FinishMarkdown 标题定级并 Render 之后的无害排版（日期/括号/软断行拼接/【】加粗）。
// 不拆序号、不改大纲结构；粘连切开须在 ToBlocks 定级前完成。
func FinishMarkdown(md string) string {
	return finishLayout(md)
}

// finishLayout 通用排版：括号内换行、括号后非标点续行、断章、【】/[] 加粗。
func finishLayout(md string) string {
	return emphasizeBracketLabels(fixStemFragments(fixChoiceBlankBreaks(joinDateBreaks(md))))
}

var (
	reDateYMDBreak = regexp.MustCompile(`(\d{2,4}|[〇零一二三四五六七八九十]{2,4})[ \t]*年[ \t]*\n+[ \t]*(\d{1,2}|[〇零一二三四五六七八九十]{1,3})[ \t]*月[ \t]*\n+[ \t]*(\d{1,2}|[〇零一二三四五六七八九十]{1,3})[ \t]*日`)
	reDateYMBreak  = regexp.MustCompile(`(\d{2,4}|[〇零一二三四五六七八九十]{2,4})[ \t]*年[ \t]*\n+[ \t]*(\d{1,2}|[〇零一二三四五六七八九十]{1,3})[ \t]*月`)
	reDateMDBreak  = regexp.MustCompile(`(\d{1,2}|[〇零一二三四五六七八九十]{1,3})[ \t]*月[ \t]*\n+[ \t]*(\d{1,2}|[〇零一二三四五六七八九十]{1,3})[ \t]*日`)
	reDateYDBreak  = regexp.MustCompile(`(\d{2,4}|[〇零一二三四五六七八九十]{2,4})[ \t]*年[ \t]*(\d{1,2}|[〇零一二三四五六七八九十]{1,3})[ \t]*月[ \t]*\n+[ \t]*(\d{1,2}|[〇零一二三四五六七八九十]{1,3})[ \t]*日`)
	reAfterParenNL = regexp.MustCompile(`([)）])[ \t]*\n+[ \t]*([^\n]+)`)
	reBracketLabel = regexp.MustCompile(`【[^【】\n]{1,80}】|\[[^\[\]\n]{1,80}\]`)
	punctAfterMark = "。！？；：，、,.!?;:…）)」』”\"】]》"
)

// joinDateBreaks removes line breaks inside dates: 1937年/8月/7日.
func joinDateBreaks(md string) string {
	if md == "" || !strings.Contains(md, "\n") {
		return md
	}
	md = reDateYMDBreak.ReplaceAllString(md, "${1}年${2}月${3}日")
	md = reDateYDBreak.ReplaceAllString(md, "${1}年${2}月${3}日")
	md = reDateYMBreak.ReplaceAllString(md, "${1}年${2}月")
	md = reDateMDBreak.ReplaceAllString(md, "${1}月${2}日")
	return md
}

// fixChoiceBlankBreaks removes newlines inside （）/() and, when the next
// character is not punctuation, the newline right after the closing paren.
func fixChoiceBlankBreaks(md string) string {
	if md == "" {
		return md
	}
	md = stripNewlinesInsideParens(md)
	return joinAfterParenIfNotPunct(md)
}

func stripNewlinesInsideParens(md string) string {
	runes := []rune(md)
	var b strings.Builder
	depth := 0
	inner := 0 // 当前未闭合括号内已写入的字符数
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '(', '（':
			depth++
			inner = 0
			b.WriteRune(r)
		case ')', '）':
			if depth > 0 {
				depth--
			}
			b.WriteRune(r)
		case '\n':
			if depth > 0 {
				// 未闭合填空后若已是新题/选项，保留换行并放弃该括号，避免吞掉整页
				j := i + 1
				for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t') {
					j++
				}
				rest := strings.TrimSpace(string(runes[j:]))
				if rest != "" && (choiceBlankTailIsNewItem(rest) || startsWithMarker(rest) || inner > 24) {
					depth = 0
					b.WriteRune(r)
					continue
				}
				continue
			}
			b.WriteRune(r)
		default:
			if depth > 0 {
				inner++
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func joinAfterParenIfNotPunct(md string) string {
	return reAfterParenNL.ReplaceAllStringFunc(md, func(s string) string {
		m := reAfterParenNL.FindStringSubmatch(s)
		if m == nil {
			return s
		}
		cont := strings.TrimSpace(m[2])
		if cont == "" || choiceBlankTailIsNewItem(cont) {
			return s
		}
		r, _ := utf8.DecodeRuneInString(cont)
		if strings.ContainsRune(punctAfterMark, r) {
			return s
		}
		return m[1] + cont
	})
}

func emphasizeBracketLabels(md string) string {
	if md == "" {
		return md
	}
	idxs := reBracketLabel.FindAllStringIndex(md, -1)
	if len(idxs) == 0 {
		return md
	}
	var b strings.Builder
	last := 0
	for _, sp := range idxs {
		start, end := sp[0], sp[1]
		if start >= 2 && md[start-2:start] == "**" {
			continue
		}
		if start > 0 && md[start-1] == '!' && md[start] == '[' {
			continue
		}
		if md[start] == '[' && end < len(md) && md[end] == '(' {
			continue
		}
		label := md[start:end]
		if strings.HasPrefix(label, "[") && !shouldEmphasizeSquare(label) {
			continue
		}
		b.WriteString(md[last:start])
		b.WriteString("**")
		b.WriteString(label)
		b.WriteString("**")
		if end < len(md) {
			r, _ := utf8.DecodeRuneInString(md[end:])
			if r != ' ' && r != '\n' && r != '\t' && !strings.ContainsRune(punctAfterMark, r) {
				b.WriteByte(' ')
			}
		}
		last = end
	}
	b.WriteString(md[last:])
	return b.String()
}

func shouldEmphasizeSquare(label string) bool {
	// [] 只加粗题库引用类，避免英文脚注 [1] / 普通 [text]
	inner := strings.TrimSuffix(strings.TrimPrefix(label, "["), "]")
	if inner == "" {
		return false
	}
	if strings.Contains(inner, "研") || strings.Contains(inner, "考研") || strings.Contains(inner, "真题") {
		return true
	}
	for _, r := range inner {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return reExamYear.MatchString(label)
}

// fixStemFragments 兜底拼软停断章（与 ShouldJoinBrokenLines 共用 lineSoftContinues）。
func fixStemFragments(md string) string {
	if md == "" {
		return md
	}
	lines := strings.Split(md, "\n")
	diFam := markDiPrefixFamily(lines)
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		cur := lines[i]
		if strings.TrimSpace(cur) == "" {
			out = append(out, cur)
			continue
		}
		for {
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j >= len(lines) {
				break
			}
			di := diFam[j]
			if !(lineSoftContinues(cur, lines[j], di) || headingLineContinues(cur, lines[j], di)) {
				break
			}
			cur = strings.TrimRight(cur, " \t") + strings.TrimSpace(lines[j])
			i = j
		}
		out = append(out, cur)
	}
	return strings.Join(out, "\n")
}

func choiceBlankTailIsNewItem(s string) bool {
	return isStructuralNextLine(s, false)
}

// splitGluedOptionLines breaks glued same-series ordinals into one line each.
func splitGluedOptionLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, splitByRepeatedOrdinals(line)...)
	}
	return out
}

func formatOrdinalLine(line string) string {
	s := PrepOrdinalLine(line)
	// 答案/解析、相关试题粘在题干后 → 拆段
	s = reAnsGlue.ReplaceAllString(s, "$1\n\n$2")
	s = reRelatedGlue.ReplaceAllString(s, "$1\n\n$2")
	s = reQGlueDot.ReplaceAllString(s, "$1\n\n$2")
	s = reQGlueDun.ReplaceAllString(s, "$1\n\n$2")
	s = splitSubDigitGlue(s)
	// 同类序号 ≥2 → 换行切开（不删中间字符）
	var flat []string
	for _, chunk := range strings.Split(s, "\n") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			flat = append(flat, "")
			continue
		}
		opts := splitByRepeatedOrdinals(chunk)
		if len(opts) == 0 {
			continue
		}
		for i, p := range opts {
			if i > 0 {
				flat = append(flat, "")
			}
			flat = append(flat, p)
		}
	}
	for i, p := range flat {
		if p == "" {
			continue
		}
		flat[i] = PrepOrdinalLine(strings.TrimRight(p, " \t"))
	}
	return strings.Join(flat, "\n")
}

func normalizeLeadingParen(line string) string {
	prefix := ""
	body := line
	if m := reHeadPrefix.FindStringSubmatch(line); m != nil {
		prefix = m[1]
		body = line[len(m[1]):]
	}
	if m := reHalfParenItem.FindStringSubmatch(body); m != nil {
		rest := body[len(m[0]):]
		return prefix + "（" + m[1] + "）" + rest
	}
	return line
}

func spaceAfterOrdinal(line string) string {
	prefix := ""
	body := line
	if m := reHeadPrefix.FindStringSubmatch(line); m != nil {
		prefix = m[1]
		body = line[len(m[1]):]
	}
	if body == "" {
		return line
	}
	// 2.坚持 / 2. 坚持（点号后非数字）
	if i := ordinalDotEnd(body); i > 0 {
		return prefix + insertSpaceAt(body, i)
	}
	// 2、坚持
	if i := ordinalDunEnd(body); i > 0 {
		return prefix + insertSpaceAt(body, i)
	}
	// 二、延安
	if i := ordinalCnDunEnd(body); i > 0 {
		return prefix + insertSpaceAt(body, i)
	}
	// （2）正文 / （二）正文
	if i := ordinalParenEnd(body); i > 0 {
		return prefix + insertSpaceAt(body, i)
	}
	// A、选项
	if i := ordinalOptEnd(body); i > 0 {
		return prefix + insertSpaceAt(body, i)
	}
	return line
}

func insertSpaceAt(body string, i int) string {
	if i >= len(body) {
		return body
	}
	if body[i] == ' ' {
		return body
	}
	return body[:i] + " " + body[i:]
}

func ordinalDotEnd(s string) int {
	// ^\d{1,3}[.．](?=[^\s\d])
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 3 {
		return 0
	}
	if i >= len(s) {
		return 0
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	if r != '.' && r != '．' {
		return 0
	}
	j := i + size
	if j >= len(s) {
		return 0
	}
	next, _ := utf8.DecodeRuneInString(s[j:])
	if next == ' ' || (next >= '0' && next <= '9') {
		return 0
	}
	return j
}

func ordinalDunEnd(s string) int {
	// ^\d{1,3}、(?=\S)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 3 {
		return 0
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	if r != '、' {
		return 0
	}
	j := i + size
	if j >= len(s) {
		return 0
	}
	next, _ := utf8.DecodeRuneInString(s[j:])
	if unicode.IsSpace(next) {
		return 0
	}
	return j
}

func ordinalCnDunEnd(s string) int {
	cn := "一二三四五六七八九十百两"
	i := 0
	for _, r := range s {
		if !strings.ContainsRune(cn, r) {
			break
		}
		i += utf8.RuneLen(r)
	}
	if i == 0 {
		return 0
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	if r != '、' {
		return 0
	}
	j := i + size
	if j >= len(s) {
		return 0
	}
	next, _ := utf8.DecodeRuneInString(s[j:])
	if unicode.IsSpace(next) {
		return 0
	}
	return j
}

func ordinalParenEnd(s string) int {
	// ^[（(]\s*(\d{1,3}|[一二三四五六七八九十两]+)\s*[）)](?=\S)
	r0, n0 := utf8.DecodeRuneInString(s)
	if r0 != '（' && r0 != '(' {
		return 0
	}
	i := n0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	start := i
	cn := "一二三四五六七八九十两"
	if i < len(s) && s[i] >= '0' && s[i] <= '9' {
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i-start == 0 || i-start > 3 {
			return 0
		}
	} else {
		for _, r := range s[i:] {
			if !strings.ContainsRune(cn, r) {
				break
			}
			i += utf8.RuneLen(r)
		}
		if i == start {
			return 0
		}
	}
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	r1, n1 := utf8.DecodeRuneInString(s[i:])
	if r1 != '）' && r1 != ')' {
		return 0
	}
	j := i + n1
	if j >= len(s) {
		return 0
	}
	next, _ := utf8.DecodeRuneInString(s[j:])
	if unicode.IsSpace(next) {
		return 0
	}
	return j
}

func ordinalOptEnd(s string) int {
	r0, n0 := utf8.DecodeRuneInString(s)
	if !((r0 >= 'A' && r0 <= 'Z') || (r0 >= 'Ａ' && r0 <= 'Ｚ')) {
		return 0
	}
	r1, n1 := utf8.DecodeRuneInString(s[n0:])
	if r1 != '、' && r1 != '.' && r1 != '．' {
		return 0
	}
	j := n0 + n1
	if j >= len(s) {
		return 0
	}
	next, _ := utf8.DecodeRuneInString(s[j:])
	if unicode.IsSpace(next) {
		return 0
	}
	return j
}

func splitSubDigitGlue(s string) string {
	for round := 0; round < 8; round++ {
		next, n := splitSubDigitGlueOnce(s)
		s = next
		if n == 0 {
			break
		}
	}
	return s
}

func splitSubDigitGlueOnce(s string) (string, int) {
	var b strings.Builder
	last := 0
	n := 0
	for _, m := range reSubDigitGlue.FindAllStringSubmatchIndex(s, -1) {
		tail := s[m[6]:m[7]]
		if len(tail) > 12 {
			tail = tail[:12]
		}
		if reYearLike.MatchString(tail) {
			continue
		}
		b.WriteString(s[last:m[3]])
		b.WriteString("\n\n")
		b.WriteString(s[m[4]:m[5]])
		b.WriteByte(' ')
		last = m[5]
		n++
		// one split per pass keeps indices valid for multi-glue lines
		break
	}
	if n == 0 {
		return s, 0
	}
	b.WriteString(s[last:])
	return b.String(), n
}
