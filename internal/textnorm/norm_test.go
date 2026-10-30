package textnorm

import (
	"strings"
	"testing"
)

func TestNormalizeKangxi(t *testing.T) {
	in := "⽬录⾼校⼈类宣⾔"
	got := Normalize(in)
	want := "目录高校人类宣言"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestJoinWrappedLines(t *testing.T) {
	in := "马克思主义是科学的世界观和方\n法论，是反映客观世界的真理。\n\n1. 下一题"
	got := JoinWrappedLines(in)
	if !strings.Contains(got, "方法论") {
		t.Fatalf("expected joined CJK, got %q", got)
	}
	if !strings.Contains(got, "1. 下一题") {
		t.Fatalf("expected structural line kept, got %q", got)
	}
}
