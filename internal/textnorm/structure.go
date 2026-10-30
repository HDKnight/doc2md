package textnorm

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"doc2md/internal/markdown"
)

var (
	reTOCNumOnly  = regexp.MustCompile(`^\d+(\.\d+){1,4}$`)
	reEndsWithNum = regexp.MustCompile(`\d+$`)
)

// PreparePlain normalizes and de-wraps extracted plain text.
func PreparePlain(text string) string {
	return fixStemFragments(fixChoiceBlankBreaks(joinDateBreaks(JoinWrappedLines(Normalize(text)))))
}

// ToBlocks 面向 OCR/扫描件扁平文本：①断行/粘连切开 → ②③标题+层级（BuildOutline）。
func ToBlocks(text string) []markdown.Block {
	text = PreparePlain(text)
	lines := splitNonEmpty(text)
	lines = stripTOCLines(lines)
	lines = expandGluedLines(lines)
	lines = splitGluedOptionLines(lines)
	return BuildOutline(lines)
}

// ToBlocksPlain 面向纯文本文档：只做 ②标题识别 + ③层级（无 OCR 断行）。
func ToBlocksPlain(lines []string) []markdown.Block {
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		ln = PrepOrdinalLine(Normalize(ln))
		if ln == "" {
			continue
		}
		out = append(out, ln)
	}
	out = stripTOCLines(out)
	return BuildOutline(out)
}

// FinishPlainMarkdown 纯文本定级后的无害排版，等同 FinishMarkdown。
func FinishPlainMarkdown(md string) string {
	return FinishMarkdown(md)
}

func splitNonEmpty(text string) []string {
	raw := strings.Split(text, "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if reTOCNumOnly.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return out
}

// stripTOCLines drops a leading 目录 block (pdf-md rule, text-only approximation).
func stripTOCLines(lines []string) []string {
	start := -1
	for i, line := range lines {
		key := strings.ReplaceAll(strings.TrimSpace(line), " ", "")
		if key == "目录" || strings.HasPrefix(key, "目录") {
			start = i
			break
		}
	}
	if start < 0 {
		return lines
	}
	end := start + 1
	for end < len(lines) {
		line := strings.TrimSpace(lines[end])
		n := utf8.RuneCountInString(line)
		hasLeaders := strings.Count(line, ".") >= 3 || strings.Contains(line, "…") || strings.Contains(line, "·")
		endsPage := reTOCNumOnly.MatchString(line) || (reEndsWithNum.MatchString(line) && hasLeaders)
		if n > 40 && !endsPage && !hasLeaders {
			break
		}
		end++
		// safety: don't eat whole document
		if end-start > 80 {
			break
		}
	}
	out := make([]string, 0, len(lines)-(end-start)+1)
	out = append(out, lines[:start]...)
	out = append(out, lines[end:]...)
	return out
}

// expandGluedLines splits common glued heading+body artifacts.
// ExpandGluedLines splits chapter titles glued to 简介/导论 etc.
func ExpandGluedLines(lines []string) []string {
	return expandGluedLines(lines)
}

func expandGluedLines(lines []string) []string {
	lines = splitGluedShortTitles(lines)
	out := make([]string, 0, len(lines)+4)
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if i := strings.Index(text, "简介"); i > 0 {
			before := strings.TrimSpace(text[:i])
			after := strings.TrimSpace(text[i+len("简介"):])
			if after != "" && strings.HasPrefix(before, "第") && strings.Contains(before, "章") {
				out = append(out, before, "简介", after)
				continue
			}
		}
		if strings.HasPrefix(text, "简介") && utf8.RuneCountInString(text) > 4 {
			rest := strings.TrimSpace(text[len("简介"):])
			if rest != "" {
				r, _ := firstRune(rest)
				if r >= 0x4e00 && r <= 0x9fff {
					out = append(out, "简介", rest)
					continue
				}
			}
		}
		out = append(out, text)
	}
	return out
}

// splitGluedShortTitles 拆 OCR 漏换行：短标题与正文粘在同一行。
// 仅当同族 1./1、 成族（≥2，中间可夹正文；序号回绕则新开一段），
// 且段内已有干净短标题可作长度参照时才拆，避免误伤长题干。
func splitGluedShortTitles(lines []string) []string {
	if len(lines) < 2 {
		return lines
	}
	type hit struct {
		idx    int
		m      *Marker
		num    int
		family string
	}
	var hits []hit
	for i, ln := range lines {
		m := ClassifyMarker(strings.TrimSpace(ln))
		if m == nil {
			continue
		}
		fam := markerFamily(m)
		if fam != "integer_dot" && fam != "integer_dun" {
			continue
		}
		hits = append(hits, hit{idx: i, m: m, num: markerInt(m.Token), family: fam})
	}
	if len(hits) < 2 {
		return lines
	}

	// 按出现顺序分段：换族或序号明显回绕 → 新段
	var runs [][]hit
	cur := []hit{hits[0]}
	for _, h := range hits[1:] {
		prev := cur[len(cur)-1]
		wrap := h.num > 0 && prev.num > 0 && h.num+1 < prev.num
		if h.family != prev.family || wrap {
			runs = append(runs, cur)
			cur = []hit{h}
			continue
		}
		cur = append(cur, h)
	}
	runs = append(runs, cur)

	splitAt := map[int]int{} // line idx → body rune split
	for _, run := range runs {
		if len(run) < 2 {
			continue
		}
		// 同族已完整的标题正文，用作前缀尺（结构完整，非字数）
		var peerBodies []string
		for _, h := range run {
			line := strings.TrimSpace(lines[h.idx])
			if IsShortStandaloneTitle(line) {
				if b := strings.TrimSpace(h.m.Body); b != "" {
					peerBodies = append(peerBodies, b)
				}
			}
		}
		for _, h := range run {
			body := strings.TrimSpace(h.m.Body)
			if body == "" || IsShortStandaloneTitle(strings.TrimSpace(lines[h.idx])) {
				continue
			}
			at := 0
			for _, pb := range peerBodies {
				if pb != "" && strings.HasPrefix(body, pb) {
					pbLen := utf8.RuneCountInString(pb)
					if utf8.RuneCountInString(body) > pbLen {
						at = pbLen
						break
					}
				}
			}
			if at == 0 {
				at = splitGluedTitleByStructure(body)
			}
			if at > 0 {
				splitAt[h.idx] = at
			}
		}
	}
	if len(splitAt) == 0 {
		return lines
	}

	out := make([]string, 0, len(lines)+len(splitAt))
	for i, ln := range lines {
		at, ok := splitAt[i]
		if !ok {
			out = append(out, ln)
			continue
		}
		m := ClassifyMarker(strings.TrimSpace(ln))
		if m == nil {
			out = append(out, ln)
			continue
		}
		body := strings.TrimSpace(m.Body)
		br := []rune(body)
		if at <= 0 || at >= len(br) {
			out = append(out, ln)
			continue
		}
		titleBody := string(br[:at])
		rest := strings.TrimSpace(string(br[at:]))
		out = append(out, joinMarkerBody(m.Token, titleBody, ln))
		if rest != "" {
			out = append(out, rest)
		}
	}
	return out
}

// splitGluedTitleByStructure：在标题收尾词后切开粘连正文（不用字数窗）。
// 按「正文起头强度」选切点：引号/“这次|党内…”优先于「思想|的对立」误切。
func splitGluedTitleByStructure(body string) int {
	body = strings.TrimSpace(body)
	if body == "" {
		return 0
	}
	br := []rune(body)
	bestEnd, bestScore := 0, 0
	for _, suf := range titleEndSuffixes {
		sr := []rune(suf)
		if len(sr) == 0 || len(sr) >= len(br) {
			continue
		}
		for i := 0; i+len(sr) < len(br); i++ {
			if string(br[i:i+len(sr)]) != suf {
				continue
			}
			end := i + len(sr)
			left := string(br[:end])
			right := string(br[end:])
			if strings.ContainsAny(left, "。！？；;，,") {
				continue
			}
			score := gluedBodyStartScore(right)
			if score == 0 {
				continue
			}
			// 更高分优先；同分取更长标题（更右的收尾，且同位置更长后缀已由列表顺序保证）
			if score > bestScore || (score == bestScore && end > bestEnd) {
				bestScore, bestEnd = score, end
			}
		}
	}
	return bestEnd
}

func gluedBodyStartScore(right string) int {
	right = strings.TrimSpace(right)
	if right == "" {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(right)
	switch r {
	case '“', '"', '《', '「':
		return 3
	}
	for _, p := range []string{
		"这次", "这是", "这个", "这种", "这些", "整风", "党内", "党的", "我国", "我们",
		"必须", "坚持", "贯彻", "实行", "开展", "通过", "为了", "由于", "如果", "然而",
		"但是", "因此", "所以", "首先", "其次", "最后", "同时", "此外", "另外",
	} {
		if strings.HasPrefix(right, p) {
			return 2
		}
	}
	if unicode.Is(unicode.Han, r) || unicode.IsLetter(r) || unicode.IsNumber(r) {
		// 「的|对立」弱续行，避免 …不同思想|的对立 误切
		if r == '的' || r == '了' || r == '着' || r == '过' {
			return 0
		}
		// 「思想|政治」「思想|工作」等是词内续写，不是标题收尾后的正文
		for _, cont := range []string{"政治", "工作", "路线", "认识", "道德", "教育", "建设", "武装"} {
			if strings.HasPrefix(right, cont) {
				return 0
			}
		}
		return 1
	}
	return 0
}

func joinMarkerBody(token, body, orig string) string {
	token = strings.TrimSpace(token)
	body = strings.TrimSpace(body)
	trim := strings.TrimSpace(orig)
	if strings.HasPrefix(trim, token+" ") || strings.HasPrefix(trim, token+"　") {
		return token + " " + body
	}
	return token + body
}

func markerInt(token string) int {
	n := 0
	for _, r := range token {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			if n > 999 {
				return n
			}
			continue
		}
		if n > 0 {
			break
		}
	}
	return n
}
