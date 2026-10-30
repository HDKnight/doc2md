# 轻量全盘重写（定级主线）

**日期：** 2026-10-29  
**目标：** 扫描 PDF 只用 **oar structure JSON（bbox/type）表征**；断行与定标题都靠表征 + 特殊序号；按三步输出标准 Markdown。

---

## 1. 范围

### 做

| 项 | 说明 |
|----|------|
| 输入 | `.pdf` / `.docx` / `.txt`（图片后续可接 OCR） |
| 扫描/图片 PDF | **oar** `structure --format json`（bbox、type） |
| 纯文字 PDF | **不走 OCR**；`pdftotext -bbox-layout` 行框 → 用**字号/行高（em）+ 坐标**断行与定标题 + 特殊序号 |
| 表征用途 | 有几何时：断行用近距/缩进；定标题用 type（oar）或字号比正文 + 特殊序号 |
| 设备 | CPU / GPU 自动判断；warm + 空闲卸载 |
| 输出 | 标准 Markdown |

### 明确删除（不要、不留默认、不留平行主路径）

| 旧管线 | 含义 | 处理 |
|--------|------|------|
| **rules** | OCR 行 → textnorm 规则定级 | **去掉** |
| **structure MD** | oar 原生 Markdown 直接当结果 | **去掉** |

PDF 默认且唯一识别路径 = **structure JSON → pipeline v2**。

### 不做

- 用字数抬/降标题；按 `。` 拆完整序号行。
- LLM 改写；换 OCR 引擎。

### 保留

- oar CLI / structure worker、device 自动、idle kill。
- docx/txt：无 JSON 时走「行文本 → 同一套 pipeline」（无 bbox 则断行信号变弱，标题仍靠特殊序号）。

---

## 2. 架构

```
pdf ─┬─ 有可用文字层 ─→ pdftotext 行框（字号/坐标）─┐
     └─ 扫描/乱码/少字 ─→ oar structure JSON ──────┼─→ Pipeline v2 ─→ Markdown
docx ─→ 段落行 ──────────────────────────────────┤
txt  ─→ 文本行 ──────────────────────────────────┘
```

**Pipeline v2（唯一后处理）：**

```
① 断行：表征几何 + 结构信号（只拼；选项粘连可切开；不拆整句题干）
② 标题：特殊序号 + oar type/字号（选项不抬；不用字数）
③ 层级：同族 peer + 栈
④ 无害排版（不改大纲）
```

---

## 3. 规则（强制）

### 3.1 断行（表征优先）

- 有 bbox：近距、同缩进 → soft-wrap；半截序号题干（虚词尾/未闭合括号等）**放宽间距**后续拼；下一行是新序号/选项则不拼。
- 页内先做阅读序（双栏则左栏后右栏，栏内按 y/x）。
- `N、短名：` 后长论述 → 标题只到冒号，其余正文（不按 `。` 拆选择题干）。
- `A、xxB、yy` 切开选项。
- oar：默认**整本一次**送 warm/CLI（worker 内仍按页推理）；内存紧时 `DOC2MD_OAR_STRUCTURE_BATCH=2`。

### 3.2 标题（表征 + 序号）

- 抬：特殊序号；oar title 类 type；字号明显高于正文（几何比，非字数）。
- 不抬：选项行、裸页码。
- **禁止**字数阈值。

### 3.3 层级

- 同系列同级；题型压题号用栈。
- 定级后不再拆序号、不改层级。

---

## 4. 代码落点（全新目录，不进 `internal/`）

**原则：新逻辑全部写在仓库根下 `engine/`；不往 `internal/` 里塞新定级代码，避免和旧实现混淆。**

```
doc2md/
  engine/                 ← 新主线（唯一实现）
    ingest/               输入：pdf→oar JSON；docx/txt→行
    oarx/                 device / warm / idle / 解析 structure JSON
    pipeline/             断行 → 标题 → 层级 → 渲染
  internal/               ← 旧代码区；定级相关只删不增
  cmd/server/             仍从这里启动；改为调用 engine
```

| 新包 | import 路径（示例） | 职责 |
|------|---------------------|------|
| 摄入 | `doc2md/engine/ingest` | pdf / docx / txt |
| oar | `doc2md/engine/oarx` | GPU/CPU、warm、JSON |
| **定级** | `doc2md/engine/pipeline` | **只在这里**写三步规则 |
| 出口 | `cmd` 或极薄 `convert` | 调用 `engine/*`；去掉 rules / structure MD |

**禁止：**

- 在 `internal/feat`、`internal/textnorm`、`internal/geom`、`internal/indra` **新增**定级/断行规则。
- 新 `engine/pipeline` **依赖**旧 `feat.MarkdownFromPages` / `textnorm.BuildOutline` 冒充重写。

**过渡期：** `internal/*` 旧管线可暂时保留编译；默认与新功能只走 `engine/`。迁完测试后删除旧定级包。

---

## 5. 设备

显式 env → GPU/DirectML 可用则 GPU → 否则 CPU；warm 空闲默认 60s 杀进程；失败回退 CLI。

---

## 6. 验收

1. PDF 只走 structure JSON，不再出现 rules / 原生 structure MD 默认结果。  
2. `54、……方法。它是指（A）。` 不被拆开。  
3. 几何允许时半截折行能拼回。  
4. 同系列不因长短分裂。  
5. 测试钉死 §3；无字数抬标题。

---

## 7. 迁移（已落地初版）

1. ✅ `engine/pipeline` + 契约测试（句号不拆、半截拼、长短同抬、选项不抬）。  
2. ✅ `convert.Convert` → `engine/ingest`（pdf=oar JSON，docx/txt=行）。  
3. ✅ rules / structure MD 不再作为可切换主路径（`ParsePipeline` 恒为 engine）。  
4. ⏳ `internal/feat`、`textnorm`、`geom` 定级代码尚未物理删除（仍可编译，但默认不走）。  
5. ⏳ UI 文案可能仍展示旧 pipeline 选项。

---

## 8. 非目标

- 替换 oar；完美任意位置语义断行；多管线并存。

---

## 9. 已确认（本次）

- **只要** oar structure JSON（bbox/type）表征定标题、合并断行。  
- **rules / structure MD 两条不要。**  
- docx/txt 仍输入，走同一 pipeline（无 bbox）。
