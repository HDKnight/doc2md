package convert

import "testing"

func TestParseFeatEngine(t *testing.T) {
	if ParseFeatEngine("") != FeatEngineOar {
		t.Fatal("empty -> oar")
	}
	if ParseFeatEngine("INDRA") != FeatEngineIndra {
		t.Fatal("INDRA")
	}
	if ResolveFeatEngine("oar") != FeatEngineOar {
		t.Fatal("oar")
	}
}
