package pipeline

import "strings"

// markIsHeading：断行之后。特殊序号 / oar type / 字号比；不用字数；不因后接选项降级。
func markIsHeading(items []item, i int, bodyEm float64) bool {
	it := items[i]
	if strings.TrimSpace(it.text) == "" {
		return false
	}
	if looksLikeDottedDate([]rune(strings.TrimSpace(it.text))) {
		return false
	}
	if isOptionLine(it.text) {
		return false
	}
	// 多句论述整段不当标题（含误标的 OCR title）
	if expandedProseBlock(it.text) {
		return false
	}
	// 短标题：置信 OCR title
	if typeIsTitle(it.typ) {
		return true
	}
	if isArabicStem(it.text) || isStructuralLabel(it.text) {
		return true
	}
	if bodyEm < 1 {
		bodyEm = it.em
	}
	if bodyEm > 0 && it.em/bodyEm >= 1.14 && !looksProse(it.text) {
		return true
	}
	return false
}

// expandedProseBlock：至少两句（≥2 个「。」）的论述整段 → 非标题。
// 不用字数。选择题干带（A）/（ ）答案空除外。
func expandedProseBlock(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" {
		return false
	}
	if hasChoiceAnswerBlank(t) {
		return false
	}
	return strings.Count(t, "。") >= 2
}

// hasChoiceAnswerBlank：题干里的（A）/（ ）类作答括号。
func hasChoiceAnswerBlank(text string) bool {
	r := []rune(text)
	for i := 0; i < len(r); i++ {
		if r[i] != '（' && r[i] != '(' {
			continue
		}
		for j := i + 1; j < len(r); j++ {
			if r[j] == '）' || r[j] == ')' {
				inner := strings.TrimSpace(string(r[i+1 : j]))
				if inner == "" {
					return true
				}
				ir := []rune(inner)
				if len(ir) <= 2 && isOptLetter(ir[0]) {
					return true
				}
				break
			}
			if r[j] == '（' || r[j] == '(' || r[j] == '。' {
				break
			}
		}
	}
	return false
}

func looksProse(text string) bool {
	t := strings.TrimSpace(text)
	if !strings.Contains(t, "。") {
		return false
	}
	// 仅末尾一个句号且是序号行 → 不当散文
	if strings.Count(t, "。") == 1 && strings.HasSuffix(strings.TrimRight(t, "\"'”’）)」』"), "。") {
		if isArabicStem(t) || isStructuralLabel(t) {
			return false
		}
	}
	return true
}
