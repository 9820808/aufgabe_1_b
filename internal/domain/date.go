package domain

import (
	"fmt"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// Date ist ein Kalendertag im Format JJJJ-MM-TT. Verliehen und reserviert wird tageweise,
// Uhrzeiten spielen keine Rolle. Normierte Daten lassen sich direkt als String vergleichen.
type Date string

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, strings.TrimSpace(s))
	if err != nil {
		return "", fmt.Errorf("%w: Datum %q muss das Format JJJJ-MM-TT haben", ErrValidation, s)
	}
	return Date(t.Format(dateLayout)), nil
}

func Today() Date {
	return Date(time.Now().Format(dateLayout))
}

func (d Date) Before(o Date) bool { return d < o }

func (d Date) AddDays(n int) Date {
	t, _ := time.Parse(dateLayout, string(d))
	return Date(t.AddDate(0, 0, n).Format(dateLayout))
}

// Period ist ein Zeitraum von From bis einschließlich To.
type Period struct {
	From Date `json:"from"`
	To   Date `json:"to"`
}

func NewPeriod(from, to string) (Period, error) {
	f, err := ParseDate(from)
	if err != nil {
		return Period{}, err
	}
	t, err := ParseDate(to)
	if err != nil {
		return Period{}, err
	}
	if t.Before(f) {
		return Period{}, fmt.Errorf("%w: Enddatum liegt vor dem Startdatum", ErrValidation)
	}
	return Period{From: f, To: t}, nil
}

// Overlaps ist true, wenn beide Zeiträume mindestens einen gemeinsamen Tag haben.
func (p Period) Overlaps(o Period) bool {
	return !p.To.Before(o.From) && !o.To.Before(p.From)
}
