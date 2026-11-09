# Prototype Project
Nur Prototypen. Priorität: schnell verständlich, leicht änderbar.
## Stack
- Backend: Go ('objektorientiert': Structs + Methoden,
Interfaces wo sinnvoll)
- DB: In-Memory (kein Postgres/SQLite, kein Object-
Relational Mapping (ORM) nötig)
- Frontend: entweder reines HTML/JS oder Flutter/Dart
- Versionierung: lokales Git, bei Bedarf GitHub
## Commands
- Backend starten: `go run .` (oder `go run ./cmd/server`)
- Tests: `go test ./...`
- Format: `gofmt -w .` bzw. `go fmt ./...`
- Lint (falls eingerichtet): `golangci-lint run`
- Frontend (HTML): einfach Dateien öffnen oder kleinen
Static-Server nutzen
- Frontend (Flutter): `flutter run`
## Architektur (Go)
- `main.go` oder `cmd/server/` → Einstieg, HTTP-Server,
Routing
- `internal/domain/` → Kern-Structs + Methoden
(Business-Logik)
- `internal/store/` → In-Memory-Store (Map + Mutex oder
sync.Map)
- `internal/handler/` → HTTP-Handler, die Domain-
Objekte aufrufen
- Keine globalen Variablen außer dem Store-Singleton
wenn nötig
- Fehler mit `fmt.Errorf("%w", err)` wrappen, nicht verschlucken
## Konventionen
- Methoden an Structs hängen statt freier Funktionen,
wo es Sinn ergibt
- Interfaces nur bei echtem Bedarf (z.B. Store-Interface
für spätere Austauschbarkeit)
- Kein Dependency Injection Framework
- Keine neuen Dependencies ohne Nachfrage
- In-Memory-Store muss thread-safe sein
- Keine Secrets im Code (auch bei Prototypen nicht)
## Frontend
- HTML-Variante: möglichst minimales JS, klare
Trennung von Backend-API
- Flutter-Variante: State-Management einfach halten (Provider oder Riverpod nur wenn nötig)
- API-Calls immer gegen die Go-Endpoints, nie direkt gegen In-Memory-Strukturen
## Regeln
- Keine generierten Dateien von Hand editieren
- Commit-Messages: kurz und imperativ
- Bei Unklarheit fragen statt raten
- Prototyp-Fokus: lieber einfach und klar als elegant und komplex