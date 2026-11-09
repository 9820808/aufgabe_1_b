// Gemeinsame Helfer für Mitarbeiter-Backend (app.js) und Kundenseite (reservierung.js).
// Beide sprechen ausschließlich mit der Go-API unter /api/.

const $ = (sel) => document.querySelector(sel);

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
}

async function api(method, path, body) {
  const opts = { method };
  if (body instanceof FormData) {
    opts.body = body;
  } else if (body !== undefined) {
    opts.headers = { "Content-Type": "application/json" };
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || res.statusText);
  return data;
}

function showMessage(text, isError = false) {
  const el = $("#message");
  el.textContent = text;
  el.className = isError ? "error" : "ok";
  el.hidden = false;
  clearTimeout(showMessage.timer);
  showMessage.timer = setTimeout(() => (el.hidden = true), 4000);
}

// Führt eine Aktion aus und zeigt Fehler einheitlich an.
async function run(action, okText) {
  try {
    await action();
    if (okText) showMessage(okText);
  } catch (e) {
    showMessage(e.message, true);
  }
}

// Datum als JJJJ-MM-TT (Format der API), lokal gerechnet.
function isoDate(d) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function addDays(days) {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return isoDate(d);
}

// Anzeige als TT.MM.JJJJ.
function fmtDate(iso) {
  const [y, m, d] = String(iso).split("-");
  return `${d}.${m}.${y}`;
}
