package geom

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"doc2md/internal/markdown"
	"doc2md/internal/textnorm"
)

// OutlineItem mirrors a PDF bookmark for Docker-style heading mode.
type OutlineItem struct {
	Level int
	Title string
	Page  int // 1-based
}

// HeadingOptions controls Docker-aligned heading emission.
type HeadingOptions struct {
	Outline  []OutlineItem
	Enhance  bool // 无书签时：bold+题号；有书签时默认把题号升为 ####
	DocTitle string
}

// headingTitleMax：仅用于极端超长题干的兜底截断（金标 eval_tiku 保留完整题干为 ####）。
const headingTitleMax = 160

var (
	rePageNo    = regexp.MustCompile(`^[\s\-—–]*\d{1,4}[\s\-—–]*$|^第\s*\d+\s*页$`)
	reTrailPage = regexp.MustCompile(`\s+\d{1,4}$`)
	// 题号：1. / 1、 后跟非空白
	reQNum      = regexp.MustCompile(`^\d{1,3}[.．、]\s*\S`)
	reQNumDunYr = regexp.MustCompile(`^\d{1,3}、\d`)
	// 含裸数字「1坚持了」：PDF 常把 ① 抽成 ASCII 1，无点号/顿号
	reBullet     = regexp.MustCompile(`^[-*·•]\s|^[（(]\s*[一二三四五六七八九十\d]|^\d{1,3}(?:[.、)）]\s*|\s+)|^\d{1,3}\p{Han}|^[①-⑳]`)
	reLeader     = regexp.MustCompile(`\.{3,}[\s\d]*$`)
	reOptLine    = regexp.MustCompile(`^[A-ZＡ-Ｚ](?:[、.)）]\s*|\s+)`)
	reAnswerLine = regexp.MustCompile(`^【?\s*答案`)
	reQuizNoise  = regexp.MustCompile(`【|（\s*）|\(\s*\)|选择题|判断题`)
	reNumHeads   = []struct {
		re  *regexp.Regexp
		lvl int
	}{
		{regexp.MustCompile(`^第[一二三四五六七八九十百零\d]+[篇部章]`), 2},
		{regexp.MustCompile(`^[一二三四五六七八九十]{1,3}\s*[、.．]\s*\S`), 3},
		{regexp.MustCompile(`^[（(]\s*[一二三四五六七八九十]{1,3}\s*[）)]\s*\S`), 4},
		{regexp.MustCompile(`^\d{1,2}\.\d{1,2}(\.\d{1,2})*\s*[^\d.]`), 4},
	}
)

func chars(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}

func numLevel(text string) (int, bool) {
	for _, h := range reNumHeads {
		if h.re.MatchString(text) {
			return h.lvl, true
		}
	}
	return 0, false
}

func cleanLeader(s string) string {
	t := strings.TrimSpace(reLeader.ReplaceAllString(s, ""))
	if t == "" {
		t = strings.TrimSpace(s)
	}
	// Drop trailing bare page numbers left after TOC leader stripping.
	if reTrailPage.MatchString(t) && chars(t) <= 40 {
		t = strings.TrimSpace(reTrailPage.ReplaceAllString(t, ""))
	}
	return t
}

func looksLikeStem(text string) bool {
	if reQNum.MatchString(text) || reOptLine.MatchString(text) {
		return true
	}
	if reQuizNoise.MatchString(text) {
		return true
	}
	return false
}

func normHeadKey(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), "")
	return strings.ReplaceAll(s, "　", "")
}

// splitHeadingTitle 仅在极端超长时截断；优先在句号/题末空白/( )/考研引用前切开，避免在逗号处撕开题干。
func splitHeadingTitle(text string, maxRunes int) (title, rest string) {
	text = strings.TrimSpace(text)
	if maxRunes <= 0 {
		maxRunes = headingTitleMax
	}
	r := []rune(text)
	if len(r) <= maxRunes {
		return text, ""
	}
	// 1) 题末标记：( ) / （ ） / [学校…研]
	for i := maxRunes; i > maxRunes/2 && i > 20; i-- {
		chunk := string(r[:i])
		if strings.HasSuffix(chunk, "( )") || strings.HasSuffix(chunk, "（ ）") ||
			strings.HasSuffix(chunk, "（）") || strings.HasSuffix(chunk, "()") {
			return chunk, strings.TrimSpace(string(r[i:]))
		}
		if i < len(r) && r[i] == '[' && (strings.Contains(chunk, "研") || strings.Contains(chunk, "(") || strings.Contains(chunk, "（")) {
			// keep [cite] in title if nearby
		}
	}
	for i := 0; i < len(r) && i < maxRunes+40; i++ {
		if r[i] == '[' {
			// include cite through closing ]
			j := i + 1
			for j < len(r) && r[j] != ']' {
				j++
			}
			if j < len(r) && j+1 <= maxRunes+30 {
				return string(r[:j+1]), strings.TrimSpace(string(r[j+1:]))
			}
			break
		}
	}
	// 2) 句号（不含逗号——逗号切开会把叙事题干撕成半句）
	cut := maxRunes
	for i := maxRunes; i > maxRunes-24 && i > 20; i-- {
		switch r[i-1] {
		case '。', '？', '!', '！', '；', ';':
			cut = i
			goto done
		}
	}
done:
	return string(r[:cut]), strings.TrimSpace(string(r[cut:]))
}

func isQuestionStem(text string) bool {
	if reOptLine.MatchString(text) || reAnswerLine.MatchString(text) {
		return false
	}
	if !reQNum.MatchString(text) {
		return false
	}
	// 年代/区间描述不当题号：16、17世纪…
	if reQNumDunYr.MatchString(text) {
		return false
	}
	return true
}

func baseSize(lines []Line) float64 {
	type bucket struct {
		size float64
		n    int
	}
	m := map[float64]int{}
	for _, ln := range lines {
		sz := round1(ln.Em())
		m[sz] += chars(ln.Text)
	}
	best := bucket{size: 12, n: 0}
	for sz, n := range m {
		if n > best.n {
			best = bucket{size: sz, n: n}
		}
	}
	return best.size
}

func sizeLevels(lines []Line, base float64) map[float64]int {
	seen := map[float64]struct{}{}
	var bigger []float64
	for _, ln := range lines {
		sz := round1(ln.Em())
		if sz > base*1.28 {
			if _, ok := seen[sz]; !ok {
				seen[sz] = struct{}{}
				bigger = append(bigger, sz)
			}
		}
	}
	sort.Slice(bigger, func(i, j int) bool { return bigger[i] > bigger[j] })
	if len(bigger) > 3 {
		bigger = bigger[:3]
	}
	out := map[float64]int{}
	for i, s := range bigger {
		lvl := 2 + i
		if lvl > 4 {
			lvl = 4
		}
		out[s] = lvl
	}
	return out
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func newPara(prev, cur string) bool {
	prev = strings.TrimSpace(prev)
	cur = strings.TrimSpace(cur)
	if prev == "" || cur == "" {
		return false
	}
	// （1）短标题 / 一、小节 不吞下行
	if textnorm.IsShortStandaloneTitle(prev) {
		return true
	}
	// 下一行是序号/选项 → 新段（含裸数字 1坚持了、①、（2））
	if textnorm.StartsWithMarker(cur) || reBullet.MatchString(cur) || reOptLine.MatchString(cur) {
		return true
	}
	return false
}

func joinPara(parts []string) string {
	out := ""
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if out == "" {
			out = s
			continue
		}
		if strings.HasSuffix(out, "-") && len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' {
			out = out[:len(out)-1] + s
			continue
		}
		last := out[len(out)-1]
		first := s[0]
		if ((last >= 'A' && last <= 'Z') || (last >= 'a' && last <= 'z') || (last >= '0' && last <= '9')) &&
			((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
			out += " " + s
			continue
		}
		out += s
	}
	return out
}

// compactBodyLines joins broken wraps but keeps marker / short-title boundaries.
func compactBodyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if len(out) > 0 && textnorm.ShouldJoinBrokenLines(out[len(out)-1], ln) {
			out[len(out)-1] = strings.TrimRight(out[len(out)-1], " \t") + strings.TrimLeft(ln, " \t")
			continue
		}
		out = append(out, ln)
	}
	return out
}

// appendOutlinedBlocks writes BuildOutline results, nesting under parentHashes (题号 #### → 子级 #####).
func appendOutlinedBlocks(dst *[]string, outlined []markdown.Block, parentHashes int) {
	minWant := parentHashes + 1
	if parentHashes <= 0 {
		minWant = 0
	}
	if minWant > 6 {
		minWant = 6
	}
	minGot, maxGot := 0, 0
	for _, b := range outlined {
		if b.Type == "heading" && b.Level > 0 {
			if minGot == 0 || b.Level < minGot {
				minGot = b.Level
			}
			if b.Level > maxGot {
				maxGot = b.Level
			}
		}
	}
	delta := 0
	if minWant > 0 && minGot > 0 && minGot < minWant {
		delta = minWant - minGot
		// 保相对层级，避免全部顶到 ###### 后 a. 与 1. 同级
		if maxGot+delta > 6 {
			delta = 6 - maxGot
		}
		if delta < 0 {
			delta = 0
		}
	}
	for _, b := range outlined {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		switch b.Type {
		case "heading":
			lvl := b.Level + delta
			if lvl < 1 {
				lvl = 1
			}
			if lvl > 6 {
				lvl = 6
			}
			*dst = append(*dst, strings.Repeat("#", lvl)+" "+text)
		default:
			*dst = append(*dst, text)
		}
	}
}

func dedupeHeadings(blocks []string) []string {
	out := make([]string, 0, len(blocks))
	var seenHead string
	for _, b := range blocks {
		if strings.HasPrefix(b, "#") {
			if b == seenHead {
				continue
			}
			seenHead = b
		} else {
			seenHead = ""
		}
		out = append(out, b)
	}
	return out
}

// BuildHeadingMarkdown emits Markdown using Docker doc2md heading rules:
// outline ≥3 → bookmark titles only; else font-size short lines + numbered short titles.
func BuildHeadingMarkdown(pages []PageInput, opt HeadingOptions) string {
	pageCount := len(pages)
	var all []Line
	for _, p := range pages {
		if p.Diagram {
			continue
		}
		all = append(all, p.Lines...)
	}
	cleaned := DropChrome(all, pageCount)

	// Edge noise: page numbers / repeated margin texts.
	edge := map[string]int{}
	for _, ln := range cleaned {
		h := ln.PageHeight
		if h <= 0 {
			h = 1
		}
		top := ln.Y0 / h
		bot := ln.Y1 / h
		txt := cleanLeader(ln.Text)
		if txt == "" {
			continue
		}
		if top < 0.08 || bot > 0.92 {
			edge[txt]++
		}
	}
	noise := map[string]struct{}{}
	thresh := pageCount * 35 / 100
	if thresh < 2 {
		thresh = 2
	}
	for t, c := range edge {
		if rePageNo.MatchString(t) || c >= thresh {
			noise[t] = struct{}{}
		}
	}

	body := make([]Line, 0, len(cleaned))
	for _, ln := range cleaned {
		txt := cleanLeader(ln.Text)
		if txt == "" || rePageNo.MatchString(txt) {
			continue
		}
		if _, bad := noise[txt]; bad {
			continue
		}
		ln.Text = txt
		body = append(body, ln)
	}

	useOutline := len(opt.Outline) >= 3
	base := baseSize(body)
	levels := sizeLevels(body, base)

	byPage := map[int][]Line{}
	for _, ln := range body {
		byPage[ln.Page] = append(byPage[ln.Page], ln)
	}

	var blocks []string
	emitted := map[string]struct{}{}
	lastLevel := 0
	qLevel := 0
	var para []string

	flush := func() {
		if len(para) == 0 {
			return
		}
		lines := compactBodyLines(para)
		para = nil
		// 答案区走频率+栈层级递归（与无书签路径同一套 BuildOutline）
		outlined := textnorm.BuildOutlineWith(lines, textnorm.OutlineConfig{KeepExamStemsBody: true})
		appendOutlinedBlocks(&blocks, outlined, lastLevel)
	}

	head := func(lvl int, text string, isQuestion bool) {
		if isQuestion {
			if qLevel > 0 {
				lvl = qLevel
			} else if lastLevel > 0 {
				lvl = lastLevel + 1
			} else {
				lvl = 5
			}
			if lvl > 5 {
				lvl = 5
			}
			qLevel = lvl
		} else {
			qLevel = 0
			if lastLevel > 0 {
				if lvl > lastLevel+1 {
					lvl = lastLevel + 1
				}
			} else if lvl > 2 {
				lvl = 2
			}
		}
		lastLevel = lvl
		blocks = append(blocks, strings.Repeat("#", lvl)+" "+text)
	}

	for _, p := range pages {
		if p.Diagram {
			flush()
			if p.AssetMarkdown != "" {
				blocks = append(blocks, p.AssetMarkdown)
			}
			continue
		}

		skipOnce := map[string]struct{}{}
		if useOutline {
			for _, o := range opt.Outline {
				if o.Page != p.Page {
					continue
				}
				ttl := strings.TrimSpace(o.Title)
				if ttl == "" {
					continue
				}
				key := normHeadKey(ttl)
				if _, ok := emitted[key]; ok {
					continue
				}
				emitted[key] = struct{}{}
				flush()
				lvl := o.Level + 1
				if lvl > 6 {
					lvl = 6
				}
				head(lvl, ttl, false)
				skipOnce[key] = struct{}{}
			}
		}

		lines := byPage[p.Page]
		sort.SliceStable(lines, func(i, j int) bool {
			if lines[i].Y0 == lines[j].Y0 {
				return lines[i].X0 < lines[j].X0
			}
			return lines[i].Y0 < lines[j].Y0
		})
		joined := JoinLines(lines)

		for _, ln := range joined {
			// 先归一序号再判标题，避免 1。/(1)题干 漏识别
			text := textnorm.PrepOrdinalLine(ln.Text)
			if text == "" {
				continue
			}
			key := normHeadKey(text)
			if _, skip := skipOnce[key]; skip {
				delete(skipOnce, key)
				continue
			}
			// Body duplicate of an already-emitted bookmark title → skip.
			if useOutline {
				if _, ok := emitted[key]; ok {
					continue
				}
			}

			size := round1(ln.Em())
			bold := ln.Bold

			lvl := 0
			isQ := false
			rest := ""
			if !useOutline {
				if !looksLikeStem(text) && size > base*1.28 && chars(text) <= 28 {
					if lv, ok := levels[size]; ok {
						lvl = lv
					}
				}
				if lvl == 0 && chars(text) <= 40 && !looksLikeStem(text) {
					if lv, ok := numLevel(text); ok {
						lvl = lv
					}
				}
				if opt.Enhance && lvl == 0 && bold && isQuestionStem(text) && chars(text) <= 80 {
					lvl = 5
					isQ = true
				}
			} else if opt.Enhance && isQuestionStem(text) {
				// 金标：题号 #### 保留完整题干（含 ( ) / [学校…研]），不截断。
				// 仅极端超长（>headingTitleMax）才按句号兜底切开，绝不在逗号处撕开。
				if chars(text) > headingTitleMax {
					title, rem := splitHeadingTitle(text, headingTitleMax)
					text = title
					rest = rem
				}
				lvl = 5
				isQ = true
			}

			if lvl > 0 {
				flush()
				head(lvl, text, isQ)
				if rest != "" {
					para = append(para, rest)
				}
				continue
			}

			// 正文整段积攒，flush 时一次性 BuildOutline（频率≥2 + 栈递归）。
			// 不可按 newPara 逐条 flush，否则单行无法达到成族频率。
			para = append(para, text)
		}
	}
	flush()
	blocks = dedupeHeadings(blocks)
	md := tidyBlocks(blocks)
	if t := strings.TrimSpace(opt.DocTitle); t != "" {
		if !strings.HasPrefix(strings.TrimLeft(md, "\n"), "# ") {
			md = "# " + t + "\n\n" + md
		}
	}
	return textnorm.FinishMarkdown(md)
}

func prefix2(s string) string {
	r := []rune(s)
	if len(r) > 2 {
		return string(r[:2])
	}
	return s
}

func tidyBlocks(blocks []string) string {
	md := strings.Join(blocks, "\n\n")
	md = regexp.MustCompile(`\n{3,}`).ReplaceAllString(md, "\n\n")
	return strings.TrimSpace(md) + "\n"
}
