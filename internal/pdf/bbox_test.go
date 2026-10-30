package pdf

import (
	"strings"
	"testing"
)

const sampleBBoxHTML = `<!DOCTYPE html>
<html><body><doc>
  <page width="594.960000" height="840.960000">
    <flow>
      <block xMin="34.99" yMin="53.21" xMax="189.04" yMax="83.98">
        <line xMin="34.99" yMin="53.21" xMax="189.04" yMax="83.98">
          <word xMin="34.99" yMin="53.21" xMax="189.04" yMax="83.98">一、单项选择题</word>
        </line>
      </block>
      <block xMin="34.99" yMin="141.05" xMax="557.23" yMax="153.05">
        <line xMin="34.99" yMin="141.05" xMax="557.23" yMax="153.05">
          <word xMin="34.99" yMin="141.05" xMax="43.99" yMax="153.05">1.</word>
          <word xMin="47.00" yMin="141.05" xMax="557.23" yMax="153.05">实践是认识的基础</word>
        </line>
      </block>
    </flow>
  </page>
  <page width="594.960000" height="840.960000">
    <flow>
      <block xMin="40" yMin="60" xMax="200" yMax="80">
        <line xMin="40" yMin="60" xMax="200" yMax="80">
          <word xMin="40" yMin="60" xMax="200" yMax="80">二、简答题</word>
        </line>
      </block>
    </flow>
  </page>
</doc></body></html>`

func TestParseBBoxLayoutHTML(t *testing.T) {
	pages, err := ParseBBoxLayoutHTML([]byte(sampleBBoxHTML))
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages=%d", len(pages))
	}
	if pages[0].Width < 590 || pages[0].Height < 840 {
		t.Fatalf("page0 size: %+v", pages[0])
	}
	if len(pages[0].Lines) != 2 {
		t.Fatalf("page0 lines=%d %#v", len(pages[0].Lines), pages[0].Lines)
	}
	l0 := pages[0].Lines[0]
	if l0.Text != "一、单项选择题" || l0.Page != 1 {
		t.Fatalf("line0: %+v", l0)
	}
	if l0.X0 <= 0 || l0.Y0 <= 0 || l0.X1 <= l0.X0 {
		t.Fatalf("bbox: %+v", l0)
	}
	l1 := pages[0].Lines[1]
	if !strings.HasPrefix(l1.Text, "1.") || !strings.Contains(l1.Text, "实践是认识的基础") {
		t.Fatalf("joined words: %q", l1.Text)
	}
	if len(pages[1].Lines) != 1 || pages[1].Lines[0].Text != "二、简答题" {
		t.Fatalf("page1: %#v", pages[1].Lines)
	}
	if pages[1].Lines[0].Page != 2 {
		t.Fatalf("page number: %d", pages[1].Lines[0].Page)
	}
}
