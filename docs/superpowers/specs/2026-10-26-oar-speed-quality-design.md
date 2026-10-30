# oar 表征加速（保质量）

> 2026-10-26 · 已确认

**目标：** 同 PDF 表征 ≤10s；质量≈现基线；默认 CPU，GPU 可选可回退。

**做：**
- `OarStructureDevice`：`cuda:*` 探测失败 → 回退 `cpu×N`；`OCRStats` 标实际 device
- CPU 副本/ORT 网格实测，只留过闸门的调参（DPI 仍 96）
- 短 bench：计时 + 关键字 + diff≤2%

**不做：** 换引擎、降 DPI、常驻 worker、不过闸门改默认
