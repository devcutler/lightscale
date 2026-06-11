package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type User struct {
	ID           int64
	Name         string
	Email        string
	PublicKey    string
	PrivateKey   string
	PresharedKey string
	IPAddress    string
	Endpoint     string
	CreatedAt    string
	UpdatedAt    string
}

type CreateUserInput struct {
	Name         string
	Email        string
	PublicKey    string
	PrivateKey   string
	PresharedKey string
	IPAddress    string
	Endpoint     string
}

func (s *Store) CreateUser(ctx context.Context, in CreateUserInput) (User, error) {
	if in.Name == "" || in.PublicKey == "" || in.PrivateKey == "" || in.PresharedKey == "" || in.IPAddress == "" {
		return User{}, ErrInvalidInput
	}
	now := nowTimestamp()
	var out User
	err := s.withTx(ctx, ChangeUsers, func(tx *sql.Tx) error {
		if err := checkNameAvailable(tx, NamespacePrincipal, in.Name, "", 0); err != nil {
			return err
		}
		if err := checkIPAvailable(tx, in.IPAddress, "", 0); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO users
			   (name, email, public_key, private_key, preshared_key, ip_address, endpoint, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.Name, nullableString(in.Email), in.PublicKey, in.PrivateKey, in.PresharedKey,
			in.IPAddress, nullableString(in.Endpoint), now, now,
		)
		if err != nil {
			return fmt.Errorf("store: insert user: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("store: last insert id: %w", err)
		}
		out = User{
			ID: id, Name: in.Name, Email: in.Email,
			PublicKey: in.PublicKey, PrivateKey: in.PrivateKey, PresharedKey: in.PresharedKey,
			IPAddress: in.IPAddress, Endpoint: in.Endpoint,
			CreatedAt: now, UpdatedAt: now,
		}
		return nil
	})
	return out, err
}

type UpdateUserInput struct {
	Name     *string
	Email    *string
	Endpoint *string
}

func (s *Store) UpdateUser(ctx context.Context, id int64, in UpdateUserInput) (User, error) {
	var out User
	err := s.withTx(ctx, ChangeUsers, func(tx *sql.Tx) error {
		current, err := userByID(tx, id)
		if err != nil {
			return err
		}
		if in.Name != nil && *in.Name != current.Name {
			if err := checkNameAvailable(tx, NamespacePrincipal, *in.Name, "users", id); err != nil {
				return err
			}
			current.Name = *in.Name
		}
		if in.Email != nil {
			current.Email = *in.Email
		}
		if in.Endpoint != nil {
			current.Endpoint = *in.Endpoint
		}
		current.UpdatedAt = nowTimestamp()
		_, err = tx.ExecContext(ctx,
			`UPDATE users SET name=?, email=?, endpoint=?, updated_at=? WHERE id=?`,
			current.Name, nullableString(current.Email), nullableString(current.Endpoint),
			current.UpdatedAt, id,
		)
		if err != nil {
			return fmt.Errorf("store: update user: %w", err)
		}
		out = current
		return nil
	})
	return out, err
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	return s.withTx(ctx, ChangeUsers, func(tx *sql.Tx) error {
		if _, err := userByID(tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM policy_rules WHERE
			    (subject_type='user' AND subject_id=?)
			 OR (object_type='user' AND object_id=?)`, id, id); err != nil {
			return fmt.Errorf("store: scrub user policies: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id); err != nil {
			return fmt.Errorf("store: delete user: %w", err)
		}
		return nil
	})
}
func (s *Store) GetUser(ctx context.Context, id int64) (User, error) {
	row := s.db.QueryRowContext(ctx, userSelect+` WHERE id=?`, id)
	return scanUser(row)
}
func (s *Store) GetUserByName(ctx context.Context, name string) (User, error) {
	row := s.db.QueryRowContext(ctx, userSelect+` WHERE name=?`, name)
	return scanUser(row)
}
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, userSelect+` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) TakenUserIPs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ip_address FROM users`)
	if err != nil {
		return nil, fmt.Errorf("store: list user IPs: %w", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return nil, err
		}
		out[ip] = struct{}{}
	}
	return out, rows.Err()
}

const userSelect = `SELECT id, name, COALESCE(email,''), public_key, private_key, preshared_key,
                          ip_address, COALESCE(endpoint,''), created_at, updated_at
                   FROM users`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(r rowScanner) (User, error) {
	var u User
	err := r.Scan(&u.ID, &u.Name, &u.Email, &u.PublicKey, &u.PrivateKey, &u.PresharedKey,
		&u.IPAddress, &u.Endpoint, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("store: scan user: %w", err)
	}
	return u, nil
}

func userByID(tx *sql.Tx, id int64) (User, error) {
	row := tx.QueryRow(userSelect+` WHERE id=?`, id)
	return scanUser(row)
}
func checkIPAvailable(tx *sql.Tx, ip string, excludeTable string, excludeID int64) error {
	for _, t := range []string{"users", "services"} {
		var id int64
		err := tx.QueryRow(fmt.Sprintf("SELECT id FROM %s WHERE ip_address=?", t), ip).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("store: ip check on %s: %w", t, err)
		}
		if t == excludeTable && id == excludeID {
			continue
		}
		return ErrIPInUse
	}
	return nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
