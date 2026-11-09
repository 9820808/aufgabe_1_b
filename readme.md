# Projekt

Web-Anwendung für das Backend einer Skiverleih Firma.

## Anforderungen

### Anforderung 0

Es soll ein Web-Backend erstellt werden. Zentraler Punkt des Backends ist
ein Dashboard. Am oberen Rand gibt es stilisierte Tab-Reiter, die auf die
Unterseiten (wie etwa Anforderung 1, Anforderung 4) umschalten lassen.

### Anforderung 1

**User Story:** Ein Mitarbeiter kann die Ski-Datenbank verwalten, indem er
neue Ski hinzufügt oder bestehende Ski löscht. Die Liste der im System
vorhandenen Ski wird angezeigt. Ist ein Ski verliehen, dann wird der Name
des Kunden dazu angezeigt.
Wird ein neuer Ski hinzugefügt, soll die Inventarnummer automatisch
vorgeschlagen werden (Siehe Anforderung 2 für das Format).
Für diese User Story soll eine eigene Seite erstellt werden.

### Anforderung 2

Die Ski-Datenbank besteht aus Einträgen mit foldengenden Feldern:
* Inventarnummer (<jahr>-<nummer> ... wobei <jahr> das Anschaffungsjahr ist,
und <nummer> eine fortlaufende Nummer.
* Marke (Freitext)
* Modell (Freitext)
* Länge in cm (Zahl)

### Anforderung 3

**User Story:** Ein Mitarbeiter kann die Kunden-Datenbank verwalten. Wie bei
Anforderung 1 soll er erstellen können, jedoch nicht löschen. Ein Kunde kann
gesperrt und entsperrt werden. Ein gesperrter Kunde kann keine Ski mehr
ausleihen.

### Anforderung 4

Die Kunden-Datenbank besteht aus Einträgen mit folgenden Feldern:
* Name
* Telefonnummer
* E-Mail
* Anschrift
* Führerschein oder Passnummer (mit selektion, ob die eingegebene Nummer ein
  Führerschein oder ein Pass ist)
* Foto-Upload für ein Foto eines Ausweises
* Bevorzugte Skilänge mit min und max.

Beim Anlegen eines Kunden muss entweder die Führerschein- bzw. Passnummer
oder das Ausweisfoto angegeben werden. Eines von beiden reicht, es können
aber auch beide eingetragen werden.

### Anforderung 5

**User Story:** Ein Mitarbeiter soll einem Kunden einen Ski verleihen
können. Dazu wählt er einen Kunden und bekommt die Ski aufgelistet, die zur
Bevorzugten Skilänge des Kunden passen und nicht bereits verliehen sind. Aus
dieser Liste kann der Mitarbeiter einen Ski auswählen. Dieser Ski wird als
Verliehen an den Kunden registriert.

### Anforderung 6

**User Story:** Ein Mitarbeiter soll einen verliehenen Ski von einem Kunden
wieder zurücknehmen können. Dazu hat er einen eigenen Schirm, in dem er
entweder einen Kunden oder einen Ski auswählen kann. Nach Auswahl wird das
jeweilige passende Gegenstück (zB Kunde, der den Ski ausgeliehen hat, oder
der Ski des Kunden) angezeigt. Mittels Auswahl kann die Ausleihe beendet
werden.

### Anforderung 7

**User Story:** Ein Kunde soll über eine eigene Frontend-Seite Ski für sich
reservieren können. Eine Reservierung ist ein zeitlich terminiertes
Ausleihen, welches in der Zukunft liegen muss. Bei einer Reservierung werden
die Ski tagesweise an die Kunden vergeben.

Klarstellungen:
* Der Kunde wählt sich auf der Kundenseite selbst aus einer Namensliste (keine
  Anmeldung). Die Liste enthält nur Namen, keine Kontakt- oder Ausweisdaten.
* Reservierungen sind frühestens ab morgen möglich und gelten von–bis
  einschließlich. Ein Ski kann pro Tag nur einmal vergeben werden.
* Beim Verleih (Anforderung 5) muss der Mitarbeiter ein Rückgabedatum
  eintragen. Ein Ski ist an den Tagen einer Ausleihe nicht reservierbar, und
  ein für einen anderen Kunden reservierter Ski kann nicht über die Reservierung
  hinweg verliehen werden. Ein überfälliger Ski ist nicht reservierbar, bis er
  zurück ist.

### Anforderung 8

Wird ein Ski reserviert, ist das in der Liste der Ski ersichtlich: Statt
„verfügbar“ steht „reserviert ab <datum>“ (erster Tag der nächsten
Reservierung). Der Ski kann weiterhin ausgeliehen werden, aber nur bis zum Tag
vor dem ersten reservierten Tag.

Klarstellungen:
* Ist ein Ski gerade verliehen und zusätzlich reserviert, werden beide
  Informationen angezeigt.
* Im Verleih (Anforderung 5) wird bei einem reservierten Ski angezeigt, bis
  wann er höchstens verliehen werden kann. Ein späteres Rückgabedatum wird
  abgelehnt.
* Der Kunde, der den Ski reserviert hat, kann ihn auch über seine eigene
  Reservierung hinweg ausleihen (Abholung der Reservierung).
