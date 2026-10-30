package textnorm

import "testing"

func TestShortIntegerTitleStandalone(t *testing.T) {
	if !IsShortStandaloneTitle("1 核心要义与内涵") {
		t.Fatal("short integer title should be standalone")
	}
	if !IsShortStandaloneTitle("2 核心内容:“八个明确”") {
		t.Fatal("short integer title with quotes")
	}
	if !IsShortStandaloneTitle("（2） “三查三整”的方法步骤") {
		t.Fatal("quoted section title should be standalone")
	}
	// 题干折行残片 / 冒号后仍有论述 → 不挡拼接
	if IsShortStandaloneTitle("4、它以共产党") {
		t.Fatal("mid-wrap stem fragment should not block join")
	}
	if IsShortStandaloneTitle("(一) 历史意义:“政治工作是红军的生命线”论断符合马克思主义的基本") {
		t.Fatal("colon prose mid-wrap should not be standalone")
	}
	if IsShortStandaloneTitle("四、 明确全面深化改革总目标是完善和发展中国特色社会主义制度、推进国") {
		t.Fatal("dun-list prose fragment should not be standalone")
	}
}
