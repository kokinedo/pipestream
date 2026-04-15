package store

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/kokinedo/pipestream/pkg/models"
	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for persisting classified events.
type Store struct {
	db *sqlx.DB
}

const createTableSQL = `
CREATE TABLE IF NOT EXISTS classified_events (
	id            TEXT PRIMARY KEY,
	event_type    TEXT NOT NULL,
	actor_login   TEXT NOT NULL,
	repo_name     TEXT NOT NULL,
	category      TEXT NOT NULL,
	interestingness INTEGER NOT NULL DEFAULT 1,
	summary       TEXT NOT NULL DEFAULT '',
	raw_payload   TEXT NOT NULL DEFAULT '',
	created_at    DATETIME NOT NULL,
	classified_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_category ON classified_events(category);
CREATE INDEX IF NOT EXISTS idx_interestingness ON classified_events(interestingness);
CREATE INDEX IF NOT EXISTS idx_created_at ON classified_events(created_at);
`

// NewStore opens (or creates) a SQLite database at dbPath, runs migrations,
// and enables WAL mode for better concurrent read performance.
func NewStore(dbPath string) (*Store, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for better concurrency.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	// Run migrations.
	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// SaveEvent inserts or replaces a single classified event.
func (s *Store) SaveEvent(ctx context.Context, e models.ClassifiedEvent) error {
	const q = `INSERT OR REPLACE INTO classified_events
		(id, event_type, actor_login, repo_name, category, interestingness, summary, raw_payload, created_at, classified_at)
		VALUES (:id, :event_type, :actor_login, :repo_name, :category, :interestingness, :summary, :raw_payload, :created_at, :classified_at)`
	_, err := s.db.NamedExecContext(ctx, q, e)
	return err
}

// SaveEvents inserts a batch of classified events inside a single transaction.
func (s *Store) SaveEvents(ctx context.Context, events []models.ClassifiedEvent) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	const q = `INSERT OR REPLACE INTO classified_events
		(id, event_type, actor_login, repo_name, category, interestingness, summary, raw_payload, created_at, classified_at)
		VALUES (:id, :event_type, :actor_login, :repo_name, :category, :interestingness, :summary, :raw_payload, :created_at, :classified_at)`

	for _, e := range events {
		if _, err := tx.NamedExecContext(ctx, q, e); err != nil {
			return fmt.Errorf("insert event %s: %w", e.ID, err)
		}
	}
	return tx.Commit()
}

// GetEvents returns classified events ordered by classified_at DESC.
func (s *Store) GetEvents(ctx context.Context, limit, offset int) ([]models.ClassifiedEvent, error) {
	var events []models.ClassifiedEvent
	err := s.db.SelectContext(ctx, &events,
		"SELECT * FROM classified_events ORDER BY classified_at DESC LIMIT ? OFFSET ?",
		limit, offset)
	return events, err
}

// GetHighlights returns events with interestingness >= minScore.
func (s *Store) GetHighlights(ctx context.Context, minScore, limit int) ([]models.ClassifiedEvent, error) {
	var events []models.ClassifiedEvent
	err := s.db.SelectContext(ctx, &events,
		"SELECT * FROM classified_events WHERE interestingness >= ? ORDER BY interestingness DESC, classified_at DESC LIMIT ?",
		minScore, limit)
	return events, err
}

// GetStats returns a count per category plus the total event count.
func (s *Store) GetStats(ctx context.Context) (map[models.Category]int, int, error) {
	type row struct {
		Category models.Category `db:"category"`
		Count    int             `db:"cnt"`
	}
	var rows []row
	if err := s.db.SelectContext(ctx, &rows, "SELECT category, COUNT(*) AS cnt FROM classified_events GROUP BY category"); err != nil {
		return nil, 0, err
	}
	m := make(map[models.Category]int, len(rows))
	total := 0
	for _, r := range rows {
		m[r.Category] = r.Count
		total += r.Count
	}
	return m, total, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
