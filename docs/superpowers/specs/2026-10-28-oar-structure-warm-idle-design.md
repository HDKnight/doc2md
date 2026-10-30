# oar structure 常驻 + 空闲杀进程

**目标：** 表征管线第二次起不再冷加载；空闲 1 分钟释放显存/内存。  
**卸载策略：** A — 直接 Kill 整个 worker 进程。

## 现状

- `runOarFeatPDF` / structure 每次 `exec oar structure …`，进程退出即卸模型。
- DirectML 单次可占满 ~12GB 显存；冷启动贵，常驻不卸又占显存。
- 已有 `oar_worker` 只服务页图 OCR，不含 structure。

## 方案

独立 sidecar **`oar_structure_worker`**（与 OCR worker 分离，避免两套模型同进程顶爆显存）。

### Worker（Rust）

- 依赖 `oar-ocr`（与现网一致；Windows 打包编 `directml`）。
- 启动时 `OARStructureBuilder::pp_structurev3().build()`，stdin 循环：
  - `STRUCTURE <pdfPath> <outDir> dpi=<N> format=json|markdown [pages=<R>]`
  - 成功：一行 JSON `{"ok":true,"pages":N,"ms":…}`，页面文件写入 `outDir`（与 CLI 同约定）
  - 失败：`{"ok":false,"error":"…"}`
  - `EXIT` → 进程退出
- 不内置空闲计时（由 Go 管）。

### Go 侧

- `StructureWarm`：懒启动 worker；串行排队任务；每次成功/失败刷新 `lastUsed`。
- 后台 goroutine：`now-lastUsed >= idle` → `Kill` + 清句柄。
- `DOC2MD_OAR_STRUCTURE_IDLE_SEC`：默认 `60`；`0` = 不自动杀（仍可手动关服务清掉）。
- `runOarFeatPDF` / structure：优先走 warm；无二进制或启动失败 → 回退现有 `oar structure` CLI。
- 表征前**不再**为腾内存而误杀无关进程的逻辑保持：仅管理 structure worker；PP-OCR pool 仍可在进表征前 Close（与现行为一致）。

### 打包

- `ensure-deps` / portable：有 Rust 则编 `tools/oar_structure_worker.exe`；无则跳过（回退 CLI）。
- UI Model 串可带 `warm` / `cold` 便于核对。

## 非目标

- 不把 structure 塞进现有 `oar_worker`。
- 不做进程内「只卸模型不杀进程」。
- 不保证 DirectML 比多路 CPU 更快；常驻只省冷启动。

## 验收

1. 连续两次同一 PDF：第二次总时长明显短于第一次（同机、同设置）。
2. 停满 idle 后任务管理器中 worker 消失，专用显存回落。
3. 无 worker 二进制时行为与今日 CLI 一致。
