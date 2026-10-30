# 表征管线（feat）完整规格

> 目标：另一实现/另一模型可仅凭本文完整重写 `internal/feat`，行为与现网一致。  
> 代码入口：`feat.MarkdownFromPages`；接入：`convert.runOarFeatPDF`（`pipeline=geom|feat|表征|json`）。  
> **独立分支**：不调用 `textnorm` / `geom` 的标题逻辑；不修改 oar 原生 structure MD 管线。

---

## 0. 总览

### 0.1 输入 / 输出

| 项 | 说明 |
|----|------|
| 上游 | `oar structure <pdf> --format json -o <dir>`，每页一个 JSON |
| 输入 | `[]Page`，每页 `blocks[]` |
| 输出 | 一份 Markdown 字符串：标题用 `#`…`######`，块之间空行（`\n\n`），末尾补一个 `\n` |
| plain | 另由 convert 层拼接各 block 的 `content`（换行连接），**不算** feat 输出 |

### 0.2 单页 JSON 形状

```json
{
  "blocks": [
    {
      "type": "paragraph_title",
      "content": "3.新时代中国特色社会主义思想。",
      "order": 2,
      "bbox": [x0, y0, x1, y1],
      "confidence": 0.87
    }
  ]
}
```

- `bbox`：`[x0,y0,x1,y1]`，缺省或长度不足时坐标按 0，宽高按最小 1。
- `type`：读入后 **ToLower + TrimSpace**。
- `content` 空串跳过。
- `confidence` / `angle`：feat **忽略**。

### 0.3 端到端流水线（三阶段；阶段内顺序固定）

对齐设计：`2026-10-29-pipeline-three-stage-design.md`。

```
① 断行
1. 每页 blocks → items（估 em/w，按 y0 再 x0 排序）
2. 按页 buildPageSegs：版式初判 heading|para + 页内 soft-wrap 合并 para
3. 全文 segs 拼接（页序）
4. splitGluedQuizSegs          // 选择题/序号粘连
5. splitEmbeddedParenMarkers   // 正文内嵌 (1)(2)；只按 。/：切开，不按字数
6. splitLongSeriesStems        // 粘连题干按结构标点拆 head+body
7. mergeBrokenParas            // 断章合并（满宽/间距/标点）

② 标题识别
8. alignPeerHeadings           // 含 demoteChoiceStems；后接选项=正文；同族补抬
                               // 不用字数长短定义标题

③ 层级
9. normalizeHeadingLevels      // 嵌套 + 防跳级
10. 渲染 Markdown
```

---

## 1. 数据结构

### 1.1 `item`（页内原始块）

| 字段 | 含义 |
|------|------|
| text | trim 后的 content |
| typ | 小写 type |
| em | `estimateLineEm(boxH, boxW, text)` |
| boxH | bbox 高（≥1） |
| w | bbox 宽（≥1） |
| x0,y0,y1 | 坐标 |
| page | 1-based 页码 |

### 1.2 `seg`（流水线中段）

| 字段 | 含义 |
|------|------|
| kind | `"heading"` \| `"para"` |
| lvl | 标题级别 1–6（渲染前再钳制） |
| text | 文本 |
| em, w, x0, y0, y1 | 布局；soft-wrap / 断章会更新 `y1`、`w`（**末行宽**）、`em` |
| series | 系列键：`arab_dun` / `arab_dot` / `cn_dun` / `paren` / `""` |
| num | 系列内序号（1-based）；无效为 0 |
| typ | 原始 oar type（可选保留） |

---

## 2. 行高估计 `estimateLineEm(boxH, boxW, text)`

用途：把「整块 bbox 高」折算成近似单行字号，供「是否明显高于正文」判断。

```
n = runeCount(text 去掉内部 \n)
若 boxH < 1 → 返回 1
若 n < 1 → 返回 boxH
pitch = 1.25, aspect = 0.88
若 n * aspect * boxH <= boxW * 1.25 → 单行，返回 boxH
否则从 lines=2..60 试：
  em = boxH / (pitch * lines)
  若 em < 4 停止
  cpl = max(1, boxW / (aspect * em))
  need = ceil(n / cpl)
  若 need <= lines → 返回该 em
返回最后试过的 best
```

### 2.1 正文字号中位 `medianBodyEm(items)`

收集候选 em：字符数 ≥ 40，且 **不是** 问句（含 `？`/`?`），且 **不是**「短阿拉伯题干」(`isArabicStem` 且 n≤56)。  
无候选则用全部 item 的 em。返回中位数；空则 12。

### 2.2 宽度中位 `medianWidth`：全部 `w` 的中位数。

---

## 3. 系列识别 `seriesOf(text)`（行首）

按优先级：

1. **paren**：首字符 `(` 或 `（`，其后阿拉伯数字或中文数字一…十，再 `)`/`）` → `("paren", n)`
2. **arab**：1–3 位数字后接：
   - 若是 `、` 且 `isLikelyNumberRangeAfterDun`（见下）→ 非系列 `("",0)`
   - `、` → `arab_dun`
   - `.` 或 `．` → `arab_dot`
3. **cn_dun**：首字在「一二三四五六七八九十」，次字符为 `、`/`.`/`．` → `cn_dun`（仅单字中文数，不解析「十一」）

### 3.1 `isLikelyNumberRangeAfterDun(runes, dunIdx)`

仅当模式为 **`数字、数字、`**（第二段数字之后还有顿号）才为 true（如 `16、17、`）。  
`27、1936年`、`5、12项`（第二段后无 `、`）→ false，仍算题干/系列。

---

## 4. 标题标记（阶段 A）`markIsHeading` —— **不定级**

返回 bool：是否标题。`kind=heading` 时 `lvl=0`，最终层级只由 §9 分配。

### 4.1 硬护栏

1. `n == 0` → false  
2. **`isOptionLine`** → false  

### 4.2 Type 置信

`typeIsTitle`：`doc_title`/`title`/`paragraph_title`/`section_title`/`aside_title` → true（`n≤120`）；**`text` 不置信**。

### 4.3 启发式（type 未命中且 `n≤70`）

`ratio = em/bodyEm`，`prose = looksProse`。任一命中即为标题：

| # | 条件 |
|---|------|
| 1 | `!prose` 且 `ratio ≥ 1.14` 且 `n ≤ 52` |
| 2a | `isArabicStem` 且 `n ≤ 56`，且（问号 **或** 下一块 `答：`） |
| 2b | `isArabicStem` 且 `n ≤ 56` 且 `!prose` 且 `n ≤ 40` 且 `nextIsTallBody` |
| 3 | `n ≤ 22` 且 `isShortLabel` 且 `nextIsTallBody` |
| 4 | `n ≤ 16` 且 `isShortLabel` 且 `w < medW*0.5` |
| 5 | `!prose` 且 `n ≤ 20` 且 `isArabicStem` 且 `nextIsTallBody` |

### 4.4 `looksProse`

- 不含 `。` → false（非散文）  
- 若 `n ≤ 48` 且全文恰好一个 `。`，且去掉尾部引号/括号后以 `。` 结尾：  
  - 前缀 `答：`/`答:` → **仍是散文 true**（防行高误抬）  
  - 否则若 `isArabicStem(原文)` 或 `isShortLabel(去句号后的 core)` → **false**（短标题腔）  
- 其它含 `。` → true

### 4.5 `nextIsTallBody`

下一块存在，且其 `runeCount ≥ 28`，且  
`next.boxH ≥ 当前.boxH * 2.2` **或** `next.boxH ≥ bodyEm * 5`。

### 4.6 `isArabicStem`

行首 1–3 位数字 + `、`/`.`/`．`；若 `、` 后是 `数字、数字、` 区间则 false。

### 4.7 `isShortLabel`

- 空或前缀 `答` → false  
- 含 `。！？` → false  
- `n > 22` → false  
- `isArabicStem` → true  
- 中文数字 + `、./．/空格` → true  
- 首字符 `（`/`(` → true  
- 否则 false  

### 4.8 `isOptionLine`

行首为 `A-Z`/`a-z`/全角字母，紧跟 `、`/`.`/`．`/`,`/`，` → true。

---

## 5. 页内 soft-wrap（仅 para）

在 `buildPageSegs` 初判之后：

对相邻两段，仅当 **两者皆 para** 时尝试合并：

```
gap = s.y0 - prev.y1
em = max(prev.em, s.em)；若 <1 用 bodyEm
sameIndent = |s.x0 - prev.x0| < em * 0.9
soft = (gap ≥ -em*0.3) && (gap < em*1.15) && sameIndent && !looksNewPara(s.text)
```

合并：`joinSoft`；`prev.y1 = s.y1`；`prev.w = s.w`（**保留末行宽**）；`em` 取较大；**清空** `series/num`。

### 5.1 `looksNewPara`（禁止 soft-wrap / 断章跨过）

任一成立即为新段：

- 前缀 `答：`/`答:`  
- `isOptionLine`  
- 首字 `第`  
- `isArabicStem`  
- 中文数字 + `、./．`  

### 5.2 `joinSoft(a,b)`

去 a 右空白、b 左空白；若 a 末与 b 首都是 ASCII 字母数字则中间插空格，否则直接拼接（中文不断开）。

---

## 6. 选择题 / 序号粘连 `splitGluedQuizSegs`

**禁止**擅自删除选项之间的 OCR 噪点字符（如 `整顿1D、` 中的 `1` 必须保留在前一段末尾）。

### 6.1 阶段 A：逐段 `splitGluedQuizText`

1. **题号粘连**（最多 8 轮替换至稳定）：  
   - `reQGlueDun`：`(汉字|字母选项|）)。！？等) + 可选空白 + (1–3位数字、 + 后续合法首字)` → `$1\n$2`  
   - `reQGlueDot`：同类，分隔符为 `.`/`．`  
   典型：`延安整风27、1936年` → 两行。
2. 按 `\n` 切开后，每行再跑 **`splitByRepeatedOrdinals`**。

### 6.2 `splitByRepeatedOrdinals`（行内）

1. `findOrdinalMarks(s)` 找所有序号标记（见 §6.4）  
2. 按 `kind` 计数；**仅当某 kind 出现次数 ≥ 2**，才把该 kind 的所有标记加入 cuts  
3. **单个**行内 `A、` **不切**（避免误切）  
4. `cutAtMarks`：在 cuts 处切开；标记前前缀单独成段；**不删**标记间字符  

### 6.3 阶段 B：`splitAcrossConfirmedRuns`（跨段）

目的：`题干。A、xxx` 单独一段时只有一个 A，行内不切；但后面还有 `B、` `C、` 时，应确认 A→B→C 连续串后再切开（含粘在题干后的 A）。

算法：

1. `collectOrdHits`：扫所有 seg，收集 `(segIdx, start, kind, num)`  
2. `confirmedHitKeys`：在 hits 的**全局顺序**上找最长连续 run：  
   - 同 `kind`  
   - `num` 严格 `前一个+1`（A=1,B=2…）  
   - run 长度 **≥ 2** → run 内全部 hit 标为 confirmed  
3. 对每个 seg：若其内部 mark 在 confirmed 集合中，则对这些 mark `cutAtMarks`  
4. 切开后：第 0 段可保留原 heading；`i>0` 或 `isOptionLine` → 强制 `para`；选项清空 series  

### 6.4 `findOrdinalMarks`

用正则找候选（**字节下标**，Go `regexp`）：

| kind | 模式 | num |
|------|------|-----|
| letter | `[A-Za-zＡ-Ｚａ-ｚ][、.．,，]` | A=1…Z=26（大小写/全角同理） |
| arab_dun | `\d{1,3}、` | 数字值 |
| arab_dot | `\d{1,3}[.．]` 且后一位不是数字（排除小数） | 数字值 |
| paren | `[（(](?:\d{1,3}|[一二三四五六七八九十]+)[）)]` | 内层数值 |
| cn_dun | `[一二三四五六七八九十百千]+、` | 中文数解析（含十一、二十等简单式） |

后处理：

- 按 start 升序、同 start 取更长 end  
- 去重叠（`start < lastEnd` 跳过）  
- `num ≤ 0` 跳过  
- **cn_dun**：若 `start > 0` 且前一 rune 是汉字 → **跳过**（防拆「红一、二、四方面军」）  

### 6.5 切开后的 seg 属性

每片继承原 seg 的几何；重算 `seriesOf`；选项行 `kind=para, series="", num=0`。

---

## 7. 内嵌括号序号 `splitEmbeddedParenMarkers`

仅处理 **`kind == para`** 的段（heading 原样保留）。

1. 用 `reParenMark = [（(]\s*[0-9一二三四五六七八九十]{1,3}\s*[）)]` 找所有标记  
2. 若第一个标记前的 prefix 非空，且 **不是** `答：` 类前缀，且 prefix 字数 **> 6** → **整段不拆**（避免正文误拆）  
3. 允许的 prefix：空，或 `looksAnswerPrefix`（整段是 `答：` 或以 `答：`/`答:` 结尾）  
4. 对每个标记到下一标记（或文末）的 chunk：  
   - `splitTitleAtPeriod`：到首个 `。`（含）为标题，其后正文  
   - 无句号：`n≤48` 整段标题；否则截到冒号（位置 <40）；再否则前 36 字标题  
5. 标题 seg：`kind=heading`，`lvl=defaultSeriesLevel("paren")`（4），series/num 来自 `seriesOf(title)` 或强制 paren  
6. 若最终没有 heading → 放弃拆分，返回原段  

---

## 7b. 长粘连题干拆分 `splitLongSeriesStems`

对 `arab_dun` / `arab_dot` 且字数 **>56** 的块：在题号后按 `？/：/；/。`（或约 24 字）切开 → **短标题 + 正文 para**，再交给 §8 对齐。  
意图：避免「10、整段粘正文」当 para、而「17、短行」当标题的同型不一致。

## 8. 同型对齐 `alignPeerHeadings`

1. 刷新空 series：`seriesOf(text)`  
2. 按 series 收集下标：已是 heading，或 `peerCandidate`  
3. 每系列至少 2 个候选才处理  
4. `splitPeerRuns`：编号差 `dn ∈ [1,2]` 且不回绕 → 同一 run；否则新开  
5. `alignOneRun`：  
   - target = run 内已有 heading 的 **最小 lvl**；若皆无 → `defaultSeriesLevel(kind)`  
   - 再扫 run 两端之间漏网的同系列 `peerCandidate`，编号落在 `[lo.num, hi.num]` 则并入  
   - run 内全部设为 `heading` + `lvl=target`  

### 8.1 `peerCandidate`

- 已是 heading → true  
- `n==0` 或 `n>56` → false  
- 含 `。` 且无问号且不以 `(`/`（` 开头 → false  
- `cn_dun` 且 `n>24` → false  
- 否则 true  

### 8.2 `defaultSeriesLevel`

| series | 默认 lvl |
|--------|----------|
| arab_dun | 2 |
| arab_dot | 3 |
| cn_dun | 3 |
| paren | 4 |
| 其它 | 3 |

---

## 9. 层级分配（阶段 B）`normalizeHeadingLevels` → `assignHeadingLevels`

**先清零**所有 heading 的 `lvl`，再按文档序用栈定级（不再使用初判里的默认 H2/H3）。

对每个 heading：

1. 弹出栈顶直至：同系列兄弟，或 `shouldNestUnder(栈顶, 当前)`  
2. 栈空 → `lvl=2`（根从 `##` 起）  
3. 否则 → `lvl = 父.lvl+1`（≤6）  
4. 压入栈  

### 9.1 `shouldNestUnder(prev, cur)`

- prev 无系列且 cur 有系列 → true  
- cur 无系列：可挂到 `arab_dun`，或非题型词的 `cn_dun`；**不**挂到 `paren`/`arab_dot`  
- cur 为题型部分标题（`cn_dun`+选择题/简答…）→ **false**  
- prev 为题型部分标题且 cur 为 `arab_dun` → **true**  
- 否则 `rank(cur) > rank(prev)`（细挂粗）  

`seriesRank`：`arab_dun=1` < `cn_dun=2` < `arab_dot=3` < `paren=4`。

---

## 10. 断章合并 `mergeBrokenParas`

段落重建（视觉断行→逻辑段），**不是**删掉所有换行。启发式打底；**不用 LLM 改写正文**。

仅 **para+para**。批内先算：

- `fullW = max(seg.w)`  
- `fullChars = max(para 的 trim 后字数)`  

**硬性不并**（任一命中即断）：

1. 上段 **句末强标点**（`endsWithSentencePunct`：`。！？!?；;…`；引号/括号收尾则剥掉再判；**`：` / `，、,` 不算句末**）  
2. **纵向间距** `largeVerticalGap`：`gap > 1.5 × 上行高`（跨页 y 重置时不据此断）  
3. **字号差** `emMismatch`：两侧均为「单行段」且 `em>0` 且 `max/min ≥ 1.3`（多行块估 em 不可靠，不参与）  
4. 下段 **`looksNewPara`**（答：/选项/题号等）  
5. 下段「短标签且 n≤16」  

**合并门槛**（硬断都未命中后）：上段 **抵满行宽** `fillsContentWidth`（`w ≥ fullW*0.85`，否则字数规则同前）。

合并：`joinSoft`；更新 `em/y1/w`（w 取下段）。

**护栏意图**：选项、短行、大间距、字号跳变不并；跨页「解决思|想认识」且上行满宽才并。

---

## 11. Markdown 渲染

- heading：`strings.Repeat("#", clamp(lvl,1..6)) + " " + text`；若 lvl<1 当 2  
- para：纯 text  
- 块之间 `\n\n`；整体 TrimSpace 后加尾 `\n`  
- 空 text 跳过  

---

## 12. Convert 层接入（重写时需对齐）

- `ParsePipeline`：`geom|feat|features|layout|表征|json` → Feat  
- `runOarFeatPDF`：oar `--format json` → 读齐 `*.json`（按页码排序）→ `feat.MarkdownFromPages`  
- **不要**再跑 textnorm 的 `FormatMarkdownOrdinals` / geom heading  

---

## 13. 不变量 / 护栏清单（验收用）

1. `paragraph_title` 等 title type → 必为标题（n≤120），与是否含 `。` 无关  
2. `text` type → 不靠 type 抬题；可通过启发式抬题  
3. 行首 `A、`…`Z、`（含小写/全角）→ 永不标题  
4. 行内同类序号 ≥2 → 切开；字符不删  
5. 跨段连续字母/数字序号串 ≥2 → 串内全切（含题干后粘的首选项）  
6. `数字、数字、` 才是区间；`27、1936年` 是题干  
7. 「红一、二、四」中紧跟汉字的 `一、` 不当 cn 序号切点  
8. 断章需无句末 + 满行宽 + 非新段起点  
9. 与 textnorm/geom **零共享标题状态**  

---

## 14. 建议回归用例（最小集）

| 场景 | 期望 |
|------|------|
| type=`paragraph_title`, 文=`3.新时代中国特色社会主义思想。` | 输出为 `#` 标题 |
| type=`text`, 短且字号大 / 后接大段 | 可启发式成标题 |
| `C、作风整顿1D、思想整顿` | 两段，第一段仍含 `1` |
| `…教育。A、反对` + 后文 `B、` `C、` `D、` | A 从题干切开 |
| 仅 `只有A、一项`（无 B） | 不切 |
| `D、延安整风27、1936年…` | D 与 27、题干分开 |
| `解决思` + `想认识…`（上行满宽、无句末） | 合并 |
| 短 `A、xxx` + `B、yyy`（不满宽） | 不合并 |
| 正文长 prefix 后出现 `(1)` | 不拆；`答：`+`(1)` 可拆 |

---

## 15. 包文件映射（现实现）

| 文件 | 职责 |
|------|------|
| `block.go` | 解析、item/seg、em、headingLevel、soft-wrap、渲染入口 |
| `options.go` | 选项/序号粘连与跨段确认串 |
| `split.go` | 内嵌 `(1)(2)` |
| `peers.go` | seriesOf、alignPeerHeadings |
| `levels.go` | normalizeHeadingLevels |
| `merge.go` | mergeBrokenParas |

---

*文档版本：与 2026-10-23 表征分支行为对齐。重写后应用 §14 用例与 `go test ./internal/feat/` 对齐。*
