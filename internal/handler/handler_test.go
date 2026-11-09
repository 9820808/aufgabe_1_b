package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"skiverleih/internal/domain"
	"skiverleih/internal/store"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	New(store.New()).Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func doJSON(t *testing.T, srv *httptest.Server, method, path string, body any, wantStatus int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, _ := http.NewRequest(method, srv.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return send(t, req, wantStatus)
}

func send(t *testing.T, req *http.Request, wantStatus int) []byte {
	t.Helper()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out bytes.Buffer
	out.ReadFrom(res.Body)
	if res.StatusCode != wantStatus {
		t.Fatalf("%s %s: status %d, want %d, body %s", req.Method, req.URL.Path, res.StatusCode, wantStatus, out.String())
	}
	return out.Bytes()
}

// createCustomer schickt ein Multipart-Formular wie der Browser, optional mit Foto.
func createCustomer(t *testing.T, srv *httptest.Server, fields map[string]string, photo []byte, wantStatus int) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if photo != nil {
		fw, _ := mw.CreateFormFile("photo", "ausweis.png")
		fw.Write(photo)
	}
	mw.Close()
	req, _ := http.NewRequest("POST", srv.URL+"/api/customers", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	body := send(t, req, wantStatus)
	var out map[string]any
	json.Unmarshal(body, &out)
	return out
}

func customerFields(name string, minCm, maxCm string) map[string]string {
	return map[string]string{
		"name": name, "phone": "+43 1", "email": "a@b.at", "address": "Weg 1",
		"idType": "pass", "idNumber": "P1", "preferredMinCm": minCm, "preferredMaxCm": maxCm,
	}
}

func pngBytes(t *testing.T) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSkiEndpoints(t *testing.T) {
	srv := newServer(t)

	var next map[string]string
	json.Unmarshal(doJSON(t, srv, "GET", "/api/skis/next-number?year=2026", nil, 200), &next)
	if next["inventoryNumber"] != "2026-001" {
		t.Errorf("Vorschlag: got %v", next)
	}

	doJSON(t, srv, "POST", "/api/skis", map[string]any{
		"inventoryNumber": "2026-001", "brand": "Atomic", "model": "G9", "lengthCm": 170,
	}, 201)
	doJSON(t, srv, "POST", "/api/skis", map[string]any{
		"inventoryNumber": "2026-001", "brand": "Atomic", "model": "G9", "lengthCm": 170,
	}, 409)
	doJSON(t, srv, "POST", "/api/skis", map[string]any{
		"inventoryNumber": "2026-002", "brand": "", "model": "G9", "lengthCm": 170,
	}, 400)
	doJSON(t, srv, "GET", "/api/skis/next-number?year=abc", nil, 400)

	json.Unmarshal(doJSON(t, srv, "GET", "/api/skis/next-number?year=2026", nil, 200), &next)
	if next["inventoryNumber"] != "2026-002" {
		t.Errorf("Vorschlag nach Anlegen: got %v", next)
	}

	var skis []map[string]any
	json.Unmarshal(doJSON(t, srv, "GET", "/api/skis", nil, 200), &skis)
	if len(skis) != 1 || skis[0]["brand"] != "Atomic" {
		t.Errorf("Liste: %v", skis)
	}

	doJSON(t, srv, "DELETE", "/api/skis/2026-001", nil, 204)
	doJSON(t, srv, "DELETE", "/api/skis/2026-001", nil, 404)
}

func TestCustomerEndpoints(t *testing.T) {
	srv := newServer(t)

	c := createCustomer(t, srv, customerFields("Anna", "160", "170"), pngBytes(t), 201)
	if c["hasPhoto"] != true || c["blocked"] != false {
		t.Errorf("angelegter Kunde: %v", c)
	}
	createCustomer(t, srv, customerFields("", "160", "170"), nil, 400)
	createCustomer(t, srv, customerFields("Bernd", "180", "170"), nil, 400)
	createCustomer(t, srv, customerFields("Kein Bild", "160", "170"), []byte("hallo welt"), 400)

	// Ausweisnummer oder Foto reicht – ohne beides wird abgelehnt.
	withoutNumber := customerFields("Nur Foto", "160", "170")
	withoutNumber["idNumber"] = ""
	createCustomer(t, srv, withoutNumber, nil, 400)
	onlyPhoto := createCustomer(t, srv, withoutNumber, pngBytes(t), 201)
	if onlyPhoto["hasPhoto"] != true || onlyPhoto["idNumber"] != "" {
		t.Errorf("Kunde nur mit Foto: %v", onlyPhoto)
	}

	res, err := http.Get(srv.URL + "/api/customers/1/photo")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" {
		t.Errorf("Foto: status %d, type %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	noPhoto := createCustomer(t, srv, customerFields("Ohne Foto", "160", "170"), nil, 201)
	doJSON(t, srv, "GET", fmt.Sprintf("/api/customers/%v/photo", noPhoto["id"]), nil, 404)

	var blocked map[string]any
	json.Unmarshal(doJSON(t, srv, "POST", "/api/customers/1/block", nil, 200), &blocked)
	if blocked["blocked"] != true {
		t.Errorf("sperren: %v", blocked)
	}
	json.Unmarshal(doJSON(t, srv, "POST", "/api/customers/1/unblock", nil, 200), &blocked)
	if blocked["blocked"] != false {
		t.Errorf("entsperren: %v", blocked)
	}
	doJSON(t, srv, "POST", "/api/customers/99/block", nil, 404)
	doJSON(t, srv, "POST", "/api/customers/abc/block", nil, 400)

	// Kunden dürfen nicht gelöscht werden: es gibt keine DELETE-Route.
	doJSON(t, srv, "DELETE", "/api/customers/1", nil, 404)
}

func TestRentalFlow(t *testing.T) {
	srv := newServer(t)
	for _, s := range []struct {
		inv    string
		length int
	}{{"2026-1", 150}, {"2026-2", 165}, {"2026-3", 175}} {
		doJSON(t, srv, "POST", "/api/skis", map[string]any{
			"inventoryNumber": s.inv, "brand": "Atomic", "model": "G9", "lengthCm": s.length,
		}, 201)
	}
	createCustomer(t, srv, customerFields("Anna", "160", "180"), nil, 201)

	var avail []map[string]any
	json.Unmarshal(doJSON(t, srv, "GET", "/api/customers/1/available-skis", nil, 200), &avail)
	if len(avail) != 2 {
		t.Fatalf("passende Ski: %v", avail)
	}

	doJSON(t, srv, "POST", "/api/rentals", map[string]any{"inventoryNumber": "2026-002", "customerId": 1, "until": string(domain.Today())}, 204)
	doJSON(t, srv, "POST", "/api/rentals", map[string]any{"inventoryNumber": "2026-002", "customerId": 1, "until": string(domain.Today())}, 409)
	doJSON(t, srv, "POST", "/api/rentals", map[string]any{"inventoryNumber": "2026-001", "customerId": 1, "until": string(domain.Today())}, 409) // zu kurz

	json.Unmarshal(doJSON(t, srv, "GET", "/api/customers/1/available-skis", nil, 200), &avail)
	if len(avail) != 1 || avail[0]["inventoryNumber"] != "2026-003" {
		t.Errorf("nach Verleih: %v", avail)
	}

	// Ski-Liste zeigt den Kundennamen.
	body := string(doJSON(t, srv, "GET", "/api/skis", nil, 200))
	if !strings.Contains(body, `"customerName":"Anna"`) {
		t.Errorf("Kundenname fehlt in Ski-Liste: %s", body)
	}

	// Verliehener Ski kann nicht gelöscht werden.
	doJSON(t, srv, "DELETE", "/api/skis/2026-002", nil, 409)

	// Gesperrter Kunde kann nicht leihen.
	doJSON(t, srv, "POST", "/api/customers/1/block", nil, 200)
	doJSON(t, srv, "POST", "/api/rentals", map[string]any{"inventoryNumber": "2026-003", "customerId": 1, "until": string(domain.Today())}, 409)
	doJSON(t, srv, "POST", "/api/customers/1/unblock", nil, 200)

	var rented []map[string]any
	json.Unmarshal(doJSON(t, srv, "GET", "/api/customers/1/skis", nil, 200), &rented)
	if len(rented) != 1 || rented[0]["inventoryNumber"] != "2026-002" {
		t.Errorf("Ski des Kunden: %v", rented)
	}

	doJSON(t, srv, "POST", "/api/returns", map[string]any{"inventoryNumber": "2026-002"}, 204)
	doJSON(t, srv, "POST", "/api/returns", map[string]any{"inventoryNumber": "2026-002"}, 409)

	var stats map[string]int
	json.Unmarshal(doJSON(t, srv, "GET", "/api/stats", nil, 200), &stats)
	if stats["skis"] != 3 || stats["skisRented"] != 0 || stats["customers"] != 1 {
		t.Errorf("Stats: %v", stats)
	}
}

func TestReservationFlow(t *testing.T) {
	srv := newServer(t)
	doJSON(t, srv, "POST", "/api/skis", map[string]any{
		"inventoryNumber": "2026-1", "brand": "Atomic", "model": "G9", "lengthCm": 170,
	}, 201)
	createCustomer(t, srv, customerFields("Anna", "160", "180"), nil, 201)

	today := domain.Today()
	from, to := string(today.AddDays(1)), string(today.AddDays(3))

	// Kundenseite sieht nur schlanke Daten, keine Ausweisnummer.
	body := string(doJSON(t, srv, "GET", "/api/customer-names", nil, 200))
	if !strings.Contains(body, `"name":"Anna"`) || strings.Contains(body, "idNumber") {
		t.Errorf("customer-names: %s", body)
	}

	var skis []map[string]any
	json.Unmarshal(doJSON(t, srv, "GET", "/api/customers/1/reservable-skis?from="+from+"&to="+to, nil, 200), &skis)
	if len(skis) != 1 {
		t.Fatalf("reservierbar: %v", skis)
	}
	doJSON(t, srv, "GET", "/api/customers/1/reservable-skis?from="+to+"&to="+from, nil, 400)

	reserve := func(from, to string, want int) {
		doJSON(t, srv, "POST", "/api/reservations", map[string]any{
			"inventoryNumber": "2026-001", "customerId": 1, "from": from, "to": to,
		}, want)
	}
	reserve(string(today), to, 400) // muss in der Zukunft liegen
	reserve(from, to, 201)
	reserve(to, to, 409) // Überschneidung

	var mine []map[string]any
	json.Unmarshal(doJSON(t, srv, "GET", "/api/customers/1/reservations", nil, 200), &mine)
	if len(mine) != 1 || mine[0]["from"] != from || mine[0]["to"] != to {
		t.Errorf("eigene Reservierungen: %v", mine)
	}

	// Ausleihe ohne Rückgabedatum wird abgelehnt.
	doJSON(t, srv, "POST", "/api/rentals", map[string]any{"inventoryNumber": "2026-001", "customerId": 1}, 400)
}

func TestSkiListShowsReservation(t *testing.T) {
	srv := newServer(t)
	doJSON(t, srv, "POST", "/api/skis", map[string]any{
		"inventoryNumber": "2026-1", "brand": "Atomic", "model": "G9", "lengthCm": 170,
	}, 201)
	createCustomer(t, srv, customerFields("Anna", "160", "180"), nil, 201)
	createCustomer(t, srv, customerFields("Bernd", "160", "180"), nil, 201)

	today := domain.Today()
	from := string(today.AddDays(3))
	doJSON(t, srv, "POST", "/api/reservations", map[string]any{
		"inventoryNumber": "2026-001", "customerId": 1, "from": from, "to": from,
	}, 201)

	body := string(doJSON(t, srv, "GET", "/api/skis", nil, 200))
	if !strings.Contains(body, `"reservedFrom":"`+from+`"`) {
		t.Errorf("reservedFrom fehlt in Ski-Liste: %s", body)
	}

	rent := func(until domain.Date, want int) {
		doJSON(t, srv, "POST", "/api/rentals", map[string]any{
			"inventoryNumber": "2026-001", "customerId": 2, "until": string(until),
		}, want)
	}
	rent(today.AddDays(3), 409) // erster reservierter Tag
	rent(today.AddDays(2), 204) // Tag davor
}
