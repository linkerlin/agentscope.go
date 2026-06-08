const SESSION_STORAGE_KEY = "agentscope-go.session-id";

function getOrCreateSessionId() {
  let id = localStorage.getItem(SESSION_STORAGE_KEY);
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem(SESSION_STORAGE_KEY, id);
  }
  return id;
}

const sessionId = getOrCreateSessionId();
document.getElementById("session-id").textContent = sessionId;

const messagesEl = document.getElementById("messages");
const form = document.getElementById("chat-form");
const input = document.getElementById("input");
const sendBtn = document.getElementById("send-btn");
const reconnectStatusEl = document.getElementById("reconnect-status");

/** @type {AbortController | null} */
let activeStream = null;

const MEANINGFUL_AGUI_EVENTS = new Set([
  "RUN_STARTED",
  "RUN_FINISHED",
  "RUN_ERROR",
  "STEP_STARTED",
  "REASONING_MESSAGE_START",
  "REASONING_MESSAGE_CONTENT",
  "TEXT_MESSAGE_CONTENT",
  "TOOL_CALL_START",
  "TOOL_CALL_ARGS",
  "TOOL_CALL_RESULT",
  "CUSTOM",
]);

form.addEventListener("submit", (e) => {
  e.preventDefault();
  const text = input.value.trim();
  if (!text || sendBtn.disabled) return;
  input.value = "";
  sendMessage(text);
});

function setReconnectStatus(text) {
  if (reconnectStatusEl) reconnectStatusEl.textContent = text;
}

function appendUserBubble(text) {
  const el = document.createElement("div");
  el.className = "bubble user";
  el.textContent = text;
  messagesEl.appendChild(el);
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return el;
}

function createAssistantRun() {
  const wrap = document.createElement("div");
  wrap.className = "bubble assistant";

  const meta = document.createElement("div");
  meta.className = "run-meta";
  meta.textContent = "Assistant · streaming…";
  wrap.appendChild(meta);

  const reasoning = document.createElement("details");
  reasoning.className = "reasoning";
  reasoning.style.display = "none";
  const summary = document.createElement("summary");
  summary.textContent = "Reasoning";
  const reasoningBody = document.createElement("div");
  reasoningBody.className = "body";
  reasoning.appendChild(summary);
  reasoning.appendChild(reasoningBody);
  wrap.appendChild(reasoning);

  const toolsEl = document.createElement("div");
  toolsEl.className = "tools";
  wrap.appendChild(toolsEl);

  const textEl = document.createElement("div");
  textEl.className = "text typing";
  wrap.appendChild(textEl);

  messagesEl.appendChild(wrap);
  messagesEl.scrollTop = messagesEl.scrollHeight;

  return {
    wrap,
    meta,
    reasoning,
    reasoningBody,
    toolsEl,
    textEl,
    tools: new Map(),
    toolArgs: new Map(),
  };
}

function getOrCreateTool(run, toolCallId, toolName) {
  if (run.tools.has(toolCallId)) return run.tools.get(toolCallId);
  const card = document.createElement("div");
  card.className = "tool-card";
  card.innerHTML = `<div class="name">${escapeHtml(toolName || toolCallId)}</div>
    <div class="args"></div><div class="result"></div>`;
  run.toolsEl.appendChild(card);
  run.tools.set(toolCallId, card);
  run.toolArgs.set(toolCallId, "");
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return card;
}

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function handleAGUIEvent(run, ev) {
  switch (ev.type) {
    case "RUN_STARTED":
      run.meta.textContent = `Run ${ev.runId || ""} · ${ev.threadId || sessionId}`;
      break;

    case "RUN_FINISHED":
      run.textEl.classList.remove("typing");
      run.meta.textContent = (run.meta.textContent || "").replace(/streaming…|重连中…/g, "done");
      break;

    case "RUN_ERROR":
      run.textEl.classList.remove("typing");
      run.textEl.innerHTML = `<span class="error-banner">${escapeHtml(ev.message || "Error")}</span>`;
      break;

    case "STEP_STARTED":
      run.meta.insertAdjacentHTML(
        "beforeend",
        ` <span class="step-pill">${escapeHtml(ev.stepName || "step")}</span>`
      );
      break;

    case "REASONING_MESSAGE_START":
      run.reasoning.style.display = "block";
      run.reasoning.open = true;
      break;

    case "REASONING_MESSAGE_CONTENT":
      run.reasoningBody.textContent += ev.delta || "";
      messagesEl.scrollTop = messagesEl.scrollHeight;
      break;

    case "TEXT_MESSAGE_CONTENT":
      run.textEl.textContent += ev.delta || "";
      messagesEl.scrollTop = messagesEl.scrollHeight;
      break;

    case "TOOL_CALL_START": {
      const card = getOrCreateTool(run, ev.toolCallId, ev.toolCallName);
      card.querySelector(".args").textContent = "args: …";
      break;
    }

    case "TOOL_CALL_ARGS": {
      const prev = run.toolArgs.get(ev.toolCallId) || "";
      const next = prev + (ev.delta || "");
      run.toolArgs.set(ev.toolCallId, next);
      const card = run.tools.get(ev.toolCallId);
      if (card) card.querySelector(".args").textContent = "args: " + next;
      break;
    }

    case "TOOL_CALL_RESULT": {
      const card = run.tools.get(ev.toolCallId);
      if (card) {
        card.querySelector(".result").textContent = "result: " + (ev.content || "");
      }
      break;
    }

    case "CUSTOM":
      if (ev.name === "require_user_confirm") {
        run.textEl.insertAdjacentHTML(
          "beforeend",
          `<div class="error-banner">HITL: user confirmation required (resume via /v2/resume)</div>`
        );
      }
      break;

    case "STREAM_DONE":
      run.textEl.classList.remove("typing");
      break;

    default:
      break;
  }
}

/**
 * Reads an SSE response body and dispatches AG-UI events.
 * @returns {boolean} whether any meaningful event was received
 */
async function consumeEventStream(res, run, { signal, onEvent } = {}) {
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let meaningful = false;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    const chunks = buffer.split("\n\n");
    buffer = chunks.pop() || "";

    for (const chunk of chunks) {
      const line = chunk.split("\n").find((l) => l.startsWith("data: "));
      if (!line) continue;
      try {
        const ev = JSON.parse(line.slice(6));
        if (MEANINGFUL_AGUI_EVENTS.has(ev.type)) {
          meaningful = true;
        }
        handleAGUIEvent(run, ev);
        onEvent?.(ev);
      } catch (parseErr) {
        console.warn("SSE parse error", parseErr, line);
      }
    }
    if (signal?.aborted) {
      reader.cancel();
      break;
    }
  }

  return meaningful;
}

async function reconnectOnLoad() {
  setReconnectStatus("重连中…");
  sendBtn.disabled = true;

  const controller = new AbortController();
  activeStream = controller;

  const run = createAssistantRun();
  run.meta.textContent = "Assistant · 重连中…";

  try {
    const url = `/v2/chat?protocol=agui&session_id=${encodeURIComponent(sessionId)}`;
    const res = await fetch(url, {
      method: "GET",
      headers: {
        Accept: "application/json, text/event-stream",
        "Agent-Session-Id": sessionId,
      },
      signal: controller.signal,
    });

    if (res.status === 404 || res.status === 503) {
      run.wrap.remove();
      setReconnectStatus("");
      return;
    }

    if (!res.ok) {
      run.wrap.remove();
      setReconnectStatus(`重连失败 (${res.status})`);
      return;
    }

    const meaningful = await consumeEventStream(res, run, { signal: controller.signal });

    if (!meaningful) {
      run.wrap.remove();
      setReconnectStatus("");
      return;
    }

    run.textEl.classList.remove("typing");
    if (!run.meta.textContent.includes("done")) {
      run.meta.textContent = (run.meta.textContent || "").replace("重连中…", "重连 · 进行中");
    }
    setReconnectStatus("已重连");
  } catch (err) {
    if (err.name !== "AbortError") {
      console.warn("reconnect failed", err);
      run.wrap.remove();
      setReconnectStatus("重连失败");
    }
  } finally {
    run.textEl.classList.remove("typing");
    sendBtn.disabled = false;
    if (activeStream === controller) {
      activeStream = null;
    }
  }
}

async function sendMessage(text) {
  if (activeStream) activeStream.abort();

  appendUserBubble(text);
  const run = createAssistantRun();
  sendBtn.disabled = true;
  setReconnectStatus("");

  const controller = new AbortController();
  activeStream = controller;

  try {
    const res = await fetch("/v2/chat?protocol=agui", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json, text/event-stream",
        "Agent-Session-Id": sessionId,
      },
      body: JSON.stringify({ text, session_id: sessionId }),
      signal: controller.signal,
    });

    if (!res.ok) {
      const errText = await res.text();
      throw new Error(`${res.status}: ${errText}`);
    }

    await consumeEventStream(res, run, { signal: controller.signal });
  } catch (err) {
    if (err.name !== "AbortError") {
      run.textEl.classList.remove("typing");
      run.textEl.innerHTML = `<span class="error-banner">${escapeHtml(err.message)}</span>`;
    }
  } finally {
    run.textEl.classList.remove("typing");
    sendBtn.disabled = false;
    activeStream = null;
  }
}

reconnectOnLoad();
