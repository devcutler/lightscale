package wire

import "time"

type Error struct {
	Error string `json:"error"`
}

type User struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email,omitempty"`
	PublicKey    string `json:"public_key"`
	PresharedKey string `json:"preshared_key"`
	IPAddress    string `json:"ip_address"`
	Endpoint     string `json:"endpoint,omitempty"`
	Migrated     bool   `json:"migrated"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
type CreateUserReq struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	IP       string `json:"ip"`
	Endpoint string `json:"endpoint"`
}
type UpdateUserReq struct {
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
	Endpoint *string `json:"endpoint,omitempty"`
}

type ServicePort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}
type Service struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	Hostname    string        `json:"hostname"`
	Origin      string        `json:"origin"`
	IPAddress   string        `json:"ip_address"`
	Description string        `json:"description,omitempty"`
	Ports       []ServicePort `json:"ports"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
}

type CreateServiceReq struct {
	Name        string `json:"name"`
	Origin      string `json:"origin"`
	Ports       string `json:"ports"`
	Hostname    string `json:"hostname"`
	IP          string `json:"ip"`
	Description string `json:"description"`
}
type UpdateServiceReq struct {
	Name        *string `json:"name,omitempty"`
	Hostname    *string `json:"hostname,omitempty"`
	Origin      *string `json:"origin,omitempty"`
	IP          *string `json:"ip,omitempty"`
	Description *string `json:"description,omitempty"`
	Ports       *string `json:"ports,omitempty"`
}

type UserGroup struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	LANMode   bool   `json:"lan_mode"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreateUserGroupReq struct {
	Name    string `json:"name"`
	LANMode bool   `json:"lan_mode"`
}

type UpdateUserGroupReq struct {
	Name    *string `json:"name,omitempty"`
	LANMode *bool   `json:"lan_mode,omitempty"`
}
type UserGroupMemberReq struct {
	UserID   int64  `json:"user_id,omitempty"`
	UserName string `json:"user_name,omitempty"`
}

type ServiceGroup struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CreateServiceGroupReq struct {
	Name string `json:"name"`
}

type UpdateServiceGroupReq struct {
	Name *string `json:"name,omitempty"`
}

type ServiceGroupMemberReq struct {
	ServiceID   int64  `json:"service_id,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

type Policy struct {
	ID          int64  `json:"id"`
	SubjectType string `json:"subject_type"`
	SubjectID   int64  `json:"subject_id"`
	SubjectName string `json:"subject_name"`
	ObjectType  string `json:"object_type"`
	ObjectID    int64  `json:"object_id"`
	ObjectName  string `json:"object_name"`
	Action      string `json:"action"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}
type CreatePolicyReq struct {
	SubjectType string `json:"subject_type,omitempty"`
	SubjectID   int64  `json:"subject_id,omitempty"`
	SubjectName string `json:"subject_name,omitempty"`
	ObjectType  string `json:"object_type,omitempty"`
	ObjectID    int64  `json:"object_id,omitempty"`
	ObjectName  string `json:"object_name,omitempty"`
	Action      string `json:"action"`
}

type Principal struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type Object struct {
	Type string `json:"type"`
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type StatusSnapshot struct {
	Running      bool      `json:"running"`
	Peers        int       `json:"peers"`
	ActiveFlows  int       `json:"active_flows"`
	StartedAt    time.Time `json:"started_at"`
	UptimeSec    int64     `json:"uptime_sec"`
	WireGuardUDP string    `json:"wireguard_udp,omitempty"`
	SocketPath   string    `json:"socket_path,omitempty"`
}

type Peer struct {
	Name              string    `json:"name,omitempty"`
	UserID            int64     `json:"user_id,omitempty"`
	IPAddress         string    `json:"ip_address,omitempty"`
	PublicKey         string    `json:"public_key"`
	PresharedKey      string    `json:"preshared_key,omitempty"`
	AllowedIPs        []string  `json:"allowed_ips"`
	Endpoint          string    `json:"endpoint,omitempty"`
	LastHandshake     time.Time `json:"last_handshake"`
	LastHandshakeAgoS int64     `json:"last_handshake_ago_sec,omitempty"`
	KeepaliveInterval int       `json:"keepalive_interval"`
	RxBytes           uint64    `json:"rx_bytes"`
	TxBytes           uint64    `json:"tx_bytes"`
}

type Connection struct {
	ID         uint64 `json:"id"`
	SrcUserID  int64  `json:"src_user_id"`
	SrcName    string `json:"src_name,omitempty"`
	SrcIP      string `json:"src_ip,omitempty"`
	ObjectType string `json:"object_type"`
	ObjectID   int64  `json:"object_id"`
	ObjectName string `json:"object_name,omitempty"`
	ObjectIP   string `json:"object_ip,omitempty"`
	Port       int    `json:"port"`
	Protocol   string `json:"protocol"`
}
