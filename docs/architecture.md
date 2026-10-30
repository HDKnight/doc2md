# 架构

当前默认且唯一的转换路径是 `engine/`。`internal/feat`、`internal/textnorm`、`internal/geom`、`internal/indra` 仍留在仓库里，服务不再走它们。

## 总览

```
pdf ─┬─ 文字层可用 ─→ pdftotext -bbox-layout（字号 / 坐标）─┐
     └─ 扫描、乱码、字太少 ─→ oar structure JSON ──────────┼─→ engine/pipeline ─→ Markdown
docx ─→ 段落 ─────────────────────────────────────────────┤
txt  ─→ 文本行 ───────────────────────────────────────────┘
```

入口：`cmd/server` → `internal/convert` → `engine/ingest`。

| 包 | 职责 |
|----|------|
| `engine/ingest` | 按扩展名分流：PDF、DOCX、TXT |
| `engine/oarx` | 调 oar `structure --format json`，解析 bbox / type，CPU/GPU 与空闲卸载 |
| `engine/pipeline` | 断行、定标题、定层级、渲染 Markdown |
| `internal/ocr` | oar 设备选择、常驻 structure worker、Poppler 路径 |
| `internal/pdf` | 文字层抽取、是否需要 OCR 的判断 |
| `internal/docx` | DOCX 段落 |
| `internal/api` | HTTP 与页面 |

## 四步

1. **断行**  
   有坐标时，近距、同缩进的行拼在一起。半截题干（虚词收尾、括号没闭）可以跨稍大的间距续上。下一行已是新序号或选项则不拼。选择题干里的句号不拆开。

2. **标题**  
   抬成标题的是特殊序号（`1、` `1.` `一、` `（1）`）、oar 的 title 类块，以及字号明显高于正文的短行。选项行不抬。不用字数决定标题。  
   `1932.7.21`、`1949.3召开` 这类点分日期不是序号，保持正文。

3. **层级**  
   同一序号族同级。题型（如「一、单项选择题」）下面用栈挂题号。定级之后不再拆序号、不再改层级。

4. **排版**  
   `N、短名：` 后面的长论述只把冒号之前留在标题里，其余落到正文。同行粘连的 `A、xxB、yy` 拆成选项。

## 扫描件

扫描 PDF 整本送给 oar（常驻 `oar_structure_worker`，否则回退 `oar` CLI）。空闲默认 60 秒退出。设备：有可用 GPU / DirectML 用 GPU，否则多路 CPU。内存紧时设 `DOC2MD_OAR_STRUCTURE_BATCH=2`。

## 不做什么

- 不按字数抬标题或降标题。
- 不把完整题干从句号处拆成两段。
- 不用大模型改写原文。
- 不把 oar 自带的 Markdown 当作结果。
