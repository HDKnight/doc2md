package markdown

import "testing"

func TestRenderHeadingAndParagraph(t *testing.T) {
	got := Render([]Block{
		{Type: "heading", Level: 1, Text: "Title"},
		{Type: "paragraph", Text: "Hello world."},
	})
	want := "# Title\n\nHello world.\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
