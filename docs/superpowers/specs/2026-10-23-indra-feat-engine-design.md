# 表征引擎分支：indra（旁路兼容 oar）

> 日期：2026-10-23  
> 状态：已确认并实现  
> 约束：**不修改**已稳定的 oar structure / `runOarFeatPDF` / `internal/feat` 成稿逻辑；indra 为独立旁路。

---

## 1. 目标

- 管线仍为「表征」（`pipeline=geom|feat|…`）。
- 增加 **表征引擎** 选择：`oar`（**默认**）| `indra`。
- indra 产出 JSON → **适配**为现有 `[]feat.Page` → 调用不变的 `feat.MarkdownFromPages`。
- UI 默认管线改为「表征」；引擎默认 `oar`。

非目标：改 oar CLI 调用、改 feat 标题/粘连算法、用 indra 替换 rules/structure 管线。

---

## 2. 架构

```
PDF bytes
   │
   ├─ pipeline=feat + engine=oar  ──► runOarFeatPDF（现有，不动）
   │                                      └─ oar structure --format json
   │                                      └─ feat.MarkdownFromPages
   │
   └─ pipeline=feat + engine=indra ─► runIndraFeatPDF（新建）
                                          └─ indra-ocr --format json --mode ocr
                                          └─ indra.ToFeatPages（字段/类型映射）
                                          └─ feat.MarkdownFromPages（同一套）
```

分流只发生在 `convertPDF` 的 `PipelineFeat` case：按 `opt.FeatEngine` 二选一；**不**改 `runOarStructure` 本体。

---

## 3. API / Options / UI

| 项 | 说明 |
|----|------|
| `Options.FeatEngine` | `""`/`oar` → oar；`indra` → indra（大小写不敏感） |
| 表单字段 | `feat_engine`（与 `pipeline` 并列） |
| UI | 「表征引擎」下拉：`oar`（默认）/ `indra`；仅当管线=表征时启用（或始终可见但非表征时忽略） |
| 默认管线 | `index.html` 中 `pipeline` 默认选中 `geom`（表征） |
| 统计 | `OCRStats.Model` 形如 `feat-indra;dpi=…`（与现有 `feat-json;dpi=…;cpu…` 并列，不混用） |
| Engine 名 | Result.Engine 用 `ocr-feat-indra`（oar 路径保持 `ocr-feat`） |

环境变量（可选，表单优先）：`DOC2MD_FEAT_ENGINE=oar|indra`。

---

## 4. indra 调用（新建 `internal/indra`）

- 解析二进制：`DOC2MD_INDRA` → `.venv-indra\Scripts\indra-ocr.exe` → PATH。
- 命令示意：  
  `indra-ocr <pdf> --format json --mode ocr --dpi <n> --model-dir <v3.1.1> --out <dir>`  
  （`--mode ocr`；dpi 默认 **200**（`DOC2MD_INDRA_DPI`）；模型默认 HF **`v3.1.1`**（含 `rec_cjk`，中文明显好于包内默认 v1.1.0）。）
- 输出为 `.jsonl`：优先用 `lines[]` 适配 feat；无 lines 再退 `blocks[]`。
- 跑前同样 `ocr.DefaultPool().Close()`，避免 PP-OCR 常驻拖慢（与 oar 旁路一致策略，写在 `runIndraFeatPDF` 内，不改 oar 函数）。

失败：indra 未安装 → 明确错误（提示 `.venv-indra` / `DOC2MD_INDRA`），**不**静默回退 oar（避免「以为开了 indra 实际还是 oar」）。

---

## 5. JSON 适配（`indra.ToFeatPages`）

indra block → `feat.Block`：

| indra | feat |
|-------|------|
| `text` | `content`（trim；空则跳过） |
| `bbox` | `bbox`（同 `[x0,y0,x1,y1]`） |
| （数组顺序） | `order` = 1-based 序号 |
| `label` | `type`（见下表映射后再 ToLower） |

类型映射（写入 `type` 的值）：

| indra `label` | feat `type` |
|---------------|-------------|
| `Section-Header` | `paragraph_title` |
| `Table-Of-Contents` | `paragraph_title` |
| `Text` | `text` |
| `List-Group` | `text` |
| `Caption` | `text` |
| `Footnote` | `text` |
| `Table` | `table`（feat 现多当正文处理亦可；先标 `table` 与 oar 对齐） |
| `Form` | `text` |
| `Page-Header` / `Page-Footer` | **丢弃**（默认不进 feat；与 indra 默认丢家具一致） |
| 未知 | `text` |

不修改 `feat.ParsePageJSON`；适配层直接构造 `[]feat.Page`。

---

## 6. 文件边界（预期新增/轻触）

| 路径 | 动作 |
|------|------|
| `internal/indra/*.go` | **新建**：ResolveBin、RunJSON、ToFeatPages、测试 |
| `internal/convert/indra_feat.go` | **新建**：`runIndraFeatPDF` |
| `internal/convert/convert.go` | Feat 分支：`if indra { runIndra… } else { runOarFeatPDF }`；Options 加字段 |
| `internal/convert/pipeline.go` | **不改** oar 函数；至多加 `ParseFeatEngine` 放同包新文件 |
| `internal/feat/*` | **不改** |
| `internal/api/convert.go` | 读 `feat_engine` |
| `internal/api/web/index.html` + `app.js` | 引擎下拉；默认 pipeline=geom |
| `docs/superpowers/specs/2026-10-23-feat-pipeline-spec.md` | **不改正文**；可在文首加「上游也可为 indra 适配结果」一句指针（可选，默认可只链到本文） |

---

## 7. 测试

- `ToFeatPages`：样例 indra block JSON → 字段/映射/丢弃 header。
- 无 indra 二进制时 `ResolveBin` 空 + `runIndraFeatPDF` 返回可识别错误。
- 回归：现有 `internal/feat` / oar 相关测试全部仍过（零改动预期）。

---

## 8. 风险

- indra 类型粗、标题弱于 oar structure → 表征标题质量可能差一档；属预期，用引擎切换对比。
- 首次拉模型需联网（`.venv-indra` / HF）；文档在错误信息中提示即可。
