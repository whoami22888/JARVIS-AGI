const state = { socket: null, connected: false, models: [] };
const $ = (selector) => document.querySelector(selector);
const eventBox = $("#task-events");
const template = $("#event-template");

function format(value) {
  return typeof value === "string" ? value : JSON.stringify(value, null, 2);
}

function addEvent(kind, payload) {
  const welcome = eventBox.querySelector(".welcome");
  if (welcome) welcome.remove();
  const node = template.content.cloneNode(true);
  node.querySelector(".event-meta").textContent = `${new Date().toLocaleTimeString()} · ${kind}`;
  node.querySelector(".event-body").textContent = format(payload);
  eventBox.prepend(node);
}

function setConnection(connected) {
  state.connected = connected;
  $("#connection-dot").className = `dot ${connected ? "online" : "offline"}`;
  $("#connection-state").textContent = connected ? "Gateway online" : "Gateway offline";
}

function renderMetrics(host) {
  if (!host) return;
  const profile = host.profile || {};
  $("#metric-profile").textContent = profile.name || "—";
  $("#metric-cpu").textContent = `${host.cpu_percent ?? "—"}%`;
  $("#metric-ram").textContent = `${host.memory_percent ?? "—"}%`;
  $("#metric-disk").textContent = host.disk_free_mb ? `${Math.round(host.disk_free_mb / 1024)} GB` : "—";
}

function renderModels(models) {
  state.models = models;
  const list = $("#models");
  const select = $("#model-select");
  list.innerHTML = "";
  select.innerHTML = '<option value="">Adaptive selection</option>';
  for (const model of models) {
    const item = document.createElement("article");
    item.className = "model";
    const ready = model.installed || model.provider !== "ollama";
    item.innerHTML = `<span class="status ${ready ? "ready" : ""}">${ready ? (model.provider === "ollama" ? "READY" : "OPTIONAL") : "NOT INSTALLED"}</span><strong>${model.id}</strong><small>${model.role} · ${model.provider}</small><small>${model.tag}</small>`;
    list.append(item);
    const opt = document.createElement("option");
    opt.value = model.id;
    opt.textContent = `${model.id} (${model.provider})`;
    select.append(opt);
  }
}

function renderHistory(tasks) {
  const history = $("#history");
  history.innerHTML = "";
  for (const task of tasks) {
    const item = document.createElement("article");
    item.className = "history-item";
    item.innerHTML = `<strong>${task.state}</strong><small>${task.route?.reason || "route pending"}</small><small>${task.prompt.slice(0, 110)}</small>`;
    item.addEventListener("click", () => addEvent(`TASK ${task.state}`, task));
    history.append(item);
  }
}

async function refresh() {
  try {
    const response = await fetch("/api/status");
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const payload = await response.json();
    renderMetrics(payload.host);
    renderModels(payload.models);
    renderHistory(payload.tasks);
    setConnection(true);
  } catch (error) {
    setConnection(false);
    addEvent("STATUS ERROR", { error: error.message });
  }
}

function connect() {
  const protocol = location.protocol === "https:" ? "wss" : "ws";
  const socket = new WebSocket(`${protocol}://${location.host}/ws/events`);
  state.socket = socket;
  socket.onopen = () => { setConnection(true); socket.send("ready"); };
  socket.onmessage = (event) => {
    const payload = JSON.parse(event.data);
    if (payload.host) renderMetrics(payload.host);
    addEvent(payload.type || payload.state || "EVENT", payload);
    if (["COMPLETED", "FAILED"].includes(payload.state)) refresh();
  };
  socket.onclose = () => { setConnection(false); window.setTimeout(connect, 1800); };
  socket.onerror = () => socket.close();
}

$("#task-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const prompt = $("#prompt").value.trim();
  if (!prompt) return;
  const submit = $("#submit");
  submit.disabled = true;
  try {
    const response = await fetch("/api/tasks", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prompt, requested_model: $("#model-select").value || null })
    });
    if (!response.ok) throw new Error((await response.json()).detail || `HTTP ${response.status}`);
    addEvent("QUEUED", await response.json());
    $("#prompt").value = "";
    refresh();
  } catch (error) {
    addEvent("TASK ERROR", { error: error.message });
  } finally {
    submit.disabled = false;
  }
});

$("#refresh").addEventListener("click", refresh);
$("#proposal").addEventListener("click", async () => {
  try {
    const response = await fetch("/api/approvals", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        kind: "elevated_windows_action",
        title: "Test elevated action",
        reason: "Verifies that approval proposals are visible and separate from normal tasks.",
        details: { executable: "not executed", scope: "test only" }
      })
    });
    addEvent("APPROVAL PROPOSAL", await response.json());
  } catch (error) {
    addEvent("APPROVAL ERROR", { error: error.message });
  }
});

refresh();
connect();
