package store

import (
	"database/sql"
	"errors"
	"fmt"
)

type Namespace int

const (
	NamespacePrincipal Namespace = iota
	NamespaceObject
)

func checkNameAvailable(tx *sql.Tx, ns Namespace, name string, excludeTable string, excludeID int64) error {
	tables := namespaceTables(ns)
	for _, table := range tables {
		var id int64
		query := fmt.Sprintf("SELECT id FROM %s WHERE name = ?", table)
		err := tx.QueryRow(query, name).Scan(&id)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue
		case err != nil:
			return fmt.Errorf("store: namespace check on %s: %w", table, err)
		}
		if table == excludeTable && id == excludeID {
			continue
		}
		return ErrNameTaken
	}
	return nil
}

func namespaceTables(ns Namespace) []string {
	switch ns {
	case NamespacePrincipal:
		return []string{"users", "user_groups"}
	case NamespaceObject:
		return []string{"services", "service_groups"}
	}
	return nil
}
