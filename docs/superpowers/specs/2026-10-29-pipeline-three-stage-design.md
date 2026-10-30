# 三阶段管道（断行 → 标题 → 层级）

**方案 A：** 保留 `feat`（OCR）与 `textnorm`（纯文本）两套入口；按阶段收口，不合并引擎。

## 路径

| 路径 | 阶段 |
|------|------|
| OCR / `feat` | ① 断行 → ② 是否标题 → ③ 层级 |
| 纯文本 / `textnorm` | ② 是否标题 → ③ 层级（`ToBlocksPlain` → `BuildOutline`） |

定级后只做无害排版（`FinishMarkdown`），不再拆序号。

## OCR（feat）阶段边界

1. **断行（优先）**：页内 soft-wrap **先于**定标题；再粘连切开、断章合并。半截题干必须先拼完整。
2. **标题**：按**特殊序号**（`1、`/`1.`/`一、`/`（一）` 等）识别；选项行（A/B/C/D）不抬；版式 type/字号补充。**不用字数**。
3. **层级**：同族 peer 对齐 + `normalizeHeadingLevels`。

## 纯文本（textnorm）

- 先 `PreparePlain` 断行/括号拼接，再 `BuildOutline`。
- 标题靠特殊序号 + 频率≥2 + 栈；选项不成标题。

## 明确不做

- 不把 feat 并进 textnorm。
- 不改 oar worker / DirectML 热路径。
- 不新增题型专用开关。
