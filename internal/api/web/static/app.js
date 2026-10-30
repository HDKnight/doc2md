(() => {
  const dropzone = document.getElementById("dropzone");
  const fileInput = document.getElementById("file");
  const fileName = document.getElementById("fileName");
  const convertBtn = document.getElementById("convertBtn");
  const copyBtn = document.getElementById("copyBtn");
  const downloadBtn = document.getElementById("downloadBtn");
  const clearBtn = document.getElementById("clearBtn");
  const statusEl = document.getElementById("status");
  const resultPanel = document.getElementById("resultPanel");
  const meta = document.getElementById("meta");
  const previewView = document.getElementById("previewView");
  const sourceView = document.getElementById("sourceView");
  const textView = document.getElementById("textView");
  const tabPreview = document.getElementById("tabPreview");
  const tabMarkdown = document.getElementById("tabMarkdown");
  const tabText = document.getElementById("tabText");

  let selectedFile = null;
  let lastMarkdown = "";
  let lastText = "";
  let lastBaseName = "output";
  let viewMode = "preview"; // preview | markdown | text
  let contentReady = false;
  let locateTimer = 0;

  if (window.marked?.setOptions) {
    marked.setOptions({ gfm: true, breaks: false });
  }

  function setStatus(type, text) {
    if (!text) {
      statusEl.hidden = true;
      statusEl.textContent = "";
      statusEl.className = "panel status";
      return;
    }
    statusEl.hidden = false;
    statusEl.className = `panel status ${type}`;
    statusEl.textContent = text;
  }

  function setFile(file) {
    if (!file) return;
    const name = file.name.toLowerCase();
    if (!name.endsWith(".pdf") && !name.endsWith(".docx") && !name.endsWith(".txt")) {
      setStatus("error", "仅支持 .pdf、.docx 或 .txt");
      return;
    }
    if (file.size > 50 * 1024 * 1024) {
      setStatus("error", "文件超过 50MB 限制");
      return;
    }
    selectedFile = file;
    fileName.textContent = `${file.name} · ${formatSize(file.size)}`;
    dropzone.classList.add("has-file");
    convertBtn.disabled = false;
    clearBtn.disabled = false;
    setStatus("", "");
  }

  function formatSize(n) {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(2)} MB`;
  }

  function viewEl(mode) {
    if (mode === "preview") return previewView;
    if (mode === "markdown") return sourceView;
    return textView;
  }

  function renderMarkdown(src) {
    if (!src) {
      previewView.innerHTML = "";
      return;
    }
    if (!window.marked?.parse) {
      previewView.textContent = src;
      return;
    }
    const html = marked.parse(src);
    previewView.innerHTML = window.DOMPurify
      ? DOMPurify.sanitize(html)
      : html;
  }

  function fillViews() {
    renderMarkdown(lastMarkdown);
    sourceView.textContent = lastMarkdown;
    textView.textContent = lastText;
    contentReady = !!(lastMarkdown || lastText);
  }

  function applyTabVisibility() {
    const isPreview = viewMode === "preview";
    const isSource = viewMode === "markdown";
    const isText = viewMode === "text";

    previewView.hidden = !isPreview;
    sourceView.hidden = !isSource;
    textView.hidden = !isText;

    tabPreview.classList.toggle("active", isPreview);
    tabMarkdown.classList.toggle("active", isSource);
    tabText.classList.toggle("active", isText);
    tabPreview.setAttribute("aria-selected", isPreview ? "true" : "false");
    tabMarkdown.setAttribute("aria-selected", isSource ? "true" : "false");
    tabText.setAttribute("aria-selected", isText ? "true" : "false");

    const downloadable = isText ? lastText : lastMarkdown;
    downloadBtn.disabled = !downloadable;
    downloadBtn.textContent = isText ? "下载原文" : "下载 Markdown";
    copyBtn.disabled = !lastMarkdown;
  }

  function scrollRatio(el) {
    const max = el.scrollHeight - el.clientHeight;
    return max <= 0 ? 0 : el.scrollTop / max;
  }

  function setScrollRatio(el, ratio) {
    const max = el.scrollHeight - el.clientHeight;
    if (max <= 0) return;
    el.scrollTop = Math.max(0, Math.min(max, max * ratio));
  }

  function cleanSnippet(s) {
    return String(s || "")
      .replace(/\u00a0/g, " ")
      .replace(/\s+/g, " ")
      .trim();
  }

  function stripMdMarkers(s) {
    return String(s || "")
      .replace(/^#{1,6}\s+/gm, "")
      .replace(/^\s*[-*+]\s+/gm, "")
      .replace(/^\s*\d+[.)]\s+/gm, "")
      .replace(/[*_`~]/g, "")
      .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
      .replace(/\s+/g, " ")
      .trim();
  }

  function compact(s) {
    return String(s || "").replace(/\s+/g, "");
  }

  function findNeedleIndex(haystack, needle) {
    const raw = cleanSnippet(needle);
    if (!raw || raw.length < 2) return -1;

    let idx = haystack.indexOf(raw);
    if (idx >= 0) return idx;

    const strippedNeedle = stripMdMarkers(raw);
    if (strippedNeedle.length >= 4) {
      idx = haystack.indexOf(strippedNeedle);
      if (idx >= 0) return idx;
      idx = stripMdMarkers(haystack).indexOf(strippedNeedle);
      if (idx >= 0) {
        // approximate: search shortened raw piece in original
        const piece = strippedNeedle.slice(0, Math.min(40, strippedNeedle.length));
        const approx = haystack.indexOf(piece);
        if (approx >= 0) return approx;
        // fallback scan line by line with compact match
        const lines = haystack.split("\n");
        let offset = 0;
        const target = compact(strippedNeedle).slice(0, 48);
        for (const line of lines) {
          if (compact(stripMdMarkers(line)).includes(target.slice(0, Math.min(24, target.length)))) {
            return offset + Math.max(0, line.search(/\S/));
          }
          offset += line.length + 1;
        }
      }
    }

    for (let len = Math.min(raw.length, 72); len >= 10; len -= 6) {
      const part = raw.slice(0, len).trim();
      if (part.length < 8) break;
      idx = haystack.indexOf(part);
      if (idx >= 0) return idx;
      const stripped = stripMdMarkers(part);
      if (stripped.length >= 8) {
        idx = haystack.indexOf(stripped);
        if (idx >= 0) return idx;
      }
    }
    return -1;
  }

  function captureSelectionNeedle(scopeEl) {
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed) return "";
    const text = cleanSnippet(sel.toString()).slice(0, 96);
    if (text.length < 2) return "";
    if (!scopeEl.contains(sel.anchorNode)) return "";
    return text;
  }

  function capturePreviewNeedle() {
    const selected = captureSelectionNeedle(previewView);
    if (selected) return selected;

    const box = previewView.getBoundingClientRect();
    const targetY = box.top + Math.min(box.height * 0.28, 120);
    const nodes = previewView.querySelectorAll(
      "h1,h2,h3,h4,h5,h6,p,li,blockquote,pre,td,th,tr"
    );
    let best = null;
    let bestDist = Infinity;
    for (const node of nodes) {
      const rect = node.getBoundingClientRect();
      if (rect.height < 2 || rect.bottom < box.top || rect.top > box.bottom) continue;
      const mid = rect.top + Math.min(rect.height * 0.35, 24);
      const dist = Math.abs(mid - targetY);
      if (dist < bestDist) {
        bestDist = dist;
        best = node;
      }
    }
    if (!best) return "";
    return cleanSnippet(best.textContent).slice(0, 96);
  }

  function capturePreNeedle(el) {
    const selected = captureSelectionNeedle(el);
    if (selected) return selected;

    const text = el.textContent || "";
    if (!text) return "";
    const node = el.firstChild;
    if (!node || node.nodeType !== Node.TEXT_NODE) {
      const ratio = scrollRatio(el);
      const approx = Math.floor(text.length * ratio);
      return cleanSnippet(text.slice(approx, approx + 96));
    }

    const box = el.getBoundingClientRect();
    const y = box.top + Math.min(box.height * 0.28, 120);
    let lo = 0;
    let hi = text.length;
    while (lo < hi) {
      const mid = (lo + hi) >> 1;
      const range = document.createRange();
      range.setStart(node, mid);
      range.setEnd(node, Math.min(mid + 1, text.length));
      const top = range.getBoundingClientRect().top;
      if (top < y) lo = mid + 1;
      else hi = mid;
    }
    const start = Math.max(0, lo - 12);
    const end = Math.min(text.length, start + 120);
    let snippet = text.slice(start, end);
    const nl = snippet.indexOf("\n");
    if (nl > 8 && nl < 80) snippet = snippet.slice(0, nl);
    return cleanSnippet(snippet).slice(0, 96);
  }

  function captureAnchor(mode) {
    const el = viewEl(mode);
    const ratio = scrollRatio(el);
    const needle =
      mode === "preview" ? capturePreviewNeedle() : capturePreNeedle(el);
    return { needle, ratio, from: mode };
  }

  function clearLocateEffects() {
    window.clearTimeout(locateTimer);
    previewView.querySelectorAll(".loc-flash").forEach((n) => n.classList.remove("loc-flash"));
    document.querySelectorAll(".loc-overlay").forEach((n) => n.remove());
    if (CSS.highlights) CSS.highlights.delete("doc2md-locate");
  }

  function flashPreviewNode(node) {
    if (!node) return;
    node.classList.add("loc-flash");
    locateTimer = window.setTimeout(() => node.classList.remove("loc-flash"), 1200);
  }

  function flashPreRange(el, start, length) {
    const node = el.firstChild;
    if (!node || node.nodeType !== Node.TEXT_NODE) return;
    const end = Math.min(node.length, start + Math.max(1, length));
    const range = document.createRange();
    try {
      range.setStart(node, Math.max(0, Math.min(start, node.length)));
      range.setEnd(node, end);
    } catch {
      return;
    }

    if (CSS.highlights) {
      CSS.highlights.set("doc2md-locate", new Highlight(range));
      locateTimer = window.setTimeout(() => {
        if (CSS.highlights) CSS.highlights.delete("doc2md-locate");
      }, 1200);
      return;
    }

    const parentRect = el.getBoundingClientRect();
    const style = getComputedStyle(el);
    if (style.position === "static") el.style.position = "relative";
    for (const rect of range.getClientRects()) {
      const mark = document.createElement("div");
      mark.className = "loc-overlay";
      mark.style.left = `${rect.left - parentRect.left + el.scrollLeft}px`;
      mark.style.top = `${rect.top - parentRect.top + el.scrollTop}px`;
      mark.style.width = `${Math.max(rect.width, 8)}px`;
      mark.style.height = `${Math.max(rect.height, 14)}px`;
      el.appendChild(mark);
      locateTimer = window.setTimeout(() => mark.remove(), 1200);
    }
  }

  function scrollPreToIndex(el, index, flashLen) {
    const node = el.firstChild;
    if (!node || node.nodeType !== Node.TEXT_NODE || index < 0) {
      return false;
    }
    const range = document.createRange();
    const safe = Math.max(0, Math.min(index, node.length));
    try {
      range.setStart(node, safe);
      range.setEnd(node, Math.min(node.length, safe + 1));
    } catch {
      return false;
    }
    const rect = range.getBoundingClientRect();
    const box = el.getBoundingClientRect();
    el.scrollTop += rect.top - box.top - Math.min(box.height * 0.28, 120);
    flashPreRange(el, safe, flashLen || 48);
    return true;
  }

  function locateInPreview(anchor) {
    const needle = cleanSnippet(anchor.needle);
    if (needle) {
      const nodes = previewView.querySelectorAll(
        "h1,h2,h3,h4,h5,h6,p,li,blockquote,pre,td,th"
      );
      const compactNeedle = compact(stripMdMarkers(needle)).slice(0, 48);
      let best = null;
      for (const node of nodes) {
        const text = cleanSnippet(node.textContent);
        if (!text) continue;
        if (text.includes(needle.slice(0, Math.min(needle.length, 40)))) {
          best = node;
          break;
        }
        if (
          compactNeedle.length >= 8 &&
          compact(stripMdMarkers(text)).includes(compactNeedle.slice(0, 24))
        ) {
          best = node;
          break;
        }
      }
      if (best) {
        best.scrollIntoView({ block: "center", behavior: "auto" });
        flashPreviewNode(best);
        return;
      }
    }
    setScrollRatio(previewView, anchor.ratio || 0);
  }

  function locateInPre(el, anchor) {
    const text = el.textContent || "";
    const idx = findNeedleIndex(text, anchor.needle);
    if (idx >= 0) {
      const flashLen = Math.min(64, cleanSnippet(anchor.needle).length || 32);
      if (scrollPreToIndex(el, idx, flashLen)) return;
    }
    setScrollRatio(el, anchor.ratio || 0);
  }

  function applyAnchor(mode, anchor) {
    if (!anchor || !contentReady) return;
    clearLocateEffects();
    if (mode === "preview") locateInPreview(anchor);
    else locateInPre(viewEl(mode), anchor);
  }

  function setView(mode, { sync = true } = {}) {
    if (mode === viewMode && contentReady) {
      applyTabVisibility();
      return;
    }
    const prev = viewMode;
    let anchor = null;
    if (sync && contentReady && prev !== mode) {
      anchor = captureAnchor(prev);
    }
    viewMode = mode;
    applyTabVisibility();
    if (anchor) {
      requestAnimationFrame(() => {
        requestAnimationFrame(() => applyAnchor(mode, anchor));
      });
    }
  }

  function clearAll() {
    selectedFile = null;
    lastMarkdown = "";
    lastText = "";
    contentReady = false;
    fileInput.value = "";
    fileName.textContent = "未选择文件";
    dropzone.classList.remove("has-file");
    convertBtn.disabled = true;
    copyBtn.disabled = true;
    downloadBtn.disabled = true;
    downloadBtn.textContent = "下载";
    clearBtn.disabled = true;
    resultPanel.hidden = true;
    meta.innerHTML = "";
    previewView.innerHTML = "";
    sourceView.textContent = "";
    textView.textContent = "";
    viewMode = "preview";
    clearLocateEffects();
    setStatus("", "");
  }

  dropzone.addEventListener("click", () => fileInput.click());
  dropzone.addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      fileInput.click();
    }
  });
  fileInput.addEventListener("change", () => {
    if (fileInput.files?.[0]) setFile(fileInput.files[0]);
  });

  ["dragenter", "dragover"].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => {
      e.preventDefault();
      dropzone.classList.add("dragover");
    });
  });
  ["dragleave", "drop"].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => {
      e.preventDefault();
      dropzone.classList.remove("dragover");
    });
  });
  dropzone.addEventListener("drop", (e) => {
    const file = e.dataTransfer?.files?.[0];
    if (file) setFile(file);
  });

  tabPreview.addEventListener("click", () => setView("preview"));
  tabMarkdown.addEventListener("click", () => setView("markdown"));
  tabText.addEventListener("click", () => setView("text"));

  convertBtn.addEventListener("click", async () => {
    if (!selectedFile) return;
    convertBtn.disabled = true;
    copyBtn.disabled = true;
    downloadBtn.disabled = true;
    setStatus("info", "转换中…");
    resultPanel.hidden = true;

    const form = new FormData();
    form.append("file", selectedFile);

    try {
      const res = await fetch("/convert", { method: "POST", body: form });
      const ctype = res.headers.get("content-type") || "";
      let payload;
      if (ctype.includes("application/json")) {
        payload = await res.json();
      } else {
        const text = await res.text();
        payload = res.ok ? { markdown: text } : { error: text || res.statusText };
      }

      if (!res.ok) {
        throw new Error(payload.error || `HTTP ${res.status}`);
      }

      lastMarkdown = payload.markdown || "";
      lastText = payload.text || "";
      lastBaseName = selectedFile.name.replace(/\.(pdf|docx|txt)$/i, "");
      viewMode = "preview";

      meta.innerHTML = [
        payload.filename ? `<span>文件 <b>${escapeHtml(payload.filename)}</b></span>` : "",
        payload.format ? `<span>格式 <b>${escapeHtml(payload.format)}</b></span>` : "",
        payload.engine ? `<span>引擎 <b>${escapeHtml(payload.engine)}</b></span>` : "",
        lastMarkdown ? `<span>MD <b>${lastMarkdown.length}</b> 字</span>` : "",
        lastText ? `<span>原文 <b>${lastText.length}</b> 字</span>` : "",
        ...ocrStatsMeta(payload.ocr_stats),
      ].filter(Boolean).join("");

      fillViews();
      applyTabVisibility();
      resultPanel.hidden = false;
      const ocrHint = formatOCRStatsHint(payload.ocr_stats);
      setStatus("info", ocrHint
        ? `转换完成 · ${ocrHint}`
        : "转换完成 · 切换预览/源码/原文会定位到当前位置");
    } catch (err) {
      setStatus("error", err.message || String(err));
    } finally {
      convertBtn.disabled = !selectedFile;
    }
  });

  copyBtn.addEventListener("click", async () => {
    if (!lastMarkdown) return;
    try {
      await navigator.clipboard.writeText(lastMarkdown);
      setStatus("info", "已复制 Markdown");
    } catch {
      setStatus("error", "复制失败，请在源码视图里手动复制");
    }
  });

  downloadBtn.addEventListener("click", () => {
    const isText = viewMode === "text";
    const content = isText ? lastText : lastMarkdown;
    if (!content) return;
    const blob = new Blob([content], {
      type: isText ? "text/plain;charset=utf-8" : "text/markdown;charset=utf-8",
    });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${lastBaseName}${isText ? ".txt" : ".md"}`;
    a.click();
    URL.revokeObjectURL(url);
  });

  clearBtn.addEventListener("click", clearAll);

  function escapeHtml(s) {
    return String(s)
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;")
      .replaceAll('"', "&quot;");
  }

  function ocrStatsMeta(stats) {
    if (!stats || !stats.used) return [];
    const avg = Number(stats.avg_ms_per_page || 0);
    const totalSec = (Number(stats.total_ms || 0) / 1000).toFixed(1);
    const avgSec = (avg / 1000).toFixed(2);
    return [
      stats.model ? `<span>OCR 模型 <b>${escapeHtml(stats.model)}</b></span>` : "",
      `<span>OCR 总时长 <b>${totalSec}s</b></span>`,
      `<span>每页平均 <b>${avgSec}s</b>（${stats.pages || 0} 页）</span>`,
    ];
  }

  function formatOCRStatsHint(stats) {
    if (!stats || !stats.used) return "";
    const totalSec = (Number(stats.total_ms || 0) / 1000).toFixed(1);
    const avgSec = (Number(stats.avg_ms_per_page || 0) / 1000).toFixed(2);
    return `OCR ${stats.model || ""} · 总 ${totalSec}s · 均 ${avgSec}s/页 · ${stats.pages || 0} 页`;
  }

})();
