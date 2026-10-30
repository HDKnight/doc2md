package pdf

import "strings"

// OCRMode controls when OCR is used.
type OCRMode string

const (
	OCRAuto OCRMode = "auto"
	OCROn   OCRMode = "on"
	OCROff  OCRMode = "off"
)

// ParseOCRMode normalizes query/form values.
func ParseOCRMode(s string) OCRMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "true", "1":
		return OCROn
	case "off", "false", "0":
		return OCROff
	default:
		return OCRAuto
	}
}

// AvgCharsPerPage returns average non-space character count per page.
func AvgCharsPerPage(pages []PageText) float64 {
	if len(pages) == 0 {
		return 0
	}
	total := 0
	for _, p := range pages {
		total += len(strings.ReplaceAll(p.Text, " ", ""))
	}
	return float64(total) / float64(len(pages))
}

// NeedsOCR decides whether OCR should run.
// threshold is minimum average chars/page to treat as digital text (default 50).
func NeedsOCR(pages []PageText, mode OCRMode, threshold float64) bool {
	if threshold <= 0 {
		threshold = 50
	}
	switch mode {
	case OCROn:
		return true
	case OCROff:
		return false
	default:
		return AvgCharsPerPage(pages) < threshold
	}
}
