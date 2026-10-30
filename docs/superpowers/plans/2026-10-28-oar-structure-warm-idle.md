# oar structure warm + idle kill Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox syntax.

**Goal:** Warm `oar_structure_worker` for feat/structure; Go kills it after 60s idle.

**Architecture:** New Rust stdin sidecar holds `pp_structurev3`; Go `StructureWarm` reuses it and idle-kills; CLI fallback unchanged.

**Tech Stack:** Rust oar-ocr 0.10.x, Go convert/ocr packages

## Global Constraints

- Idle unload = Kill whole process (default 60s; env `DOC2MD_OAR_STRUCTURE_IDLE_SEC`)
- Separate from `oar_worker` (OCR-only)
- No worker binary → existing `oar structure` CLI

---

### Task 1: Rust `oar_structure_worker`

- [ ] Create `tools/oar_structure_worker/` (Cargo.toml + main.rs)
- [ ] Protocol: STRUCTURE / EXIT; write page JSON to outDir; one-line JSON reply
- [ ] Build release (directml on Windows if feasible)

### Task 2: Go `StructureWarm` + idle killer

- [ ] `internal/ocr/structure_warm.go` (+ tests with fake worker)
- [ ] Resolve bin; lazy start; serial jobs; idle Kill

### Task 3: Wire pipeline + ensure-deps

- [ ] `runOarStructurePDF` prefer warm; Model note warm/cold
- [ ] ensure-deps / pack copy worker exe
- [ ] Rebuild doc2md + smoke
