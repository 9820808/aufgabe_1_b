package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"skiverleih/internal/domain"
	"skiverleih/internal/store"
)

// seed legt Testdaten an: Ski, Kunden (einer gesperrt), laufende Ausleihen und Reservierungen.
// Datumsangaben sind relativ zu heute, damit die Daten immer aktuell wirken.
func seed(s *store.MemoryStore) error {
	skis := []struct {
		inv, brand, model string
		length            int
	}{
		{"2023-001", "Atomic", "Redster G9", 175},
		{"2023-002", "Atomic", "Redster G9", 182},
		{"2024-001", "Fischer", "RC4 The Curv", 165},
		{"2024-002", "Head", "Supershape e-Rally", 170},
		{"2024-003", "Rossignol", "Experience 82", 160},
		{"2025-001", "Völkl", "Deacon 80", 177},
		{"2025-002", "Salomon", "Stance 88", 184},
		{"2025-003", "Elan", "Ripstick 96", 152},
		{"2026-001", "Blizzard", "Brahma 88", 171},
		{"2026-002", "K2", "Mindbender 85", 145},
	}
	for _, d := range skis {
		ski, err := domain.NewSki(d.inv, d.brand, d.model, d.length)
		if err != nil {
			return fmt.Errorf("Ski %s: %w", d.inv, err)
		}
		if err := s.AddSki(ski); err != nil {
			return fmt.Errorf("Ski %s: %w", d.inv, err)
		}
	}

	customers := []domain.Customer{
		{Name: "Anna Berger", Phone: "+43 660 1234567", Email: "anna.berger@example.com",
			Address: "Hauptstraße 12, 6020 Innsbruck", IDType: domain.IDDriversLicense, IDNumber: "FS-0815-4711",
			PreferredMinCm: 160, PreferredMaxCm: 172, Photo: placeholderPhoto(color.RGBA{70, 130, 180, 255})},
		{Name: "Markus Huber", Phone: "+43 664 7654321", Email: "markus.huber@example.com",
			Address: "Bergweg 3, 5020 Salzburg", IDType: domain.IDPassport, IDNumber: "P1234567",
			PreferredMinCm: 175, PreferredMaxCm: 186},
		{Name: "Lena Moser", Phone: "+43 676 5550001", Email: "lena.moser@example.com",
			Address: "Seeufer 7, 9020 Klagenfurt", IDType: domain.IDPassport, IDNumber: "P7654321",
			PreferredMinCm: 140, PreferredMaxCm: 155},
		{Name: "Thomas Gruber", Phone: "+43 699 1112223", Email: "t.gruber@example.com",
			Address: "Lindenallee 21, 1010 Wien", IDType: domain.IDDriversLicense, IDNumber: "FS-2024-0042",
			PreferredMinCm: 165, PreferredMaxCm: 180},
		{Name: "Sophie Wallner", Phone: "+43 650 9998887", Email: "sophie.wallner@example.com",
			Address:        "Almweg 5, 6370 Kitzbühel", // nur Ausweisfoto, keine Ausweisnummer
			PreferredMinCm: 150, PreferredMaxCm: 165, Photo: placeholderPhoto(color.RGBA{46, 139, 87, 255})},
	}
	ids := map[string]int{}
	for _, c := range customers {
		created, err := s.AddCustomer(c)
		if err != nil {
			return fmt.Errorf("Kunde %s: %w", c.Name, err)
		}
		ids[c.Name] = created.ID
	}

	today := domain.Today()
	rentals := []struct {
		inv, customer string
		days          int // Ausleihe bis heute + days
	}{
		{"2024-001", "Anna Berger", 3},
		{"2025-002", "Markus Huber", 7},
		{"2023-001", "Thomas Gruber", 0},
	}
	for _, r := range rentals {
		if err := s.Rent(r.inv, ids[r.customer], today.AddDays(r.days)); err != nil {
			return fmt.Errorf("Ausleihe %s an %s: %w", r.inv, r.customer, err)
		}
	}

	// Erst nach der Ausleihe sperren: gesperrter Kunde mit noch offenem Ski.
	if err := s.SetBlocked(ids["Thomas Gruber"], true); err != nil {
		return fmt.Errorf("Kunde sperren: %w", err)
	}

	reservations := []struct {
		inv, customer string
		from, to      int // Tage ab heute
	}{
		{"2024-002", "Anna Berger", 5, 8},
		{"2026-002", "Lena Moser", 2, 4},
		{"2024-003", "Sophie Wallner", 10, 12},
	}
	for _, r := range reservations {
		p := domain.Period{From: today.AddDays(r.from), To: today.AddDays(r.to)}
		if _, err := s.Reserve(r.inv, ids[r.customer], p); err != nil {
			return fmt.Errorf("Reservierung %s für %s: %w", r.inv, r.customer, err)
		}
	}
	return nil
}

// placeholderPhoto erzeugt ein einfarbiges PNG als Platzhalter für ein Ausweisfoto.
func placeholderPhoto(c color.Color) *domain.Photo {
	img := image.NewRGBA(image.Rect(0, 0, 160, 100))
	for x := 0; x < 160; x++ {
		for y := 0; y < 100; y++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return &domain.Photo{ContentType: "image/png", Data: buf.Bytes()}
}
