package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"skiverleih/internal/domain"
	"skiverleih/internal/store"
)

const maxPhotoBytes = 5 << 20 // 5 MB

// Handler stellt die JSON-API unter /api/ bereit.
type Handler struct {
	store *store.MemoryStore
}

func New(s *store.MemoryStore) *Handler {
	return &Handler{store: s}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stats", h.stats)

	mux.HandleFunc("GET /api/skis", h.listSkis)
	mux.HandleFunc("GET /api/skis/next-number", h.nextInventoryNumber)
	mux.HandleFunc("POST /api/skis", h.createSki)
	mux.HandleFunc("DELETE /api/skis/{inv}", h.deleteSki)

	mux.HandleFunc("GET /api/customers", h.listCustomers)
	mux.HandleFunc("POST /api/customers", h.createCustomer)
	mux.HandleFunc("GET /api/customers/{id}/photo", h.customerPhoto)
	mux.HandleFunc("POST /api/customers/{id}/block", h.setBlocked(true))
	mux.HandleFunc("POST /api/customers/{id}/unblock", h.setBlocked(false))
	mux.HandleFunc("GET /api/customers/{id}/available-skis", h.availableSkis)
	mux.HandleFunc("GET /api/customers/{id}/skis", h.rentedSkis)

	mux.HandleFunc("POST /api/rentals", h.rent)
	mux.HandleFunc("POST /api/returns", h.returnSki)

	// Kundenseite (Reservierung)
	mux.HandleFunc("GET /api/customer-names", h.customerNames)
	mux.HandleFunc("GET /api/customers/{id}/reservable-skis", h.reservableSkis)
	mux.HandleFunc("GET /api/customers/{id}/reservations", h.customerReservations)
	mux.HandleFunc("POST /api/reservations", h.reserve)
}

// ---------- Dashboard ----------

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Stats())
}

// ---------- Ski ----------

func (h *Handler) listSkis(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.store.Skis())
}

func (h *Handler) nextInventoryNumber(w http.ResponseWriter, r *http.Request) {
	year := time.Now().Year()
	if y := r.URL.Query().Get("year"); y != "" {
		var err error
		if year, err = strconv.Atoi(y); err != nil || year < 1900 || year > 2999 {
			writeError(w, fmt.Errorf("%w: ungültiges Jahr", domain.ErrValidation))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"inventoryNumber": h.store.NextInventoryNumber(year)})
}

type createSkiRequest struct {
	InventoryNumber string `json:"inventoryNumber"`
	Brand           string `json:"brand"`
	Model           string `json:"model"`
	LengthCm        int    `json:"lengthCm"`
}

func (h *Handler) createSki(w http.ResponseWriter, r *http.Request) {
	var req createSkiRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	ski, err := domain.NewSki(req.InventoryNumber, req.Brand, req.Model, req.LengthCm)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.store.AddSki(ski); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ski)
}

func (h *Handler) deleteSki(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteSki(r.PathValue("inv")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Kunden ----------

// customerResponse ergänzt den Kunden um hasPhoto (das Foto selbst kommt über eigenen Endpoint).
type customerResponse struct {
	domain.Customer
	HasPhoto bool `json:"hasPhoto"`
}

func toCustomerResponse(c domain.Customer) customerResponse {
	return customerResponse{Customer: c, HasPhoto: c.HasPhoto()}
}

func (h *Handler) listCustomers(w http.ResponseWriter, r *http.Request) {
	list := h.store.Customers()
	out := make([]customerResponse, 0, len(list))
	for _, c := range list {
		out = append(out, toCustomerResponse(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// createCustomer erwartet multipart/form-data (wegen Foto-Upload).
func (h *Handler) createCustomer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes+1<<20)
	if err := r.ParseMultipartForm(maxPhotoBytes); err != nil {
		writeError(w, fmt.Errorf("%w: Formular nicht lesbar (max. 5 MB Foto): %v", domain.ErrValidation, err))
		return
	}
	minCm, _ := strconv.Atoi(r.FormValue("preferredMinCm"))
	maxCm, _ := strconv.Atoi(r.FormValue("preferredMaxCm"))
	c := domain.Customer{
		Name:           r.FormValue("name"),
		Phone:          r.FormValue("phone"),
		Email:          r.FormValue("email"),
		Address:        r.FormValue("address"),
		IDType:         domain.IDType(r.FormValue("idType")),
		IDNumber:       r.FormValue("idNumber"),
		PreferredMinCm: minCm,
		PreferredMaxCm: maxCm,
	}

	photo, err := readPhoto(r)
	if err != nil {
		writeError(w, err)
		return
	}
	c.Photo = photo

	created, err := h.store.AddCustomer(c)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toCustomerResponse(created))
}

// readPhoto liest das optionale Formularfeld "photo". Nur Bilder sind erlaubt.
func readPhoto(r *http.Request) (*domain.Photo, error) {
	file, _, err := r.FormFile("photo")
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: Foto nicht lesbar: %v", domain.ErrValidation, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes+1))
	if err != nil {
		return nil, fmt.Errorf("Foto lesen: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > maxPhotoBytes {
		return nil, fmt.Errorf("%w: Foto größer als 5 MB", domain.ErrValidation)
	}
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("%w: Datei ist kein Bild (%s)", domain.ErrValidation, contentType)
	}
	return &domain.Photo{ContentType: contentType, Data: data}, nil
}

func (h *Handler) customerPhoto(w http.ResponseWriter, r *http.Request) {
	c, ok := h.customerFromPath(w, r)
	if !ok {
		return
	}
	if !c.HasPhoto() {
		writeError(w, fmt.Errorf("%w: kein Foto vorhanden", domain.ErrNotFound))
		return
	}
	w.Header().Set("Content-Type", c.Photo.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(c.Photo.Data)
}

func (h *Handler) setBlocked(blocked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := customerID(w, r)
		if !ok {
			return
		}
		if err := h.store.SetBlocked(id, blocked); err != nil {
			writeError(w, err)
			return
		}
		c, err := h.store.Customer(id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toCustomerResponse(c))
	}
}

// ---------- Verleih / Rückgabe ----------

func (h *Handler) availableSkis(w http.ResponseWriter, r *http.Request) {
	id, ok := customerID(w, r)
	if !ok {
		return
	}
	until := domain.Today()
	if u := r.URL.Query().Get("until"); u != "" {
		var err error
		if until, err = domain.ParseDate(u); err != nil {
			writeError(w, err)
			return
		}
	}
	skis, err := h.store.AvailableSkisFor(id, until)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skis)
}

func (h *Handler) rentedSkis(w http.ResponseWriter, r *http.Request) {
	id, ok := customerID(w, r)
	if !ok {
		return
	}
	skis, err := h.store.SkisRentedBy(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skis)
}

type rentRequest struct {
	InventoryNumber string `json:"inventoryNumber"`
	CustomerID      int    `json:"customerId"`
	Until           string `json:"until"` // Rückgabedatum, Pflicht
}

func (h *Handler) rent(w http.ResponseWriter, r *http.Request) {
	var req rentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	until, err := domain.ParseDate(req.Until)
	if err != nil {
		writeError(w, fmt.Errorf("Rückgabedatum: %w", err))
		return
	}
	if err := h.store.Rent(req.InventoryNumber, req.CustomerID, until); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type returnRequest struct {
	InventoryNumber string `json:"inventoryNumber"`
}

func (h *Handler) returnSki(w http.ResponseWriter, r *http.Request) {
	var req returnRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if err := h.store.Return(req.InventoryNumber); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- Reservierung (Kundenseite) ----------

// customerName ist die schlanke Kundenansicht für die Kundenseite – ohne Kontakt- und Ausweisdaten.
type customerName struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	PreferredMinCm int    `json:"preferredMinCm"`
	PreferredMaxCm int    `json:"preferredMaxCm"`
	Blocked        bool   `json:"blocked"`
}

func (h *Handler) customerNames(w http.ResponseWriter, r *http.Request) {
	list := h.store.Customers()
	out := make([]customerName, 0, len(list))
	for _, c := range list {
		out = append(out, customerName{ID: c.ID, Name: c.Name, PreferredMinCm: c.PreferredMinCm, PreferredMaxCm: c.PreferredMaxCm, Blocked: c.Blocked})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) reservableSkis(w http.ResponseWriter, r *http.Request) {
	id, ok := customerID(w, r)
	if !ok {
		return
	}
	p, err := domain.NewPeriod(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, err)
		return
	}
	skis, err := h.store.ReservableSkisFor(id, p)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, skis)
}

func (h *Handler) customerReservations(w http.ResponseWriter, r *http.Request) {
	id, ok := customerID(w, r)
	if !ok {
		return
	}
	list, err := h.store.ReservationsOf(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type reserveRequest struct {
	InventoryNumber string `json:"inventoryNumber"`
	CustomerID      int    `json:"customerId"`
	From            string `json:"from"`
	To              string `json:"to"`
}

func (h *Handler) reserve(w http.ResponseWriter, r *http.Request) {
	var req reserveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	p, err := domain.NewPeriod(req.From, req.To)
	if err != nil {
		writeError(w, err)
		return
	}
	res, err := h.store.Reserve(req.InventoryNumber, req.CustomerID, p)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// ---------- Helfer ----------

func (h *Handler) customerFromPath(w http.ResponseWriter, r *http.Request) (domain.Customer, bool) {
	id, ok := customerID(w, r)
	if !ok {
		return domain.Customer{}, false
	}
	c, err := h.store.Customer(id)
	if err != nil {
		writeError(w, err)
		return domain.Customer{}, false
	}
	return c, true
}

func customerID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, fmt.Errorf("%w: ungültige Kunden-ID", domain.ErrValidation))
		return 0, false
	}
	return id, true
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: JSON nicht lesbar: %v", domain.ErrValidation, err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("JSON schreiben: %v", err)
	}
}

// writeError mappt Domain-Fehler auf HTTP-Statuscodes.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrDuplicate),
		errors.Is(err, domain.ErrSkiRented),
		errors.Is(err, domain.ErrSkiNotRented),
		errors.Is(err, domain.ErrCustomerBlocked),
		errors.Is(err, domain.ErrLengthMismatch),
		errors.Is(err, domain.ErrNotAvailable):
		status = http.StatusConflict
	}
	if status == http.StatusInternalServerError {
		log.Printf("interner Fehler: %v", err)
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
