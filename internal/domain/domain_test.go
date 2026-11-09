package domain

import (
	"errors"
	"testing"
)

func TestParseInventoryNumber(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"2026-1", "2026-001", false},
		{"2026-007", "2026-007", false},
		{" 2024-1234 ", "2024-1234", false},
		{"2026", "", true},
		{"abcd-1", "", true},
		{"2026-0", "", true},
		{"2026-x", "", true},
		{"1800-1", "", true},
	}
	for _, tt := range tests {
		got, err := ParseInventoryNumber(tt.in)
		if tt.wantErr {
			if !errors.Is(err, ErrValidation) {
				t.Errorf("%q: erwartete ErrValidation, bekam %v", tt.in, err)
			}
			continue
		}
		if err != nil || got.String() != tt.want {
			t.Errorf("%q: got %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestInventoryNumberLess(t *testing.T) {
	a := InventoryNumber{2025, 10}
	b := InventoryNumber{2026, 1}
	c := InventoryNumber{2026, 2}
	if !a.Less(b) || !b.Less(c) || c.Less(a) {
		t.Error("Sortierung nach Jahr, dann Nummer stimmt nicht")
	}
}

func TestNewSkiValidation(t *testing.T) {
	if _, err := NewSki("2026-1", "Atomic", "G9", 170); err != nil {
		t.Fatalf("gültiger Ski abgelehnt: %v", err)
	}
	invalid := []struct {
		inv, brand, model string
		length            int
	}{
		{"2026-1", "", "G9", 170},
		{"2026-1", "Atomic", " ", 170},
		{"2026-1", "Atomic", "G9", 0},
		{"2026-1", "Atomic", "G9", 300},
		{"kaputt", "Atomic", "G9", 170},
	}
	for _, d := range invalid {
		if _, err := NewSki(d.inv, d.brand, d.model, d.length); !errors.Is(err, ErrValidation) {
			t.Errorf("%+v: erwartete ErrValidation, bekam %v", d, err)
		}
	}
}

func validCustomer() *Customer {
	return &Customer{
		ID: 1, Name: "Anna", Phone: "123", Email: "a@b.at", Address: "Weg 1",
		IDType: IDPassport, IDNumber: "P1", PreferredMinCm: 160, PreferredMaxCm: 170,
	}
}

func TestCustomerValidate(t *testing.T) {
	if err := validCustomer().Validate(); err != nil {
		t.Fatalf("gültiger Kunde abgelehnt: %v", err)
	}
	mutations := map[string]func(c *Customer){
		"ohne Name":              func(c *Customer) { c.Name = "" },
		"ohne Telefon":           func(c *Customer) { c.Phone = "" },
		"E-Mail ohne @":          func(c *Customer) { c.Email = "kaputt" },
		"ohne Anschrift":         func(c *Customer) { c.Address = "" },
		"falscher Typ":           func(c *Customer) { c.IDType = "personalausweis" },
		"weder Nummer noch Foto": func(c *Customer) { c.IDNumber = "" },
		"min > max":              func(c *Customer) { c.PreferredMinCm = 180 },
		"Länge fehlt":            func(c *Customer) { c.PreferredMaxCm = 0 },
	}
	for name, mutate := range mutations {
		c := validCustomer()
		mutate(c)
		if err := c.Validate(); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: erwartete ErrValidation, bekam %v", name, err)
		}
	}
}

// Ausweisnummer oder Ausweisfoto reicht, beides ist auch erlaubt.
func TestCustomerIDNumberOrPhoto(t *testing.T) {
	photo := &Photo{ContentType: "image/png", Data: []byte{1}}

	onlyPhoto := validCustomer()
	onlyPhoto.IDNumber = ""
	onlyPhoto.Photo = photo
	if err := onlyPhoto.Validate(); err != nil {
		t.Errorf("nur Foto: %v", err)
	}
	if onlyPhoto.IDType != "" {
		t.Errorf("ohne Nummer sollte kein Ausweistyp gesetzt sein, ist %q", onlyPhoto.IDType)
	}

	onlyNumber := validCustomer()
	if err := onlyNumber.Validate(); err != nil {
		t.Errorf("nur Nummer: %v", err)
	}

	both := validCustomer()
	both.Photo = photo
	if err := both.Validate(); err != nil {
		t.Errorf("beides: %v", err)
	}

	// Typ ist ohne Nummer egal (Formular schickt immer einen Wert mit).
	photoWithoutType := validCustomer()
	photoWithoutType.IDNumber = ""
	photoWithoutType.IDType = "irgendwas"
	photoWithoutType.Photo = photo
	if err := photoWithoutType.Validate(); err != nil {
		t.Errorf("Foto ohne gültigen Typ: %v", err)
	}
}

func TestSkiRentAndReturn(t *testing.T) {
	ski, _ := NewSki("2026-1", "Atomic", "G9", 165)
	c := validCustomer()

	if err := ski.RentTo(c, "2026-01-12", "2026-01-10"); err != nil {
		t.Fatalf("Verleih fehlgeschlagen: %v", err)
	}
	if ski.RentedTo != c.ID {
		t.Errorf("RentedTo = %d, want %d", ski.RentedTo, c.ID)
	}
	if err := ski.RentTo(c, "2026-01-12", "2026-01-10"); !errors.Is(err, ErrSkiRented) {
		t.Errorf("Doppelverleih: erwartete ErrSkiRented, bekam %v", err)
	}
	if err := ski.Return(); err != nil {
		t.Fatalf("Rückgabe fehlgeschlagen: %v", err)
	}
	if ski.IsRented() {
		t.Error("Ski ist nach Rückgabe noch verliehen")
	}
	if err := ski.Return(); !errors.Is(err, ErrSkiNotRented) {
		t.Errorf("doppelte Rückgabe: erwartete ErrSkiNotRented, bekam %v", err)
	}
}

func TestSkiRentRules(t *testing.T) {
	c := validCustomer()

	tooLong, _ := NewSki("2026-1", "Atomic", "G9", 171)
	if err := tooLong.RentTo(c, "2026-01-12", "2026-01-10"); !errors.Is(err, ErrLengthMismatch) {
		t.Errorf("zu lang: erwartete ErrLengthMismatch, bekam %v", err)
	}

	edge, _ := NewSki("2026-2", "Atomic", "G9", 170)
	c.Block()
	if err := edge.RentTo(c, "2026-01-12", "2026-01-10"); !errors.Is(err, ErrCustomerBlocked) {
		t.Errorf("gesperrt: erwartete ErrCustomerBlocked, bekam %v", err)
	}
	c.Unblock()
	if err := edge.RentTo(c, "2026-01-12", "2026-01-10"); err != nil {
		t.Errorf("Grenzwert max sollte passen: %v", err)
	}
}

func TestRentNeedsFutureOrTodayEnd(t *testing.T) {
	ski, _ := NewSki("2026-1", "Atomic", "G9", 165)
	if err := ski.RentTo(validCustomer(), "2026-01-09", "2026-01-10"); !errors.Is(err, ErrValidation) {
		t.Errorf("Rückgabe gestern: erwartete ErrValidation, bekam %v", err)
	}
	if err := ski.RentTo(validCustomer(), "2026-01-10", "2026-01-10"); err != nil {
		t.Errorf("Rückgabe heute sollte gehen: %v", err)
	}
	if ski.RentedUntil != "2026-01-10" {
		t.Errorf("RentedUntil = %q", ski.RentedUntil)
	}
}

func TestDateAndPeriod(t *testing.T) {
	if d, err := ParseDate(" 2026-02-28 "); err != nil || d.AddDays(1) != "2026-03-01" {
		t.Errorf("ParseDate/AddDays: %q, %v", d, err)
	}
	for _, bad := range []string{"", "28.02.2026", "2026-02-30"} {
		if _, err := ParseDate(bad); !errors.Is(err, ErrValidation) {
			t.Errorf("%q: erwartete ErrValidation, bekam %v", bad, err)
		}
	}
	if _, err := NewPeriod("2026-01-12", "2026-01-11"); !errors.Is(err, ErrValidation) {
		t.Errorf("Ende vor Start: erwartete ErrValidation, bekam %v", err)
	}

	p := Period{"2026-01-10", "2026-01-12"}
	overlaps := map[Period]bool{
		{"2026-01-12", "2026-01-15"}: true, // gemeinsamer letzter Tag
		{"2026-01-01", "2026-01-10"}: true, // gemeinsamer erster Tag
		{"2026-01-11", "2026-01-11"}: true,
		{"2026-01-13", "2026-01-15"}: false,
		{"2026-01-01", "2026-01-09"}: false,
	}
	for o, want := range overlaps {
		if p.Overlaps(o) != want || o.Overlaps(p) != want {
			t.Errorf("%v überschneidet %v: want %v", p, o, want)
		}
	}
}

func TestNewReservation(t *testing.T) {
	const today Date = "2026-01-10"
	ski, _ := NewSki("2026-1", "Atomic", "G9", 165)
	c := validCustomer()

	if _, err := NewReservation(ski, c, Period{"2026-01-10", "2026-01-11"}, today); !errors.Is(err, ErrValidation) {
		t.Errorf("ab heute: erwartete ErrValidation, bekam %v", err)
	}
	r, err := NewReservation(ski, c, Period{"2026-01-11", "2026-01-12"}, today)
	if err != nil || r.CustomerID != c.ID || r.InventoryNumber != "2026-001" {
		t.Fatalf("ab morgen: %+v, %v", r, err)
	}

	tooLong, _ := NewSki("2026-2", "Atomic", "G9", 190)
	if _, err := NewReservation(tooLong, c, Period{"2026-01-11", "2026-01-12"}, today); !errors.Is(err, ErrLengthMismatch) {
		t.Errorf("zu lang: erwartete ErrLengthMismatch, bekam %v", err)
	}

	c.Block()
	if _, err := NewReservation(ski, c, Period{"2026-01-11", "2026-01-12"}, today); !errors.Is(err, ErrCustomerBlocked) {
		t.Errorf("gesperrt: erwartete ErrCustomerBlocked, bekam %v", err)
	}
	c.Unblock()

	// Ski ist bis 13. verliehen: Reservierung ab 13. kollidiert, ab 14. nicht.
	if err := ski.RentTo(c, "2026-01-13", today); err != nil {
		t.Fatal(err)
	}
	if _, err := NewReservation(ski, c, Period{"2026-01-13", "2026-01-14"}, today); !errors.Is(err, ErrNotAvailable) {
		t.Errorf("während Ausleihe: erwartete ErrNotAvailable, bekam %v", err)
	}
	if _, err := NewReservation(ski, c, Period{"2026-01-14", "2026-01-15"}, today); err != nil {
		t.Errorf("nach Ausleihe: %v", err)
	}
	// Überfällig (Rückgabe war gestern, Ski noch draußen): gilt als belegt.
	if _, err := NewReservation(ski, c, Period{"2026-01-20", "2026-01-21"}, "2026-01-14"); !errors.Is(err, ErrNotAvailable) {
		t.Errorf("überfällig: erwartete ErrNotAvailable, bekam %v", err)
	}
}
