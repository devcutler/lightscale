package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Service struct {
	ID          int64
	Name        string
	Hostname    string
	Origin      string
	IPAddress   string
	Description string
	Ports       []ServicePort
	CreatedAt   string
	UpdatedAt   string
}
type ServicePort struct {
	ID        int64
	ServiceID int64
	Port      int
	Protocol  string
}
type CreateServiceInput struct {
	Name        string
	Hostname    string
	Origin      string
	IPAddress   string
	Description string
	Ports       []ServicePort
}

func (s *Store) CreateService(ctx context.Context, in CreateServiceInput) (Service, error) {
	if in.Name == "" || in.Hostname == "" || in.Origin == "" || in.IPAddress == "" {
		return Service{}, ErrInvalidInput
	}
	now := nowTimestamp()
	var out Service
	err := s.withTx(ctx, ChangeServices, func(tx *sql.Tx) error {
		if err := checkNameAvailable(tx, NamespaceObject, in.Name, "", 0); err != nil {
			return err
		}
		if err := checkIPAvailable(tx, in.IPAddress, "", 0); err != nil {
			return err
		}
		if err := checkHostnameAvailable(tx, in.Hostname, 0); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO services (name, hostname, origin, ip_address, description, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			in.Name, in.Hostname, in.Origin, in.IPAddress, nullableString(in.Description), now, now)
		if err != nil {
			return fmt.Errorf("store: insert service: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		ports, err := replaceServicePorts(ctx, tx, id, in.Ports)
		if err != nil {
			return err
		}
		out = Service{
			ID: id, Name: in.Name, Hostname: in.Hostname, Origin: in.Origin,
			IPAddress: in.IPAddress, Description: in.Description, Ports: ports,
			CreatedAt: now, UpdatedAt: now,
		}
		return nil
	})
	return out, err
}

type UpdateServiceInput struct {
	Name         *string
	Hostname     *string
	Origin       *string
	IPAddress    *string
	Description  *string
	Ports        []ServicePort
	ReplacePorts bool
}

func (s *Store) UpdateService(ctx context.Context, id int64, in UpdateServiceInput) (Service, error) {
	var out Service
	err := s.withTx(ctx, ChangeServices, func(tx *sql.Tx) error {
		current, err := serviceByID(tx, id)
		if err != nil {
			return err
		}
		if in.Name != nil && *in.Name != current.Name {
			if err := checkNameAvailable(tx, NamespaceObject, *in.Name, "services", id); err != nil {
				return err
			}
			current.Name = *in.Name
		}
		if in.Hostname != nil && *in.Hostname != current.Hostname {
			if err := checkHostnameAvailable(tx, *in.Hostname, id); err != nil {
				return err
			}
			current.Hostname = *in.Hostname
		}
		if in.Origin != nil {
			current.Origin = *in.Origin
		}
		if in.IPAddress != nil && *in.IPAddress != current.IPAddress {
			if err := checkIPAvailable(tx, *in.IPAddress, "services", id); err != nil {
				return err
			}
			current.IPAddress = *in.IPAddress
		}
		if in.Description != nil {
			current.Description = *in.Description
		}
		current.UpdatedAt = nowTimestamp()
		_, err = tx.ExecContext(ctx,
			`UPDATE services SET name=?, hostname=?, origin=?, ip_address=?, description=?, updated_at=? WHERE id=?`,
			current.Name, current.Hostname, current.Origin, current.IPAddress,
			nullableString(current.Description), current.UpdatedAt, id)
		if err != nil {
			return fmt.Errorf("store: update service: %w", err)
		}
		if in.ReplacePorts {
			ports, err := replaceServicePorts(ctx, tx, id, in.Ports)
			if err != nil {
				return err
			}
			current.Ports = ports
		} else {
			ports, err := loadServicePorts(ctx, tx, id)
			if err != nil {
				return err
			}
			current.Ports = ports
		}
		out = current
		return nil
	})
	return out, err
}

func (s *Store) DeleteService(ctx context.Context, id int64) error {
	return s.withTx(ctx, ChangeServices, func(tx *sql.Tx) error {
		if _, err := serviceByID(tx, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM policy_rules WHERE object_type='service' AND object_id=?`, id); err != nil {
			return fmt.Errorf("store: scrub service policies: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM services WHERE id=?`, id); err != nil {
			return fmt.Errorf("store: delete service: %w", err)
		}
		return nil
	})
}

func (s *Store) GetService(ctx context.Context, id int64) (Service, error) {
	results, err := s.queryServices(ctx, `WHERE id=?`, id)
	if err != nil {
		return Service{}, err
	}
	if len(results) == 0 {
		return Service{}, ErrNotFound
	}
	return results[0], nil
}

func (s *Store) GetServiceByName(ctx context.Context, name string) (Service, error) {
	results, err := s.queryServices(ctx, `WHERE name=?`, name)
	if err != nil {
		return Service{}, err
	}
	if len(results) == 0 {
		return Service{}, ErrNotFound
	}
	return results[0], nil
}

func (s *Store) ListServices(ctx context.Context) ([]Service, error) {
	return s.queryServices(ctx, `ORDER BY id`)
}
func (s *Store) TakenServiceIPs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT ip_address FROM services`)
	if err != nil {
		return nil, fmt.Errorf("store: list service IPs: %w", err)
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

const serviceSelect = `SELECT id, name, hostname, origin, ip_address, COALESCE(description,''),
                              created_at, updated_at FROM services`

func (s *Store) queryServices(ctx context.Context, tail string, args ...any) ([]Service, error) {
	rows, err := s.db.QueryContext(ctx, serviceSelect+" "+tail, args...)
	if err != nil {
		return nil, fmt.Errorf("store: list services: %w", err)
	}
	defer rows.Close()
	var out []Service
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		ports, err := loadServicePortsDB(ctx, s.db, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Ports = ports
	}
	return out, nil
}

func scanService(r rowScanner) (Service, error) {
	var svc Service
	err := r.Scan(&svc.ID, &svc.Name, &svc.Hostname, &svc.Origin, &svc.IPAddress,
		&svc.Description, &svc.CreatedAt, &svc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Service{}, ErrNotFound
	}
	if err != nil {
		return Service{}, fmt.Errorf("store: scan service: %w", err)
	}
	return svc, nil
}

func serviceByID(tx *sql.Tx, id int64) (Service, error) {
	row := tx.QueryRow(serviceSelect+` WHERE id=?`, id)
	svc, err := scanService(row)
	if err != nil {
		return Service{}, err
	}
	ports, err := loadServicePorts(context.Background(), tx, id)
	if err != nil {
		return Service{}, err
	}
	svc.Ports = ports
	return svc, nil
}

func loadServicePorts(ctx context.Context, tx *sql.Tx, serviceID int64) ([]ServicePort, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, service_id, port, protocol FROM service_ports WHERE service_id=? ORDER BY port, protocol`,
		serviceID)
	if err != nil {
		return nil, fmt.Errorf("store: load service ports: %w", err)
	}
	defer rows.Close()
	return scanPorts(rows)
}

func loadServicePortsDB(ctx context.Context, db *sql.DB, serviceID int64) ([]ServicePort, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, service_id, port, protocol FROM service_ports WHERE service_id=? ORDER BY port, protocol`,
		serviceID)
	if err != nil {
		return nil, fmt.Errorf("store: load service ports: %w", err)
	}
	defer rows.Close()
	return scanPorts(rows)
}

func scanPorts(rows *sql.Rows) ([]ServicePort, error) {
	var out []ServicePort
	for rows.Next() {
		var p ServicePort
		if err := rows.Scan(&p.ID, &p.ServiceID, &p.Port, &p.Protocol); err != nil {
			return nil, fmt.Errorf("store: scan service port: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func replaceServicePorts(ctx context.Context, tx *sql.Tx, serviceID int64, ports []ServicePort) ([]ServicePort, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM service_ports WHERE service_id=?`, serviceID); err != nil {
		return nil, fmt.Errorf("store: clear service ports: %w", err)
	}
	out := make([]ServicePort, 0, len(ports))
	for _, p := range ports {
		if p.Protocol != "tcp" && p.Protocol != "udp" {
			return nil, fmt.Errorf("%w: bad protocol %q", ErrInvalidInput, p.Protocol)
		}
		if p.Port <= 0 || p.Port > 65535 {
			return nil, fmt.Errorf("%w: bad port %d", ErrInvalidInput, p.Port)
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO service_ports (service_id, port, protocol) VALUES (?, ?, ?)`,
			serviceID, p.Port, p.Protocol)
		if err != nil {
			return nil, fmt.Errorf("store: insert service port: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		out = append(out, ServicePort{ID: id, ServiceID: serviceID, Port: p.Port, Protocol: p.Protocol})
	}
	return out, nil
}

func checkHostnameAvailable(tx *sql.Tx, hostname string, excludeID int64) error {
	var id int64
	err := tx.QueryRow(`SELECT id FROM services WHERE hostname=?`, hostname).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: hostname check: %w", err)
	}
	if id == excludeID {
		return nil
	}
	return fmt.Errorf("%w: hostname %q already in use", ErrNameTaken, hostname)
}
