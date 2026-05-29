package storage

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS apps (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    bundle_id TEXT NOT NULL UNIQUE,
    name      TEXT NOT NULL,
    app_type  TEXT NOT NULL DEFAULT 'other'
);

CREATE TABLE IF NOT EXISTS domains (
    id     INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS directories (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS sessions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id          INTEGER NOT NULL REFERENCES apps(id),
    domain_id       INTEGER REFERENCES domains(id),
    directory_id    INTEGER REFERENCES directories(id),
    started_at      INTEGER NOT NULL,
    ended_at        INTEGER NOT NULL,
    keystroke_count INTEGER NOT NULL DEFAULT 0,
    word_count      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS words (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   INTEGER NOT NULL REFERENCES sessions(id),
    word         TEXT NOT NULL,
    is_hashed    INTEGER NOT NULL DEFAULT 0,
    typed_at     INTEGER NOT NULL,
    app_id       INTEGER NOT NULL REFERENCES apps(id),
    domain_id    INTEGER REFERENCES domains(id),
    directory_id INTEGER REFERENCES directories(id)
);

CREATE INDEX IF NOT EXISTS idx_words_typed_at ON words(typed_at);
CREATE INDEX IF NOT EXISTS idx_words_word ON words(word);
CREATE INDEX IF NOT EXISTS idx_words_app ON words(app_id);
CREATE INDEX IF NOT EXISTS idx_sessions_time ON sessions(started_at);

CREATE VIRTUAL TABLE IF NOT EXISTS words_fts USING fts5(word, content=words, content_rowid=id);

CREATE TABLE IF NOT EXISTS hourly_stats (
    hour_ts         INTEGER NOT NULL,
    app_id          INTEGER NOT NULL,
    domain_id       INTEGER NOT NULL DEFAULT 0,
    directory_id    INTEGER NOT NULL DEFAULT 0,
    keystroke_count INTEGER NOT NULL DEFAULT 0,
    word_count      INTEGER NOT NULL DEFAULT 0,
    unique_words    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (hour_ts, app_id, domain_id, directory_id)
);
`
