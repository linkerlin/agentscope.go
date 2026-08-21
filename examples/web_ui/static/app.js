// AgentScope Go Console — zero-build dashboard SPA.
// Vanilla ES5-ish JS (no modules/bundler) so it runs from go:embed with no
// build step. Sections: router · chat (AG-UI SSE) · knowledge bases · system.

// ───────────────────────── shared helpers ─────────────────────────

function getSessionId() {
  const KEY = "agentscope-go.session-id";
  let id = localStorage.getItem(KEY);
  if (!id) { id = crypto.randomUUID(); localStorage.setItem(KEY, id); }
  return id;
}
const sessionId = getSessionId();
document.getElementById("session-id").textContent = sessionId.slice(0, 8) + "…";

function escapeHtml(s) {
  return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) { opts.headers["Content-Type"] = "application/json"; opts.body = JSON.stringify(body); }
  const res = await fetch(path, opts);
  if (res.status === 204) return null;
  const txt = await res.text();
  let data = txt;
  try { data = txt ? JSON.parse(txt) : null; } catch (_) {}
  if (!res.ok) throw new Error(typeof data === "object" && data ? (data.error || data.message || res.statusText) : `${res.status}`);
  return data;
}

// ───────────────────────── router ─────────────────────────

document.querySelectorAll(".nav-item").forEach(btn => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".nav-item").forEach(b => b.classList.remove("active"));
    btn.classList.add("active");
    const view = btn.dataset.view;
    document.querySelectorAll(".view").forEach(v => v.classList.remove("active"));
    document.getElementById("view-" + view).classList.add("active");
    if (view === "kb") loadKBList();
    if (view === "system") loadSystem();
    if (view === "cp") loadCPGoals();
  });
});

// ───────────────────────── chat (AG-UI SSE) ─────────────────────────

const SESSION_STORAGE_KEY = "agentscope-go.session-id";
const messagesEl = document.getElementById("messages");
const form = document.getElementById("chat-form");
const input = document.getElementById("input");
const sendBtn = document.getElementById("send-btn");
const reconnectStatusEl = document.getElementById("reconnect-status");
let activeStream = null;

const AGUI_EVENTS = new Set([
  "RUN_STARTED","RUN_FINISHED","RUN_ERROR","STEP_STARTED",
  "REASONING_MESSAGE_START","REASONING_MESSAGE_CONTENT",
  "TEXT_MESSAGE_CONTENT","TOOL_CALL_START","TOOL_CALL_ARGS",
  "TOOL_CALL_RESULT","CUSTOM","STREAM_DONE",
]);

form.addEventListener("submit", (e) => {
  e.preventDefault();
  const text = input.value.trim();
  if (!text || sendBtn.disabled) return;
  input.value = "";
  sendMessage(text);
});

// ── mid-turn steering / interrupt (Phase 14.3) ──
const steerInput = document.getElementById("steer-input");
const steerBtn = document.getElementById("steer-btn");
const interruptBtn = document.getElementById("interrupt-btn");
function appendSystemNote(text) {
  const el = document.createElement("div");
  el.className = "system-note";
  el.textContent = text;
  messagesEl.appendChild(el);
  messagesEl.scrollTop = messagesEl.scrollHeight;
}
if (steerBtn) steerBtn.addEventListener("click", () => {
  const text = (steerInput.value || "").trim();
  if (!text) return;
  steerInput.value = "";
  api("POST", `/v2/sessions/${encodeURIComponent(sessionId)}/steer`, { text })
    .then(() => appendSystemNote("steer 注入: " + text))
    .catch(err => appendSystemNote("steer 失败: " + err.message));
});
if (interruptBtn) interruptBtn.addEventListener("click", () => {
  api("POST", `/v2/sessions/${encodeURIComponent(sessionId)}/interrupt`)
    .then(() => appendSystemNote("已发送打断"))
    .catch(err => appendSystemNote("打断失败: " + err.message));
});
function setReconnectStatus(t) { if (reconnectStatusEl) reconnectStatusEl.textContent = t; }
function appendUserBubble(text) {
  const el = document.createElement("div");
  el.className = "bubble user";
  el.textContent = text;
  messagesEl.appendChild(el);
  messagesEl.scrollTop = messagesEl.scrollHeight;
}
function createAssistantRun() {
  const wrap = document.createElement("div");
  wrap.className = "bubble assistant";
  const meta = document.createElement("div");
  meta.className = "run-meta"; meta.textContent = "Assistant · streaming…";
  wrap.appendChild(meta);
  const reasoning = document.createElement("details");
  reasoning.className = "reasoning"; reasoning.style.display = "none";
  const summary = document.createElement("summary"); summary.textContent = "Reasoning";
  const reasoningBody = document.createElement("div"); reasoningBody.className = "body";
  reasoning.appendChild(summary); reasoning.appendChild(reasoningBody);
  wrap.appendChild(reasoning);
  const toolsEl = document.createElement("div"); toolsEl.className = "tools";
  wrap.appendChild(toolsEl);
  const textEl = document.createElement("div"); textEl.className = "text typing";
  wrap.appendChild(textEl);
  messagesEl.appendChild(wrap);
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return { wrap, meta, reasoning, reasoningBody, toolsEl, textEl, tools: new Map(), toolArgs: new Map() };
}
function getOrCreateTool(run, id, name) {
  if (run.tools.has(id)) return run.tools.get(id);
  const card = document.createElement("div");
  card.className = "tool-card";
  card.innerHTML = `<div class="name">${escapeHtml(name || id)}</div><div class="args"></div><div class="result"></div>`;
  run.toolsEl.appendChild(card);
  run.tools.set(id, card); run.toolArgs.set(id, "");
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return card;
}
function handleAGUIEvent(run, ev) {
  switch (ev.type) {
    case "RUN_STARTED": run.meta.textContent = `Run ${ev.runId || ""} · ${ev.threadId || sessionId}`; break;
    case "RUN_FINISHED": run.textEl.classList.remove("typing"); run.meta.textContent = (run.meta.textContent || "").replace(/streaming…|重连中…/g, "done"); break;
    case "RUN_ERROR": run.textEl.classList.remove("typing"); run.textEl.innerHTML = `<span class="error-banner">${escapeHtml(ev.message || "Error")}</span>`; break;
    case "STEP_STARTED": run.meta.insertAdjacentHTML("beforeend", ` <span class="step-pill">${escapeHtml(ev.stepName || "step")}</span>`); break;
    case "REASONING_MESSAGE_START": run.reasoning.style.display = "block"; run.reasoning.open = true; break;
    case "REASONING_MESSAGE_CONTENT": run.reasoningBody.textContent += ev.delta || ""; messagesEl.scrollTop = messagesEl.scrollHeight; break;
    case "TEXT_MESSAGE_CONTENT": run.textEl.textContent += ev.delta || ""; messagesEl.scrollTop = messagesEl.scrollHeight; break;
    case "TOOL_CALL_START": { const c = getOrCreateTool(run, ev.toolCallId, ev.toolCallName); c.querySelector(".args").textContent = "args: …"; break; }
    case "TOOL_CALL_ARGS": { const prev = run.toolArgs.get(ev.toolCallId) || ""; const next = prev + (ev.delta || ""); run.toolArgs.set(ev.toolCallId, next); const c = run.tools.get(ev.toolCallId); if (c) c.querySelector(".args").textContent = "args: " + next; break; }
    case "TOOL_CALL_RESULT": { const c = run.tools.get(ev.toolCallId); if (c) c.querySelector(".result").textContent = "result: " + (ev.content || ""); break; }
    case "CUSTOM": if (ev.name === "require_user_confirm") run.textEl.insertAdjacentHTML("beforeend", `<div class="error-banner">HITL: user confirmation required (resume via /v2/resume)</div>`); break;
    case "STREAM_DONE": run.textEl.classList.remove("typing"); break;
  }
}
async function consumeEventStream(res, run, signal) {
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "", meaningful = false;
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const chunks = buffer.split("\n\n");
    buffer = chunks.pop() || "";
    for (const chunk of chunks) {
      const line = chunk.split("\n").find(l => l.startsWith("data: "));
      if (!line) continue;
      try {
        const ev = JSON.parse(line.slice(6));
        if (AGUI_EVENTS.has(ev.type)) meaningful = true;
        handleAGUIEvent(run, ev);
      } catch (e) { /* ignore parse error */ }
    }
    if (signal?.aborted) { reader.cancel(); break; }
  }
  return meaningful;
}
async function reconnectOnLoad() {
  setReconnectStatus("重连中…"); sendBtn.disabled = true;
  const controller = new AbortController(); activeStream = controller;
  const run = createAssistantRun(); run.meta.textContent = "Assistant · 重连中…";
  try {
    const url = `/v2/chat?protocol=agui&session_id=${encodeURIComponent(sessionId)}`;
    const res = await fetch(url, { method: "GET", headers: { Accept: "application/json, text/event-stream", "Agent-Session-Id": sessionId }, signal: controller.signal });
    if (res.status === 404 || res.status === 503) { run.wrap.remove(); setReconnectStatus(""); return; }
    if (!res.ok) { run.wrap.remove(); setReconnectStatus(`重连失败 (${res.status})`); return; }
    const meaningful = await consumeEventStream(res, run, controller.signal);
    if (!meaningful) { run.wrap.remove(); setReconnectStatus(""); return; }
    run.textEl.classList.remove("typing");
    setReconnectStatus("已重连");
  } catch (err) {
    if (err.name !== "AbortError") { run.wrap.remove(); setReconnectStatus("重连失败"); }
  } finally {
    run.textEl.classList.remove("typing"); sendBtn.disabled = false;
    if (activeStream === controller) activeStream = null;
  }
}
async function sendMessage(text) {
  if (activeStream) activeStream.abort();
  appendUserBubble(text);
  const run = createAssistantRun();
  sendBtn.disabled = true; setReconnectStatus("");
  const controller = new AbortController(); activeStream = controller;
  try {
    const res = await fetch("/v2/chat?protocol=agui", {
      method: "POST",
      headers: { "Content-Type": "application/json", Accept: "application/json, text/event-stream", "Agent-Session-Id": sessionId },
      body: JSON.stringify({ text, session_id: sessionId }),
      signal: controller.signal,
    });
    if (!res.ok) { const t = await res.text(); throw new Error(`${res.status}: ${t}`); }
    await consumeEventStream(res, run, controller.signal);
  } catch (err) {
    if (err.name !== "AbortError") { run.textEl.classList.remove("typing"); run.textEl.innerHTML = `<span class="error-banner">${escapeHtml(err.message)}</span>`; }
  } finally {
    run.textEl.classList.remove("typing"); sendBtn.disabled = false; activeStream = null;
  }
}
reconnectOnLoad();

// ───────────────────────── knowledge bases ─────────────────────────

const kbListEl = document.getElementById("kb-list");
const kbDetailEl = document.getElementById("kb-detail");
const kbModal = document.getElementById("kb-modal");

async function loadKBList() {
  try {
    const data = await api("GET", "/api/v1/knowledge-bases");
    const kbs = (data && data.knowledge_bases) || [];
    if (!kbs.length) { kbListEl.innerHTML = `<div class="empty">尚无知识库，点击「+ 新建」创建。</div>`; return; }
    kbListEl.innerHTML = kbs.map(kb => `
      <div class="kb-card" data-name="${escapeHtml(kb.name)}">
        <button class="kb-del" data-del="${escapeHtml(kb.name)}" title="删除">✕</button>
        <h3>${escapeHtml(kb.name)}</h3>
        <div class="kb-meta">${escapeHtml(kb.description || "无描述")}<br>collection: ${escapeHtml(kb.collection || "")}</div>
      </div>`).join("");
    kbListEl.querySelectorAll(".kb-card").forEach(card => {
      card.addEventListener("click", e => {
        if (e.target.dataset.del) return;
        openKBDetail(card.dataset.name);
      });
    });
    kbListEl.querySelectorAll(".kb-del").forEach(btn => {
      btn.addEventListener("click", async e => {
        e.stopPropagation();
        if (!confirm("删除知识库 " + btn.dataset.del + "?")) return;
        await api("DELETE", "/api/v1/knowledge-bases/" + encodeURIComponent(btn.dataset.del));
        loadKBList();
      });
    });
  } catch (err) {
    kbListEl.innerHTML = `<div class="empty">知识库 API 不可用：${escapeHtml(err.message)}<br><span class="dim">（需在服务端配置 KBService）</span></div>`;
  }
}

document.getElementById("kb-new-btn").addEventListener("click", () => kbModal.classList.remove("hidden"));
document.getElementById("kb-modal-cancel").addEventListener("click", () => kbModal.classList.add("hidden"));
document.getElementById("kb-modal-create").addEventListener("click", async () => {
  const name = document.getElementById("kb-form-name").value.trim();
  const desc = document.getElementById("kb-form-desc").value.trim();
  if (!name) return;
  try {
    await api("POST", "/api/v1/knowledge-bases", { name, description: desc, embedder_id: "stub" });
    kbModal.classList.add("hidden");
    document.getElementById("kb-form-name").value = "";
    document.getElementById("kb-form-desc").value = "";
    loadKBList();
  } catch (err) { alert("创建失败：" + err.message); }
});

let currentKB = null;
function openKBDetail(name) {
  currentKB = name;
  kbListEl.classList.add("hidden");
  document.querySelector("#view-kb .view-head").classList.add("hidden");
  kbDetailEl.classList.remove("hidden");
  document.getElementById("kb-detail-name").textContent = name;
  loadKBDocs(name);
  document.getElementById("kb-search-results").innerHTML = "";
}
document.getElementById("kb-back-btn").addEventListener("click", () => {
  kbDetailEl.classList.add("hidden");
  document.querySelector("#view-kb .view-head").classList.remove("hidden");
  kbListEl.classList.remove("hidden");
  currentKB = null;
});

async function loadKBDocs(name) {
  const docList = document.getElementById("kb-doc-list");
  try {
    const data = await api("GET", "/api/v1/knowledge-bases/" + encodeURIComponent(name));
    const docs = (data && data.documents) || [];
    if (!docs.length) { docList.innerHTML = `<div class="empty">尚无文档，点击「上传文档」。</div>`; return; }
    docList.innerHTML = docs.map(d => `
      <div class="doc-item">
        <span>📄 ${escapeHtml(d.source || d.doc_id)} <span class="dim">(${d.chunks} chunks)</span></span>
        <button class="doc-del" data-doc="${escapeHtml(d.doc_id)}">✕</button>
      </div>`).join("");
    docList.querySelectorAll(".doc-del").forEach(btn => {
      btn.addEventListener("click", async () => {
        await api("DELETE", `/api/v1/knowledge-bases/${encodeURIComponent(name)}/documents/${encodeURIComponent(btn.dataset.doc)}`);
        loadKBDocs(name);
      });
    });
  } catch (err) { docList.innerHTML = `<div class="empty">${escapeHtml(err.message)}</div>`; }
}

document.getElementById("kb-upload-input").addEventListener("change", async (e) => {
  const files = Array.from(e.target.files);
  if (!files.length || !currentKB) return;
  for (const file of files) {
    const fd = new FormData();
    fd.append("file", file);
    try {
      const res = await fetch(`/api/v1/knowledge-bases/${encodeURIComponent(currentKB)}/documents`, { method: "POST", body: fd });
      if (!res.ok) { const t = await res.text(); alert(`上传 ${file.name} 失败: ${t}`); }
    } catch (err) { alert(`上传 ${file.name} 失败: ${err.message}`); }
  }
  e.target.value = "";
  loadKBDocs(currentKB);
});

document.getElementById("kb-search-btn").addEventListener("click", doKBSearch);
document.getElementById("kb-search-input").addEventListener("keydown", e => { if (e.key === "Enter") doKBSearch(); });
async function doKBSearch() {
  if (!currentKB) return;
  const query = document.getElementById("kb-search-input").value.trim();
  if (!query) return;
  const resultsEl = document.getElementById("kb-search-results");
  resultsEl.innerHTML = `<div class="empty">检索中…</div>`;
  try {
    const data = await api("POST", `/api/v1/knowledge-bases/${encodeURIComponent(currentKB)}/search`, { query });
    const results = (data && data.results) || [];
    if (!results.length) { resultsEl.innerHTML = `<div class="empty">无匹配结果。</div>`; return; }
    resultsEl.innerHTML = results.map(r => `
      <div class="result-item">
        <span class="score">${r.score ? r.score.toFixed(3) : ""}</span>
        <div class="src">${escapeHtml(r.source || "")}</div>
        <div>${escapeHtml(r.text || "")}</div>
      </div>`).join("");
  } catch (err) { resultsEl.innerHTML = `<div class="empty">${escapeHtml(err.message)}</div>`; }
}

// ───────────────────────── system ─────────────────────────

async function loadSystem() {
  const healthEl = document.getElementById("sys-health");
  const modelsEl = document.getElementById("sys-models");
  try {
    const res = await fetch("/health");
    healthEl.textContent = `health: ${res.status} ${res.statusText}`;
  } catch (e) { healthEl.textContent = "health: " + e.message; }
  try {
    const data = await api("GET", "/api/v1/models");
    const models = (data && (data.models || data.data)) || [];
    if (!models.length) { modelsEl.textContent = "(models API 未配置或为空)"; return; }
    modelsEl.innerHTML = models.map(m => `<div>• ${escapeHtml(m.id || m.name || JSON.stringify(m))}</div>`).join("");
  } catch (e) { modelsEl.textContent = "(" + e.message + ")"; }
}

// ───────────────────────── control plane ─────────────────────────
// Long-running agent governance (LoopX-style): lifetime goals, gates,
// quota-gated ShouldRun, evidence-backed writeback, decision lineage.
// Talks to /api/v1/controlplane/* registered by gateway.RegisterControlPlaneRoutes.

const cpListEl = document.getElementById("cp-goal-list");
const cpDetailEl = document.getElementById("cp-detail");
const cpModal = document.getElementById("cp-modal");
const STATE_COLOR = { active: "var(--green)", paused: "#d29922", completed: "var(--text-dim)", abandoned: "var(--red)" };

function cpBadge(state) {
  const c = STATE_COLOR[state] || "var(--text-dim)";
  return `<span class="badge" style="background:${c}22;color:${c}">${escapeHtml(state)}</span>`;
}

const CAP_STATUS_COLOR = { stable: "var(--green)", experimental: "#d29922", "compatibility-facade": "var(--text-dim)" };

async function loadCPCapabilities() {
  const el = document.getElementById("cp-capabilities");
  if (!el) return;
  try {
    const data = await api("GET", "/api/v1/controlplane/capabilities");
    const caps = (data && data.capabilities) || [];
    if (!caps.length) { el.innerHTML = `<div class="empty">无能力。控制平面可能未启用。</div>`; return; }
    el.innerHTML = caps.map(c => {
      const color = CAP_STATUS_COLOR[c.status] || "var(--text-dim)";
      const lane = (c.lane || []).map(s => `<span class="lane-stage${s.gate ? " gated" : ""}" title="${s.gate ? "gated" : ""}">${escapeHtml(s.label)}${s.gate ? "🔒" : ""}</span>`).join(`<span class="lane-arrow">→</span>`);
      return `<div class="cap-card">
        <div class="cap-head"><span class="badge" style="background:${color}22;color:${color}">${escapeHtml(c.status)}</span> <strong>${escapeHtml(c.id)}</strong></div>
        <div class="dim" style="font-size:12px;margin:4px 0">${escapeHtml(c.user_value || "")}</div>
        <div class="lane">${lane}</div>
      </div>`;
    }).join("");
  } catch (err) { el.innerHTML = `<div class="empty dim">能力 API 不可用：${escapeHtml(err.message)}</div>`; }
}

async function loadCPGoals() {
  loadCPCapabilities();
  try {
    const data = await api("GET", "/api/v1/controlplane/goals");
    const goals = (data && data.goals) || [];
    if (!goals.length) { cpListEl.innerHTML = `<div class="empty">无目标。控制平面可能未启用，或点击「+ New Goal」。</div>`; return; }
    cpListEl.innerHTML = goals.map(g => `
      <div class="goal-card" data-id="${escapeHtml(g.id)}">
        <div class="goal-card-head">
          ${cpBadge(g.state)}
          <span class="dim mono" style="font-size:10px">${escapeHtml(g.id.slice(0,8))}…</span>
        </div>
        <h3>${escapeHtml(g.objective || "(no objective)")}</h3>
        <div class="dim" style="font-size:12px;margin-top:6px">current todo: ${escapeHtml(g.current_todo_id || "—")}</div>
        <div class="dim" style="font-size:12px">quota: compute=${(g.quota && g.quota.compute) || 0}</div>
      </div>`).join("");
    cpListEl.querySelectorAll(".goal-card").forEach(card => {
      card.addEventListener("click", () => openCPGoal(card.dataset.id));
    });
  } catch (err) {
    cpListEl.innerHTML = `<div class="empty">控制平面 API 不可用：${escapeHtml(err.message)}<br><span class="dim">（需 AppConfig.ControlPlane）</span></div>`;
  }
}

document.getElementById("cp-new-btn").addEventListener("click", () => cpModal.classList.remove("hidden"));document.getElementById("cp-modal-cancel").addEventListener("click", () => cpModal.classList.add("hidden"));
document.getElementById("cp-modal-create").addEventListener("click", async () => {
  const objective = document.getElementById("cp-form-obj").value.trim();
  if (!objective) return;
  const scopeRaw = document.getElementById("cp-form-scope").value.trim();
  const scope = scopeRaw ? scopeRaw.split(",").map(s => s.trim()).filter(Boolean) : [];
  try {
    await api("POST", "/api/v1/controlplane/goals", { objective, scope });
    cpModal.classList.add("hidden");
    document.getElementById("cp-form-obj").value = "";
    document.getElementById("cp-form-scope").value = "";
    loadCPGoals();
  } catch (err) { alert("创建失败：" + err.message); }
});

let currentCPGoal = null;
function openCPGoal(id) {
  currentCPGoal = id;
  cpListEl.classList.add("hidden");
  document.getElementById("cp-list-head").classList.add("hidden");
  cpDetailEl.classList.remove("hidden");
  loadCPDetail(id);
}
document.getElementById("cp-back-btn").addEventListener("click", () => {
  cpDetailEl.classList.add("hidden");
  document.getElementById("cp-list-head").classList.remove("hidden");
  cpListEl.classList.remove("hidden");
  currentCPGoal = null;
});

async function loadCPDetail(id) {
  const pkt = await api("GET", `/api/v1/controlplane/goals/${id}/review`);
  const g = pkt.goal || {};
  document.getElementById("cp-obj").textContent = g.objective || "(no objective)";
  document.getElementById("cp-meta").innerHTML =
    `${cpBadge(g.state)} · scope: ${escapeHtml((g.scope || []).join(", ") || "—")}<br>` +
    `<span class="mono">id=${escapeHtml(g.id)}</span>`;

  // State transition buttons (legal moves only).
  const actionsEl = document.getElementById("cp-actions");
  const transitions = { active: ["paused", "completed", "abandoned"], paused: ["active", "abandoned"] };
  const allowed = transitions[g.state] || [];
  actionsEl.innerHTML = allowed.map(t => `<button class="btn" data-trans="${t}">${t}</button>`).join("");
  actionsEl.querySelectorAll("button[data-trans]").forEach(b => {
    b.addEventListener("click", () => cpTransition(id, b.dataset.trans));
  });

  // Quota bar.
  const q = pkt.quota || {};
  const pct = q.allowed > 0 ? Math.min(100, Math.round((q.spent / q.allowed) * 100)) : 0;
  document.getElementById("cp-quota").textContent =
    `spent ${q.spent || 0} / allowed ${q.allowed || 0}  (compute=${q.compute || 0}, window=${q.window_hours || 0}h)`;
  document.getElementById("cp-quota-fill").style.width = pct + "%";

  // Open todos (each with a Supersede action: replace with a new approach).
  const todos = pkt.open_todos || [];
  const todosEl = document.getElementById("cp-todos");
  todosEl.innerHTML = todos.length
    ? todos.map(t => `
        <div class="doc-item">
          <span>${escapeHtml(t.description || t.id)} <span class="dim">[${escapeHtml(t.task_class)}] owner=${escapeHtml(t.claimed_by || "—")}</span></span>
          <button class="doc-del" data-sup="${escapeHtml(t.id)}" title="Supersede（替换为新方案）">↻</button>
        </div>`).join("")
    : `<div class="empty">无开放 todo。</div>`;
  todosEl.querySelectorAll("button[data-sup]").forEach(b => {
    b.addEventListener("click", () => cpSupersedeTodo(id, b.dataset.sup));
  });

  // Pending gates with resolve buttons.
  const gates = pkt.pending_gates || [];
  const gatesEl = document.getElementById("cp-gates");
  gatesEl.innerHTML = gates.length
    ? gates.map(gate => `
        <div class="gate-item">
          <div class="gate-q">🔒 ${escapeHtml(gate.question || "(no question)")}</div>
          <div class="dim mono" style="font-size:11px">gate=${escapeHtml(gate.gate_id)} scope=${escapeHtml((gate.scope && gate.scope.kind) || "")}:${escapeHtml((gate.scope && gate.scope.scope_key) || "")}</div>
          <div class="gate-actions">
            <button class="btn primary" data-gate="${escapeHtml(gate.gate_id)}" data-dec="approve">Approve</button>
            <button class="btn" data-gate="${escapeHtml(gate.gate_id)}" data-dec="reject">Reject</button>
          </div>
        </div>`).join("")
    : `<div class="empty">无待决门。</div>`;
  gatesEl.querySelectorAll("button[data-gate]").forEach(b => {
    b.addEventListener("click", () => cpResolveGate(id, b.dataset.gate, b.dataset.dec));
  });

  // Reward policies with revoke buttons.
  const policies = pkt.active_policies || [];
  const rewardsEl = document.getElementById("cp-rewards");
  rewardsEl.innerHTML = policies.length
    ? policies.filter(p => p.lifecycle === "active" && p.class === "hard_policy").map(p => `
        <div class="gate-item" style="border-left-color:var(--red)">
          <div class="gate-q">⛔ [${escapeHtml(p.class)}] ${escapeHtml(p.content || "(no content)")}</div>
          <div class="dim mono" style="font-size:11px">id=${escapeHtml(p.id || "—")} scope=${escapeHtml((p.scope && p.scope.scope_key) || "")}</div>
          <div class="gate-actions"><button class="btn" data-rid="${escapeHtml(p.id)}">Revoke</button></div>
        </div>`).join("")
      : `<div class="empty">无活动策略。</div>`;
  rewardsEl.querySelectorAll("button[data-rid]").forEach(b => {
    b.addEventListener("click", () => cpRevokeReward(id, b.dataset.rid));
  });

  // Decision lineage.
  const lineage = pkt.decision_lineage || [];
  const linEl = document.getElementById("cp-lineage");
  linEl.innerHTML = lineage.length
    ? lineage.map(e => `<div class="lineage-item"><span class="lineage-kind" data-kind="${escapeHtml(e.kind)}">${escapeHtml(e.kind)}</span> <span class="mono">${escapeHtml(e.type)}</span>${e.outcome ? ` <span class="dim">(${escapeHtml(e.outcome)})</span>` : ""}</div>`).join("")
    : `<div class="empty">无谱系记录。</div>`;

  document.getElementById("cp-decision").textContent = "点击「检查」查询…";
}

document.getElementById("cp-check-btn").addEventListener("click", async () => {
  if (!currentCPGoal) return;
  const el = document.getElementById("cp-decision");
  el.textContent = "查询中…";
  try {
    const dec = await api("GET", `/api/v1/controlplane/goals/${currentCPGoal}/should-run?agent=operator`);
    let s = `should_run=${dec.should_run}\nstate=${dec.state}  route=${dec.route}\nmode=${dec.mode}  notify=${dec.notify}\nscheduler=${dec.scheduler}`;
    if (dec.question) s += `\n gated by ${dec.gate_id}: ${dec.question}`;
    if (dec.fallback_authorized) s += `\n fallback authorized: ${dec.fallback && dec.fallback.action}`;
    if (dec.reason) s += `\n reason: ${dec.reason}`;
    el.textContent = s;
  } catch (err) { el.innerHTML = `<span class="error-banner">${escapeHtml(err.message)}</span>`; }
});

async function cpTransition(id, to) {
  try {
    await api("PATCH", `/api/v1/controlplane/goals/${id}`, { state: to });
    loadCPDetail(id);
  } catch (err) { alert("状态转移失败：" + err.message); }
}

async function cpResolveGate(goalId, gateId, decision) {
  try {
    await api("POST", `/api/v1/controlplane/goals/${goalId}/gates/${gateId}/resolve`, { decision, by: "operator" });
    loadCPDetail(goalId);
  } catch (err) { alert("解门失败：" + err.message); }
}

async function cpRevokeReward(goalId, recordId) {
  if (!confirm("Revoke this policy? The veto will stop blocking this goal.")) return;
  try {
    await api("POST", `/api/v1/controlplane/goals/${goalId}/rewards/${recordId}/revoke`);
    loadCPDetail(goalId);
  } catch (err) { alert("撤销失败：" + err.message); }
}

async function cpSupersedeTodo(goalId, todoId) {
  const desc = prompt("Supersede（替换）——新方案描述：", "");
  if (!desc) return;
  if (!confirm("旧 todo 将标记 deferred 并链接到新 todo。继续？")) return;
  try {
    await api("POST", `/api/v1/controlplane/goals/${goalId}/todos/${todoId}/supersede`,
      { description: desc, agent: "operator", reason: "replaced via console" });
    loadCPDetail(goalId);
  } catch (err) { alert("Supersede 失败：" + err.message); }
}

// Record a reward policy: prompts for class + content, then POSTs it.document.getElementById("cp-reward-btn").addEventListener("click", () => {
  if (!currentCPGoal) return;
  const cls = prompt("策略类别（hard_policy = 否决，soft_preference = 建议）", "hard_policy");
  if (!cls) return;
  const content = prompt("策略内容", "no deploys until audit clears");
  if (!content) return;
  api("POST", `/api/v1/controlplane/goals/${currentCPGoal}/rewards`, { class: cls, content, confidence: "high" })
    .then(() => loadCPDetail(currentCPGoal))
    .catch(err => alert("记录失败：" + err.message));
});

// Kanban projection + row lineage. Renders columns by TodoState; each card
// shows its row-lifecycle badge so an operator sees supersession as data.
const KANBAN_COLS = [
  { key: "open", label: "Open" },
  { key: "blocked", label: "Blocked" },
  { key: "done", label: "Done" },
  { key: "deferred", label: "Deferred" },
];
const LIFE_BADGE = { current: "var(--green)", superseded: "#d29922", retired: "var(--text-dim)" };

document.getElementById("cp-kanban-btn").addEventListener("click", () => { if (currentCPGoal) loadCPKanban(currentCPGoal); });

async function loadCPKanban(id) {
  const el = document.getElementById("cp-kanban");
  el.innerHTML = `<div class="empty">加载看板…</div>`;
  try {
    const board = await api("GET", `/api/v1/controlplane/goals/${id}/kanban`);
    const cols = board.columns || {};
    const lineage = board.lineage || [];
    el.innerHTML = `<div class="kanban-cols">${KANBAN_COLS.map(c => {
      const cards = cols[c.key] || [];
      return `<div class="kanban-col"><div class="kanban-col-head">${c.label} <span class="dim">(${cards.length})</span></div>${cards.map(card => renderKanbanCard(card)).join("") || `<div class="empty" style="padding:10px">—</div>`}</div>`;
    }).join("")}</div>`;
    if (lineage.length) {
      el.innerHTML += `<div class="dim mono" style="margin-top:10px;font-size:11px">lineage: ${lineage.map(e => `${e.from.slice(0,6)}→${e.to.slice(0,6)}`).join(", ")}</div>`;
    }
  } catch (err) { el.innerHTML = `<div class="empty">${escapeHtml(err.message)}</div>`; }
}

function renderKanbanCard(card) {
  const t = card.todo || {};
  const lc = card.lifecycle || "current";
  const color = LIFE_BADGE[lc] || "var(--text-dim)";
  const gate = card.has_open_gate ? ` <span title="gated">🔒</span>` : "";
  const sup = lc === "superseded" && card.superseded_by ? ` <span class="dim" title="replaced by">→${escapeHtml(card.superseded_by.slice(0,6))}</span>` : "";
  const owner = t.claimed_by ? ` · ${escapeHtml(t.claimed_by)}` : "";
  return `<div class="kanban-card" data-lifecycle="${escapeHtml(lc)}">
    <div class="kanban-card-head"><span class="badge" style="background:${color}22;color:${color}">${escapeHtml(lc)}</span>${gate}</div>
    <div class="kanban-card-desc">${escapeHtml(t.description || t.id)}${sup}</div>
    <div class="dim" style="font-size:11px">${escapeHtml(t.task_class || "")}${owner}</div>
  </div>`;
}
