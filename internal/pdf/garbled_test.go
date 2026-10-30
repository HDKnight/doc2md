package pdf

import "testing"

func TestIsGarbledDetectsPDFJunk(t *testing.T) {
	pages := []PageText{{
		Page: 1,
		Text: "D:20210922082453+00'00' GitBook endobj endstream /Type /Font *( *( *( j5EFQQ(j5EFQQ(pk)",
	}}
	if !IsGarbled(pages) {
		t.Fatal("expected garbled")
	}
}

func TestIsGarbledAllowsNormalText(t *testing.T) {
	pages := []PageText{{
		Page: 1,
		Text: "马克思主义基本原理是考研政治的重要组成部分。本章主要考查唯物论与辩证法。",
	}}
	if IsGarbled(pages) {
		t.Fatal("expected readable Chinese text")
	}
}
