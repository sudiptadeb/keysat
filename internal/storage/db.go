package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a *sql.DB connection to the keysat SQLite database.
type DB struct {
	conn *sql.DB
}

// DefaultDBPath returns the default database path: ~/.keysat/keysat.db
func DefaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".keysat", "keysat.db")
}

// Open opens (or creates) the SQLite database at dbPath, creates parent
// directories if needed, and runs the schema DDL.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run schema: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// GetOrCreateApp returns the id for the given app, inserting it if it doesn't exist.
func (db *DB) GetOrCreateApp(bundleID, name, appType string) (int64, error) {
	var id int64
	err := db.conn.QueryRow("SELECT id FROM apps WHERE bundle_id = ?", bundleID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := db.conn.Exec(
		"INSERT INTO apps (bundle_id, name, app_type) VALUES (?, ?, ?)",
		bundleID, name, appType,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetOrCreateDomain returns the id for the given domain, inserting it if it
// doesn't exist. Returns nil if domain is empty or not a valid web domain.
func (db *DB) GetOrCreateDomain(domain string) (*int64, error) {
	if domain == "" || !strings.Contains(domain, ".") {
		return nil, nil
	}

	var id int64
	err := db.conn.QueryRow("SELECT id FROM domains WHERE domain = ?", domain).Scan(&id)
	if err == nil {
		return &id, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	res, err := db.conn.Exec("INSERT INTO domains (domain) VALUES (?)", domain)
	if err != nil {
		return nil, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// GetOrCreateDirectory returns the id for the given directory path, inserting it
// if it doesn't exist. Returns nil if path is empty.
func (db *DB) GetOrCreateDirectory(path string) (*int64, error) {
	if path == "" {
		return nil, nil
	}

	var id int64
	err := db.conn.QueryRow("SELECT id FROM directories WHERE path = ?", path).Scan(&id)
	if err == nil {
		return &id, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	res, err := db.conn.Exec("INSERT INTO directories (path) VALUES (?)", path)
	if err != nil {
		return nil, err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &id, nil
}
