package feat

import (
	"strings"
	"unicode/utf8"
)

// mergeBrokenParas：段落重建（视觉断行 → 逻辑段），不是「删掉所有换行」。
//
// 对照分层（启发式打底，不做 LLM 改写）：
//  1) 坐标：上行抵满行宽；纵向间距过大则硬断；字号(em)差过大则硬断
//  2) 文本：句末强标点硬断；逗号/顿号等软收尾可并；looksNewPara / 短标签硬断
//  3) LLM：本项目不启用（避免原文漂移）
//
// 仅 para+para。用于跨页/断行碎片（如「解决思」+「想认识问题」）。
func mergeBrokenParas(segs []seg) []seg {
	if len(segs) < 2 {
		return segs
	}
	fullW := contentFullWidth(segs)
	fullChars := contentFullChars(segs)
	out := make([]seg, 0, len(segs))
	for _, s := range segs {
		if len(out) == 0 {
			out = append(out, s)
			continue
		}
		prev := &out[len(out)-1]
		if prev.kind != "para" || s.kind != "para" {
			out = append(out, s)
			continue
		}
		// —— 硬性不合并（误合并破坏更大）——
		if endsWithSentencePunct(prev.text) {
			out = append(out, s)
			continue
		}
		if largeVerticalGap(*prev, s) {
			out = append(out, s)
			continue
		}
		if emMismatch(*prev, s) {
			out = append(out, s)
			continue
		}
		if looksNewPara(s.text) {
			out = append(out, s)
			continue
		}
		if isStructuralLabel(s.text) {
			out = append(out, s)
			continue
		}
		// —— 合并门槛：上行须像「未写完的满行」——
		if !fillsContentWidth(*prev, fullW, fullChars) {
			out = append(out, s)
			continue
		}
		prev.text = joinSoft(strings.TrimRight(prev.text, " \t"), strings.TrimLeft(s.text, " \t"))
		if s.em > prev.em {
			prev.em = s.em
		}
		prev.y1 = s.y1
		prev.w = s.w
	}
	return out
}

// contentFullWidth：本页（批）段落中最大 bbox 宽，近似版心/满行宽度。
func contentFullWidth(segs []seg) float64 {
	var max float64
	for _, s := range segs {
		if s.w > max {
			max = s.w
		}
	}
	return max
}

// contentFullChars：本批段落中最长正文字符数，作无 bbox 时的满行参照。
func contentFullChars(segs []seg) int {
	var max int
	for _, s := range segs {
		if s.kind != "para" {
			continue
		}
		n := utf8.RuneCountInString(strings.TrimSpace(s.text))
		if n > max {
			max = n
		}
	}
	return max
}

// fillsContentWidth：上一句（段末行）是否抵满版心。优先用 bbox 宽，否则用字数。
func fillsContentWidth(s seg, fullW float64, fullChars int) bool {
	const ratio = 0.85
	if fullW > 0 && s.w > 0 {
		return s.w >= fullW*ratio
	}
	n := utf8.RuneCountInString(strings.TrimSpace(s.text))
	if fullChars > 0 {
		return n >= int(float64(fullChars)*ratio+0.5)
	}
	return n >= 24
}

// largeVerticalGap：间距 > 1.5× 上行高 → 新段（对照「行距阈值」）。
// 跨页时 y 坐标系重置，gap 常为负或无意义，不据此硬断（交给满宽+标点）。
func largeVerticalGap(prev, next seg) bool {
	if prev.y1 <= prev.y0 || next.y1 <= next.y0 {
		return false
	}
	h := prev.y1 - prev.y0
	gap := next.y0 - prev.y1
	if gap <= 0 {
		return false
	}
	return gap > 1.5*h
}

// emMismatch：单行段字号差 ≥30% → 结构变化（标题/图注），硬断。
// 多行块的 em 是由框高估出的行高，跨页断句时不可靠，故只对「像单行」的段生效。
func emMismatch(prev, next seg) bool {
	if prev.em <= 0 || next.em <= 0 {
		return false
	}
	if !looksSingleLineSeg(prev) || !looksSingleLineSeg(next) {
		return false
	}
	lo, hi := prev.em, next.em
	if lo > hi {
		lo, hi = hi, lo
	}
	return hi/lo >= 1.3
}

func looksSingleLineSeg(s seg) bool {
	h := s.y1 - s.y0
	if h <= 0 || s.em <= 0 {
		return false
	}
	return h <= s.em*2.0
}

func endsWithSentencePunct(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return true // 空段不并
	}
	r, _ := utf8.DecodeLastRuneInString(t)
	switch r {
	case '。', '！', '？', '!', '?', '；', ';', '…':
		return true
	case '\u201d', '\u2019', '）', ')', '」', '』', '"', '\'':
		// 引号/括号收尾：再往前看真正句末
		t2 := strings.TrimRightFunc(t, func(r rune) bool {
			switch r {
			case '"', '\'', '\u201d', '\u2019', '）', ')', '」', '』':
				return true
			default:
				return false
			}
		})
		if t2 == "" || t2 == t {
			return false
		}
		return endsWithSentencePunct(t2)
	default:
		// ，、, 等软收尾：不算句末，允许在满宽时合并
		return false
	}
}
