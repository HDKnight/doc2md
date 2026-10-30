# doc2md API Design

**Date:** 2026-10-16  
**Status:** Approved  
**Location:** `f:\code\java\workspace\aitest\doc2md` (standalone Go module)

## Goal

Low-cost, low-memory HTTP service that converts PDF and Word (DOCX) files to GitHub Flavored Markdown. Prefer text-layer extraction; use a small OCR model only when needed.

## Non-Goals (v1)

- Auth, multi-tenant quotas, job queues
- GPU / DeepDoc / MinerU / Docling
- Full layout reconstruction (complex tables, formulas)
- Legacy `.doc` (Word 97) unless trivial later

## API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Liveness |
| `POST` | `/convert` | Multipart upload → Markdown |

### `POST /convert`

- **Content-Type:** `multipart/form-data`
- **Fields:**
  - `file` (required): `.pdf` or `.docx`
  - `ocr` (optional): `auto` \| `on` \| `off` (default `auto`)
- **Query:** `raw=1` → response body is `text/markdown` only

**JSON success (default):**

```json
{
  "filename": "a.pdf",
  "format": "pdf",
  "engine": "text|ocr|docx",
  "markdown": "# ..."
}
```

**Errors:** `400` unsupported / empty / ocr off on scan; `413` over size; `500` parse failure.

**Limits:** max upload 50MB; OCR concurrency default 1; render DPI default 150.

## Processing

```
upload → size check → by extension
  .docx → OOXML paragraphs/tables → Markdown
  .pdf  → extract text layer
          if enough chars/page (≥50) and ocr≠on → text engine
          else if ocr≠off → page render → RapidOCR subprocess → MD
          else → 400
```

## Architecture

- Single Go binary, stdlib `net/http`
- Internal packages: `api`, `convert`, `pdf`, `docx`, `markdown`, `ocr`
- Intermediate `[]Block` then render to GFM
- OCR via external RapidOCR-json (or compatible) subprocess; not loaded when unused

## Resource Targets

- Resident without OCR: prefer &lt; 50MB
- OCR process: start on demand, exit after use
- Default DPI: 150

## Tech Stack

- Go 1.22+ (stdlib HTTP; no required Go module deps)
- `archive/zip` + encoding/xml for DOCX
- Poppler `pdftotext` / `pdftoppm` subprocess for PDF text and page render
- OCR: RapidOCR-json (or compatible) subprocess via `DOC2MD_OCR_CMD`
