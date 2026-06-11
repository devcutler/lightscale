package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"

	"github.com/devcutler/lightscale/daemon/store"
	wgserver "github.com/devcutler/lightscale/daemon/wg"
	"github.com/devcutler/lightscale/shared/config"
)

type e2eHarness struct {
	t       *testing.T
	dir     string
	cfg     config.Config
	cancel  context.CancelFunc
	doneCh  chan error
	apiURL  string
	client  *http.Client
	udpPort int
	pubKey  string
}

func startHarness(t *testing.T) *e2eHarness {
	t.Helper()

	dir := t.TempDir()
	udpPort := freeUDPPort(t)

	sockPath := filepath.Join(dir, "ls.sock")

	cfg := config.Defaults()
	cfg.PublicEndpoint = fmt.Sprintf("127.0.0.1:%d", udpPort)
	cfg.Domain = "test.lightscale"
	cfg.WireGuard.Port = udpPort
	cfg.Socket.Path = sockPath
	cfg.Socket.Group = ""
	cfg.Storage.Database = filepath.Join(dir, "ls.db")
	cfg.Docker.Socket = ""

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sockPath)
			},
		},
	}

	var logOut io.Writer = io.Discard
	if testing.Verbose() {
		logOut = testWriter{t: t}
	}
	logger := slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- Run(ctx, cfg, logger, nil, nil)
	}()

	apiURL := "http://lightscale"
	waitForAPI(t, client, apiURL)

	st, err := store.Open(cfg.Storage.Database)
	if err != nil {
		cancel()
		t.Fatalf("open store: %v", err)
	}
	pub, err := st.GetSetting(ctx, "server_public_key")
	if err != nil {
		t.Fatalf("get pubkey: %v", err)
	}
	_ = st.Close()

	h := &e2eHarness{
		t:       t,
		dir:     dir,
		cfg:     cfg,
		cancel:  cancel,
		doneCh:  doneCh,
		apiURL:  apiURL,
		client:  client,
		udpPort: udpPort,
		pubKey:  pub,
	}
	t.Cleanup(h.stop)
	return h
}

func (h *e2eHarness) stop() {
	h.cancel()
	select {
	case <-h.doneCh:
	case <-time.After(5 * time.Second):
		h.t.Logf("daemon shutdown timed out")
	}
}

type PeerHandle struct {
	device *device.Device
	tnet   *netstack.Net
	addr   netip.Addr
}

func (p *PeerHandle) Dial(target netip.AddrPort, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.tnet.DialContextTCPAddrPort(ctx, target)
}

func (h *e2eHarness) addPeer(name, pubKey string) netip.Addr {
	conf := h.apiPost("/api/users", map[string]any{"name": name})
	id := int64(conf["id"].(float64))
	ip, err := netip.ParseAddr(conf["ip_address"].(string))
	if err != nil {
		h.t.Fatalf("parse peer ip: %v", err)
	}

	st, err := store.Open(h.cfg.Storage.Database)
	if err != nil {
		h.t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	if _, err := st.DB().Exec(`UPDATE users SET public_key=?, preshared_key='' WHERE id=?`, pubKey, id); err != nil {
		h.t.Fatalf("override pubkey: %v", err)
	}
	h.apiPatch(fmt.Sprintf("/api/users/%d", id), map[string]any{"email": ""})
	time.Sleep(150 * time.Millisecond)
	return ip
}

func (h *e2eHarness) startPeer(name string) *PeerHandle {
	priv, pub, err := wgserver.GenerateKeypair()
	if err != nil {
		h.t.Fatalf("gen peer keypair: %v", err)
	}
	allocated := h.addPeer(name, pub)

	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{allocated},
		[]netip.Addr{netip.MustParseAddr("8.8.8.8")},
		1420,
	)
	if err != nil {
		h.t.Fatalf("client netstack: %v", err)
	}

	clientLogger := &device.Logger{
		Verbosef: func(string, ...any) {},
		Errorf:   func(format string, a ...any) { h.t.Logf("client: "+format, a...) },
	}
	if testing.Verbose() {
		clientLogger.Verbosef = func(format string, a ...any) { h.t.Logf("client: "+format, a...) }
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), clientLogger)

	privHex, err := wgserver.Base64ToHex(priv)
	if err != nil {
		h.t.Fatalf("priv hex: %v", err)
	}
	serverHex, err := wgserver.Base64ToHex(h.pubKey)
	if err != nil {
		h.t.Fatalf("server hex: %v", err)
	}
	cfg := strings.Join([]string{
		"private_key=" + privHex,
		"public_key=" + serverHex,
		"endpoint=127.0.0.1:" + fmt.Sprint(h.udpPort),
		"allowed_ip=10.6.0.0/23",
		"persistent_keepalive_interval=5",
	}, "\n") + "\n"
	if err := dev.IpcSet(cfg); err != nil {
		h.t.Fatalf("client ipc: %v", err)
	}
	if err := dev.Up(); err != nil {
		h.t.Fatalf("client up: %v", err)
	}

	peer := &PeerHandle{device: dev, tnet: tnet, addr: allocated}
	h.t.Cleanup(func() { dev.Close() })
	return peer
}

func (h *e2eHarness) apiGet(path string) map[string]any {
	req, _ := http.NewRequest("GET", h.apiURL+path, nil)
	return h.apiDo(req, http.StatusOK)
}

func (h *e2eHarness) apiPost(path string, body any) map[string]any {
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", h.apiURL+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return h.apiDoMulti(req, http.StatusCreated, http.StatusNoContent)
}

func (h *e2eHarness) apiPatch(path string, body any) map[string]any {
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest("PATCH", h.apiURL+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return h.apiDo(req, http.StatusOK)
}

func (h *e2eHarness) apiDelete(path string) {
	req, _ := http.NewRequest("DELETE", h.apiURL+path, nil)
	h.apiDo(req, http.StatusNoContent)
}

func (h *e2eHarness) apiDoMulti(req *http.Request, expects ...int) map[string]any {
	h.t.Helper()
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("api %s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ok := false
	for _, e := range expects {
		if resp.StatusCode == e {
			ok = true
			break
		}
	}
	if !ok {
		h.t.Fatalf("api %s %s: expected %v, got %d: %s",
			req.Method, req.URL.Path, expects, resp.StatusCode, string(body))
	}
	if resp.StatusCode == http.StatusNoContent || len(body) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil
	}
	return out
}

func (h *e2eHarness) apiDo(req *http.Request, expect int) map[string]any {
	h.t.Helper()
	resp, err := h.client.Do(req)
	if err != nil {
		h.t.Fatalf("api %s %s: %v", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expect {
		h.t.Fatalf("api %s %s: expected %d, got %d: %s",
			req.Method, req.URL.Path, expect, resp.StatusCode, string(body))
	}
	if resp.StatusCode == http.StatusNoContent || len(body) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil
	}
	return out
}

func startEchoBackend(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen backend: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().(*net.TCPAddr).Port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("free udp port: %v", err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port
}

func waitForAPI(t *testing.T, client *http.Client, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/api/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("api never became ready: %s", url)
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimRight(string(p), "\n"))
	return len(p), nil
}
