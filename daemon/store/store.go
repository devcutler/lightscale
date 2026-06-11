package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	_ "modernc.org/sqlite"

	"github.com/devcutler/lightscale/daemon/store/migrations"
)

var (
	ErrNotFound     = errors.New("store: not found")
	ErrNameTaken    = errors.New("store: name already in use")
	ErrIPInUse      = errors.New("store: ip address already in use")
	ErrPortConflict = errors.New("store: port already declared on service")
	ErrInvalidInput = errors.New("store: invalid input")
)

type ChangeKind string

const (
	ChangeUsers         ChangeKind = "users"
	ChangeUserGroups    ChangeKind = "user_groups"
	ChangeServices      ChangeKind = "services"
	ChangeServiceGroups ChangeKind = "service_groups"
	ChangePolicies      ChangeKind = "policies"
	ChangeMembership    ChangeKind = "membership"
	ChangeSettings      ChangeKind = "settings"
)

type Listener func(kind ChangeKind)
type Store struct {
	db        *sql.DB
	listeners []Listener
}

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Subscribe(fn Listener) {
	s.listeners = append(s.listeners, fn)
}
func (s *Store) notify(kind ChangeKind) {
	for _, fn := range s.listeners {
		fn(kind)
	}
}

func (s *Store) withTx(ctx context.Context, kind ChangeKind, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	if kind != "" {
		s.notify(kind)
	}
	return nil
}

func nowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func runMigrations(db *sql.DB) error {
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  id         TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return fmt.Errorf("store: list migrations: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		var seen int
		if err := db.QueryRowContext(ctx,
			"SELECT count(*) FROM schema_migrations WHERE id = ?", name).Scan(&seen); err != nil {
			return fmt.Errorf("store: check migration %s: %w", name, err)
		}
		if seen > 0 {
			continue
		}

		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("store: begin migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_migrations (id, applied_at) VALUES (?, ?)",
			name, nowTimestamp()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %s: %w", name, err)
		}
	}
	return nil
}
