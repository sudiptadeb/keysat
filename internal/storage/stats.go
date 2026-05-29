package storage

import "time"

// TimeBucket holds aggregated counts for a time interval.
type TimeBucket struct {
	Timestamp      int64 `json:"timestamp"`
	KeystrokeCount int   `json:"keystroke_count"`
	WordCount      int   `json:"word_count"`
}

// AppStat holds per-application aggregated stats.
type AppStat struct {
	AppName        string `json:"name"`
	AppType        string `json:"type"`
	WordCount      int    `json:"words"`
	KeystrokeCount int    `json:"keystrokes"`
}

// DomainStat holds per-domain aggregated stats.
type DomainStat struct {
	Domain    string `json:"domain"`
	WordCount int    `json:"words"`
}

// DirStat holds per-directory aggregated stats.
type DirStat struct {
	Path      string `json:"path"`
	WordCount int    `json:"words"`
}

// VocabBucket holds vocabulary growth data for a time interval.
type VocabBucket struct {
	Timestamp  int64 `json:"timestamp"`
	NewWords   int   `json:"new_words"`
	TotalWords int   `json:"total_words"`
}

// DaySummary holds a summary of today's typing activity.
type DaySummary struct {
	TotalKeystrokes int
	TotalWords      int
	UniqueWords     int
	TopApp          string
	TopDomain       string
	ActiveMinutes   int
}

// GetTypingVolume returns keystroke and word counts grouped into time buckets.
// bucketSeconds defines the width of each bucket (e.g. 3600 for hourly).
// since/until are unix milliseconds. The result is ZERO-FILLED: every bucket
// (aligned to bucketMs) in [since, until) is present, with zero counts where
// no sessions fall in that bucket.
func (db *DB) GetTypingVolume(since, until int64, bucketSeconds int64) ([]TimeBucket, error) {
	bucketMs := bucketSeconds * 1000
	rows, err := db.conn.Query(`
		SELECT (started_at / ? * ?) AS bucket_ts,
		       SUM(keystroke_count) AS key_count,
		       SUM(word_count) AS w_count
		FROM sessions
		WHERE started_at >= ? AND started_at < ?
		GROUP BY bucket_ts
		ORDER BY bucket_ts
	`, bucketMs, bucketMs, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type counts struct {
		keystrokes int
		words      int
	}
	found := make(map[int64]counts)
	for rows.Next() {
		var ts int64
		var c counts
		if err := rows.Scan(&ts, &c.keystrokes, &c.words); err != nil {
			return nil, err
		}
		found[ts] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if bucketMs <= 0 {
		return nil, nil
	}

	// Zero-fill every aligned bucket in [since, until).
	start := (since / bucketMs) * bucketMs
	var buckets []TimeBucket
	for ts := start; ts < until; ts += bucketMs {
		c := found[ts]
		buckets = append(buckets, TimeBucket{
			Timestamp:      ts,
			KeystrokeCount: c.keystrokes,
			WordCount:      c.words,
		})
	}
	return buckets, nil
}

// GetAppStats returns per-app word and keystroke counts aggregated from
// sessions in [since, until). since/until are unix milliseconds.
func (db *DB) GetAppStats(since, until int64, limit int) ([]AppStat, error) {
	rows, err := db.conn.Query(`
		SELECT a.name, a.app_type,
		       SUM(s.word_count) AS word_count,
		       SUM(s.keystroke_count) AS keystroke_count
		FROM sessions s
		JOIN apps a ON a.id = s.app_id
		WHERE s.started_at >= ? AND s.started_at < ?
		GROUP BY s.app_id
		ORDER BY word_count DESC
		LIMIT ?
	`, since, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []AppStat
	for rows.Next() {
		var s AppStat
		if err := rows.Scan(&s.AppName, &s.AppType, &s.WordCount, &s.KeystrokeCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// GetDomainStats returns per-domain word counts from sessions in [since, until).
// since/until are unix milliseconds.
func (db *DB) GetDomainStats(since, until int64, limit int) ([]DomainStat, error) {
	rows, err := db.conn.Query(`
		SELECT d.domain, SUM(s.word_count) AS word_count
		FROM sessions s
		JOIN domains d ON d.id = s.domain_id
		WHERE s.started_at >= ? AND s.started_at < ? AND s.domain_id IS NOT NULL
		GROUP BY s.domain_id
		HAVING word_count > 0
		ORDER BY word_count DESC
		LIMIT ?
	`, since, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DomainStat
	for rows.Next() {
		var s DomainStat
		if err := rows.Scan(&s.Domain, &s.WordCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// GetDirectoryStats returns per-directory word counts from sessions in
// [since, until). since/until are unix milliseconds.
func (db *DB) GetDirectoryStats(since, until int64, limit int) ([]DirStat, error) {
	rows, err := db.conn.Query(`
		SELECT dir.path, SUM(s.word_count) AS word_count
		FROM sessions s
		JOIN directories dir ON dir.id = s.directory_id
		WHERE s.started_at >= ? AND s.started_at < ? AND s.directory_id IS NOT NULL
		GROUP BY s.directory_id
		HAVING word_count > 0
		ORDER BY word_count DESC
		LIMIT ?
	`, since, until, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DirStat
	for rows.Next() {
		var s DirStat
		if err := rows.Scan(&s.Path, &s.WordCount); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// GetVocabGrowth returns vocabulary growth over time, tracking new and
// cumulative unique words in each time bucket. since/until are unix
// milliseconds; only words first seen in [since, until) are counted.
func (db *DB) GetVocabGrowth(since, until int64, bucketSeconds int64) ([]VocabBucket, error) {
	bucketMs := bucketSeconds * 1000
	rows, err := db.conn.Query(`
		SELECT (first_seen / ? * ?) AS bucket_ts,
		       COUNT(*) AS new_words
		FROM (
			SELECT word, MIN(typed_at) AS first_seen
			FROM words
			WHERE is_hashed = 0
			GROUP BY word
		)
		WHERE first_seen >= ? AND first_seen < ?
		GROUP BY bucket_ts
		ORDER BY bucket_ts
	`, bucketMs, bucketMs, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []VocabBucket
	total := 0
	for rows.Next() {
		var b VocabBucket
		if err := rows.Scan(&b.Timestamp, &b.NewWords); err != nil {
			return nil, err
		}
		total += b.NewWords
		b.TotalWords = total
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

// GetSummary returns a summary of typing activity over [since, until).
// since/until are unix milliseconds.
func (db *DB) GetSummary(since, until int64) (*DaySummary, error) {
	var ds DaySummary

	// Total keystrokes and words from sessions.
	err := db.conn.QueryRow(`
		SELECT COALESCE(SUM(keystroke_count), 0),
		       COALESCE(SUM(word_count), 0)
		FROM sessions
		WHERE started_at >= ? AND started_at < ?
	`, since, until).Scan(&ds.TotalKeystrokes, &ds.TotalWords)
	if err != nil {
		return nil, err
	}

	// Unique words.
	err = db.conn.QueryRow(`
		SELECT COUNT(DISTINCT word)
		FROM words
		WHERE typed_at >= ? AND typed_at < ? AND is_hashed = 0
	`, since, until).Scan(&ds.UniqueWords)
	if err != nil {
		return nil, err
	}

	// Top app by keystroke count.
	err = db.conn.QueryRow(`
		SELECT COALESCE(a.name, '')
		FROM sessions s
		JOIN apps a ON a.id = s.app_id
		WHERE s.started_at >= ? AND s.started_at < ?
		GROUP BY s.app_id
		ORDER BY SUM(s.keystroke_count) DESC
		LIMIT 1
	`, since, until).Scan(&ds.TopApp)
	if err != nil {
		ds.TopApp = ""
	}

	// Top domain by word count.
	err = db.conn.QueryRow(`
		SELECT COALESCE(d.domain, '')
		FROM sessions s
		JOIN domains d ON d.id = s.domain_id
		WHERE s.started_at >= ? AND s.started_at < ? AND s.domain_id IS NOT NULL
		GROUP BY s.domain_id
		ORDER BY SUM(s.word_count) DESC
		LIMIT 1
	`, since, until).Scan(&ds.TopDomain)
	if err != nil {
		ds.TopDomain = ""
	}

	// Active minutes: count distinct minutes that had word entries.
	err = db.conn.QueryRow(`
		SELECT COUNT(DISTINCT typed_at / 60000)
		FROM words
		WHERE typed_at >= ? AND typed_at < ?
	`, since, until).Scan(&ds.ActiveMinutes)
	if err != nil {
		ds.ActiveMinutes = 0
	}

	return &ds, nil
}

// GetTodayStats returns a summary of today's typing activity, computed over
// the window [local midnight, now). It is a thin wrapper over GetSummary that
// preserves the original dashboard numbers.
func (db *DB) GetTodayStats() (*DaySummary, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return db.GetSummary(startOfDay.UnixMilli(), now.UnixMilli())
}

// MaterializeHourlyStats aggregates word and session data for the given hour
// (hourTs is a unix millisecond timestamp at the start of the hour) and
// upserts into the hourly_stats table.
func (db *DB) MaterializeHourlyStats(hourTs int64) error {
	nextHour := hourTs + 3600*1000

	_, err := db.conn.Exec(`
		INSERT OR REPLACE INTO hourly_stats (hour_ts, app_id, domain_id, directory_id,
		                                     keystroke_count, word_count, unique_words)
		SELECT ? AS hour_ts,
		       s.app_id,
		       s.domain_id,
		       s.directory_id,
		       COALESCE(SUM(s.keystroke_count), 0),
		       COALESCE(SUM(s.word_count), 0),
		       (SELECT COUNT(DISTINCT w.word)
		        FROM words w
		        WHERE w.app_id = s.app_id
		          AND COALESCE(w.domain_id, 0) = COALESCE(s.domain_id, 0)
		          AND COALESCE(w.directory_id, 0) = COALESCE(s.directory_id, 0)
		          AND w.typed_at >= ? AND w.typed_at < ?
		          AND w.is_hashed = 0)
		FROM sessions s
		WHERE s.started_at >= ? AND s.started_at < ?
		GROUP BY s.app_id, s.domain_id, s.directory_id
	`, hourTs, hourTs, nextHour, hourTs, nextHour)
	return err
}
