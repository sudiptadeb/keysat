package web

import (
	"encoding/json"
	"net/http"
)

// setCORS sets CORS headers for the chrome extension and shell hook reporters.
func setCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// domainReport is the JSON body for POST /api/report/domain.
type domainReport struct {
	Domain string `json:"domain"`
}

// handleReportDomain receives the current browser domain from the chrome extension.
func (s *Server) handleReportDomain(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var body domainReport
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.resolver.SetDomain(body.Domain)
	if _, err := s.db.GetOrCreateDomain(body.Domain); err != nil {
		s.logger.Error("persist domain", "err", err)
	}
	s.logger.Info("domain reported", "domain", body.Domain)
	w.WriteHeader(http.StatusOK)
}

// directoryReport is the JSON body for POST /api/report/directory.
type directoryReport struct {
	Path string `json:"path"`
}

// handleReportDirectory receives the current working directory from the shell hook.
func (s *Server) handleReportDirectory(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	var body directoryReport
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	s.resolver.SetDirectory(body.Path)
	if _, err := s.db.GetOrCreateDirectory(body.Path); err != nil {
		s.logger.Error("persist directory", "err", err)
	}
	s.logger.Info("directory reported", "path", body.Path)
	w.WriteHeader(http.StatusOK)
}
