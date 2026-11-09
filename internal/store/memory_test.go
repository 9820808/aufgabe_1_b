package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"skiverleih/internal/domain"
)

// Feste Daten, damit die Tests nicht vom echten Datum abhängen.
const (
	testToday domain.Date = "2026-01-10"
	inAWeek   domain.Date = "2026-01-17"
)

func newTestStore() *MemoryStore {
	s := New()
	s.today = func() domain.Date { return testToday }
	return s
}

func mustSki(t *testing.T, s *MemoryStore, inv string, length int) {
	t.Helper()
	ski, err := domain.NewSki(inv, "Atomic", "G9", length)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddSki(ski); err != nil {
		t.Fatal(err)
	}
}

func mustCustomer(t *testing.T, s *MemoryStore, name string, minCm, maxCm int) domain.Customer {
	t.Helper()
	c, err := s.AddCustomer(domain.Customer{
		Name: name, Phone: "1", Email: "x@y.at", Address: "Weg 1",
		IDType: domain.IDPassport, IDNumber: "P1", PreferredMinCm: minCm, PreferredMaxCm: maxCm,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNextInventoryNumber(t *testing.T) {
	s := newTestStore()
	if got := s.NextInventoryNumber(2026); got != "2026-001" {
		t.Errorf("leerer Store: got %s", got)
	}
	mustSki(t, s, "2026-1", 170)
	mustSki(t, s, "2026-2", 170)
	mustSki(t, s, "2025-5", 170)
	if got := s.NextInventoryNumber(2026); got != "2026-003" {
		t.Errorf("2026: got %s", got)
	}
	if got := s.NextInventoryNumber(2025); got != "2025-006" {
		t.Errorf("2025: got %s", got)
	}

	// Gelöschte Nummern werden nicht wiederverwendet.
	if err := s.DeleteSki("2026-002"); err != nil {
		t.Fatal(err)
	}
	if got := s.NextInventoryNumber(2026); got != "2026-003" {
		t.Errorf("nach Löschen: got %s", got)
	}
}

func TestAddSkiDuplicate(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-001", 170)
	ski, _ := domain.NewSki("2026-1", "Head", "X", 170) // gleiche Nummer, andere Schreibweise
	if err := s.AddSki(ski); !errors.Is(err, domain.ErrDuplicate) {
		t.Errorf("erwartete ErrDuplicate, bekam %v", err)
	}
	if err := s.DeleteSki("2026-001"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSki(ski); !errors.Is(err, domain.ErrDuplicate) {
		t.Errorf("Nummer eines gelöschten Ski: erwartete ErrDuplicate, bekam %v", err)
	}
}

func TestSkisSortedWithCustomerName(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-10", 170)
	mustSki(t, s, "2026-2", 165)
	mustSki(t, s, "2024-1", 160)
	c := mustCustomer(t, s, "Anna", 160, 170)
	if err := s.Rent("2026-002", c.ID, inAWeek); err != nil {
		t.Fatal(err)
	}

	list := s.Skis()
	want := []string{"2024-001", "2026-002", "2026-010"}
	for i, inv := range want {
		if list[i].InventoryNumber != inv {
			t.Errorf("Position %d: got %s, want %s", i, list[i].InventoryNumber, inv)
		}
	}
	if list[1].CustomerName != "Anna" || list[0].CustomerName != "" {
		t.Errorf("Kundenname falsch: %+v", list)
	}
}

func TestDeleteSki(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 170)
	c := mustCustomer(t, s, "Anna", 160, 180)
	if err := s.Rent("2026-1", c.ID, inAWeek); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSki("2026-1"); !errors.Is(err, domain.ErrSkiRented) {
		t.Errorf("verliehenen Ski löschen: erwartete ErrSkiRented, bekam %v", err)
	}
	if err := s.Return("2026-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSki("2026-1"); err != nil {
		t.Errorf("löschen nach Rückgabe: %v", err)
	}
	if err := s.DeleteSki("2026-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zweimal löschen: erwartete ErrNotFound, bekam %v", err)
	}
}

func TestCustomers(t *testing.T) {
	s := newTestStore()
	b := mustCustomer(t, s, "Bernd", 160, 170)
	a := mustCustomer(t, s, "Anna", 160, 170)
	if a.ID == b.ID {
		t.Error("IDs müssen eindeutig sein")
	}
	list := s.Customers()
	if len(list) != 2 || list[0].Name != "Anna" {
		t.Errorf("Sortierung nach Name falsch: %+v", list)
	}
	if _, err := s.AddCustomer(domain.Customer{Name: "Ohne Rest"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("ungültiger Kunde: erwartete ErrValidation, bekam %v", err)
	}
	if _, err := s.Customer(999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannter Kunde: erwartete ErrNotFound, bekam %v", err)
	}
}

func TestBlockedCustomerCannotRent(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 170)
	c := mustCustomer(t, s, "Anna", 160, 180)

	if err := s.SetBlocked(c.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Rent("2026-1", c.ID, inAWeek); !errors.Is(err, domain.ErrCustomerBlocked) {
		t.Errorf("erwartete ErrCustomerBlocked, bekam %v", err)
	}
	if err := s.SetBlocked(c.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Rent("2026-1", c.ID, inAWeek); err != nil {
		t.Errorf("nach Entsperren: %v", err)
	}
}

func TestAvailableSkisFor(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 150) // zu kurz
	mustSki(t, s, "2026-2", 160) // passt (Grenze)
	mustSki(t, s, "2026-3", 170) // passt, wird verliehen
	mustSki(t, s, "2026-4", 175) // passt (Grenze)
	mustSki(t, s, "2026-5", 176) // zu lang
	anna := mustCustomer(t, s, "Anna", 160, 175)
	bernd := mustCustomer(t, s, "Bernd", 160, 180)
	if err := s.Rent("2026-3", bernd.ID, inAWeek); err != nil {
		t.Fatal(err)
	}

	skis, err := s.AvailableSkisFor(anna.ID, inAWeek)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ski := range skis {
		got = append(got, ski.InventoryNumber)
	}
	if fmt.Sprint(got) != "[2026-002 2026-004]" {
		t.Errorf("got %v", got)
	}
}

func TestRentAndReturn(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 170)
	mustSki(t, s, "2026-2", 170)
	c := mustCustomer(t, s, "Anna", 160, 180)

	for _, inv := range []string{"2026-1", "2026-2"} {
		if err := s.Rent(inv, c.ID, inAWeek); err != nil {
			t.Fatal(err)
		}
	}
	rented, _ := s.SkisRentedBy(c.ID)
	if len(rented) != 2 {
		t.Errorf("Kunde sollte 2 Ski haben, hat %d", len(rented))
	}
	if st := s.Stats(); st.SkisRented != 2 || st.CustomersRenting != 1 || st.SkisAvailableNow != 0 {
		t.Errorf("Stats falsch: %+v", st)
	}

	if err := s.Return("2026-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Return("2026-1"); !errors.Is(err, domain.ErrSkiNotRented) {
		t.Errorf("erwartete ErrSkiNotRented, bekam %v", err)
	}
	rented, _ = s.SkisRentedBy(c.ID)
	if len(rented) != 1 || rented[0].InventoryNumber != "2026-002" {
		t.Errorf("nach Rückgabe: %+v", rented)
	}
	if err := s.Rent("2026-9", c.ID, inAWeek); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unbekannter Ski: erwartete ErrNotFound, bekam %v", err)
	}
}

// Viele Mitarbeiter wollen gleichzeitig denselben Ski verleihen – nur einer darf gewinnen.
// Mit `go test -race` laufen lassen, um Data Races zu finden.
func TestConcurrentRent(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 170)
	var ids []int
	for i := 0; i < 50; i++ {
		ids = append(ids, mustCustomer(t, s, fmt.Sprintf("K%d", i), 160, 180).ID)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if s.Rent("2026-1", id, inAWeek) == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
			s.Skis()
			s.Stats()
		}(id)
	}
	wg.Wait()
	if success != 1 {
		t.Errorf("genau ein Verleih erwartet, waren %d", success)
	}
}

func TestReserve(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 170)
	mustSki(t, s, "2026-2", 170)
	anna := mustCustomer(t, s, "Anna", 160, 180)
	bernd := mustCustomer(t, s, "Bernd", 160, 180)

	p := domain.Period{From: "2026-01-12", To: "2026-01-14"}
	if _, err := s.Reserve("2026-1", anna.ID, p); err != nil {
		t.Fatal(err)
	}
	// Gleicher Ski, überschneidender Tag → abgelehnt; direkt danach → ok.
	if _, err := s.Reserve("2026-1", bernd.ID, domain.Period{From: "2026-01-14", To: "2026-01-15"}); !errors.Is(err, domain.ErrNotAvailable) {
		t.Errorf("Überschneidung: erwartete ErrNotAvailable, bekam %v", err)
	}
	if _, err := s.Reserve("2026-1", bernd.ID, domain.Period{From: "2026-01-15", To: "2026-01-15"}); err != nil {
		t.Errorf("Folgetag: %v", err)
	}

	free, _ := s.ReservableSkisFor(bernd.ID, domain.Period{From: "2026-01-13", To: "2026-01-13"})
	if len(free) != 1 || free[0].InventoryNumber != "2026-002" {
		t.Errorf("reservierbar am 13.: %+v", free)
	}

	list, _ := s.ReservationsOf(anna.ID)
	if len(list) != 1 || list[0].From != "2026-01-12" || list[0].Brand != "Atomic" {
		t.Errorf("Reservierungen von Anna: %+v", list)
	}
	if st := s.Stats(); st.Reservations != 2 {
		t.Errorf("Stats.Reservations = %d, want 2", st.Reservations)
	}

	// Reservierter Ski kann nicht gelöscht werden.
	if err := s.DeleteSki("2026-1"); !errors.Is(err, domain.ErrNotAvailable) {
		t.Errorf("reservierten Ski löschen: erwartete ErrNotAvailable, bekam %v", err)
	}
}

func TestRentRespectsReservations(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 170)
	anna := mustCustomer(t, s, "Anna", 160, 180)
	bernd := mustCustomer(t, s, "Bernd", 160, 180)
	if _, err := s.Reserve("2026-1", anna.ID, domain.Period{From: "2026-01-13", To: "2026-01-14"}); err != nil {
		t.Fatal(err)
	}

	// Bernd bis 12. geht, bis 13. kollidiert mit Annas Reservierung.
	if skis, _ := s.AvailableSkisFor(bernd.ID, "2026-01-13"); len(skis) != 0 {
		t.Errorf("verfügbar bis 13.: %+v", skis)
	}
	if err := s.Rent("2026-1", bernd.ID, "2026-01-13"); !errors.Is(err, domain.ErrNotAvailable) {
		t.Errorf("erwartete ErrNotAvailable, bekam %v", err)
	}
	if err := s.Rent("2026-1", bernd.ID, "2026-01-12"); err != nil {
		t.Errorf("bis 12.: %v", err)
	}
	if err := s.Return("2026-1"); err != nil {
		t.Fatal(err)
	}

	// Anna darf ihren reservierten Ski ausleihen.
	if err := s.Rent("2026-1", anna.ID, "2026-01-14"); err != nil {
		t.Errorf("eigene Reservierung: %v", err)
	}
	// Verliehener Ski ist im Ausleihzeitraum nicht reservierbar.
	if skis, _ := s.ReservableSkisFor(bernd.ID, domain.Period{From: "2026-01-14", To: "2026-01-16"}); len(skis) != 0 {
		t.Errorf("reservierbar trotz Ausleihe: %+v", skis)
	}
}

func TestReservedSkiShownAndRentableUntilDayBefore(t *testing.T) {
	s := newTestStore()
	mustSki(t, s, "2026-1", 170)
	anna := mustCustomer(t, s, "Anna", 160, 180)
	bernd := mustCustomer(t, s, "Bernd", 160, 180)
	// Zwei Reservierungen: angezeigt wird die frühere.
	for _, p := range []domain.Period{{From: "2026-01-20", To: "2026-01-21"}, {From: "2026-01-15", To: "2026-01-16"}} {
		if _, err := s.Reserve("2026-1", anna.ID, p); err != nil {
			t.Fatal(err)
		}
	}

	if got := s.Skis()[0].ReservedFrom; got != "2026-01-15" {
		t.Errorf("Ski-Liste ReservedFrom = %q, want 2026-01-15", got)
	}

	// Für Bernd verfügbar bis zum Vortag, mit Hinweis auf die Reservierung.
	skis, _ := s.AvailableSkisFor(bernd.ID, "2026-01-14")
	if len(skis) != 1 || skis[0].ReservedFrom != "2026-01-15" {
		t.Errorf("verfügbar bis 14.: %+v", skis)
	}
	// Für Anna selbst ist die eigene Reservierung kein Hindernis und kein Hinweis.
	skis, _ = s.AvailableSkisFor(anna.ID, "2026-01-16")
	if len(skis) != 1 || skis[0].ReservedFrom != "" {
		t.Errorf("Anna bis 16.: %+v", skis)
	}

	if err := s.Rent("2026-1", bernd.ID, "2026-01-15"); !errors.Is(err, domain.ErrNotAvailable) {
		t.Errorf("bis erster Reservierungstag: erwartete ErrNotAvailable, bekam %v", err)
	}
	if err := s.Rent("2026-1", bernd.ID, "2026-01-14"); err != nil {
		t.Errorf("bis Vortag: %v", err)
	}
	if ov := s.Skis()[0]; ov.CustomerName != "Bernd" || ov.ReservedFrom != "2026-01-15" {
		t.Errorf("verliehen und reserviert: %+v", ov)
	}

	// Abgelaufene Reservierungen werden nicht mehr angezeigt.
	s.today = func() domain.Date { return "2026-01-22" }
	if got := s.Skis()[0].ReservedFrom; got != "" {
		t.Errorf("nach Ablauf ReservedFrom = %q", got)
	}
}
