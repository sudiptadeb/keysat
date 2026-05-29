package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/sudiptadeb/keysat/internal/context"
	"github.com/sudiptadeb/keysat/internal/storage"
)

//go:embed static/*
var staticFiles embed.FS

// Server is the HTTP server for the keysat web dashboard.
type Server struct {
	db       *storage.DB
	resolver *context.Resolver
	logger   *slog.Logger
	mux      *http.ServeMux
}

// NewServer creates a Server and registers all routes.
func NewServer(db *storage.DB, resolver *context.Resolver) *Server {
	s := &Server{
		db:       db,
		resolver: resolver,
		logger:   slog.Default(),
		mux:      http.NewServeMux(),
	}

	// Static files.
	staticFS, _ := fs.Sub(staticFiles, "static")
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// API routes.
	s.mux.HandleFunc("GET /api/stats/summary", s.handleAPISummary)
	// Back-compat alias: /api/stats/today maps to the same summary handler.
	s.mux.HandleFunc("GET /api/stats/today", s.handleAPISummary)
	s.mux.HandleFunc("GET /api/stats/volume", s.handleAPIVolume)
	s.mux.HandleFunc("GET /api/stats/apps", s.handleAPIApps)
	s.mux.HandleFunc("GET /api/stats/domains", s.handleAPIDomains)
	s.mux.HandleFunc("GET /api/stats/dirs", s.handleAPIDirs)
	s.mux.HandleFunc("GET /api/stats/words", s.handleAPIWords)
	s.mux.HandleFunc("GET /api/stats/vocab", s.handleAPIVocab)
	s.mux.HandleFunc("GET /api/search", s.handleAPISearch)

	// Reporter routes.
	s.mux.HandleFunc("POST /api/report/domain", s.handleReportDomain)
	s.mux.HandleFunc("OPTIONS /api/report/domain", s.handleReportDomain)
	s.mux.HandleFunc("POST /api/report/directory", s.handleReportDirectory)
	s.mux.HandleFunc("OPTIONS /api/report/directory", s.handleReportDirectory)

	// SPA fallback: serve index.html for all other GET routes.
	s.mux.HandleFunc("GET /", s.serveSPA)

	return s
}

// serveSPA serves the embedded index.html for any non-API, non-static route.
func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on the given address.
func (s *Server) ListenAndServe(addr string) error {
	s.logger.Info("starting web server", "addr", addr)
	return http.ListenAndServe(addr, s)
}
