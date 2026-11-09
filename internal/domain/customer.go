package domain

import (
	"fmt"
	"strings"
)

type IDType string

const (
	IDDriversLicense IDType = "fuehrerschein"
	IDPassport       IDType = "pass"
)

func (t IDType) Valid() bool {
	return t == IDDriversLicense || t == IDPassport
}

// Photo ist das hochgeladene Ausweisfoto.
type Photo struct {
	ContentType string
	Data        []byte
}

type Customer struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Phone          string `json:"phone"`
	Email          string `json:"email"`
	Address        string `json:"address"`
	IDType         IDType `json:"idType"`
	IDNumber       string `json:"idNumber"`
	PreferredMinCm int    `json:"preferredMinCm"`
	PreferredMaxCm int    `json:"preferredMaxCm"`
	Blocked        bool   `json:"blocked"`
	Photo          *Photo `json:"-"`
}

// Validate prüft Pflichtfelder und die bevorzugte Skilänge.
// Zur Identifikation reicht Ausweisnummer oder Ausweisfoto (beides ist erlaubt).
// Der Ausweistyp ist nur zusammen mit einer Nummer relevant.
func (c *Customer) Validate() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Phone = strings.TrimSpace(c.Phone)
	c.Email = strings.TrimSpace(c.Email)
	c.Address = strings.TrimSpace(c.Address)
	c.IDNumber = strings.TrimSpace(c.IDNumber)
	if c.IDNumber == "" {
		c.IDType = "" // ohne Nummer kein Typ, sonst zeigt die Liste z.B. „Führerschein“ ohne Nummer
	}

	switch {
	case c.Name == "":
		return fmt.Errorf("%w: Name fehlt", ErrValidation)
	case c.Phone == "":
		return fmt.Errorf("%w: Telefonnummer fehlt", ErrValidation)
	case !strings.Contains(c.Email, "@"):
		return fmt.Errorf("%w: E-Mail ungültig", ErrValidation)
	case c.Address == "":
		return fmt.Errorf("%w: Anschrift fehlt", ErrValidation)
	case c.IDNumber == "" && !c.HasPhoto():
		return fmt.Errorf("%w: Ausweisnummer oder Ausweisfoto erforderlich", ErrValidation)
	case c.IDNumber != "" && !c.IDType.Valid():
		return fmt.Errorf("%w: Ausweistyp muss Führerschein oder Pass sein", ErrValidation)
	case c.PreferredMinCm <= 0 || c.PreferredMaxCm <= 0:
		return fmt.Errorf("%w: bevorzugte Skilänge fehlt", ErrValidation)
	case c.PreferredMinCm > c.PreferredMaxCm:
		return fmt.Errorf("%w: minimale Skilänge größer als maximale", ErrValidation)
	}
	return nil
}

func (c *Customer) FitsLength(cm int) bool {
	return cm >= c.PreferredMinCm && cm <= c.PreferredMaxCm
}

func (c *Customer) HasPhoto() bool {
	return c.Photo != nil && len(c.Photo.Data) > 0
}

func (c *Customer) Block()   { c.Blocked = true }
func (c *Customer) Unblock() { c.Blocked = false }
