package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/sudiptadeb/keysat/internal/storage"
)

// parseUnixParam reads a query parameter as a unix timestamp (seconds), returning def if missing.
func parseUnixParam(r *http.Request, name string, def int64) int64 {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}

// parseIntParam reads a query parameter as an int, returning def if missing.
func parseIntParam(r *http.Request, name string, def int) int {
	s := r.URL.Query().Get(name)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// writeJSON marshals v and writes it as a JSON response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "json encode error", http.StatusInternalServerError)
	}
}

// emptyArray is used to marshal nil slices as [] instead of null.
var emptyArray = []struct{}{}

// --- API handlers ---

// handleAPISummary returns a typing summary over [since, until). When since is
// absent it defaults to local midnight and until defaults to now, preserving
// the original today-only dashboard numbers. Serves both /api/stats/summary and
// the /api/stats/today back-compat alias.
func (s *Server) handleAPISummary(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	// Default since = local midnight; default until = now.
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since := parseUnixParam(r, "since", startOfDay.Unix())
	until := parseUnixParam(r, "until", now.Unix())

	stats, err := s.db.GetSummary(since*1000, until*1000)
	if err != nil {
		s.logger.Error("api summary", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"keystrokes":     stats.TotalKeystrokes,
		"words":          stats.TotalWords,
		"unique_words":   stats.UniqueWords,
		"active_minutes": stats.ActiveMinutes,
		"top_app":        stats.TopApp,
		"top_domain":     stats.TopDomain,
	})
}

// handleAPIVolume returns keystroke/word volume bucketed over time.
// GET /api/stats/volume?since=UNIX&until=UNIX&bucket=3600
func (s *Server) handleAPIVolume(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()
	since := parseUnixParam(r, "since", now-24*3600)
	until := parseUnixParam(r, "until", now)
	bucket := parseUnixParam(r, "bucket", 3600)

	// Convert seconds to milliseconds for DB (timestamps stored as ms).
	data, err := s.db.GetTypingVolume(since*1000, until*1000, bucket)
	if err != nil {
		s.logger.Error("api volume", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		writeJSON(w, emptyArray)
		return
	}
	type entry struct {
		Timestamp  int64 `json:"timestamp"`
		Keystrokes int   `json:"keystrokes"`
		Words      int   `json:"words"`
	}
	out := make([]entry, len(data))
	for i, b := range data {
		out[i] = entry{
			Timestamp:  b.Timestamp / 1000, // ms -> seconds for API
			Keystrokes: b.KeystrokeCount,
			Words:      b.WordCount,
		}
	}
	writeJSON(w, out)
}

// handleAPIApps returns top apps by word count.
// GET /api/stats/apps?since=UNIX&limit=10
func (s *Server) handleAPIApps(w http.ResponseWriter, r *http.Request) {
	since := parseUnixParam(r, "since", time.Now().Add(-7*24*time.Hour).Unix())
	until := parseUnixParam(r, "until", time.Now().Unix())
	limit := parseIntParam(r, "limit", 10)

	data, err := s.db.GetAppStats(since*1000, until*1000, limit)
	if err != nil {
		s.logger.Error("api apps", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		writeJSON(w, emptyArray)
		return
	}
	type entry struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Words      int    `json:"words"`
		Keystrokes int    `json:"keystrokes"`
	}
	out := make([]entry, len(data))
	for i, a := range data {
		out[i] = entry{
			Name:       a.AppName,
			Type:       a.AppType,
			Words:      a.WordCount,
			Keystrokes: a.KeystrokeCount,
		}
	}
	writeJSON(w, out)
}

// handleAPIDomains returns top domains by word count.
// GET /api/stats/domains?since=UNIX&limit=10
func (s *Server) handleAPIDomains(w http.ResponseWriter, r *http.Request) {
	since := parseUnixParam(r, "since", time.Now().Add(-7*24*time.Hour).Unix())
	until := parseUnixParam(r, "until", time.Now().Unix())
	limit := parseIntParam(r, "limit", 10)

	data, err := s.db.GetDomainStats(since*1000, until*1000, limit)
	if err != nil {
		s.logger.Error("api domains", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		writeJSON(w, emptyArray)
		return
	}
	type entry struct {
		Domain string `json:"domain"`
		Words  int    `json:"words"`
	}
	out := make([]entry, len(data))
	for i, d := range data {
		out[i] = entry{Domain: d.Domain, Words: d.WordCount}
	}
	writeJSON(w, out)
}

// handleAPIDirs returns top directories by word count.
// GET /api/stats/dirs?since=UNIX&limit=10
func (s *Server) handleAPIDirs(w http.ResponseWriter, r *http.Request) {
	since := parseUnixParam(r, "since", time.Now().Add(-7*24*time.Hour).Unix())
	until := parseUnixParam(r, "until", time.Now().Unix())
	limit := parseIntParam(r, "limit", 10)

	data, err := s.db.GetDirectoryStats(since*1000, until*1000, limit)
	if err != nil {
		s.logger.Error("api dirs", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		writeJSON(w, emptyArray)
		return
	}
	type entry struct {
		Path  string `json:"path"`
		Words int    `json:"words"`
	}
	out := make([]entry, len(data))
	for i, d := range data {
		out[i] = entry{Path: d.Path, Words: d.WordCount}
	}
	writeJSON(w, out)
}

// handleAPIWords returns top words by frequency.
// GET /api/stats/words?since=UNIX&limit=20
func (s *Server) handleAPIWords(w http.ResponseWriter, r *http.Request) {
	since := parseUnixParam(r, "since", time.Now().Add(-7*24*time.Hour).Unix())
	until := parseUnixParam(r, "until", time.Now().Unix())
	limit := parseIntParam(r, "limit", 20)

	data, err := s.db.GetWordFrequency(since*1000, until*1000, limit)
	if err != nil {
		s.logger.Error("api words", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		writeJSON(w, emptyArray)
		return
	}
	type entry struct {
		Word  string `json:"word"`
		Count int    `json:"count"`
	}
	out := make([]entry, len(data))
	for i, wf := range data {
		out[i] = entry{Word: wf.Word, Count: wf.Count}
	}
	writeJSON(w, out)
}

// handleAPIVocab returns vocabulary growth over time.
// GET /api/stats/vocab?since=UNIX&bucket=86400
func (s *Server) handleAPIVocab(w http.ResponseWriter, r *http.Request) {
	since := parseUnixParam(r, "since", time.Now().Add(-30*24*time.Hour).Unix())
	until := parseUnixParam(r, "until", time.Now().Unix())
	bucket := parseUnixParam(r, "bucket", 86400)

	data, err := s.db.GetVocabGrowth(since*1000, until*1000, bucket)
	if err != nil {
		s.logger.Error("api vocab", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if data == nil {
		writeJSON(w, emptyArray)
		return
	}
	type entry struct {
		Timestamp  int64 `json:"timestamp"`
		NewWords   int   `json:"new_words"`
		TotalWords int   `json:"total_words"`
	}
	out := make([]entry, len(data))
	for i, b := range data {
		out[i] = entry{
			Timestamp:  b.Timestamp / 1000,
			NewWords:   b.NewWords,
			TotalWords: b.TotalWords,
		}
	}
	writeJSON(w, out)
}

// handleAPISearch performs a paged FTS search with optional time/app/domain
// filters. The query is sanitised so it never causes a SQL error; an empty
// query returns {total:0, results:[]}. since/until are unix seconds (optional,
// 0 = ignore) and filter on typed_at. Results keep typed_at in milliseconds.
// GET /api/search?q=&since=&until=&app=&domain=&limit=&offset=
func (s *Server) handleAPISearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, storage.SearchPage{Total: 0, Results: []storage.SearchResult{}})
		return
	}

	since := parseUnixParam(r, "since", 0)
	until := parseUnixParam(r, "until", 0)
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	opts := storage.SearchOpts{
		Query:  q,
		App:    r.URL.Query().Get("app"),
		Domain: r.URL.Query().Get("domain"),
		Limit:  limit,
		Offset: offset,
	}
	if since > 0 {
		opts.Since = since * 1000
	}
	if until > 0 {
		opts.Until = until * 1000
	}

	page, err := s.db.SearchPaged(opts)
	if err != nil {
		s.logger.Error("api search", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, page)
}
