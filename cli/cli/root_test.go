package cli

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestNewRootShape(t *testing.T) {
	root := New()
	if root.Use != "lightscale" {
		t.Errorf("root.Use=%q want lightscale", root.Use)
	}
	want := map[string]bool{
		"user": false, "user-group": false, "service": false,
		"service-group": false, "policy": false, "status": false,
		"dns": false, "peers": false, "connections": false,
		"serve": false,
	}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestPersistentFlags(t *testing.T) {
	root := New()
	if root.PersistentFlags().Lookup("socket") == nil {
		t.Error("missing --socket persistent flag")
	}
	if root.PersistentFlags().Lookup("json") == nil {
		t.Error("missing --json persistent flag")
	}
}

func find(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	cur := root
	for _, seg := range path {
		var next *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == seg {
				next = c
				break
			}
		}
		if next == nil {
			t.Fatalf("command %q not found under %q", seg, cur.Name())
		}
		cur = next
	}
	return cur
}

func TestArgsValidators(t *testing.T) {
	root := New()
	cases := []struct {
		path []string
		n    int
	}{
		{[]string{"user", "create"}, 1},
		{[]string{"policy", "add"}, 3},
		{[]string{"user-group", "join"}, 2},
	}
	for _, c := range cases {
		cmd := find(t, root, c.path...)
		if cmd.Args == nil {
			t.Errorf("%v: no Args validator", c.path)
			continue
		}
		if err := cmd.Args(cmd, []string{}); err == nil {
			t.Errorf("%v: expected error for 0 args (needs %d)", c.path, c.n)
		}
		good := make([]string, c.n)
		for i := range good {
			good[i] = "x"
		}
		if err := cmd.Args(cmd, good); err != nil {
			t.Errorf("%v: unexpected error for %d args: %v", c.path, c.n, err)
		}
		if err := cmd.Args(cmd, append(good, "extra")); err == nil {
			t.Errorf("%v: expected error for %d args", c.path, c.n+1)
		}
	}
}

func TestSignalCtxCancel(t *testing.T) {
	ctx, cancel := signalCtx()
	select {
	case <-ctx.Done():
		t.Fatal("ctx done before cancel")
	default:
	}
	cancel()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("ctx not cancelled after cancel()")
	}
}
