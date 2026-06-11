package policy

import "sync"

type Flow struct {
	SrcUserID  int64
	ObjectType string
	ObjectID   int64
	Port       int
	Protocol   string

	Close func()
}
type FlowTable struct {
	mu    sync.Mutex
	next  uint64
	flows map[uint64]Flow
}

func NewFlowTable() *FlowTable {
	return &FlowTable{flows: map[uint64]Flow{}}
}
func (t *FlowTable) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.flows)
}

type FlowSnapshot struct {
	ID         uint64
	SrcUserID  int64
	ObjectType string
	ObjectID   int64
	Port       int
	Protocol   string
}

func (t *FlowTable) Snapshot() []FlowSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]FlowSnapshot, 0, len(t.flows))
	for id, f := range t.flows {
		out = append(out, FlowSnapshot{
			ID:         id,
			SrcUserID:  f.SrcUserID,
			ObjectType: f.ObjectType,
			ObjectID:   f.ObjectID,
			Port:       f.Port,
			Protocol:   f.Protocol,
		})
	}
	return out
}

func (t *FlowTable) Add(f Flow) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	id := t.next
	t.flows[id] = f
	return id
}
func (t *FlowTable) Remove(id uint64) {
	t.mu.Lock()
	delete(t.flows, id)
	t.mu.Unlock()
}

func (t *FlowTable) Reap(idx *Index) {
	t.mu.Lock()
	toClose := make([]Flow, 0)
	toDelete := make([]uint64, 0)
	for id, f := range t.flows {
		if f.allowed(idx) {
			continue
		}
		toClose = append(toClose, f)
		toDelete = append(toDelete, id)
	}
	for _, id := range toDelete {
		delete(t.flows, id)
	}
	t.mu.Unlock()

	for _, f := range toClose {
		if f.Close != nil {
			f.Close()
		}
	}
}
func (f Flow) allowed(idx *Index) bool {
	user, ok := idx.UserByID[f.SrcUserID]
	if !ok {
		return false
	}
	switch f.ObjectType {
	case "service":
		svc, ok := idx.ServiceByID[f.ObjectID]
		if !ok {
			return false
		}
		d, _, _ := idx.CheckService(user.IPAddress, svc.IPAddress, f.Port, f.Protocol)
		return d == Allow
	case "user":
		dst, ok := idx.UserByID[f.ObjectID]
		if !ok {
			return false
		}
		d, _, _ := idx.CheckPeer(user.IPAddress, dst.IPAddress)
		return d == Allow
	}
	return false
}
