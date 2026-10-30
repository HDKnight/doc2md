# doc2md

本地把 **PDF / DOCX / TXT** 转成带标题层级的 **Markdown**。浏览器里拖入文件即可，也提供 HTTP API。转换在本机完成，不上传到第三方。

纯文字 PDF 用版面行框（字号、坐标）断行和定标题。扫描件走 [oar](https://crates.io/crates/oar-ocr-cli) 的 structure JSON。DOCX 与 TXT 进入同一套定级规则。

设计说明见 [docs/architecture.md](docs/architecture.md)。

## 能处理什么

- `.pdf`：文字层够用时直接抽取；扫描件、乱码或字太少时用 oar 表征。
- `.docx`、`.txt`：按段落进入同一条管线。
- 标题来自特殊序号（`1、` `1.` `一、` `（1）`）、版面标题类型，或明显更大的字号。
- 选项（`A、`）不会被当成标题。
- `1932.7.21` 这类点分日期保持正文，不当序号。
- 单文件上限 50MB。

## 环境

| 依赖 | 用途 |
|------|------|
| Go 1.26+ | 编译服务 |
| [Poppler](https://github.com/oschwartz10612/poppler-windows) 的 `pdftotext` | 文字版 PDF |
| `oar`（`cargo install oar-ocr-cli`） | 扫描 PDF。没有它时，文字版 PDF / DOCX / TXT 仍可用 |
| Rust（可选） | 编译常驻 `tools/oar_structure_worker`。不编译则回退 `oar` 命令 |

Windows 可 `winget install oschwartz10612.Poppler`，装完新开一个终端。

## 开发运行

```bash
go run ./cmd/server
```

默认 `http://127.0.0.1:8080/`。换端口：

```bash
# Windows PowerShell
$env:DOC2MD_ADDR=":8484"
go run ./cmd/server
```

```bash
# Linux / macOS
DOC2MD_ADDR=:8484 go run ./cmd/server
```

## Windows 一键启动

先编译，再双击 `start.bat`（或 `start.bat 9000`）。首次会用 `ensure-deps.ps1` 准备便携 Python、Poppler 和 `tools\oar.exe`（本机已有 `oar` 则复制，否则 `cargo install oar-ocr-cli`）。`stop.bat` 停止服务。

```bat
go build -o doc2md.exe ./cmd/server
start.bat
```

已在一台机器上跑通、并生成过 oar 模型后，可以打便携目录：

```bat
powershell -NoProfile -ExecutionPolicy Bypass -File pack-portable.ps1
```

产出在 `dist\doc2md-portable\`。该目录不进入 Git。

## HTTP

### `GET /health`

```bash
curl http://127.0.0.1:8080/health
```

### `POST /convert`

表单字段 `file` 必填。`raw=1` 时响应体就是 Markdown，否则是 JSON（`markdown`、`text`、`engine` 等）。

```bash
curl -s -F file=@./sample.docx "http://127.0.0.1:8080/convert?raw=1" -o out.md
```

服务固定走 `engine` 管线：有可用文字层就不用 OCR，否则用 oar structure JSON。请求里旧的 `pipeline`、`ocr`、`feat_engine` 会收下，但不切换实现。

## 环境变量

| 变量 | 说明 |
|------|------|
| `DOC2MD_ADDR` | 监听地址，默认 `:8080` |
| `DOC2MD_PDFTOTEXT` | `pdftotext` 路径 |
| `DOC2MD_PDFTOPPM` | `pdftoppm` 路径（扫描件渲染） |
| `DOC2MD_OCR_OAR` | `oar` 可执行文件 |
| `DOC2MD_OCR_OAR_STRUCTURE_WORKER` | 常驻 structure worker，默认找 `tools/oar_structure_worker.exe` |
| `DOC2MD_OAR_STRUCTURE_DPI` | 表征 DPI，默认 `96` |
| `DOC2MD_OAR_DEVICE` | `auto`（默认）、`auto-cpu`、`cuda:0` 等 |
| `DOC2MD_OAR_STRUCTURE_IDLE_SEC` | worker 空闲退出秒数，默认 `60`；`0` 表示不退出 |
| `DOC2MD_OAR_STRUCTURE_BATCH` | 每次送入的页数；默认整本一次。内存紧时设 `2` |
| `DOC2MD_OAR_CPU_REPLICAS` | 强制 CPU 路数 1–8 |
| `DOC2MD_OAR_ZIP_URL` | 含 `oar.exe` 的 zip，给不能编译的机器 |

## 目录

```
cmd/server/          启动入口
engine/ingest/       PDF / DOCX / TXT 分流
engine/oarx/         oar structure JSON
engine/pipeline/     断行、标题、层级
internal/api/        HTTP 与页面
internal/pdf/        文字层
internal/docx/       DOCX
tools/               oar worker 源码与 ocr_cli.py（二进制与 target 不入库）
docs/architecture.md 定级规则
```

## 测试

```bash
go test ./engine/...
```

## 许可证

[MIT](LICENSE)
