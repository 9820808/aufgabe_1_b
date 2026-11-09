package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// InventoryNumber hat das Format <jahr>-<nummer>, z.B. 2026-007.
// Jahr = Anschaffungsjahr, Nummer = fortlaufend pro Jahr.
type InventoryNumber struct {
	Year int
	Seq  int
}

// ParseInventoryNumber liest "2026-7" oder "2026-007".
func ParseInventoryNumber(s string) (InventoryNumber, error) {
	yearStr, seqStr, ok := strings.Cut(strings.TrimSpace(s), "-")
	if !ok {
		return InventoryNumber{}, fmt.Errorf("%w: Inventarnummer %q muss das Format <jahr>-<nummer> haben", ErrValidation, s)
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 1900 || year > 2999 {
		return InventoryNumber{}, fmt.Errorf("%w: ungültiges Jahr in Inventarnummer %q", ErrValidation, s)
	}
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return InventoryNumber{}, fmt.Errorf("%w: ungültige Nummer in Inventarnummer %q", ErrValidation, s)
	}
	return InventoryNumber{Year: year, Seq: seq}, nil
}

// String liefert die normierte Schreibweise mit mindestens drei Stellen.
func (n InventoryNumber) String() string {
	return fmt.Sprintf("%d-%03d", n.Year, n.Seq)
}

// Less sortiert nach Jahr, dann Nummer.
func (n InventoryNumber) Less(o InventoryNumber) bool {
	if n.Year != o.Year {
		return n.Year < o.Year
	}
	return n.Seq < o.Seq
}

type Ski struct {
	InventoryNumber string `json:"inventoryNumber"`
	Brand           string `json:"brand"`
	Model           string `json:"model"`
	LengthCm        int    `json:"lengthCm"`
	RentedTo        int    `json:"rentedTo,omitempty"`    // Kunden-ID, 0 = nicht verliehen
	RentedUntil     Date   `json:"rentedUntil,omitempty"` // letzter Tag der Ausleihe
}

// NewSki validiert die Eingaben und normiert die Inventarnummer.
func NewSki(inventoryNumber, brand, model string, lengthCm int) (*Ski, error) {
	inv, err := ParseInventoryNumber(inventoryNumber)
	if err != nil {
		return nil, err
	}
	brand, model = strings.TrimSpace(brand), strings.TrimSpace(model)
	if brand == "" {
		return nil, fmt.Errorf("%w: Marke fehlt", ErrValidation)
	}
	if model == "" {
		return nil, fmt.Errorf("%w: Modell fehlt", ErrValidation)
	}
	if lengthCm < 50 || lengthCm > 250 {
		return nil, fmt.Errorf("%w: Länge muss zwischen 50 und 250 cm liegen", ErrValidation)
	}
	return &Ski{InventoryNumber: inv.String(), Brand: brand, Model: model, LengthCm: lengthCm}, nil
}

func (s *Ski) IsRented() bool {
	return s.RentedTo != 0
}

// IsOverdue ist true, wenn der Ski nach dem vereinbarten Rückgabetag noch verliehen ist.
func (s *Ski) IsOverdue(today Date) bool {
	return s.IsRented() && s.RentedUntil.Before(today)
}

// BusyDuring ist true, wenn die laufende Ausleihe in den Zeitraum hineinreicht.
// Ein überfälliger Ski gilt als belegt, weil unklar ist, wann er zurückkommt.
func (s *Ski) BusyDuring(p Period, today Date) bool {
	return s.IsRented() && (s.IsOverdue(today) || !s.RentedUntil.Before(p.From))
}

// RentTo prüft alle Verleih-Regeln und registriert den Ski ab heute bis einschließlich until beim Kunden.
func (s *Ski) RentTo(c *Customer, until, today Date) error {
	if until.Before(today) {
		return fmt.Errorf("%w: Rückgabedatum liegt in der Vergangenheit", ErrValidation)
	}
	if s.IsRented() {
		return fmt.Errorf("%w: %s", ErrSkiRented, s.InventoryNumber)
	}
	if c.Blocked {
		return fmt.Errorf("%w: %s", ErrCustomerBlocked, c.Name)
	}
	if !c.FitsLength(s.LengthCm) {
		return fmt.Errorf("%w: %d cm nicht in %d–%d cm", ErrLengthMismatch, s.LengthCm, c.PreferredMinCm, c.PreferredMaxCm)
	}
	s.RentedTo = c.ID
	s.RentedUntil = until
	return nil
}

// Return beendet die Ausleihe.
func (s *Ski) Return() error {
	if !s.IsRented() {
		return fmt.Errorf("%w: %s", ErrSkiNotRented, s.InventoryNumber)
	}
	s.RentedTo = 0
	s.RentedUntil = ""
	return nil
}
