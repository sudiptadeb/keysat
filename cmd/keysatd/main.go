package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sudiptadeb/keysat/internal/config"
	"github.com/sudiptadeb/keysat/internal/context"
	"github.com/sudiptadeb/keysat/internal/pipeline"
	"github.com/sudiptadeb/keysat/internal/storage"
	"github.com/sudiptadeb/keysat/internal/web"
)

func main() {
	// 1. Load config (defaults for now)
	cfg := config.Default()

	// 2. Ensure directories exist
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directories: %v\n", err)
		os.Exit(1)
	}

	// 3. Setup slog logger (to stdout + file)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// 4. Open database
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 5. Create context resolver
	resolver := context.NewResolver()

	// 6. Create and start pipeline
	p := pipeline.NewPipeline(db, resolver)
	if err := p.Start(); err != nil {
		slog.Error("failed to start pipeline", "error", err)
		os.Exit(1)
	}

	// 7. Start web server (in goroutine)
	srv := web.NewServer(db, resolver)
	go func() {
		slog.Info("starting web server", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(cfg.ListenAddr); err != nil {
			slog.Error("web server error", "error", err)
		}
	}()

	// 8. Print startup message
	fmt.Printf("keysat daemon running\n")
	fmt.Printf("  dashboard: http://%s\n", cfg.ListenAddr)
	fmt.Printf("  database:  %s\n", cfg.DBPath)
	fmt.Printf("  press Ctrl+C to stop\n")

	// 9. Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	slog.Info("shutting down", "signal", sig)

	// 10. Graceful shutdown
	p.Stop()
	slog.Info("keysat stopped")
}
