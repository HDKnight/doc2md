package feat

import "strings"

// normalizeHeadingLevels：阶段 B —— 仅根据已标记的 heading 序列定级。
// 阶段 A（markIsHeading / peer 补抬）不得写入最终层级；此处先清零再分配。
func normalizeHeadingLevels(segs []seg) {
	if len(segs) == 0 {
		return
	}
	for i := range segs {
		if segs[i].kind == "heading" {
			segs[i].lvl = 0
		}
	}
	assignHeadingLevels(segs)
}

// seriesRank：越小越粗（仅用于「细挂粗」；部分标题↔题干另见 shouldNestUnder）。
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
	// 无系列小节 → 挂编号项；大题 → 挂无系列小节（三查三整）
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

// assignHeadingLevels：文档序 + 父子规则 + 同系列同级；根标题从 2 起（##）。
func assignHeadingLevels(segs []seg) {
	var stack []int
	for i := range segs {
		if segs[i].kind != "heading" {
			continue
		}
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			prev, cur := &segs[top], &segs[i]
			// 同系列连续编号 → 同级兄弟
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
