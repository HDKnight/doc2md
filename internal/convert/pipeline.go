package convert

// Pipeline 为 API 兼容保留；主路径已固定为 engine。
type Pipeline string

const (
	PipelineEngine    Pipeline = "engine"
	PipelineRules     Pipeline = "rules"     // legacy alias → engine
	PipelineFeat      Pipeline = "geom"      // legacy alias → engine
	PipelineStructure Pipeline = "structure" // legacy alias → engine
)

// ParsePipeline 旧参数名全部归一到 engine。
func ParsePipeline(s string) Pipeline {
	_ = s
	return PipelineEngine
}
