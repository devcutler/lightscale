package daemon

import (
	"encoding/json"
	"io"
	"testing"
)

func TestE2E_DaemonBoots(t *testing.T) {
	h := startHarness(t)

	resp, err := h.client.Get(h.apiURL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var status map[string]any
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("decode: %v body=%s", err, string(body))
	}
	if status["running"] != true {
		t.Fatalf("expected running=true, got %#v", status)
	}
	if v, _ := status["socket_path"].(string); v != h.cfg.Socket.Path {
		t.Fatalf("expected socket path %q, got %q", h.cfg.Socket.Path, v)
	}
}
