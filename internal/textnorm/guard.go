package textnorm

import "doc2md/internal/markdown"

// GuardStructure is retained for compatibility with older call sites.
// Universal outline guardrails are implemented in BuildOutline (pdf-md rules).
func GuardStructure(blocks []markdown.Block) []markdown.Block {
	return blocks
}
