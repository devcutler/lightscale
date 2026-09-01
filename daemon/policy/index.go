package policy

import (
	"context"
	"sync/atomic"

	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/origin"
)

type Index struct {
	PeerByIP        map[string]UserSnapshot
	UserByID        map[int64]UserSnapshot
	ServiceByVIP    map[string]ServiceSnapshot
	ServiceByID     map[int64]ServiceSnapshot
	GroupsByUser    map[int64][]int64
	GroupsByService map[int64][]int64
	UserGroups      map[int64]UserGroupSnapshot

	Rules []RuleSnapshot
}
type UserSnapshot struct {
	ID           int64
	Name         string
	IPAddress    string
	PublicKey    string
	PresharedKey string
}
type ServiceSnapshot struct {
	ID        int64
	Name      string
	Hostname  string
	Origin    origin.Spec
	IPAddress string
	Ports     []PortSpec
}
type PortSpec struct {
	Port     int
	Protocol string
}
type UserGroupSnapshot struct {
	ID      int64
	Name    string
	LANMode bool
	Members []int64
}
type RuleSnapshot struct {
	ID          int64
	SubjectType string
	SubjectID   int64
	ObjectType  string
	ObjectID    int64
	Action      string
}
type Holder struct {
	ptr atomic.Pointer[Index]
}

func (h *Holder) Load() *Index   { return h.ptr.Load() }
func (h *Holder) Store(i *Index) { h.ptr.Store(i) }

func Build(ctx context.Context, s *store.Store) (*Index, error) {
	users, err := s.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	userGroups, err := s.ListUserGroups(ctx)
	if err != nil {
		return nil, err
	}
	services, err := s.ListServices(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := s.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	idx := &Index{
		PeerByIP:        make(map[string]UserSnapshot, len(users)),
		UserByID:        make(map[int64]UserSnapshot, len(users)),
		ServiceByVIP:    make(map[string]ServiceSnapshot, len(services)),
		ServiceByID:     make(map[int64]ServiceSnapshot, len(services)),
		GroupsByUser:    make(map[int64][]int64),
		GroupsByService: make(map[int64][]int64),
		UserGroups:      make(map[int64]UserGroupSnapshot, len(userGroups)),
		Rules:           make([]RuleSnapshot, 0, len(rules)),
	}

	for _, u := range users {
		snap := UserSnapshot{
			ID: u.ID, Name: u.Name, IPAddress: u.IPAddress,
			PublicKey: u.PublicKey, PresharedKey: u.PresharedKey,
		}
		idx.PeerByIP[u.IPAddress] = snap
		idx.UserByID[u.ID] = snap
	}

	for _, svc := range services {
		ports := make([]PortSpec, 0, len(svc.Ports))
		for _, p := range svc.Ports {
			ports = append(ports, PortSpec{Port: p.Port, Protocol: p.Protocol})
		}
		snap := ServiceSnapshot{
			ID: svc.ID, Name: svc.Name, Hostname: svc.Hostname,
			Origin: svc.Origin, IPAddress: svc.IPAddress, Ports: ports,
		}
		idx.ServiceByVIP[svc.IPAddress] = snap
		idx.ServiceByID[svc.ID] = snap
	}

	for _, g := range userGroups {
		members, err := s.UserGroupMembers(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(members))
		for _, m := range members {
			ids = append(ids, m.ID)
			idx.GroupsByUser[m.ID] = append(idx.GroupsByUser[m.ID], g.ID)
		}
		idx.UserGroups[g.ID] = UserGroupSnapshot{
			ID: g.ID, Name: g.Name, LANMode: g.LANMode, Members: ids,
		}
	}

	for _, svc := range services {
		gids, err := s.ServiceGroupIDsForService(ctx, svc.ID)
		if err != nil {
			return nil, err
		}
		idx.GroupsByService[svc.ID] = gids
	}

	for _, r := range rules {
		idx.Rules = append(idx.Rules, RuleSnapshot{
			ID: r.ID, SubjectType: r.SubjectType, SubjectID: r.SubjectID,
			ObjectType: r.ObjectType, ObjectID: r.ObjectID, Action: r.Action,
		})
	}

	return idx, nil
}
