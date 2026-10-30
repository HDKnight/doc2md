package markdown

import (
	"strings"
)

// Render converts blocks to GitHub Flavored Markdown.
func Render(blocks []Block) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	for i, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		switch block.Type {
		case "heading":
			level := block.Level
			if level < 1 {
				level = 1
			}
			if level > 6 {
				level = 6
			}
			b.WriteString(strings.Repeat("#", level))
			b.WriteByte(' ')
			b.WriteString(text)
			b.WriteString("\n\n")
		case "table":
			b.WriteString(text)
			if !strings.HasSuffix(text, "\n") {
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		case "list":
			for _, line := range strings.Split(text, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
					b.WriteString(line)
				} else {
					b.WriteString("- ")
					b.WriteString(line)
				}
				b.WriteByte('\n')
			}
			b.WriteByte('\n')
		default: // paragraph
			b.WriteString(text)
			b.WriteString("\n\n")
		}
		_ = i
	}
	return strings.TrimSpace(b.String()) + "\n"
}
