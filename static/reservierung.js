// Kundenseite: Ski für einen Zeitraum in der Zukunft reservieren (Anforderung 7).
// Helfer ($, esc, api, run, …) kommen aus common.js.

let customers = [];

function selectedCustomer() {
  return customers.find((c) => String(c.id) === $("#res-customer").value);
}

function dayCount(from, to) {
  return Math.round((new Date(to) - new Date(from)) / 86400000) + 1;
}

async function init() {
  const from = $("#res-from");
  const to = $("#res-to");
  from.min = to.min = addDays(1);
  from.value = addDays(1);
  to.value = addDays(1);

  customers = await api("GET", "/api/customer-names");
  $("#res-customer").innerHTML = `<option value="">– Name wählen –</option>` +
    customers.map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join("");
}

async function searchSkis() {
  const c = selectedCustomer();
  const info = $("#res-info");
  const list = $("#res-list");
  if (!c) {
    info.textContent = "";
    list.innerHTML = "";
    return;
  }
  if (c.blocked) {
    info.textContent = "Ihr Kundenkonto ist gesperrt – bitte wenden Sie sich an unser Personal.";
    list.innerHTML = "";
    return;
  }
  const from = $("#res-from").value;
  const to = $("#res-to").value;
  const skis = await api("GET", `/api/customers/${c.id}/reservable-skis?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`);
  info.textContent = `${skis.length} passende Ski frei vom ${fmtDate(from)} bis ${fmtDate(to)} ` +
    `(${dayCount(from, to)} Tag(e), Ihre Skilänge ${c.preferredMinCm}–${c.preferredMaxCm} cm).`;
  list.innerHTML = skis.map((s) => `
    <tr>
      <td>${esc(s.inventoryNumber)}</td>
      <td>${esc(s.brand)}</td>
      <td>${esc(s.model)}</td>
      <td>${s.lengthCm} cm</td>
      <td><button data-reserve="${esc(s.inventoryNumber)}">Reservieren</button></td>
    </tr>`).join("");
}

async function loadMyReservations() {
  const c = selectedCustomer();
  const target = $("#my-reservations");
  if (!c) return void (target.innerHTML = "");
  const list = await api("GET", `/api/customers/${c.id}/reservations`);
  target.innerHTML = list.map((r) => `
    <tr>
      <td>${fmtDate(r.from)}</td>
      <td>${fmtDate(r.to)}</td>
      <td>${dayCount(r.from, r.to)}</td>
      <td>${esc(r.inventoryNumber)}</td>
      <td>${esc(r.brand)} ${esc(r.model)}, ${r.lengthCm} cm</td>
    </tr>`).join("") || `<tr><td colspan="5">Keine Reservierungen.</td></tr>`;
}

// "Bis" darf nicht vor "Von" liegen.
$("#res-from").addEventListener("change", () => {
  const from = $("#res-from").value;
  $("#res-to").min = from;
  if ($("#res-to").value < from) $("#res-to").value = from;
});

$("#res-customer").addEventListener("change", () => run(() => Promise.all([searchSkis(), loadMyReservations()])));

$("#res-form").addEventListener("submit", (e) => {
  e.preventDefault();
  run(searchSkis);
});

$("#res-list").addEventListener("click", (e) => {
  const inv = e.target.dataset.reserve;
  if (!inv) return;
  const from = $("#res-from").value;
  const to = $("#res-to").value;
  run(async () => {
    await api("POST", "/api/reservations", { inventoryNumber: inv, customerId: selectedCustomer().id, from, to });
    await Promise.all([searchSkis(), loadMyReservations()]);
  }, `Ski ${inv} reserviert vom ${fmtDate(from)} bis ${fmtDate(to)}`);
});

run(init);
