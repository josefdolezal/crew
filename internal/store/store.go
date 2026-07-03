// Package store persists the agent registry in SQLite at ~/.crew/crew.db.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/josefdolezal/crew/internal/proto"
)

var ErrNotFound = errors.New("agent not found")

type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS agents (
	name       TEXT PRIMARY KEY,
	runtime    TEXT NOT NULL,
	model      TEXT NOT NULL DEFAULT '',
	cwd        TEXT NOT NULL,
	parent     TEXT NOT NULL,
	task       TEXT NOT NULL DEFAULT '',
	backend    TEXT NOT NULL DEFAULT 'tmux',
	session    TEXT NOT NULL,
	worktree   TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	sender     TEXT NOT NULL,
	recipient  TEXT NOT NULL,
	kind       TEXT NOT NULL DEFAULT 'message', -- message | report | event
	status     TEXT NOT NULL DEFAULT '',        -- reports: done | blocked
	body       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	read_at    INTEGER
);
CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient, read_at);
CREATE INDEX IF NOT EXISTS idx_messages_sender ON messages(sender, kind);
CREATE TABLE IF NOT EXISTS deliveries (
	identity   TEXT PRIMARY KEY,
	session    TEXT NOT NULL,
	created_at INTEGER NOT NULL
);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// modernc/sqlite serializes writes; a single connection avoids lock races.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	// Idempotent migration for DBs created before the worktree column.
	if _, err := db.Exec(`ALTER TABLE agents ADD COLUMN worktree TEXT NOT NULL DEFAULT ''`); err != nil &&
		!strings.Contains(err.Error(), "duplicate column") {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Insert(a proto.Agent) error {
	_, err := s.db.Exec(
		`INSERT INTO agents (name, runtime, model, cwd, parent, task, backend, session, worktree, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Name, a.Runtime, a.Model, a.Cwd, a.Parent, a.Task, a.Backend, a.Session, a.Worktree, a.CreatedAt.Unix(),
	)
	return err
}

func (s *Store) Delete(name string) error {
	res, err := s.db.Exec(`DELETE FROM agents WHERE name = ?`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Get(name string) (proto.Agent, error) {
	row := s.db.QueryRow(`SELECT name, runtime, model, cwd, parent, task, backend, session, worktree, created_at
		FROM agents WHERE name = ?`, name)
	return scanAgent(row)
}

func (s *Store) List() ([]proto.Agent, error) {
	rows, err := s.db.Query(`SELECT name, runtime, model, cwd, parent, task, backend, session, worktree, created_at
		FROM agents ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []proto.Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAgent(row scanner) (proto.Agent, error) {
	var a proto.Agent
	var createdAt int64
	err := row.Scan(&a.Name, &a.Runtime, &a.Model, &a.Cwd, &a.Parent, &a.Task, &a.Backend, &a.Session, &a.Worktree, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	a.CreatedAt = time.Unix(createdAt, 0)
	return a, nil
}
