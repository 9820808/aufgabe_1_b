// Mitarbeiter-Backend. Helfer ($, esc, api, run, …) kommen aus common.js.

const idTypeLabel = { fuehrerschein: "Führerschein", pass: "Pass" };

// ---------- Tabs ----------

const loaders = {
  dashboard: loadDashboard,
  skis: loadSkis,
  customers: loadCustomers,
  rent: loadRent,
  return: loadReturn,
};

function showTab() {
  const tab = location.hash.slice(1) in loaders ? location.hash.slice(1) : "dashboard";
  document.querySelectorAll(".tab").forEach((s) => (s.hidden = s.id !== "tab-" + tab));
  document.querySelectorAll("#tabs a").forEach((a) => a.classList.toggle("active", a.dataset.tab === tab));
  run(loaders[tab]);
}

// ---------- Dashboard ----------

async function loadDashboard() {
  const s = await api("GET", "/api/stats");
  const cards = [
    ["Ski gesamt", s.skis],
    ["Ski verliehen", s.skisRented],
    ["Ski verfügbar", s.skisAvailable],
    ["Kunden", s.customers],
    ["Kunden mit Ausleihe", s.customersRenting],
    ["Kunden gesperrt", s.customersBlocked],
    ["Ski überfällig", s.skisOverdue],
    ["Reservierungen", s.reservations],
  ];
  $("#stats").innerHTML = cards
    .map(([label, value]) => `<div class="card"><div class="value">${value}</div><div class="label">${label}</div></div>`)
    .join("");
}

// ---------- Ski ----------

async function loadSkis() {
  const skis = await api("GET", "/api/skis");
  $("#ski-list").innerHTML = skis.map((s) => `
    <tr>
      <td>${esc(s.inventoryNumber)}</td>
      <td>${esc(s.brand)}</td>
      <td>${esc(s.model)}</td>
      <td>${s.lengthCm} cm</td>
      <td>${rentalBadge(s)}</td>
      <td><button class="danger" data-delete="${esc(s.inventoryNumber)}" ${s.rentedTo ? "disabled title='verliehen'" : ""}>Löschen</button></td>
    </tr>`).join("");

  const form = $("#ski-form");
  if (!form.year.value) form.year.value = new Date().getFullYear();
  await suggestInventoryNumber();
}

// Badge mit Kunde und Rückgabedatum (rot, wenn überfällig) bzw. "verfügbar";
// bei einer anstehenden Reservierung "reserviert ab <datum>" statt "verfügbar".
function rentalBadge(ski) {
  const reserved = ski.reservedFrom ? `<span class="badge reserved">reserviert ab ${fmtDate(ski.reservedFrom)}</span>` : "";
  if (!ski.rentedTo) return reserved || `<span class="badge free">verfügbar</span>`;
  const overdue = ski.rentedUntil < isoDate(new Date());
  return `<span class="badge ${overdue ? "blocked" : "rented"}">${esc(ski.customerName)} bis ${fmtDate(ski.rentedUntil)}${overdue ? " – überfällig" : ""}</span> ${reserved}`;
}

// Tag vor dem Datum (JJJJ-MM-TT), lokal gerechnet.
function dayBefore(iso) {
  const [y, m, d] = iso.split("-").map(Number);
  return isoDate(new Date(y, m - 1, d - 1));
}

async function suggestInventoryNumber() {
  const year = $("#ski-form").year.value;
  const res = await api("GET", "/api/skis/next-number?year=" + encodeURIComponent(year));
  $("#ski-form").inventoryNumber.value = res.inventoryNumber;
}

$("#ski-form").year.addEventListener("change", () => run(suggestInventoryNumber));

$("#ski-form").addEventListener("submit", (e) => {
  e.preventDefault();
  const f = e.target;
  run(async () => {
    await api("POST", "/api/skis", {
      inventoryNumber: f.inventoryNumber.value,
      brand: f.brand.value,
      model: f.model.value,
      lengthCm: Number(f.lengthCm.value),
    });
    f.brand.value = f.model.value = f.lengthCm.value = "";
    await loadSkis();
  }, "Ski hinzugefügt");
});

$("#ski-list").addEventListener("click", (e) => {
  const inv = e.target.dataset.delete;
  if (!inv || !confirm(`Ski ${inv} wirklich löschen?`)) return;
  run(async () => {
    await api("DELETE", "/api/skis/" + encodeURIComponent(inv));
    await loadSkis();
  }, `Ski ${inv} gelöscht`);
});

// ---------- Kunden ----------

async function loadCustomers() {
  const customers = await api("GET", "/api/customers");
  $("#customer-list").innerHTML = customers.map((c) => `
    <tr class="${c.blocked ? "blocked" : ""}">
      <td>${esc(c.name)}</td>
      <td>${esc(c.phone)}<br>${esc(c.email)}</td>
      <td>${esc(c.address)}</td>
      <td>${c.idNumber ? `${idTypeLabel[c.idType] || esc(c.idType)}<br>${esc(c.idNumber)}` : "–"}</td>
      <td>${c.preferredMinCm}–${c.preferredMaxCm} cm</td>
      <td>${c.hasPhoto ? `<a href="/api/customers/${c.id}/photo" target="_blank"><img class="thumb" src="/api/customers/${c.id}/photo" alt="Ausweis"></a>` : "–"}</td>
      <td>${c.blocked ? `<span class="badge blocked">gesperrt</span>` : `<span class="badge free">aktiv</span>`}</td>
      <td><button class="${c.blocked ? "secondary" : "danger"}" data-id="${c.id}" data-action="${c.blocked ? "unblock" : "block"}">
        ${c.blocked ? "Entsperren" : "Sperren"}</button></td>
    </tr>`).join("");
}

$("#customer-form").addEventListener("submit", (e) => {
  e.preventDefault();
  const f = e.target;
  run(async () => {
    await api("POST", "/api/customers", new FormData(f));
    f.reset();
    await loadCustomers();
  }, "Kunde angelegt");
});

$("#customer-list").addEventListener("click", (e) => {
  const { id, action } = e.target.dataset;
  if (!id) return;
  run(async () => {
    await api("POST", `/api/customers/${id}/${action}`);
    await loadCustomers();
  }, action === "block" ? "Kunde gesperrt" : "Kunde entsperrt");
});

// ---------- Verleih ----------

let customersCache = [];

function fillCustomerSelect(select, customers, labelFn) {
  const current = select.value;
  select.innerHTML = `<option value="">– Kunde wählen –</option>` +
    customers.map((c) => `<option value="${c.id}">${esc(labelFn(c))}</option>`).join("");
  if (customers.some((c) => String(c.id) === current)) select.value = current;
}

async function loadRent() {
  const until = $("#rent-until");
  until.min = isoDate(new Date());
  if (!until.value || until.value < until.min) until.value = until.min;
  customersCache = await api("GET", "/api/customers");
  fillCustomerSelect($("#rent-customer"), customersCache,
    (c) => `${c.name} (${c.preferredMinCm}–${c.preferredMaxCm} cm)${c.blocked ? " – gesperrt" : ""}`);
  await loadAvailableSkis();
}

async function loadAvailableSkis() {
  const id = $("#rent-customer").value;
  const info = $("#rent-info");
  const list = $("#rent-list");
  if (!id) {
    info.textContent = "";
    list.innerHTML = "";
    return;
  }
  const customer = customersCache.find((c) => String(c.id) === id);
  const until = $("#rent-until").value;
  const skis = await api("GET", `/api/customers/${id}/available-skis?until=${encodeURIComponent(until)}`);
  info.textContent = customer.blocked
    ? `${customer.name} ist gesperrt und kann keine Ski ausleihen.`
    : `${skis.length} passende Ski bis ${fmtDate(until)} verfügbar (${customer.preferredMinCm}–${customer.preferredMaxCm} cm).`;
  list.innerHTML = skis.map((s) => `
    <tr>
      <td>${esc(s.inventoryNumber)}</td>
      <td>${esc(s.brand)}</td>
      <td>${esc(s.model)}</td>
      <td>${s.lengthCm} cm</td>
      <td>${s.reservedFrom ? `<span class="badge reserved">reserviert ab ${fmtDate(s.reservedFrom)} – nur bis ${fmtDate(dayBefore(s.reservedFrom))}</span>` : ""}</td>
      <td><button data-rent="${esc(s.inventoryNumber)}" ${customer.blocked ? "disabled" : ""}>Verleihen</button></td>
    </tr>`).join("");
}

$("#rent-customer").addEventListener("change", () => run(loadAvailableSkis));
$("#rent-until").addEventListener("change", () => run(loadAvailableSkis));

$("#rent-list").addEventListener("click", (e) => {
  const inv = e.target.dataset.rent;
  if (!inv) return;
  run(async () => {
    await api("POST", "/api/rentals", {
      inventoryNumber: inv,
      customerId: Number($("#rent-customer").value),
      until: $("#rent-until").value,
    });
    await loadAvailableSkis();
  }, `Ski ${inv} verliehen`);
});

// ---------- Rückgabe ----------

let rentedSkisCache = [];

async function loadReturn() {
  const [customers, skis] = await Promise.all([api("GET", "/api/customers"), api("GET", "/api/skis")]);
  rentedSkisCache = skis.filter((s) => s.rentedTo);
  const renterIds = new Set(rentedSkisCache.map((s) => s.rentedTo));

  fillCustomerSelect($("#return-customer"), customers.filter((c) => renterIds.has(c.id)), (c) => c.name);

  const skiSelect = $("#return-ski");
  const current = skiSelect.value;
  skiSelect.innerHTML = `<option value="">– Ski wählen –</option>` +
    rentedSkisCache.map((s) => `<option value="${esc(s.inventoryNumber)}">${esc(s.inventoryNumber)} – ${esc(s.brand)} ${esc(s.model)}</option>`).join("");
  if (rentedSkisCache.some((s) => s.inventoryNumber === current)) skiSelect.value = current;

  await Promise.all([showSkisOfCustomer(), showCustomerOfSki()]);
}

function returnButton(inv) {
  return `<button data-return="${esc(inv)}">Ausleihe beenden</button>`;
}

async function showSkisOfCustomer() {
  const id = $("#return-customer").value;
  const target = $("#return-by-customer");
  if (!id) return void (target.innerHTML = "");
  const skis = await api("GET", `/api/customers/${id}/skis`);
  target.innerHTML = skis.map((s) => `
    <div class="result">
      <p><strong>${esc(s.inventoryNumber)}</strong> – ${esc(s.brand)} ${esc(s.model)}, ${s.lengthCm} cm</p>
      <p>Rückgabe bis ${fmtDate(s.rentedUntil)}</p>
      ${returnButton(s.inventoryNumber)}
    </div>`).join("") || `<p>Keine Ski ausgeliehen.</p>`;
}

function showCustomerOfSki() {
  const inv = $("#return-ski").value;
  const target = $("#return-by-ski");
  const ski = rentedSkisCache.find((s) => s.inventoryNumber === inv);
  if (!ski) return void (target.innerHTML = "");
  target.innerHTML = `
    <div class="result">
      <p>Ausgeliehen von: <strong>${esc(ski.customerName)}</strong></p>
      <p>${esc(ski.brand)} ${esc(ski.model)}, ${ski.lengthCm} cm</p>
      <p>Rückgabe bis ${fmtDate(ski.rentedUntil)}</p>
      ${returnButton(ski.inventoryNumber)}
    </div>`;
}

$("#return-customer").addEventListener("change", () => run(showSkisOfCustomer));
$("#return-ski").addEventListener("change", showCustomerOfSki);

$("#tab-return").addEventListener("click", (e) => {
  const inv = e.target.dataset.return;
  if (!inv) return;
  run(async () => {
    await api("POST", "/api/returns", { inventoryNumber: inv });
    await loadReturn();
  }, `Ski ${inv} zurückgenommen`);
});

// ---------- Start ----------

window.addEventListener("hashchange", showTab);
showTab();
