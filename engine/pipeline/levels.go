package pipeline

import "strings"

// seriesRank：越小越粗（仅配合 shouldNestUnder；空系列=无序号小节）。
func seriesRank(kind string) int {
	switch kind {
	case "arab_dun":
		return 1
	case "cn_dun":
		return 2
	case "arab_dot":
		return 3
	case "paren":
		return 4
	case "":
		return 0
	default:
		return 5
	}
}

func shouldNestUnder(prev, cur *seg) bool {
	// 无序号小节 → 挂编号项（「三查三整」→ (1)(2)）
	if prev.series == "" && cur.series != "" {
		return true
	}
	if cur.series == "" {
		// 不挂到 (1)/1. 下；可挂到 1、 大题或「非题型」中文小节下
		if prev.series == "arab_dun" {
			return true
		}
		if prev.series == "cn_dun" && !looksQuizPartSection(prev.text) {
			return true
		}
		return false
	}
	// 「二、简答题」与「一、」同级，不挂到上一道 1、 下
	if cur.series == "cn_dun" && looksQuizPartSection(cur.text) {
		return false
	}
	// 「一、单项选择题」下挂 1、2、题干
	if prev.series == "cn_dun" && cur.series == "arab_dun" && looksQuizPartSection(prev.text) {
		return true
	}
	return seriesRank(cur.series) > seriesRank(prev.series)
}

func looksQuizPartSection(text string) bool {
	t := strings.TrimSpace(text)
	keys := []string{
		"选择题", "单选", "多选", "不定项",
		"填空", "判断", "简答", "论述", "辨析",
		"名词解释", "材料", "分析题", "问答题", "计算题",
	}
	for _, k := range keys {
		if strings.Contains(t, k) {
			return true
		}
	}
	return false
}

func alignPeerHeadings(segs []seg) {
	for i := range segs {
		if segs[i].series == "" {
			segs[i].series, segs[i].num = seriesOf(segs[i].text)
		}
	}
	byKind := map[string][]int{}
	for i, s := range segs {
		if s.series == "" || s.num <= 0 || isOptionLine(s.text) {
			continue
		}
		byKind[s.series] = append(byKind[s.series], i)
	}
	for _, idxs := range byKind {
		if len(idxs) < 2 {
			continue
		}
		for _, i := range idxs {
			// 多句论述整段不因同族补抬
			if expandedProseBlock(segs[i].text) {
				continue
			}
			segs[i].kind = "heading"
		}
	}
}

// assignLevels：对齐旧 feat —— 同系列同级；无序号小节互为兄弟；(1) 挂其下。
func assignLevels(segs []seg) {
	for i := range segs {
		if segs[i].kind == "heading" {
			segs[i].lvl = 0
		}
	}
	var stack []int
	for i := range segs {
		if segs[i].kind != "heading" {
			continue
		}
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			prev, cur := &segs[top], &segs[i]
			if cur.series != "" && cur.series == prev.series && cur.num > 0 && prev.num > 0 {
				stack = stack[:len(stack)-1]
				continue
			}
			if shouldNestUnder(prev, cur) {
				break
			}
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			segs[i].lvl = 2
		} else {
			lvl := segs[stack[len(stack)-1]].lvl + 1
			if lvl > 6 {
				lvl = 6
			}
			segs[i].lvl = lvl
		}
		stack = append(stack, i)
	}
}
