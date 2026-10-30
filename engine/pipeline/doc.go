// Package pipeline 是 Markdown 定级唯一实现：断行 → 特殊序号/表征定标题 → 层级。
// 不依赖 internal/feat、textnorm 的标题 API；禁止用字数抬标题，禁止按「。」拆完整题干。
package pipeline
