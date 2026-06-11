package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devcutler/lightscale/daemon/store"
	"github.com/devcutler/lightscale/shared/config"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cfg := config.Defaults()
	cfg.PublicEndpoint = "vpn.example.com:51820"
	cfg.Domain = "lightscale.local"

	if err := s.SetSetting(context.Background(), "server_public_key", "FAKE_SERVER_PUBLIC_KEY=="); err != nil {
		t.Fatal(err)
	}

	return New(Deps{Store: s, Config: &cfg})
}

func do(t *testing.T, srv *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func TestUserCreateGetConfig(t *testing.T) {
	srv := newTestServer(t)

	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var u userDTO
	if err := json.NewDecoder(rr.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.IPAddress == "" {
		t.Fatal("expected IP allocation")
	}
	if u.PresharedKey == "" || u.PublicKey == "" {
		t.Fatal("expected keys")
	}

	rr = do(t, srv, "GET", "/api/users/"+itoa(u.ID)+"/config", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("config: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "[Interface]") || !strings.Contains(rr.Body.String(), "vpn.example.com:51820") {
		t.Fatalf("config body wrong:\n%s", rr.Body.String())
	}
}

func TestServicePoliciesAndDNS(t *testing.T) {
	srv := newTestServer(t)

	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create user: %d %s", rr.Code, rr.Body.String())
	}

	rr = do(t, srv, "POST", "/api/services", map[string]any{
		"name": "jellyfin", "origin": "host", "ports": "8096/tcp",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create service: %d %s", rr.Code, rr.Body.String())
	}

	rr = do(t, srv, "POST", "/api/policies", map[string]any{
		"subject_name": "alice", "object_name": "jellyfin", "action": "allow",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("create policy: %d %s", rr.Code, rr.Body.String())
	}

	rr = do(t, srv, "GET", "/api/policies", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("list policies: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"alice"`) || !strings.Contains(rr.Body.String(), `"jellyfin"`) {
		t.Fatalf("policies body missing names:\n%s", rr.Body.String())
	}

	rr = do(t, srv, "GET", "/api/dns", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("dns: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "$ORIGIN lightscale.local") {
		t.Fatalf("dns body wrong:\n%s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "jellyfin") {
		t.Fatalf("dns missing record:\n%s", rr.Body.String())
	}
}

func TestNamespaceCollision(t *testing.T) {
	srv := newTestServer(t)

	rr := do(t, srv, "POST", "/api/users", map[string]any{"name": "alice"})
	if rr.Code != http.StatusCreated {
		t.Fatal(rr.Body.String())
	}
	rr = do(t, srv, "POST", "/api/user-groups", map[string]any{"name": "alice"})
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d %s", rr.Code, rr.Body.String())
	}
}
func doRaw(t *testing.T, srv *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func TestAPIFailurePaths(t *testing.T) {
	srv := newTestServer(t)

	t.Run("malformed JSON body -> 400", func(t *testing.T) {
		rr := doRaw(t, srv, "POST", "/api/users", `{"name":`)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("missing required name -> 400", func(t *testing.T) {
		rr := do(t, srv, "POST", "/api/users", map[string]any{"email": "x@y.z"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("service missing origin -> 400", func(t *testing.T) {
		rr := do(t, srv, "POST", "/api/services", map[string]any{"name": "x"})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid port spec -> 400", func(t *testing.T) {
		rr := do(t, srv, "POST", "/api/services", map[string]any{
			"name": "bad", "origin": "192.168.1.5", "ports": "notaport/tcp",
		})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("wildcard host service -> 400", func(t *testing.T) {
		rr := do(t, srv, "POST", "/api/services", map[string]any{
			"name": "wild", "origin": "host",
		})
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("get nonexistent user -> 404", func(t *testing.T) {
		rr := do(t, srv, "GET", "/api/users/99999", nil)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid id -> 400", func(t *testing.T) {
		rr := do(t, srv, "GET", "/api/users/abc", nil)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("got %d: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("policy with unknown names -> error (not 2xx)", func(t *testing.T) {
		rr := do(t, srv, "POST", "/api/policies", map[string]any{
			"subject_name": "ghost", "object_name": "phantom", "action": "allow",
		})
		if rr.Code < 400 {
			t.Fatalf("expected an error status, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

func itoa(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
