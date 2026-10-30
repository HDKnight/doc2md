package oarx

import (
	"encoding/json"

	"doc2md/engine/pipeline"
)

type pageJSON struct {
	Blocks []pipeline.Block `json:"blocks"`
}

// ParsePageJSON parses one oar structure JSON page file.
func ParsePageJSON(raw []byte) (pipeline.Page, error) {
	var p pageJSON
	if err := json.Unmarshal(raw, &p); err != nil {
		return pipeline.Page{}, err
	}
	return pipeline.Page{Blocks: p.Blocks}, nil
}
