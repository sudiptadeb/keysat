package storage

// StartSession inserts a new session row and returns its id.
func (db *DB) StartSession(appID int64, domainID, directoryID *int64, startedAt int64) (int64, error) {
	res, err := db.conn.Exec(`
		INSERT INTO sessions (app_id, domain_id, directory_id, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?)
	`, appID, domainID, directoryID, startedAt, startedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// EndSession updates the session with final timestamps and counts.
func (db *DB) EndSession(id int64, endedAt int64, keystrokeCount, wordCount int) error {
	_, err := db.conn.Exec(`
		UPDATE sessions
		SET ended_at = ?, keystroke_count = ?, word_count = ?
		WHERE id = ?
	`, endedAt, keystrokeCount, wordCount, id)
	return err
}

// UpdateSessionCounts updates the running keystroke and word counts for an
// active session so that stats queries see up-to-date numbers.
func (db *DB) UpdateSessionCounts(id int64, keystrokeCount, wordCount int) error {
	_, err := db.conn.Exec(`
		UPDATE sessions
		SET keystroke_count = ?, word_count = ?
		WHERE id = ?
	`, keystrokeCount, wordCount, id)
	return err
}

// GetRecentSessions returns the most recent sessions ordered by start time descending.
func (db *DB) GetRecentSessions(limit int) ([]Session, error) {
	rows, err := db.conn.Query(`
		SELECT id, app_id, domain_id, directory_id, started_at, ended_at,
		       keystroke_count, word_count
		FROM sessions
		ORDER BY started_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.AppID, &s.DomainID, &s.DirectoryID,
			&s.StartedAt, &s.EndedAt, &s.KeystrokeCount, &s.WordCount); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}
