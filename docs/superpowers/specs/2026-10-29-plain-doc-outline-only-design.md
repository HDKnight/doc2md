# 定级后无害排版（纯文档 + OCR 共用原则）

**原则：** 粘连/断行在定级前；`BuildOutline` 定级后只做无害排版（`FinishMarkdown`）。

## 纯文档（docx）

```
段落行 → ToBlocksPlain（仅定级）→ Render → FinishMarkdown
```

## OCR / rules / geom

```
PreparePlain + 粘连切开 → BuildOutline → Render → FinishMarkdown
```

`FormatMarkdownOrdinals` 仅用于尚未定级的扁平试题串（或测试）；定级后的 MD 禁止再拆序号。
