# -*- coding: utf-8 -*-
import re
from collections import Counter
from pathlib import Path

REF = Path(r"E:\Administrator\Documents\Downloads\《马克思主义基本原理》配套题库（考研真题精选＋章节题库） (8).md")
GO = Path(r"f:\code\java\workspace\aitest\doc2md\testdata\out\tiku_v5.md")


def stats(name: str, md: str) -> None:
    heads = re.findall(r"^(#{1,6})\s+(.+)$", md, re.M)
    print(name, "chars", len(md), "heads", len(heads),
          "depth", dict(sorted(Counter(len(a) for a, _ in heads).items())))
    print("  h3:", [b for a, b in heads if len(a) == 3])
    print("  long80:", sum(1 for _, b in heads if len(b) > 80))
    print("  sample:")
    for a, b in heads[:14]:
        print("   ", a, b[:56])
    bad = [(a, b[:50]) for a, b in heads if re.search(r"一、\s*概念题", b)]
    print("  headings 一、概念题:", bad[:8])


def main() -> None:
    stats("ref", REF.read_text(encoding="utf-8"))
    stats("go", GO.read_text(encoding="utf-8"))


if __name__ == "__main__":
    main()
