package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type PolicyRule struct {
	ID          int64
	SubjectType string
	SubjectID   int64
	ObjectType  string
	ObjectID    int64
	Action      string
	CreatedAt   string
	UpdatedAt   string
}

type CreatePolicyInput struct {
	SubjectType string
	SubjectID   int64
	ObjectType  string
	ObjectID    int64
	Action      string
}

func (s *Store) CreatePolicy(ctx context.Context, in CreatePolicyInput) (PolicyRule, error) {
	if !validSubject(in.SubjectType) || !validObject(in.ObjectType) || !validAction(in.Action) {
		return PolicyRule{}, ErrInvalidInput
	}
	now := nowTimestamp()
	var out PolicyRule
	err := s.withTx(ctx, ChangePolicies, func(tx *sql.Tx) error {
		if err := assertExists(tx, in.SubjectType, in.SubjectID); err != nil {
			return err
		}
		if err := assertExists(tx, in.ObjectType, in.ObjectID); err != nil {
			return err
		}

		var existingID int64
		var createdAt string
		err := tx.QueryRow(
			`SELECT id, created_at FROM policy_rules
			  WHERE subject_type=? AND subject_id=? AND object_type=? AND object_id=?`,
			in.SubjectType, in.SubjectID, in.ObjectType, in.ObjectID).Scan(&existingID, &createdAt)
		switch {
		case err == nil:
			if _, err := tx.ExecContext(ctx,
				`UPDATE policy_rules SET action=?, updated_at=? WHERE id=?`,
				in.Action, now, existingID); err != nil {
				return fmt.Errorf("store: update policy: %w", err)
			}
			out = PolicyRule{
				ID: existingID, SubjectType: in.SubjectType, SubjectID: in.SubjectID,
				ObjectType: in.ObjectType, ObjectID: in.ObjectID, Action: in.Action,
				CreatedAt: createdAt, UpdatedAt: now,
			}
			return nil
		case errors.Is(err, sql.ErrNoRows):
		default:
			return fmt.Errorf("store: lookup policy: %w", err)
		}

		res, err := tx.ExecContext(ctx,
			`INSERT INTO policy_rules (subject_type, subject_id, object_type, object_id, action, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			in.SubjectType, in.SubjectID, in.ObjectType, in.ObjectID, in.Action, now, now)
		if err != nil {
			return fmt.Errorf("store: insert policy: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		out = PolicyRule{
			ID: id, SubjectType: in.SubjectType, SubjectID: in.SubjectID,
			ObjectType: in.ObjectType, ObjectID: in.ObjectID, Action: in.Action,
			CreatedAt: now, UpdatedAt: now,
		}
		return nil
	})
	return out, err
}

func (s *Store) DeletePolicy(ctx context.Context, id int64) error {
	return s.withTx(ctx, ChangePolicies, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM policy_rules WHERE id=?`, id)
		if err != nil {
			return fmt.Errorf("store: delete policy: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) ListPolicies(ctx context.Context) ([]PolicyRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, subject_type, subject_id, object_type, object_id, action, created_at, updated_at
		   FROM policy_rules ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list policies: %w", err)
	}
	defer rows.Close()
	var out []PolicyRule
	for rows.Next() {
		var p PolicyRule
		if err := rows.Scan(&p.ID, &p.SubjectType, &p.SubjectID, &p.ObjectType, &p.ObjectID,
			&p.Action, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan policy: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func validSubject(t string) bool { return t == "user" || t == "user_group" }
func validObject(t string) bool {
	return t == "user" || t == "user_group" || t == "service" || t == "service_group"
}
func validAction(a string) bool { return a == "allow" || a == "deny" }

func assertExists(tx *sql.Tx, kind string, id int64) error {
	var table string
	switch kind {
	case "user":
		table = "users"
	case "user_group":
		table = "user_groups"
	case "service":
		table = "services"
	case "service_group":
		table = "service_groups"
	default:
		return fmt.Errorf("%w: bad type %q", ErrInvalidInput, kind)
	}
	var n int64
	err := tx.QueryRow(fmt.Sprintf("SELECT 1 FROM %s WHERE id=?", table), id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s id %d", ErrNotFound, kind, id)
	}
	if err != nil {
		return fmt.Errorf("store: assertExists %s: %w", kind, err)
	}
	return nil
}
