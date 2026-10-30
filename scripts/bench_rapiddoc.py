"""Bench RapidDoc vs wall-clock on a sample PDF."""
from __future__ import annotations

import time
from pathlib import Path

from rapid_doc import RapidDoc

PDF = Path(r"C:\Users\96511\AppData\Local\Temp\doc2md-1531639136.pdf")
OUT = Path(__file__).resolve().parents[1] / "testdata" / "out" / "rapiddoc_bench"


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    # 关闭公式以贴近「卷子 OCR」；表格先开（选项区可能被当表）
    engine = RapidDoc(
        formula_enable=False,
        table_enable=True,
        lang="ch",
        parse_method="ocr",  # 扫描件
        preload_model=True,
    )
    print("warmup…", flush=True)
    t0 = time.perf_counter()
    engine.warmup()
    print(f"warmup {time.perf_counter() - t0:.1f}s", flush=True)

    print("parse full PDF…", flush=True)
    t1 = time.perf_counter()
    out = engine(
        str(PDF),
        output_dir=str(OUT),
        formula_enable=False,
        table_enable=True,
        parse_method="ocr",
        f_dump_middle_json=True,
        f_dump_content_list=True,
        f_draw_layout_bbox=False,
        f_draw_span_bbox=False,
    )
    elapsed = time.perf_counter() - t1
    if isinstance(out, list):
        out = out[0]
    md = out.markdown or ""
    md_path = OUT / "result.md"
    md_path.write_text(md, encoding="utf-8")
    print(f"elapsed={elapsed:.1f}s md_chars={len(md)} -> {md_path}", flush=True)
    print("--- md head ---", flush=True)
    print(md[:1500], flush=True)


if __name__ == "__main__":
    main()
