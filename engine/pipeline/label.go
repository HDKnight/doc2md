package pipeline

import "strings"

// splitShortLabelBodies：N、短名：/短名。长论述 → 标题只到分隔符，其余改正文。
func splitShortLabelBodies(segs []seg) []seg {
	out := make([]seg, 0, len(segs)+2)
	for _, s := range segs {
		head, body, ok := splitLabelColon(s.text)
		if !ok {
			head, body, ok = splitLabelPeriod(s.text)
		}
		if !ok {
			out = append(out, s)
			continue
		}
		h := s
		h.text = head
		h.kind = "heading"
		h.series, h.num = seriesOf(head)
		out = append(out, h)
		b := s
		b.text = body
		b.kind = "para"
		b.lvl = 0
		b.series, b.num = "", 0
		out = append(out, b)
	}
	return out
}

// splitLabelPeriod：(1)短名。后文… → 标题到句号；选择题干（它是指/答案空）不拆。
// 仅 parenthesis 序号；一、/1、整段论述靠 expandedProse，避免「二、…矛盾。必须」被误拆成短标题。
func splitLabelPeriod(text string) (head, body string, ok bool) {
	t := strings.TrimSpace(text)
	ser, _ := seriesOf(t)
	if ser != "paren" {
		return "", "", false
	}
	if hasChoiceAnswerBlank(t) {
		return "", "", false
	}
	idx := strings.Index(t, "。")
	if idx < 0 {
		return "", "", false
	}
	before := strings.TrimSpace(t[:idx])
	after := strings.TrimSpace(t[idx+len("。"):])
	if after == "" {
		return "", "", false
	}
	// 句号前已有句号 → 多句论述，不按短标题拆
	if strings.Contains(before, "。") {
		return "", "", false
	}
	mBody := markerBody(before)
	if mBody == "" {
		return "", "", false
	}
	// 标签侧已有逗号 → 像叙述句，不是「短名。」（比冒号拆更严）
	if strings.Contains(mBody, "，") || strings.Contains(mBody, ",") {
		return "", "", false
	}
	// 选择题干续语不拆
	for _, p := range []string{"它是指", "也就是", "即是", "即指", "包括"} {
		if strings.HasPrefix(after, p) {
			return "", "", false
		}
	}
	head = before + "。"
	body = after
	return head, body, true
}

func splitLabelColon(text string) (head, body string, ok bool) {
	t := strings.TrimSpace(text)
	ser, _ := seriesOf(t)
	if ser != "arab_dun" && ser != "arab_dot" && ser != "cn_dun" && ser != "paren" {
		return "", "", false
	}
	// 找第一个中文/英文冒号
	idx := strings.Index(t, "：")
	sepLen := len("：")
	if idx < 0 {
		idx = strings.Index(t, ":")
		sepLen = 1
	}
	if idx < 0 {
		return "", "", false
	}
	before := strings.TrimSpace(t[:idx])
	after := strings.TrimSpace(t[idx+sepLen:])
	if after == "" {
		return "", "", false
	}
	// 冒号前已是完整叙述句（含句号）→ 不拆（避免误伤）
	if strings.Contains(before, "。") || strings.Contains(before, "？") || strings.Contains(before, "?") {
		return "", "", false
	}
	// 冒号前须仍是「序号+短标签」形态：去掉序号后不应像长论述（用逗号密度作结构启发，不用字数阈值抬题）
	mBody := markerBody(before)
	if mBody == "" {
		return "", "", false
	}
	// 标签侧含多个逗号 → 更像题干叙述，不拆
	if strings.Count(mBody, "，")+strings.Count(mBody, ",") >= 2 {
		return "", "", false
	}
	head = strings.TrimSpace(t[:idx+sepLen])
	body = after
	return head, body, true
}

func markerBody(text string) string {
	t := strings.TrimSpace(text)
	r := []rune(t)
	if len(r) == 0 {
		return ""
	}
	i := 0
	for i < len(r) && r[i] >= '0' && r[i] <= '9' {
		i++
	}
	if i > 0 && i < len(r) && (r[i] == '、' || r[i] == '.' || r[i] == '．') {
		return strings.TrimSpace(string(r[i+1:]))
	}
	if r[0] == '（' || r[0] == '(' {
		j := 1
		for j < len(r) && r[j] != '）' && r[j] != ')' {
			j++
		}
		if j < len(r) {
			return strings.TrimSpace(string(r[j+1:]))
		}
	}
	cn := "一二三四五六七八九十"
	if strings.ContainsRune(cn, r[0]) && len(r) > 1 && (r[1] == '、' || r[1] == '.') {
		return strings.TrimSpace(string(r[2:]))
	}
	return t
}
