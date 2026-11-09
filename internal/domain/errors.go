package domain

import "errors"

// Fehler der Business-Logik. Handler mappen diese per errors.Is auf HTTP-Statuscodes.
var (
	ErrValidation      = errors.New("ungültige Eingabe")
	ErrNotFound        = errors.New("nicht gefunden")
	ErrDuplicate       = errors.New("existiert bereits")
	ErrSkiRented       = errors.New("Ski ist verliehen")
	ErrSkiNotRented    = errors.New("Ski ist nicht verliehen")
	ErrCustomerBlocked = errors.New("Kunde ist gesperrt")
	ErrLengthMismatch  = errors.New("Skilänge passt nicht zum Kunden")
	ErrNotAvailable    = errors.New("Ski ist im Zeitraum nicht verfügbar")
)
