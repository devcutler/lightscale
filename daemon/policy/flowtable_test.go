package policy

import (
	"sync"
	"sync/atomic"
	"testing"
)

func allowIndex() *Index {
	idx := newIndex()
	idx.addUser(1, "10.6.0.2")
	idx.addUser(2, "10.6.0.3")
	idx.addService(1, "10.6.1.5", []PortSpec{{8096, "tcp"}})
	idx.Rules = []RuleSnapshot{
		{ID: 1, SubjectType: "user", SubjectID: 1, ObjectType: "service", ObjectID: 1, Action: "allow"},
		{ID: 2, SubjectType: "user", SubjectID: 1, ObjectType: "user", ObjectID: 2, Action: "allow"},
	}
	return idx
}

func TestFlowTableAddIncreasingIDs(t *testing.T) {
	tbl := NewFlowTable()
	id1 := tbl.Add(Flow{SrcUserID: 1})
	id2 := tbl.Add(Flow{SrcUserID: 1})
	id3 := tbl.Add(Flow{SrcUserID: 1})
	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Fatalf("ids should increase 1,2,3; got %d,%d,%d", id1, id2, id3)
	}
}

func TestFlowTableRemoveAndLen(t *testing.T) {
	tbl := NewFlowTable()
	id1 := tbl.Add(Flow{SrcUserID: 1})
	tbl.Add(Flow{SrcUserID: 2})
	if tbl.Len() != 2 {
		t.Fatalf("len want 2, got %d", tbl.Len())
	}
	tbl.Remove(id1)
	if tbl.Len() != 1 {
		t.Fatalf("len after remove want 1, got %d", tbl.Len())
	}
	tbl.Remove(99999)
	if tbl.Len() != 1 {
		t.Fatalf("len after no-op remove want 1, got %d", tbl.Len())
	}
}

func TestFlowTableSnapshotFieldsAndCopy(t *testing.T) {
	tbl := NewFlowTable()
	id := tbl.Add(Flow{SrcUserID: 7, ObjectType: "service", ObjectID: 3, Port: 443, Protocol: "tcp"})
	snap := tbl.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot len want 1, got %d", len(snap))
	}
	s := snap[0]
	if s.ID != id || s.SrcUserID != 7 || s.ObjectType != "service" || s.ObjectID != 3 || s.Port != 443 || s.Protocol != "tcp" {
		t.Fatalf("snapshot fields mismatch: %+v", s)
	}
	snap[0].Port = 1
	snap = tbl.Snapshot()
	if snap[0].Port != 443 {
		t.Fatalf("snapshot should be a copy; table port changed to %d", snap[0].Port)
	}
}

func TestFlowTableReapClosesOnlyDisallowed(t *testing.T) {
	idx := allowIndex()
	tbl := NewFlowTable()
	var allowedClosed, deniedClosed int32

	tbl.Add(Flow{SrcUserID: 1, ObjectType: "service", ObjectID: 1, Port: 8096, Protocol: "tcp",
		Close: func() { atomic.AddInt32(&allowedClosed, 1) }})
	tbl.Add(Flow{SrcUserID: 1, ObjectType: "service", ObjectID: 1, Port: 22, Protocol: "tcp",
		Close: func() { atomic.AddInt32(&deniedClosed, 1) }})

	tbl.Reap(idx)

	if atomic.LoadInt32(&allowedClosed) != 0 {
		t.Fatalf("allowed flow should NOT be closed")
	}
	if atomic.LoadInt32(&deniedClosed) != 1 {
		t.Fatalf("disallowed flow should be closed exactly once, got %d", deniedClosed)
	}
	if tbl.Len() != 1 {
		t.Fatalf("only allowed flow should remain; len=%d", tbl.Len())
	}
}

func TestFlowTableReapClosesOutsideLock(t *testing.T) {
	idx := allowIndex()
	tbl := NewFlowTable()
	done := make(chan struct{})
	tbl.Add(Flow{SrcUserID: 1, ObjectType: "service", ObjectID: 1, Port: 22, Protocol: "tcp",
		Close: func() {
			tbl.Add(Flow{SrcUserID: 9})
			_ = tbl.Len()
			close(done)
		}})
	tbl.Reap(idx)
	select {
	case <-done:
	default:
		t.Fatal("Close was not invoked (or deadlocked) during Reap")
	}
}

func TestFlowTableConcurrentAddRemoveSnapshotLen(t *testing.T) {
	tbl := NewFlowTable()
	var wg sync.WaitGroup
	const workers = 16
	const iters = 500

	for range workers {
		wg.Go(func() {
			for range iters {
				id := tbl.Add(Flow{SrcUserID: 1, ObjectType: "service", ObjectID: 1, Port: 8096, Protocol: "tcp"})
				_ = tbl.Len()
				_ = tbl.Snapshot()
				tbl.Remove(id)
			}
		})
	}
	wg.Wait()
	if tbl.Len() != 0 {
		t.Fatalf("all flows removed; len=%d", tbl.Len())
	}
}

func TestFlowTableConcurrentReap(t *testing.T) {
	idx := allowIndex()
	tbl := NewFlowTable()
	var reapers, churn sync.WaitGroup
	stop := make(chan struct{})

	for range 4 {
		reapers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					tbl.Reap(idx)
				}
			}
		})
	}
	for range 8 {
		churn.Go(func() {
			for i := range 1000 {
				port := 22
				if i%2 == 0 {
					port = 8096
				}
				id := tbl.Add(Flow{SrcUserID: 1, ObjectType: "service", ObjectID: 1, Port: port, Protocol: "tcp",
					Close: func() {}})
				_ = tbl.Snapshot()
				tbl.Remove(id)
			}
		})
	}

	churn.Wait()
	close(stop)
	reapers.Wait()
}

func TestHolderConcurrentLoadStore(t *testing.T) {
	var h Holder
	h.Store(newIndex())
	var wg sync.WaitGroup

	for range 8 {
		wg.Go(func() {
			for i := range 1000 {
				idx := newIndex()
				idx.addUser(int64(i), "10.6.0.2")
				h.Store(idx)
			}
		})
	}
	for range 8 {
		wg.Go(func() {
			for range 1000 {
				got := h.Load()
				if got == nil || got.PeerByIP == nil {
					t.Errorf("Load returned torn/invalid index: %v", got)
					return
				}
			}
		})
	}
	wg.Wait()
}
