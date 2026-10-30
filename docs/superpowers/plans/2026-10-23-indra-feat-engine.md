# Indra Feat Engine Implementation Plan

> **For agentic workers:** Implement task-by-task. Do **not** modify `runOarStructure` / `runOarFeatPDF` / `internal/feat` logic.

**Goal:** Add feat-engine switch (`oar` default | `indra`) under 表征 pipeline; indra adapts JSON → `feat.Page` → existing `MarkdownFromPages`.

**Architecture:** New `internal/indra` + `convert/indra_feat.go`; branch only in `convertPDF` Feat case. UI: engine select + default pipeline=geom.

**Tech Stack:** Go, indra-ocr CLI (`.venv-indra`), existing feat package.

## Global Constraints

- Do not change oar feat/structure code paths beyond a dispatch `if`.
- Missing indra → hard error (no silent fallback to oar).
- Default FeatEngine = oar; default UI pipeline = geom.

---

## Task 1: `internal/indra` adapter + ResolveBin

- [ ] `ToFeatPages` + label map + drop Page-Header/Footer
- [ ] Unit tests for mapping
- [ ] `ResolveBin`, `RunJSON` (CLI `--format json --mode ocr`)

## Task 2: `runIndraFeatPDF` + Options dispatch

- [ ] `Options.FeatEngine`, `ParseFeatEngine`
- [ ] `runIndraFeatPDF` in new file; `convertPDF` branch
- [ ] API `feat_engine` form field

## Task 3: UI

- [ ] Engine select; enable when pipeline=geom; default pipeline=geom
- [ ] app.js send `feat_engine`

## Task 4: Verify

- [ ] `go test ./internal/indra/ ./internal/feat/ ./internal/convert/`
- [ ] Rebuild `doc2md.exe`
