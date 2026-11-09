package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net/http"

	"skiverleih/internal/handler"
	"skiverleih/internal/store"
)

//go:embed static
var staticFiles embed.FS

func main() {
	addr := flag.String("addr", ":8080", "HTTP-Adresse")
	withSeed := flag.Bool("seed", true, "Testdaten beim Start anlegen")
	flag.Parse()

	s := store.New()
	if *withSeed {
		if err := seed(s); err != nil {
			log.Fatalf("Testdaten anlegen: %v", err)
		}
	}

	mux := http.NewServeMux()
	handler.New(s).Register(mux)

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("static-Verzeichnis: %v", err)
	}
	mux.Handle("/", noCache(http.FileServerFS(static)))

	log.Printf("Skiverleih läuft auf %s (z.B. http://localhost:8080)", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// noCache zwingt den Browser, Frontend-Dateien bei jedem Laden neu zu prüfen –
// sonst läuft nach Änderungen oft noch eine alte app.js.
func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
