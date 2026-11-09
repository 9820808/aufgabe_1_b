package store

import (
	"fmt"
	"sort"
	"sync"

	"skiverleih/internal/domain"
)

// SkiOverview ist ein Ski inkl. Name des Kunden, falls verliehen, und Beginn der nächsten Reservierung.
type SkiOverview struct {
	domain.Ski
	CustomerName string      `json:"customerName,omitempty"`
	ReservedFrom domain.Date `json:"reservedFrom,omitempty"` // erster Tag der nächsten Reservierung
}

type Stats struct {
	Skis             int `json:"skis"`
	SkisRented       int `json:"skisRented"`
	Customers        int `json:"customers"`
	CustomersBlocked int `json:"customersBlocked"`
	CustomersRenting int `json:"customersRenting"`
	SkisAvailableNow int `json:"skisAvailable"`
	SkisOverdue      int `json:"skisOverdue"`
	Reservations     int `json:"reservations"` // noch nicht abgelaufene
}

// ReservationOverview ist eine Reservierung inkl. Ski-Daten für die Anzeige.
type ReservationOverview struct {
	domain.Reservation
	Brand    string `json:"brand"`
	Model    string `json:"model"`
	LengthCm int    `json:"lengthCm"`
}

// MemoryStore hält alle Daten im Speicher. Alle Methoden sind thread-safe.
// Nach außen werden nur Kopien herausgegeben, damit niemand am Lock vorbei ändert.
type MemoryStore struct {
	mu             sync.RWMutex
	skis           map[string]*domain.Ski // Key: normierte Inventarnummer
	customers      map[int]*domain.Customer
	nextCustomerID int
	maxSeqPerYear  map[int]int     // höchste je vergebene Nummer pro Jahr (auch gelöschte)
	usedNumbers    map[string]bool // alle je vergebenen Inventarnummern (auch gelöschte)
	reservations   map[int]*domain.Reservation
	nextResID      int
	today          func() domain.Date // austauschbar für Tests
}

func New() *MemoryStore {
	return &MemoryStore{
		skis:           map[string]*domain.Ski{},
		customers:      map[int]*domain.Customer{},
		nextCustomerID: 1,
		maxSeqPerYear:  map[int]int{},
		usedNumbers:    map[string]bool{},
		reservations:   map[int]*domain.Reservation{},
		nextResID:      1,
		today:          domain.Today,
	}
}

// ---------- Ski ----------

// NextInventoryNumber schlägt die nächste freie Nummer für ein Anschaffungsjahr vor.
// Nummern gelöschter Ski werden nicht wiederverwendet.
func (s *MemoryStore) NextInventoryNumber(year int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return domain.InventoryNumber{Year: year, Seq: s.maxSeqPerYear[year] + 1}.String()
}

func (s *MemoryStore) AddSki(ski *domain.Ski) error {
	inv, err := domain.ParseInventoryNumber(ski.InventoryNumber)
	if err != nil {
		return fmt.Errorf("Ski anlegen: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.skis[inv.String()]; exists {
		return fmt.Errorf("%w: Inventarnummer %s", domain.ErrDuplicate, inv)
	}
	if s.usedNumbers[inv.String()] {
		// Nummer gehörte einem gelöschten Ski – nicht erneut verwenden.
		return fmt.Errorf("%w: Inventarnummer %s wurde bereits vergeben", domain.ErrDuplicate, inv)
	}
	s.usedNumbers[inv.String()] = true
	s.maxSeqPerYear[inv.Year] = max(s.maxSeqPerYear[inv.Year], inv.Seq)
	cp := *ski
	cp.InventoryNumber = inv.String()
	s.skis[cp.InventoryNumber] = &cp
	return nil
}

func (s *MemoryStore) DeleteSki(inventoryNumber string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ski, err := s.skiLocked(inventoryNumber)
	if err != nil {
		return err
	}
	if ski.IsRented() {
		return fmt.Errorf("löschen nicht möglich: %w", domain.ErrSkiRented)
	}
	today := s.today()
	for _, r := range s.reservations {
		if r.InventoryNumber == ski.InventoryNumber && !r.To.Before(today) {
			return fmt.Errorf("löschen nicht möglich: %w (reserviert ab %s)", domain.ErrNotAvailable, r.From)
		}
	}
	delete(s.skis, ski.InventoryNumber)
	return nil
}

// Skis liefert alle Ski sortiert nach Inventarnummer, inkl. Kundenname und nächster Reservierung.
func (s *MemoryStore) Skis() []SkiOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SkiOverview, 0, len(s.skis))
	for _, ski := range s.sortedSkisLocked() {
		ov := SkiOverview{Ski: *ski}
		if c, ok := s.customers[ski.RentedTo]; ok {
			ov.CustomerName = c.Name
		}
		if r := s.nextReservationLocked(ski.InventoryNumber, 0); r != nil {
			ov.ReservedFrom = r.From
		}
		out = append(out, ov)
	}
	return out
}

// ---------- Kunden ----------

// AddCustomer validiert, vergibt eine ID und speichert den Kunden.
func (s *MemoryStore) AddCustomer(c domain.Customer) (domain.Customer, error) {
	if err := c.Validate(); err != nil {
		return domain.Customer{}, fmt.Errorf("Kunde anlegen: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c.ID = s.nextCustomerID
	c.Blocked = false
	s.nextCustomerID++
	s.customers[c.ID] = &c
	return c, nil
}

func (s *MemoryStore) Customer(id int) (domain.Customer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.customerLocked(id)
	if err != nil {
		return domain.Customer{}, err
	}
	return *c, nil
}

// Customers liefert alle Kunden sortiert nach Name.
func (s *MemoryStore) Customers() []domain.Customer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Customer, 0, len(s.customers))
	for _, c := range s.customers {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *MemoryStore) SetBlocked(id int, blocked bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.customerLocked(id)
	if err != nil {
		return err
	}
	if blocked {
		c.Block()
	} else {
		c.Unblock()
	}
	return nil
}

// ---------- Verleih ----------

// AvailableSkisFor liefert nicht verliehene Ski, die zur bevorzugten Länge des Kunden passen
// und von heute bis until nicht für andere Kunden reserviert sind. Ein später reservierter Ski
// ist enthalten; ReservedFrom zeigt dann, dass er nur bis zum Vortag verliehen werden kann.
func (s *MemoryStore) AvailableSkisFor(customerID int, until domain.Date) ([]SkiOverview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.customerLocked(customerID)
	if err != nil {
		return nil, err
	}
	p := domain.Period{From: s.today(), To: until}
	out := []SkiOverview{}
	for _, ski := range s.sortedSkisLocked() {
		if ski.IsRented() || !c.FitsLength(ski.LengthCm) || s.reservedByOtherLocked(ski.InventoryNumber, p, customerID) != nil {
			continue
		}
		ov := SkiOverview{Ski: *ski}
		if r := s.nextReservationLocked(ski.InventoryNumber, customerID); r != nil {
			ov.ReservedFrom = r.From
		}
		out = append(out, ov)
	}
	return out, nil
}

// SkisRentedBy liefert alle Ski, die der Kunde aktuell ausgeliehen hat.
func (s *MemoryStore) SkisRentedBy(customerID int) ([]domain.Ski, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.customerLocked(customerID); err != nil {
		return nil, err
	}
	out := []domain.Ski{}
	for _, ski := range s.sortedSkisLocked() {
		if ski.RentedTo == customerID {
			out = append(out, *ski)
		}
	}
	return out, nil
}

// Rent verleiht den Ski ab heute bis einschließlich until. Eigene Reservierungen des Kunden
// stören nicht (so wird aus einer Reservierung am Abholtag eine Ausleihe), fremde schon.
func (s *MemoryStore) Rent(inventoryNumber string, customerID int, until domain.Date) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ski, err := s.skiLocked(inventoryNumber)
	if err != nil {
		return err
	}
	c, err := s.customerLocked(customerID)
	if err != nil {
		return err
	}
	today := s.today()
	if !until.Before(today) {
		if r := s.reservedByOtherLocked(ski.InventoryNumber, domain.Period{From: today, To: until}, customerID); r != nil {
			return fmt.Errorf("verleihen: %w: reserviert von %s bis %s", domain.ErrNotAvailable, r.From, r.To)
		}
	}
	if err := ski.RentTo(c, until, today); err != nil {
		return fmt.Errorf("verleihen: %w", err)
	}
	return nil
}

func (s *MemoryStore) Return(inventoryNumber string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ski, err := s.skiLocked(inventoryNumber)
	if err != nil {
		return err
	}
	if err := ski.Return(); err != nil {
		return fmt.Errorf("zurücknehmen: %w", err)
	}
	return nil
}

// ---------- Reservierung ----------

// ReservableSkisFor liefert Ski, die zur bevorzugten Länge passen und im ganzen Zeitraum frei sind.
func (s *MemoryStore) ReservableSkisFor(customerID int, p domain.Period) ([]domain.Ski, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, err := s.customerLocked(customerID)
	if err != nil {
		return nil, err
	}
	today := s.today()
	out := []domain.Ski{}
	for _, ski := range s.sortedSkisLocked() {
		if c.FitsLength(ski.LengthCm) && !ski.BusyDuring(p, today) && s.reservedByOtherLocked(ski.InventoryNumber, p, 0) == nil {
			out = append(out, *ski)
		}
	}
	return out, nil
}

// Reserve legt eine Reservierung an, wenn der Ski im Zeitraum frei ist.
func (s *MemoryStore) Reserve(inventoryNumber string, customerID int, p domain.Period) (domain.Reservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ski, err := s.skiLocked(inventoryNumber)
	if err != nil {
		return domain.Reservation{}, err
	}
	c, err := s.customerLocked(customerID)
	if err != nil {
		return domain.Reservation{}, err
	}
	res, err := domain.NewReservation(ski, c, p, s.today())
	if err != nil {
		return domain.Reservation{}, fmt.Errorf("reservieren: %w", err)
	}
	if r := s.reservedByOtherLocked(ski.InventoryNumber, p, 0); r != nil {
		return domain.Reservation{}, fmt.Errorf("reservieren: %w: bereits reserviert von %s bis %s", domain.ErrNotAvailable, r.From, r.To)
	}
	res.ID = s.nextResID
	s.nextResID++
	s.reservations[res.ID] = res
	return *res, nil
}

// ReservationsOf liefert die noch nicht abgelaufenen Reservierungen eines Kunden, sortiert nach Beginn.
func (s *MemoryStore) ReservationsOf(customerID int) ([]ReservationOverview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.customerLocked(customerID); err != nil {
		return nil, err
	}
	today := s.today()
	out := []ReservationOverview{}
	for _, r := range s.reservations {
		if r.CustomerID != customerID || r.To.Before(today) {
			continue
		}
		ov := ReservationOverview{Reservation: *r}
		if ski, ok := s.skis[r.InventoryNumber]; ok {
			ov.Brand, ov.Model, ov.LengthCm = ski.Brand, ski.Model, ski.LengthCm
		}
		out = append(out, ov)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From.Before(out[j].From)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Stats liefert Kennzahlen für das Dashboard.
func (s *MemoryStore) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	today := s.today()
	st := Stats{Skis: len(s.skis), Customers: len(s.customers)}
	renting := map[int]bool{}
	for _, ski := range s.skis {
		if ski.IsRented() {
			st.SkisRented++
			renting[ski.RentedTo] = true
		}
		if ski.IsOverdue(today) {
			st.SkisOverdue++
		}
	}
	for _, r := range s.reservations {
		if !r.To.Before(today) {
			st.Reservations++
		}
	}
	st.SkisAvailableNow = st.Skis - st.SkisRented
	st.CustomersRenting = len(renting)
	for _, c := range s.customers {
		if c.Blocked {
			st.CustomersBlocked++
		}
	}
	return st
}

// ---------- interne Helfer (Lock muss gehalten werden) ----------

func (s *MemoryStore) skiLocked(inventoryNumber string) (*domain.Ski, error) {
	inv, err := domain.ParseInventoryNumber(inventoryNumber)
	if err != nil {
		return nil, err
	}
	ski, ok := s.skis[inv.String()]
	if !ok {
		return nil, fmt.Errorf("%w: Ski %s", domain.ErrNotFound, inv)
	}
	return ski, nil
}

func (s *MemoryStore) customerLocked(id int) (*domain.Customer, error) {
	c, ok := s.customers[id]
	if !ok {
		return nil, fmt.Errorf("%w: Kunde %d", domain.ErrNotFound, id)
	}
	return c, nil
}

// reservedByOtherLocked liefert eine Reservierung des Ski, die sich mit p überschneidet und nicht
// dem Kunden exceptCustomerID gehört (0 = keine Ausnahme), sonst nil.
func (s *MemoryStore) reservedByOtherLocked(inventoryNumber string, p domain.Period, exceptCustomerID int) *domain.Reservation {
	for _, r := range s.reservations {
		if r.InventoryNumber == inventoryNumber && r.CustomerID != exceptCustomerID && r.Overlaps(p) {
			return r
		}
	}
	return nil
}

// nextReservationLocked liefert die früheste noch nicht abgelaufene Reservierung des Ski, die nicht
// dem Kunden exceptCustomerID gehört (0 = keine Ausnahme), sonst nil.
func (s *MemoryStore) nextReservationLocked(inventoryNumber string, exceptCustomerID int) *domain.Reservation {
	today := s.today()
	var next *domain.Reservation
	for _, r := range s.reservations {
		if r.InventoryNumber != inventoryNumber || r.CustomerID == exceptCustomerID || r.To.Before(today) {
			continue
		}
		if next == nil || r.From.Before(next.From) {
			next = r
		}
	}
	return next
}

func (s *MemoryStore) sortedSkisLocked() []*domain.Ski {
	list := make([]*domain.Ski, 0, len(s.skis))
	for _, ski := range s.skis {
		list = append(list, ski)
	}
	sort.Slice(list, func(i, j int) bool {
		a, _ := domain.ParseInventoryNumber(list[i].InventoryNumber)
		b, _ := domain.ParseInventoryNumber(list[j].InventoryNumber)
		return a.Less(b)
	})
	return list
}
