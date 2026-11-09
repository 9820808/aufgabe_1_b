package domain

import "fmt"

// Reservation ist eine Ausleihe in der Zukunft für ganze Tage (From bis einschließlich To).
type Reservation struct {
	ID              int    `json:"id"`
	InventoryNumber string `json:"inventoryNumber"`
	CustomerID      int    `json:"customerId"`
	Period
}

// NewReservation prüft die Regeln, die sich aus Ski, Kunde und Zeitraum ergeben.
// Überschneidungen mit anderen Reservierungen prüft der Store, der alle kennt.
func NewReservation(ski *Ski, c *Customer, p Period, today Date) (*Reservation, error) {
	if !today.Before(p.From) {
		return nil, fmt.Errorf("%w: Reservierung muss in der Zukunft liegen (frühestens ab morgen)", ErrValidation)
	}
	if c.Blocked {
		return nil, fmt.Errorf("%w: %s", ErrCustomerBlocked, c.Name)
	}
	if !c.FitsLength(ski.LengthCm) {
		return nil, fmt.Errorf("%w: %d cm nicht in %d–%d cm", ErrLengthMismatch, ski.LengthCm, c.PreferredMinCm, c.PreferredMaxCm)
	}
	if ski.BusyDuring(p, today) {
		return nil, fmt.Errorf("%w: %s ist im Zeitraum verliehen", ErrNotAvailable, ski.InventoryNumber)
	}
	return &Reservation{InventoryNumber: ski.InventoryNumber, CustomerID: c.ID, Period: p}, nil
}
