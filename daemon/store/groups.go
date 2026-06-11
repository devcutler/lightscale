package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type UserGroup struct {
	ID        int64
	Name      string
	LANMode   bool
	CreatedAt string
	UpdatedAt string
}
type ServiceGroup struct {
	ID        int64
	Name      string
	CreatedAt string
	UpdatedAt string
}

func (s *Store) CreateUserGroup(ctx context.Context, name string, lanMode bool) (UserGroup, error) {
	if name == "" {
		return UserGroup{}, ErrInvalidInput
	}
	now := nowTimestamp()
	var out UserGroup
	err := s.withTx(ctx, ChangeUserGroups, func(tx *sql.Tx) error {
		if err := checkNameAvailable(tx, NamespacePrincipal, name, "", 0); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO user_groups (name, lan_mode, created_at, updated_at) VALUES (?, ?, ?, ?)`,
			name, boolToInt(lanMode), now, now)
		if err != nil {
			return fmt.Errorf("store: insert user_group: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		out = UserGroup{ID: id, Name: name, LANMode: lanMode, CreatedAt: now, UpdatedAt: now}
		return nil
	})
	return out, err
}

type UpdateUserGroupInput struct {
	Name    *string
	LANMode *bool
}

func (s *Store) UpdateUserGroup(ctx context.Context, id int64, in UpdateUserGroupInput) (UserGroup, error) {
	var out UserGroup
	err := s.withTx(ctx, ChangeUserGroups, func(tx *sql.Tx) error {
		current, err := userGroupByID(tx, id)
		if err != nil {
			return err
		}
		if in.Name != nil && *in.Name != current.Name {
			if err := checkNameAvailable(tx, NamespacePrincipal, *in.Name, "user_groups", id); err != nil {
				return err
			}
			current.Name = *in.Name
		}
		if in.LANMode != nil {
			current.LANMode = *in.LANMode
		}
		current.UpdatedAt = nowTimestamp()
		_, err = tx.ExecContext(ctx,
			`UPDATE user_groups SET name=?, lan_mode=?, updated_at=? WHERE id=?`,
			current.Name, boolToInt(current.LANMode), current.UpdatedAt, id)
		if err != nil {
			return fmt.Errorf("store: update user_group: %w", err)
		}
		out = current
		return nil
	})
	return out, err
}

func (s *Store) DeleteUserGroup(ctx context.Context, id int64) error {
	return s.withTx(ctx, ChangeUserGroups, func(tx *sql.Tx) error {
		if _, err := userGroupByID(tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM policy_rules WHERE
			    (subject_type='user_group' AND subject_id=?)
			 OR (object_type='user_group' AND object_id=?)`, id, id); err != nil {
			return fmt.Errorf("store: scrub user_group policies: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_groups WHERE id=?`, id); err != nil {
			return fmt.Errorf("store: delete user_group: %w", err)
		}
		return nil
	})
}

func (s *Store) ListUserGroups(ctx context.Context) ([]UserGroup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, lan_mode, created_at, updated_at FROM user_groups ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list user_groups: %w", err)
	}
	defer rows.Close()
	var out []UserGroup
	for rows.Next() {
		g, err := scanUserGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) GetUserGroup(ctx context.Context, id int64) (UserGroup, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, lan_mode, created_at, updated_at FROM user_groups WHERE id=?`, id)
	return scanUserGroup(row)
}

func (s *Store) GetUserGroupByName(ctx context.Context, name string) (UserGroup, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, lan_mode, created_at, updated_at FROM user_groups WHERE name=?`, name)
	return scanUserGroup(row)
}
func (s *Store) AddUserToGroup(ctx context.Context, groupID, userID int64) error {
	return s.withTx(ctx, ChangeMembership, func(tx *sql.Tx) error {
		if _, err := userGroupByID(tx, groupID); err != nil {
			return err
		}
		if _, err := userByID(tx, userID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO user_group_members (user_id, group_id) VALUES (?, ?)`,
			userID, groupID)
		return err
	})
}

func (s *Store) RemoveUserFromGroup(ctx context.Context, groupID, userID int64) error {
	return s.withTx(ctx, ChangeMembership, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM user_group_members WHERE user_id=? AND group_id=?`,
			userID, groupID)
		return err
	})
}
func (s *Store) UserGroupMembers(ctx context.Context, groupID int64) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, userSelect+`
		WHERE id IN (SELECT user_id FROM user_group_members WHERE group_id=?)
		ORDER BY id`, groupID)
	if err != nil {
		return nil, fmt.Errorf("store: list user_group members: %w", err)
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
func (s *Store) UserGroupIDsForUser(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_id FROM user_group_members WHERE user_id=?`, userID)
	if err != nil {
		return nil, fmt.Errorf("store: list groups for user: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) CreateServiceGroup(ctx context.Context, name string) (ServiceGroup, error) {
	if name == "" {
		return ServiceGroup{}, ErrInvalidInput
	}
	now := nowTimestamp()
	var out ServiceGroup
	err := s.withTx(ctx, ChangeServiceGroups, func(tx *sql.Tx) error {
		if err := checkNameAvailable(tx, NamespaceObject, name, "", 0); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO service_groups (name, created_at, updated_at) VALUES (?, ?, ?)`,
			name, now, now)
		if err != nil {
			return fmt.Errorf("store: insert service_group: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		out = ServiceGroup{ID: id, Name: name, CreatedAt: now, UpdatedAt: now}
		return nil
	})
	return out, err
}

type UpdateServiceGroupInput struct {
	Name *string
}

func (s *Store) UpdateServiceGroup(ctx context.Context, id int64, in UpdateServiceGroupInput) (ServiceGroup, error) {
	var out ServiceGroup
	err := s.withTx(ctx, ChangeServiceGroups, func(tx *sql.Tx) error {
		current, err := serviceGroupByID(tx, id)
		if err != nil {
			return err
		}
		if in.Name != nil && *in.Name != current.Name {
			if err := checkNameAvailable(tx, NamespaceObject, *in.Name, "service_groups", id); err != nil {
				return err
			}
			current.Name = *in.Name
		}
		current.UpdatedAt = nowTimestamp()
		_, err = tx.ExecContext(ctx,
			`UPDATE service_groups SET name=?, updated_at=? WHERE id=?`,
			current.Name, current.UpdatedAt, id)
		if err != nil {
			return fmt.Errorf("store: update service_group: %w", err)
		}
		out = current
		return nil
	})
	return out, err
}

func (s *Store) DeleteServiceGroup(ctx context.Context, id int64) error {
	return s.withTx(ctx, ChangeServiceGroups, func(tx *sql.Tx) error {
		if _, err := serviceGroupByID(tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM policy_rules WHERE object_type='service_group' AND object_id=?`, id); err != nil {
			return fmt.Errorf("store: scrub service_group policies: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM service_groups WHERE id=?`, id); err != nil {
			return fmt.Errorf("store: delete service_group: %w", err)
		}
		return nil
	})
}

func (s *Store) ListServiceGroups(ctx context.Context) ([]ServiceGroup, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, created_at, updated_at FROM service_groups ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: list service_groups: %w", err)
	}
	defer rows.Close()
	var out []ServiceGroup
	for rows.Next() {
		g, err := scanServiceGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) GetServiceGroup(ctx context.Context, id int64) (ServiceGroup, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at, updated_at FROM service_groups WHERE id=?`, id)
	return scanServiceGroup(row)
}

func (s *Store) GetServiceGroupByName(ctx context.Context, name string) (ServiceGroup, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_at, updated_at FROM service_groups WHERE name=?`, name)
	return scanServiceGroup(row)
}

func (s *Store) AddServiceToGroup(ctx context.Context, groupID, serviceID int64) error {
	return s.withTx(ctx, ChangeMembership, func(tx *sql.Tx) error {
		if _, err := serviceGroupByID(tx, groupID); err != nil {
			return err
		}
		if _, err := serviceByID(tx, serviceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO service_group_members (service_id, group_id) VALUES (?, ?)`,
			serviceID, groupID)
		return err
	})
}

func (s *Store) RemoveServiceFromGroup(ctx context.Context, groupID, serviceID int64) error {
	return s.withTx(ctx, ChangeMembership, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM service_group_members WHERE service_id=? AND group_id=?`,
			serviceID, groupID)
		return err
	})
}

func (s *Store) ServiceGroupMembers(ctx context.Context, groupID int64) ([]Service, error) {
	return s.queryServices(ctx,
		`WHERE id IN (SELECT service_id FROM service_group_members WHERE group_id=?)
		 ORDER BY id`, groupID)
}
func (s *Store) ServiceGroupIDsForService(ctx context.Context, serviceID int64) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT group_id FROM service_group_members WHERE service_id=?`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("store: list groups for service: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func scanUserGroup(r rowScanner) (UserGroup, error) {
	var g UserGroup
	var lanMode int
	err := r.Scan(&g.ID, &g.Name, &lanMode, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UserGroup{}, ErrNotFound
	}
	if err != nil {
		return UserGroup{}, fmt.Errorf("store: scan user_group: %w", err)
	}
	g.LANMode = lanMode != 0
	return g, nil
}

func scanServiceGroup(r rowScanner) (ServiceGroup, error) {
	var g ServiceGroup
	err := r.Scan(&g.ID, &g.Name, &g.CreatedAt, &g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ServiceGroup{}, ErrNotFound
	}
	if err != nil {
		return ServiceGroup{}, fmt.Errorf("store: scan service_group: %w", err)
	}
	return g, nil
}

func userGroupByID(tx *sql.Tx, id int64) (UserGroup, error) {
	row := tx.QueryRow(`SELECT id, name, lan_mode, created_at, updated_at FROM user_groups WHERE id=?`, id)
	return scanUserGroup(row)
}

func serviceGroupByID(tx *sql.Tx, id int64) (ServiceGroup, error) {
	row := tx.QueryRow(`SELECT id, name, created_at, updated_at FROM service_groups WHERE id=?`, id)
	return scanServiceGroup(row)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
