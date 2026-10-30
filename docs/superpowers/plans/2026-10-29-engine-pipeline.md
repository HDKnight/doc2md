# Engine Pipeline Implementation Plan

> **For agentic workers:** implement task-by-task. Spec: `docs/superpowers/specs/2026-10-29-lightweight-rewrite-design.md`

**Goal:** New `engine/` markdown path from oar JSON / docx / txt; remove rules & structure-MD as defaults.

**Architecture:** `engine/pipeline` owns 断行→标题→层级; `engine/oarx` wraps structure JSON+device; `engine/ingest` feeds convert.

**Tech Stack:** Go, existing oar warm under `internal/ocr` (call only, no feat/textnorm title APIs).

## Global Constraints

- No length-based titles; no split intact ordinal stems at `。`
- New code only under `engine/`; zero dependency on `feat.MarkdownFromPages` / `textnorm.BuildOutline`

---

### Task 1: engine/pipeline core + tests

### Task 2: engine/oarx parse + structure run via internal/ocr

### Task 3: engine/ingest pdf/docx/txt

### Task 4: convert wires to engine; drop rules/structure MD defaults

### Task 5: go test ./engine/... and build
