package storage

import (
	"fmt"
	"strings"
)

// WordInsert holds the data needed to insert a word row.
type WordInsert struct {
	Word        string
	IsHashed    bool
	TypedAt     int64
	SessionID   int64
	AppID       int64
	DomainID    *int64
	DirectoryID *int64
}

// WordFreq holds a word and its occurrence count.
type WordFreq struct {
	Word  string
	Count int
}

// InsertWords batch-inserts words within a single transaction.
// Non-hashed words are also inserted into the words_fts table.
func (db *DB) InsertWords(words []WordInsert) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	wordStmt, err := tx.Prepare(`
		INSERT INTO words (session_id, word, is_hashed, typed_at, app_id, domain_id, directory_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare word insert: %w", err)
	}
	defer wordStmt.Close()

	ftsStmt, err := tx.Prepare(`
		INSERT INTO words_fts (rowid, word) VALUES (?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare fts insert: %w", err)
	}
	defer ftsStmt.Close()

	for _, w := range words {
		isHashed := 0
		if w.IsHashed {
			isHashed = 1
		}

		res, err := wordStmt.Exec(w.SessionID, w.Word, isHashed, w.TypedAt, w.AppID, w.DomainID, w.DirectoryID)
		if err != nil {
			return fmt.Errorf("insert word: %w", err)
		}

		if !w.IsHashed {
			id, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("get last insert id: %w", err)
			}
			if _, err := ftsStmt.Exec(id, w.Word); err != nil {
				return fmt.Errorf("insert fts: %w", err)
			}
		}
	}

	return tx.Commit()
}

// GetWordFrequency returns the top N most frequent non-hashed words typed in
// [since, until). since/until are unix milliseconds.
func (db *DB) GetWordFrequency(since, until int64, limit int) ([]WordFreq, error) {
	rows, err := db.conn.Query(`
		SELECT word, COUNT(*) as cnt
		FROM words
		WHERE typed_at >= ? AND typed_at < ? AND is_hashed = 0
		GROUP BY word
		ORDER BY cnt DESC
		LIMIT ?
	`, since, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []WordFreq
	for rows.Next() {
		var wf WordFreq
		if err := rows.Scan(&wf.Word, &wf.Count); err != nil {
			return nil, err
		}
		results = append(results, wf)
	}
	return results, rows.Err()
}

// buildFTSMatch re-tokenises a raw user query into a safe FTS5 MATCH string.
// It splits on whitespace, escapes embedded double-quotes by doubling them,
// wraps each token as a quoted phrase, and appends * for prefix matching.
// Tokens are joined with " OR " (the word column holds a single token, so OR
// is the useful join). An empty/whitespace-only query yields "".
//
// Examples:  hel -> "hel"*  |  foo-bar -> "foo-bar"*  |  a"b -> "a""b"*
func buildFTSMatch(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	tokens := strings.Fields(q)
	quoted := make([]string, 0, len(tokens))
	for _, t := range tokens {
		escaped := strings.ReplaceAll(t, `"`, `""`)
		quoted = append(quoted, `"`+escaped+`"*`)
	}
	return strings.Join(quoted, " OR ")
}

// SearchResult holds a single search hit with app name and domain joined.
type SearchResult struct {
	Word      string `json:"word"`
	AppName   string `json:"app_name"`
	Domain    string `json:"domain"`
	TypedAt   int64  `json:"typed_at"`
	SessionID int64  `json:"session_id"`
}

// SearchOpts holds the parameters for a paged FTS search.
// Since/Until are unix milliseconds; 0 means "ignore" (unbounded on that side).
// App/Domain are optional case-insensitive substring filters.
type SearchOpts struct {
	Query  string
	Since  int64 // ms, 0 = ignore
	Until  int64 // ms, 0 = ignore
	App    string
	Domain string
	Limit  int
	Offset int
}

// SearchPage is the paged search response.
type SearchPage struct {
	Total   int            `json:"total"`
	Results []SearchResult `json:"results"`
}

// SearchPaged performs an FTS5 search with optional time/app/domain filters and
// pagination. It returns the total match count and one page of results ordered
// by typed_at DESC. typed_at is returned in milliseconds.
func (db *DB) SearchPaged(opts SearchOpts) (SearchPage, error) {
	page := SearchPage{Results: []SearchResult{}}

	match := buildFTSMatch(opts.Query)
	if match == "" {
		return page, nil
	}

	// Build the shared WHERE clause and args.
	where := "words_fts MATCH ?"
	args := []any{match}

	if opts.Since > 0 {
		where += " AND w.typed_at >= ?"
		args = append(args, opts.Since)
	}
	if opts.Until > 0 {
		where += " AND w.typed_at < ?"
		args = append(args, opts.Until)
	}
	if opts.App != "" {
		where += " AND a.name LIKE ? COLLATE NOCASE"
		args = append(args, "%"+opts.App+"%")
	}
	if opts.Domain != "" {
		where += " AND COALESCE(d.domain, '') LIKE ? COLLATE NOCASE"
		args = append(args, "%"+opts.Domain+"%")
	}

	fromJoin := `
		FROM words_fts fts
		JOIN words w ON w.id = fts.rowid
		LEFT JOIN apps a ON a.id = w.app_id
		LEFT JOIN domains d ON d.id = w.domain_id
		WHERE ` + where

	// Total count.
	err := db.conn.QueryRow(`SELECT COUNT(*) `+fromJoin, args...).Scan(&page.Total)
	if err != nil {
		return SearchPage{Results: []SearchResult{}}, err
	}

	// Paged results.
	pagedArgs := append(append([]any{}, args...), opts.Limit, opts.Offset)
	rows, err := db.conn.Query(`
		SELECT w.word, COALESCE(a.name, ''), COALESCE(d.domain, ''),
		       w.typed_at, w.session_id
		`+fromJoin+`
		ORDER BY w.typed_at DESC
		LIMIT ? OFFSET ?
	`, pagedArgs...)
	if err != nil {
		return SearchPage{Results: []SearchResult{}}, err
	}
	defer rows.Close()

	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.Word, &r.AppName, &r.Domain, &r.TypedAt, &r.SessionID); err != nil {
			return SearchPage{Results: []SearchResult{}}, err
		}
		page.Results = append(page.Results, r)
	}
	return page, rows.Err()
}
