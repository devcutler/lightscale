package wg

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"

	"github.com/devcutler/lightscale/daemon/policy"
)

type Server struct {
	logger     *slog.Logger
	device     *device.Device
	netDevice  *netDevice
	udpPort    int
	pubKeyB64  string
	privKeyB64 string

	mu    sync.Mutex
	peers map[string]struct{}
}
type Options struct {
	Logger      *slog.Logger
	UDPPort     int
	MTU         int
	DaemonIP    netip.Addr
	ServiceVIPs []netip.Addr
	PrivateKey  string
	DNSServers  []netip.Addr
}

func Open(opts Options) (*Server, string, error) {
	if opts.MTU == 0 {
		opts.MTU = 1420
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	addrs := append([]netip.Addr{opts.DaemonIP}, opts.ServiceVIPs...)

	netDev, err := newNetDevice(addrs, opts.MTU)
	if err != nil {
		return nil, "", fmt.Errorf("wg: create netstack: %w", err)
	}

	logger := &device.Logger{
		Verbosef: func(format string, a ...any) {
			opts.Logger.Debug(fmt.Sprintf(format, a...))
		},
		Errorf: func(format string, a ...any) {
			opts.Logger.Warn(fmt.Sprintf("wg-go: "+format, a...))
		},
	}

	dev := device.NewDevice(netDev, conn.NewDefaultBind(), logger)

	priv := opts.PrivateKey
	if priv == "" {
		priv, err = generatePrivateKey()
		if err != nil {
			dev.Close()
			netDev.Close()
			return nil, "", err
		}
	}
	privHex, err := base64ToHex(priv)
	if err != nil {
		dev.Close()
		netDev.Close()
		return nil, "", fmt.Errorf("wg: decode private key: %w", err)
	}

	cfg := strings.Join([]string{
		"private_key=" + privHex,
		"listen_port=" + strconv.Itoa(opts.UDPPort),
	}, "\n") + "\n"
	if err := dev.IpcSet(cfg); err != nil {
		dev.Close()
		netDev.Close()
		return nil, "", fmt.Errorf("wg: ipc set: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		netDev.Close()
		return nil, "", fmt.Errorf("wg: device up: %w", err)
	}

	pubKey, err := derivePublicKey(privHex)
	if err != nil {
		dev.Close()
		netDev.Close()
		return nil, "", err
	}
	pubB64 := base64.StdEncoding.EncodeToString(pubKey)

	return &Server{
		logger:     opts.Logger,
		device:     dev,
		netDevice:  netDev,
		udpPort:    opts.UDPPort,
		pubKeyB64:  pubB64,
		privKeyB64: priv,
		peers:      map[string]struct{}{},
	}, pubB64, nil
}
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	if s.device != nil {
		s.device.Close()
	}
	if s.netDevice != nil {
		_ = s.netDevice.Close()
	}
	return nil
}
func (s *Server) PublicKey() string  { return s.pubKeyB64 }
func (s *Server) PrivateKey() string { return s.privKeyB64 }
func (s *Server) PeerCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.peers)
}

func (s *Server) SetRelay(hook RelayHook) {
	s.netDevice.SetRelay(hook)
}

type PeerStatus struct {
	PublicKey         string    `json:"public_key"`
	PresharedKey      string    `json:"preshared_key,omitempty"`
	AllowedIPs        []string  `json:"allowed_ips"`
	Endpoint          string    `json:"endpoint,omitempty"`
	LastHandshake     time.Time `json:"last_handshake"`
	KeepaliveInterval int       `json:"keepalive_interval"`
	RxBytes           uint64    `json:"rx_bytes"`
	TxBytes           uint64    `json:"tx_bytes"`
}

func (s *Server) PeerStatus() ([]PeerStatus, error) {
	dump, err := s.device.IpcGet()
	if err != nil {
		return nil, fmt.Errorf("wg: ipc get: %w", err)
	}
	return parsePeerStatus(dump), nil
}

func parsePeerStatus(dump string) []PeerStatus {
	var peers []PeerStatus
	var current *PeerStatus
	flush := func() {
		if current != nil {
			peers = append(peers, *current)
			current = nil
		}
	}
	for line := range strings.SplitSeq(dump, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		before, after, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value := before, after
		switch key {
		case "public_key":
			flush()
			raw, err := hex.DecodeString(value)
			if err != nil {
				continue
			}
			current = &PeerStatus{PublicKey: base64.StdEncoding.EncodeToString(raw)}
		case "preshared_key":
			if current == nil {
				continue
			}
			raw, err := hex.DecodeString(value)
			if err != nil {
				continue
			}
			zero := true
			for _, b := range raw {
				if b != 0 {
					zero = false
					break
				}
			}
			if !zero {
				current.PresharedKey = base64.StdEncoding.EncodeToString(raw)
			}
		case "allowed_ip":
			if current == nil {
				continue
			}
			current.AllowedIPs = append(current.AllowedIPs, value)
		case "endpoint":
			if current != nil {
				current.Endpoint = value
			}
		case "last_handshake_time_sec":
			if current != nil {
				if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
					current.LastHandshake = time.Unix(v, 0)
				}
			}
		case "rx_bytes":
			if current != nil {
				if v, err := strconv.ParseUint(value, 10, 64); err == nil {
					current.RxBytes = v
				}
			}
		case "tx_bytes":
			if current != nil {
				if v, err := strconv.ParseUint(value, 10, 64); err == nil {
					current.TxBytes = v
				}
			}
		case "persistent_keepalive_interval":
			if current != nil {
				if v, err := strconv.Atoi(value); err == nil {
					current.KeepaliveInterval = v
				}
			}
		}
	}
	flush()
	return peers
}

func (s *Server) Inject(packet []byte) {
	s.netDevice.Inject(packet)
}
func (s *Server) AddVIP(ip netip.Addr) error {
	return s.netDevice.AddAddress(ip)
}
func (s *Server) RemoveVIP(ip netip.Addr) error {
	return s.netDevice.RemoveAddress(ip)
}

func (s *Server) ApplyPeers(idx *policy.Index) error {
	if idx == nil {
		return nil
	}
	want := map[string]policy.UserSnapshot{}
	for _, u := range idx.UserByID {
		if u.PublicKey == "" {
			continue
		}
		hexKey, err := base64ToHex(u.PublicKey)
		if err != nil {
			s.logger.Warn("wg: skipping peer with bad pubkey", "user", u.Name, "err", err)
			continue
		}
		want[hexKey] = u
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var b strings.Builder
	for hexKey := range s.peers {
		if _, keep := want[hexKey]; !keep {
			fmt.Fprintf(&b, "public_key=%s\nremove=true\n", hexKey)
			delete(s.peers, hexKey)
		}
	}
	for hexKey, user := range want {
		if _, present := s.peers[hexKey]; present {
			continue
		}
		fmt.Fprintf(&b, "public_key=%s\n", hexKey)
		if user.PresharedKey != "" {
			pskHex, err := base64ToHex(user.PresharedKey)
			if err != nil {
				s.logger.Warn("wg: skip peer with bad psk", "user", user.Name, "err", err)
				continue
			}
			fmt.Fprintf(&b, "preshared_key=%s\n", pskHex)
		}
		fmt.Fprintf(&b, "allowed_ip=%s/32\n", user.IPAddress)
		s.peers[hexKey] = struct{}{}
	}
	if b.Len() == 0 {
		return nil
	}
	if err := s.device.IpcSet(b.String()); err != nil {
		return fmt.Errorf("wg: apply peers: %w", err)
	}
	return nil
}
func (s *Server) ListenTCP(vip netip.Addr, port int) (net.Listener, error) {
	return s.netDevice.listenTCP(netip.AddrPortFrom(vip, uint16(port)))
}
func (s *Server) ListenUDP(vip netip.Addr, port int) (net.PacketConn, error) {
	return s.netDevice.listenUDPAddr(netip.AddrPortFrom(vip, uint16(port)))
}

func GenerateKeypair() (priv, pub string, err error) {
	priv, err = generatePrivateKey()
	if err != nil {
		return "", "", err
	}
	privHex, err := base64ToHex(priv)
	if err != nil {
		return "", "", err
	}
	pubBytes, err := derivePublicKey(privHex)
	if err != nil {
		return "", "", err
	}
	return priv, base64.StdEncoding.EncodeToString(pubBytes), nil
}
func generatePrivateKey() (string, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", err
	}
	// curve25519 clamp (RFC 7748): clear low 3 bits, clear top bit, set bit 254
	key[0] &= 248
	key[31] &= 127
	key[31] |= 64
	return base64.StdEncoding.EncodeToString(key[:]), nil
}
func derivePublicKey(privHex string) ([]byte, error) {
	priv, err := hex.DecodeString(privHex)
	if err != nil || len(priv) != 32 {
		return nil, fmt.Errorf("wg: bad private key length")
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("wg: derive public key: %w", err)
	}
	return pub, nil
}

func Base64ToHex(b string) (string, error) {
	return base64ToHex(b)
}

func base64ToHex(b string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b)
	if err != nil {
		return "", err
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("wg: key must decode to 32 bytes, got %d", len(raw))
	}
	return hex.EncodeToString(raw), nil
}
