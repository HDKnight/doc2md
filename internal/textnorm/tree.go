package textnorm

import (
	"strings"

	"doc2md/internal/markdown"
)

type stackEntry struct {
	level  int
	index  int
	marker bool
	once   bool
}

type outlineNode struct {
	level *int // nil => body paragraph
	text  string
	kind  string // heading|paragraph|option|answer
}

// OutlineHint forces a bookmark/title line into the heading stack (bypasses frequency).
type OutlineHint struct {
	Title string
	Level int // 1-based PDF outline level → stack level Level-1
}

// OutlineConfig tunes BuildOutline for different document modes.
type OutlineConfig struct {
	Hints []OutlineHint
	// KeepExamStemsBody：题库答案区把带 ( ) / […研] 的标记行留在正文，
	// 避免选择题干再被抬成子标题。同族序号仍按 levels[family] 同级递归。
	KeepExamStemsBody bool
}

// BuildOutline applies pdf-md style frequency + stack guardrails to plain lines.
func BuildOutline(lines []string) []markdown.Block {
	return BuildOutlineWith(lines, OutlineConfig{})
}

// BuildOutlineOpt is BuildOutline with optional bookmark anchors.
func BuildOutlineOpt(lines []string, hints []OutlineHint) []markdown.Block {
	return BuildOutlineWith(lines, OutlineConfig{Hints: hints})
}

// BuildOutlineWith is BuildOutline with full options.
func BuildOutlineWith(lines []string, cfg OutlineConfig) []markdown.Block {
	if len(lines) == 0 {
		return nil
	}
	lines = NormalizeMarkers(lines)
	lines = splitReplyTexts(lines)

	hintByNorm := map[string]OutlineHint{}
	for _, h := range cfg.Hints {
		t := normTitle(h.Title)
		if t == "" {
			continue
		}
		if _, ok := hintByNorm[t]; !ok {
			hintByNorm[t] = h
		}
	}

	classified := make([]*Marker, len(lines))
	for i, line := range lines {
		classified[i] = ClassifyMarker(line)
	}

	firstMarker := -1
	for i, m := range classified {
		if markerFamily(m) != "" {
			firstMarker = i
			break
		}
	}

	short := make([]bool, len(lines))
	for i, line := range lines {
		m := classified[i]
		prev := ""
		if i > 0 {
			prev = lines[i-1]
		}
		before := firstMarker < 0 || i < firstMarker
		short[i] = isShortTitleLine(line, prev, before, m)
	}

	nodes := make([]outlineNode, 0, len(lines))
	stack := make([]stackEntry, 0, 8)
	levels := map[string]int{}
	outlineAt := -1

	for i, line := range lines {
		text := strings.TrimSpace(line)
		m := classified[i]
		family := markerFamily(m)

		if isAnswerLabel(text) {
			// 只加粗【答案】/【解析】标签，避免整行 **…** 折行后标记错位
			nodes = append(nodes, outlineNode{text: boldAnswerLabel(text), kind: "answer"})
			continue
		}
		if m != nil && m.Family == "option" {
			nodes = append(nodes, outlineNode{text: text, kind: "option"})
			continue
		}
		if isReplyLine(text) {
			nodes = append(nodes, outlineNode{text: text, kind: "paragraph"})
			continue
		}

		asMarker := false
		onceTitle := false
		var level int

		// Bookmark / forced title: stable level, bypass frequency (层级锚点).
		if hint, ok := hintByNorm[normTitle(text)]; ok {
			level = hint.Level - 1
			if level < 0 {
				level = 0
			}
			if family != "" {
				if _, exists := levels[family]; !exists {
					levels[family] = level
				} else {
					level = levels[family]
				}
			}
			if outlineAt < 0 {
				outlineAt = i
			}
			asMarker = true
			delete(hintByNorm, normTitle(text)) // one-shot consume
		} else if short[i] {
			if outlineAt < 0 {
				level = 0
			} else {
				parent := markerHost(stack)
				start, end := sectionRange(parent, classified, levels)
				found := 0
				for c := start; c < end; c++ {
					if short[c] && c >= outlineAt {
						found++
					}
				}
				if found < 2 {
					nodes = append(nodes, outlineNode{text: text, kind: "paragraph"})
					continue
				}
				if parent == nil {
					level = 1
				} else {
					level = parent.level + 1
				}
			}
		} else if isZhang(m) {
			level = 0
			if outlineAt < 0 {
				outlineAt = i
			}
			asMarker = true
		} else if family != "" {
			// Match pdf-md: frequency ≥ 2 (or once-title) enters the stack.
			// Bare page numbers / empty bodies never become headings.
			if isNonOutlineMarker(m, text) {
				nodes = append(nodes, outlineNode{text: text, kind: "paragraph"})
				continue
			}
			if lv, ok := levels[family]; ok {
				level = lv
			} else {
				if len(stack) == 0 {
					level = 1
				} else {
					level = stack[len(stack)-1].level + 1
				}
			}
			// 特殊序号默认抬标题；仅题库答案区把考研标注题干留正文
			if cfg.KeepExamStemsBody && hasExamCite(m.Body) {
				nodes = append(nodes, outlineNode{text: text, kind: "paragraph"})
				continue
			}
			parent := hostBelow(stack, level)
			start, end := sectionRange(parent, classified, levels)
			found := 0
			for c := start; c < end; c++ {
				if markerFamily(classified[c]) == family {
					found++
				}
			}
			// 弱序号（无点/顿）：不成族、或数字不连续 → 正文；且不可走 once-title 捷径
			if m.Weak && (found < 2 || !weakIntegerHasConsecutivePeer(classified, start, end, i)) {
				nodes = append(nodes, outlineNode{text: text, kind: "paragraph"})
				continue
			}
			once := found < 2 && !m.Weak && isOnceTitle(m)
			if found < 2 && !once {
				nodes = append(nodes, outlineNode{text: text, kind: "paragraph"})
				continue
			}
			if once {
				if len(stack) == 0 {
					level = 1
				} else {
					level = stack[len(stack)-1].level
				}
				onceTitle = true
			} else {
				if _, ok := levels[family]; !ok {
					levels[family] = level
				}
				level = levels[family]
			}
			if outlineAt < 0 {
				outlineAt = i
			}
			asMarker = true
		} else {
			nodes = append(nodes, outlineNode{text: text, kind: "paragraph"})
			continue
		}

		// once-title forces following same-level markers one deeper
		for j := len(stack) - 1; j >= 0; j-- {
			if stack[j].once && !onceTitle && level == stack[j].level {
				level = stack[j].level + 1
				break
			}
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= level {
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, stackEntry{level: level, index: i, marker: asMarker, once: onceTitle})
		lv := level
		nodes = append(nodes, outlineNode{level: &lv, text: text, kind: "heading"})
	}

	return outlineToBlocks(nodes)
}

// boldAnswerLabel wraps only the 【答案】/【解析】 prefix so body text stays normal.
// 标签后补空格：CommonMark/多数渲染器要求 ** 闭合后有边界，否则 **【解析】**正文 加粗不生效。
func boldAnswerLabel(text string) string {
	text = strings.TrimSpace(text)
	for _, label := range []string{"【答案】", "【解析】"} {
		if !strings.HasPrefix(text, label) {
			continue
		}
		rest := text[len(label):]
		if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "　") {
			rest = " " + rest
		}
		return "**" + label + "**" + rest
	}
	return text
}

// weakIntegerHasConsecutivePeer：弱序号须能在同节同族里找到数字相差 1 的同伴。
func weakIntegerHasConsecutivePeer(classified []*Marker, start, end, i int) bool {
	me := markerLeadingNumber(classified[i])
	if me < 0 {
		return false
	}
	fam := markerFamily(classified[i])
	for c := start; c < end; c++ {
		if c == i || markerFamily(classified[c]) != fam {
			continue
		}
		n := markerLeadingNumber(classified[c])
		if n >= 0 && (n == me-1 || n == me+1) {
			return true
		}
	}
	return false
}

func markerLeadingNumber(m *Marker) int {
	if m == nil {
		return -1
	}
	n := 0
	digits := 0
	for _, r := range m.Token {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		digits++
	}
	if digits == 0 {
		return -1
	}
	return n
}

func isNonOutlineMarker(m *Marker, text string) bool {
	if m == nil {
		return true
	}
	if strings.TrimSpace(m.Body) == "" {
		return true
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	// Pure page numbers like "74" / "12."
	digits := 0
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			digits++
			continue
		}
		if r == '.' || r == '．' || r == '、' {
			continue
		}
		return false
	}
	return digits > 0
}

func normTitle(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), "")
	s = strings.ReplaceAll(s, "　", "")
	return s
}

func isShortTitleLine(text, prev string, beforeFirstMarker bool, m *Marker) bool {
	_ = prev
	_ = beforeFirstMarker
	text = strings.TrimSpace(text)
	if text == "" || isReplyLine(text) || m != nil || isAnswerLabel(text) {
		return false
	}
	if strings.HasSuffix(text, "。") || strings.HasSuffix(text, "！") || strings.HasSuffix(text, "？") || strings.HasSuffix(text, "；") {
		return false
	}
	// 固定篇章名；不用字数阈值抬标题
	switch text {
	case "简介", "导论", "目录", "前言", "绪论", "结语", "参考文献", "附录":
		return true
	}
	return false
}

func markerHost(stack []stackEntry) *stackEntry {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].marker {
			return &stack[i]
		}
	}
	return nil
}

func hostBelow(stack []stackEntry, level int) *stackEntry {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].level < level {
			return &stack[i]
		}
	}
	return nil
}

func sectionRange(parent *stackEntry, classified []*Marker, levels map[string]int) (int, int) {
	start := 0
	var parentLevel *int
	if parent != nil {
		start = parent.index + 1
		lv := parent.level
		parentLevel = &lv
	}
	if parentLevel == nil {
		return start, len(classified)
	}
	for i := start; i < len(classified); i++ {
		if isZhang(classified[i]) {
			return start, i
		}
		fam := markerFamily(classified[i])
		if fam == "" {
			continue
		}
		if known, ok := levels[fam]; ok && known <= *parentLevel {
			return start, i
		}
	}
	return start, len(classified)
}

func outlineToBlocks(nodes []outlineNode) []markdown.Block {
	hasZero := false
	for _, n := range nodes {
		if n.level != nil && *n.level == 0 {
			hasZero = true
			break
		}
	}
	maxHeadingLogical := 4
	if !hasZero {
		maxHeadingLogical = 5
	}

	out := make([]markdown.Block, 0, len(nodes))
	var listRun []string
	flushList := func() {
		if len(listRun) == 0 {
			return
		}
		out = append(out, markdown.Block{Type: "list", Text: strings.Join(listRun, "\n")})
		listRun = nil
	}

	for _, n := range nodes {
		if n.level == nil {
			flushList()
			switch n.kind {
			case "option":
				out = append(out, markdown.Block{Type: "paragraph", Text: n.text})
			case "answer":
				out = append(out, markdown.Block{Type: "paragraph", Text: n.text})
			default:
				out = append(out, markdown.Block{Type: "paragraph", Text: n.text})
			}
			continue
		}
		hashes := *n.level + 1
		if hasZero {
			hashes = *n.level + 2
		}
		if hashes > 6 {
			depth := *n.level - maxHeadingLogical
			if depth < 1 {
				depth = 1
			}
			listRun = append(listRun, strings.Repeat("  ", depth-1)+"- "+n.text)
			continue
		}
		flushList()
		out = append(out, markdown.Block{Type: "heading", Level: hashes, Text: n.text})
	}
	flushList()
	return dedupeNearHeadings(out)
}

func dedupeNearHeadings(blocks []markdown.Block) []markdown.Block {
	if len(blocks) == 0 {
		return blocks
	}
	out := make([]markdown.Block, 0, len(blocks))
	var lastKey string
	for _, b := range blocks {
		if b.Type != "heading" {
			lastKey = ""
			out = append(out, b)
			continue
		}
		key := strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(b.Text), " ", ""), "()（）")
		if key == lastKey {
			continue
		}
		lastKey = key
		out = append(out, b)
	}
	return out
}

func splitReplyTexts(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		text := strings.ReplaceAll(line, "答:", "答：")
		text = strings.ReplaceAll(text, "答 ：", "答：")
		parts := strings.Split(text, "答：")
		if len(parts) == 1 {
			out = append(out, strings.TrimSpace(line))
			continue
		}
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if i > 0 {
				out = append(out, "答：")
			}
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}
