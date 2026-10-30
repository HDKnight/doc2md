#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""doc2md OCR sidecar：faster-paddle（PP-OCRv6 ONNX，默认 tiny）+ 常驻 worker。

用法:
  python ocr_cli.py <image.jpg>
  python ocr_cli.py --worker
      # 每行一个图片路径 → 一行 JSON
      # 或 BATCH <json-array-of-paths> → {"pages":[...]}

环境变量:
  DOC2MD_OCR_MODEL=small|tiny   默认 tiny
  DOC2MD_OCR_THREADS=N          ORT/推理线程，默认 min(8, cpu)
  DOC2MD_OCR_BATCH=N            ocr_batch 窗口，默认 4

输出: {"data":[{"text":"...","x0":..,"y0":..,"x1":..,"y1":..}, ...]}  （stdout 始终 UTF-8）
优先用 bounds 检测框（按阅读顺序），无框时回退 structured_text。
"""
from __future__ import annotations

import json
import os
import sys
import time
from typing import Any


def _model_size() -> str:
    m = (os.environ.get("DOC2MD_OCR_MODEL") or "tiny").strip().lower()
    return "tiny" if m != "small" else "small"


def _threads() -> int:
    raw = (os.environ.get("DOC2MD_OCR_THREADS") or "").strip()
    if raw.isdigit():
        n = int(raw)
        if 1 <= n <= 32:
            return n
    return min(8, max(2, os.cpu_count() or 4))


def _batch_size() -> int:
    raw = (os.environ.get("DOC2MD_OCR_BATCH") or "4").strip()
    try:
        n = int(raw)
        if 1 <= n <= 32:
            return n
    except ValueError:
        pass
    return 4


def build_engine():
    from faster_paddle import OcrEngine

    return OcrEngine(model_size=_model_size(), threads=_threads())


def _lines_from_bounds(bounds: Any) -> list[dict[str, Any]]:
    """Emit reading-order lines from detector boxes."""
    if not isinstance(bounds, dict) or not bounds:
        return []
    items: list[dict[str, Any]] = []
    for _k, v in bounds.items():
        if not isinstance(v, dict):
            continue
        text = (v.get("text") or "").strip()
        if not text:
            continue
        tl = v.get("topLeftCoord") or (0, 0)
        br = v.get("bottomRightCoord") or (0, 0)
        try:
            x0, y0 = float(tl[0]), float(tl[1])
            x1, y1 = float(br[0]), float(br[1])
        except (TypeError, ValueError, IndexError):
            continue
        if x1 < x0:
            x0, x1 = x1, x0
        if y1 < y0:
            y0, y1 = y1, y0
        items.append({"text": text, "x0": x0, "y0": y0, "x1": x1, "y1": y1})
    items.sort(key=lambda it: (it["y0"], it["x0"]))
    return items


def _lines_from_result(result: dict[str, Any]) -> list[dict[str, Any]]:
    from_bounds = _lines_from_bounds(result.get("bounds"))
    if from_bounds:
        return from_bounds
    text = (result.get("structured_text") or result.get("text") or "").replace("\r\n", "\n")
    out: list[dict[str, Any]] = []
    for ln in text.split("\n"):
        t = ln.strip()
        if t:
            out.append({"text": t})
    return out


def run_one(engine, path: str) -> dict:
    t0 = time.perf_counter()
    with open(path, "rb") as f:
        blob = f.read()
    result = engine.ocr(blob)
    ms = int((time.perf_counter() - t0) * 1000)
    return {"data": _lines_from_result(result), "ms": ms, "model": _model_size()}


def run_batch(engine, paths: list[str]) -> dict:
    t0 = time.perf_counter()
    # 控制窗口，避免一次加载过多页图导致进程被杀
    window = _batch_size()
    pages: list[dict] = []
    for i in range(0, len(paths), window):
        chunk = paths[i : i + window]
        blobs = []
        for p in chunk:
            with open(p, "rb") as f:
                blobs.append(f.read())
        results = engine.ocr_batch(blobs, batch_size=window)
        pages.extend({"data": _lines_from_result(r)} for r in results)
        del blobs, results
    ms = int((time.perf_counter() - t0) * 1000)
    return {"pages": pages, "ms": ms, "model": _model_size()}


def write_json(obj: dict) -> None:
    payload = json.dumps(obj, ensure_ascii=False) + "\n"
    sys.stdout.buffer.write(payload.encode("utf-8"))
    sys.stdout.buffer.flush()


def main() -> int:
    args = sys.argv[1:]
    worker = bool(args and args[0] in ("--worker", "-w"))
    if not worker and (not args or args[0].startswith("-")):
        print("usage: ocr_cli.py <image> | ocr_cli.py --worker", file=sys.stderr)
        return 2
    try:
        engine = build_engine()
    except ImportError:
        print(
            "faster-paddle not installed: pip install faster-paddle",
            file=sys.stderr,
        )
        return 1

    if worker:
        print(
            f"[ocr_cli] worker ready backend=faster-paddle model={_model_size()} "
            f"threads={_threads()} batch={_batch_size()}",
            file=sys.stderr,
            flush=True,
        )
        for line in sys.stdin:
            raw = line.strip()
            if not raw or raw.upper() in ("EXIT", "QUIT"):
                break
            try:
                if raw.upper().startswith("BATCH "):
                    paths = json.loads(raw[6:])
                    if not isinstance(paths, list) or not paths:
                        write_json({"pages": [], "error": "batch paths empty"})
                        continue
                    write_json(run_batch(engine, [str(p) for p in paths]))
                else:
                    write_json(run_one(engine, raw))
            except Exception as e:  # noqa: BLE001 — keep worker alive
                if raw.upper().startswith("BATCH "):
                    write_json({"pages": [], "error": str(e)})
                else:
                    write_json({"data": [], "error": str(e)})
        return 0

    write_json(run_one(engine, args[0]))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
