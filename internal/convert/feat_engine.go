package convert

import (
	"os"
	"strings"
)

// FeatEngine selects structure JSON source for PipelineFeat.
type FeatEngine string

const (
	FeatEngineOar   FeatEngine = "oar"
	FeatEngineIndra FeatEngine = "indra"
)

// ParseFeatEngine normalizes form/env values. Empty → oar.
func ParseFeatEngine(s string) FeatEngine {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "indra", "indra-ocr", "indra_ocr":
		return FeatEngineIndra
	default:
		return FeatEngineOar
	}
}

// ResolveFeatEngine: explicit opt, else DOC2MD_FEAT_ENGINE, else oar.
func ResolveFeatEngine(explicit string) FeatEngine {
	if strings.TrimSpace(explicit) != "" {
		return ParseFeatEngine(explicit)
	}
	return ParseFeatEngine(os.Getenv("DOC2MD_FEAT_ENGINE"))
}
