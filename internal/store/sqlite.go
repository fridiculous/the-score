package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fridiculous/the-score/internal/model"
	_ "modernc.org/sqlite"
)

func NewSQLite(path string) (*Store, error) {
	if path == "" {
		var err error
		path, err = DefaultSQLitePath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		_ = db.Close()
		return nil, err
	}
	st := New()
	st.storagePath = path
	st.db = db
	if err := st.initSQLite(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := st.loadSQLite(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return nil
	}
	return db.Close()
}

func (s *Store) initSQLite() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	payload TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS workspaces (
	id TEXT PRIMARY KEY,
	payload TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS edges (
	id TEXT PRIMARY KEY,
	payload TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sources (
	id TEXT PRIMARY KEY,
	payload TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS events (
	seq INTEGER PRIMARY KEY,
	payload TEXT NOT NULL,
	observed_at TEXT NOT NULL
);
`)
	return err
}

func (s *Store) loadSQLite() error {
	if err := loadResourceTable(s.db, "sessions", func(session model.Session) {
		s.sessions[session.ID] = session
	}); err != nil {
		return err
	}
	if err := loadResourceTable(s.db, "workspaces", func(workspace model.Workspace) {
		s.workspaces[workspace.ID] = workspace
	}); err != nil {
		return err
	}
	if err := loadResourceTable(s.db, "edges", func(edge model.Edge) {
		s.edges[edge.ID] = edge
	}); err != nil {
		return err
	}
	if err := loadResourceTable(s.db, "sources", func(source model.Source) {
		s.sources[source.ID] = source
	}); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT payload FROM events ORDER BY seq ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return err
		}
		var event model.Event
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return err
		}
		s.events = append(s.events, event)
		if event.Seq > s.nextSeq {
			s.nextSeq = event.Seq
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(s.events) > s.maxEvents {
		s.events = s.events[len(s.events)-s.maxEvents:]
	}
	return nil
}

func loadResourceTable[T any](db *sql.DB, table string, put func(T)) error {
	rows, err := db.Query(fmt.Sprintf(`SELECT payload FROM %s`, table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return err
		}
		var value T
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			return err
		}
		put(value)
	}
	return rows.Err()
}

func (s *Store) persistResource(table, id string, value interface{}) {
	db := s.sqliteDB()
	if db == nil || id == "" {
		return
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return
	}
	_, _ = db.Exec(
		fmt.Sprintf(`INSERT INTO %s (id, payload, updated_at) VALUES (?, ?, ?) ON CONFLICT(id) DO UPDATE SET payload = excluded.payload, updated_at = excluded.updated_at`, table),
		id,
		string(payload),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
}

func (s *Store) deleteResource(table, id string) {
	db := s.sqliteDB()
	if db == nil || id == "" {
		return
	}
	_, _ = db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, table), id)
}

func (s *Store) persistEvent(event model.Event) {
	db := s.sqliteDB()
	if db == nil || event.Seq == 0 {
		return
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = db.Exec(
		`INSERT INTO events (seq, payload, observed_at) VALUES (?, ?, ?) ON CONFLICT(seq) DO UPDATE SET payload = excluded.payload, observed_at = excluded.observed_at`,
		event.Seq,
		string(payload),
		event.ObservedAt.Format(time.RFC3339Nano),
	)
	if s.maxEvents > 0 {
		_, _ = db.Exec(`DELETE FROM events WHERE seq <= ?`, event.Seq-int64(s.maxEvents))
	}
}

func (s *Store) sqliteDB() *sql.DB {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db
}
