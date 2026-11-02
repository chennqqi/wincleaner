let latest = null;
let groupKey = "";
let itemKey = "";
const groupRows = new Map();
const itemRows = new Map();

// api is the bound Go cleaner. It is absent when the page is opened outside the window.
function api() {
  if (!window.go || !window.go.main || !window.go.main.App) {
    return null;
  }
  return window.go.main.App;
}

// ask shows the modal and resolves true when the primary button is pressed.
function ask(title, body, yes, no) {
  const scrim = document.getElementById("dialog");
  const noBtn = document.getElementById("dialog-no");
  document.getElementById("dialog-title").textContent = title;
  document.getElementById("dialog-body").textContent = body;
  document.getElementById("dialog-yes").textContent = yes;
  noBtn.hidden = !no;
  if (no) {
    noBtn.textContent = no;
  }
  scrim.hidden = false;
  return new Promise((resolve) => {
    const finish = (value) => {
      scrim.hidden = true;
      document.getElementById("dialog-yes").onclick = null;
      noBtn.onclick = null;
      resolve(value);
    };
    document.getElementById("dialog-yes").onclick = () => finish(true);
    noBtn.onclick = () => finish(false);
  });
}

// render draws a snapshot. Unchanged lists keep their scroll position.
function render(state) {
  if (!state) {
    return;
  }
  latest = state;
  document.getElementById("status").textContent = state.status || "";
  document.getElementById("chosen-bytes").textContent = state.chosen_text || "0 B";
  const bar = document.getElementById("progress");
  if (state.total > 0) {
    bar.hidden = false;
    const pct = Math.max(0, Math.min(100, (state.done / state.total) * 100));
    document.getElementById("progress-bar").style.width = pct + "%";
  } else {
    bar.hidden = true;
  }
  const busy = !!state.busy;
  for (const id of ["scan", "clean", "defaults", "all", "none", "invert"]) {
    document.getElementById(id).disabled = busy;
  }
  const log = document.getElementById("log");
  const atEnd = log.scrollTop + log.clientHeight >= log.scrollHeight - 8;
  log.textContent = (state.logs || []).join("\n");
  if (atEnd) {
    log.scrollTop = log.scrollHeight;
  }
  const nextGroups = (state.groups || []).map((g) => g.id).join("\n");
  if (nextGroups !== groupKey) {
    groupKey = nextGroups;
    rebuildGroups(state);
  } else {
    updateGroups(state);
  }
  const nextItems = (state.selected_id || "") + "\n" + (state.items || []).map((it) => it.path).join("\n");
  if (nextItems !== itemKey) {
    itemKey = nextItems;
    rebuildItems(state);
  } else {
    updateItems(state);
  }
  document.getElementById("item-title").textContent = itemHeading(state);
}

// itemHeading is the title above the file list.
function itemHeading(state) {
  const groups = state.groups || [];
  for (const g of groups) {
    if (g.id === state.selected_id) {
      return g.title;
    }
  }
  return "内容";
}

// rebuildGroups replaces the pattern list when membership or order changes.
function rebuildGroups(state) {
  const root = document.getElementById("groups");
  root.replaceChildren();
  groupRows.clear();
  const groups = state.groups || [];
  if (groups.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.textContent = "扫描完成后在这里选择分组";
    root.appendChild(empty);
    return;
  }
  for (const g of groups) {
    const row = document.createElement("div");
    row.className = "row";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = !!g.checked;
    box.addEventListener("click", (ev) => {
      ev.stopPropagation();
      const bound = api();
      if (!bound) {
        return;
      }
      bound.SetGroup(g.id, box.checked);
    });
    const text = document.createElement("div");
    const title = document.createElement("div");
    title.className = "title";
    title.textContent = g.title;
    const meta = document.createElement("div");
    meta.className = "meta";
    const size = document.createElement("div");
    size.className = "size";
    text.append(title, meta);
    row.append(box, text, size);
    row.addEventListener("click", () => {
      const bound = api();
      if (!bound) {
        return;
      }
      bound.SelectGroup(g.id);
    });
    root.appendChild(row);
    groupRows.set(g.id, { row, box, meta, size });
    paintGroup(g, state.selected_id, state.busy);
  }
}

// updateGroups refreshes sizes and checks without scrolling back to the top.
function updateGroups(state) {
  for (const g of state.groups || []) {
    paintGroup(g, state.selected_id, state.busy);
  }
}

// paintGroup writes one pattern row.
function paintGroup(g, selectedID, busy) {
  const row = groupRows.get(g.id);
  if (!row) {
    return;
  }
  row.box.disabled = !!busy;
  row.box.checked = !!g.checked;
  row.meta.textContent = g.count + " 项 · 已选 " + g.selected + "/" + g.count + " · " + (g.reason || "");
  row.size.textContent = g.size_text || "";
  row.row.classList.toggle("selected", g.id === selectedID);
}

// rebuildItems replaces the file list when the selected group changes.
function rebuildItems(state) {
  const root = document.getElementById("items");
  root.replaceChildren();
  itemRows.clear();
  const items = state.items || [];
  if (!state.selected_id) {
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.textContent = "选择左侧分组后查看目录和文件";
    root.appendChild(empty);
    return;
  }
  if (items.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.textContent = "这个分组里没有文件";
    root.appendChild(empty);
    return;
  }
  const groupID = state.selected_id;
  for (const it of items) {
    const row = document.createElement("div");
    row.className = "row";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = !!it.checked;
    box.addEventListener("click", (ev) => {
      ev.stopPropagation();
      const bound = api();
      if (!bound) {
        return;
      }
      bound.SetItem(groupID, it.index, box.checked);
    });
    const text = document.createElement("div");
    const title = document.createElement("div");
    title.className = "title";
    title.textContent = it.name;
    title.title = it.path || "";
    const meta = document.createElement("div");
    meta.className = "item-path";
    meta.textContent = it.path || "";
    const size = document.createElement("div");
    size.className = "size";
    text.append(title, meta);
    row.append(box, text, size);
    root.appendChild(row);
    itemRows.set(it.index, { box, meta, size });
    paintItem(it, state.busy);
  }
}

// updateItems refreshes checks and sizes in the open group.
function updateItems(state) {
  for (const it of state.items || []) {
    paintItem(it, state.busy);
  }
}

// paintItem writes one file row.
function paintItem(it, busy) {
  const row = itemRows.get(it.index);
  if (!row) {
    return;
  }
  row.box.disabled = !!busy;
  row.box.checked = !!it.checked;
  row.meta.textContent = (it.kind || "") + "  " + (it.path || "");
  row.size.textContent = it.size_text || "";
}

// onClean asks before deleting, then scans again after the result is dismissed.
async function onClean() {
  const bound = api();
  const state = latest;
  if (!bound || !state || state.busy) {
    return;
  }
  if (!state.chosen) {
    await ask("确认清理", "请先勾选要清理的项目", "确定", "");
    return;
  }
  const body = "将清理 " + state.chosen + " 项，大约 " + state.chosen_text + "。\n这些文件位于临时目录，删除后无法恢复。\n正在使用的文件会跳过。";
  const ok = await ask("确认清理", body, "清理", "取消");
  if (!ok) {
    return;
  }
  const result = await bound.Clean();
  if (!result) {
    return;
  }
  if (result.empty) {
    await ask("确认清理", result.message || "请先勾选要清理的项目", "确定", "");
    return;
  }
  await ask("清理结果", result.message || "", "确定", "");
  bound.Scan();
}

// bindControls connects the toolbar once the Go bridge exists.
function bindControls() {
  const bound = api();
  if (!bound || !window.runtime) {
    document.getElementById("status").textContent = "请从 WinCleaner 窗口打开";
    return;
  }
  window.runtime.EventsOn("state", render);
  bound.State().then(render);
  document.getElementById("scan").onclick = () => bound.Scan();
  document.getElementById("clean").onclick = () => onClean();
  document.getElementById("defaults").onclick = () => bound.SelectDefault();
  document.getElementById("all").onclick = () => bound.SelectAll();
  document.getElementById("none").onclick = () => bound.SelectNone();
  document.getElementById("invert").onclick = () => bound.Invert();
  document.getElementById("win-min").onclick = () => window.runtime.WindowMinimise();
  document.getElementById("win-max").onclick = () => window.runtime.WindowToggleMaximise();
  document.getElementById("win-close").onclick = () => window.runtime.Quit();
  document.querySelector("header").addEventListener("dblclick", (ev) => {
    if (ev.target.closest("button")) {
      return;
    }
    window.runtime.WindowToggleMaximise();
  });
}

window.addEventListener("DOMContentLoaded", bindControls);
