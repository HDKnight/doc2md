# -*- coding: utf-8 -*-
import json
import re
from collections import Counter
from pathlib import Path

import requests

BASE = Path("/mnt/c/Users/96511/Documents/党政学习/马克思主义基本原理")
OUT = Path("/mnt/f/code/java/workspace/aitest/doc2md/testdata/out")
CASES = [
    ("tiku", "《马克思主义基本原理》配套题库（考研真题精选＋章节题库）.pdf"),
    ("notes", "《马克思主义基本原理》期末章节笔记.pdf"),
]


def main() -> None:
    for cid, name in CASES:
        path = BASE / name
        data = path.read_bytes()
        r = requests.post(
            "http://127.0.0.1:8000/convert",
            files={"file": (name, data, "application/pdf")},
            data={"enhance": "false", "images": "false", "use_llm": "false"},
            timeout=600,
        )
        print(cid, "status", r.status_code, "ctype", r.headers.get("content-type"))
        if r.status_code >= 400:
            print(r.text[:500])
            continue
        md = r.text
        meta = {}
        if r.headers.get("X-Meta"):
            try:
                meta = json.loads(r.headers["X-Meta"])
            except Exception:
                meta = {"raw": r.headers.get("X-Meta")}
        heads = re.findall(r"^(#{1,6})\s+(.+)$", md, re.M)
        print(
            cid,
            "md",
            len(md),
            "heads",
            len(heads),
            "depth",
            dict(sorted(Counter(len(h[0]) for h in heads).items())),
            "meta",
            {k: meta.get(k) for k in ("mode", "outline", "headings", "pages", "parser") if k in meta},
        )
        for a, b in heads[:10]:
            print(" ", a, b[:70])
        (OUT / f"{cid}_docker8000.md").write_text(md, encoding="utf-8")


if __name__ == "__main__":
    main()
