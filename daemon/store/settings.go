package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Setting struct {
	Key   string
	Value string
}

func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: get setting: %w", err)
	}
	return value, nil
}
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	return s.withTx(ctx, ChangeSettings, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO settings (key, value) VALUES (?, ?)
			 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
		if err != nil {
			return fmt.Errorf("store: set setting: %w", err)
		}
		return nil
	})
}
func (s *Store) ListSettings(ctx context.Context) ([]Setting, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("store: list settings: %w", err)
	}
	defer rows.Close()
	var out []Setting
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.Key, &s.Value); err != nil {
			return nil, fmt.Errorf("store: scan setting: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
