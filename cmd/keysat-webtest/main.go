// Command keysat-webtest runs the keysat web dashboard against an existing
// database without starting the capture daemon (no eventtap, no pipeline).
//
// It is a development / verification harness: point it at a real keysat.db and
// browse the dashboard read-only. It also serves as the natural end-to-end way
// to exercise the rewritten /api/search endpoint.
//
//	go run -tags fts5 ./cmd/keysat-webtest
//	go run -tags fts5 ./cmd/keysat-webtest -addr 127.0.0.1:7899 -db /path/to/keysat.db
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sudiptadeb/keysat/internal/context"
	"github.com/sudiptadeb/keysat/internal/storage"
	"github.com/sudiptadeb/keysat/internal/web"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:7899", "address to listen on")
	dbPath := flag.String("db", storage.DefaultDBPath(), "path to the keysat SQLite database")
	flag.Parse()

	db, err := storage.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database %q: %v\n", *dbPath, err)
		os.Exit(1)
	}
	defer db.Close()

	// Build the server with a resolver but do NOT start it: no polling, no
	// eventtap, no pipeline. This is a read-only view of the dashboard.
	resolver := context.NewResolver()
	srv := web.NewServer(db, resolver)

	fmt.Printf("keysat-webtest serving dashboard\n")
	fmt.Printf("  url:      http://%s\n", *addr)
	fmt.Printf("  database: %s\n", *dbPath)

	if err := srv.ListenAndServe(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "web server error: %v\n", err)
		os.Exit(1)
	}
}
