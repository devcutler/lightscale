export interface User {
	id: number;
	name: string;
	email?: string;
	public_key: string;
	preshared_key: string;
	ip_address: string;
	endpoint?: string;
	migrated: boolean;
	created_at: string;
	updated_at: string;
}

interface ServicePort {
	port: number;
	protocol: string;
}

export type OriginKind = "host" | "container" | "ip" | "hostname";

export interface Service {
	id: number;
	name: string;
	hostname: string;
	origin_kind: OriginKind;
	origin_value: string;
	origin_network?: string;
	ip_address: string;
	description?: string;
	ports: ServicePort[];
	created_at: string;
	updated_at: string;
}

export interface OriginCheck {
	reachable: boolean;
	dial_host?: string;
	network?: string;
	detail?: string;
	error?: string;
}

export interface UserGroup {
	id: number;
	name: string;
	lan_mode: boolean;
	created_at: string;
	updated_at: string;
}

export interface ServiceGroup {
	id: number;
	name: string;
	created_at: string;
	updated_at: string;
}

export interface Policy {
	id: number;
	subject_type: string;
	subject_id: number;
	subject_name: string;
	object_type: string;
	object_id: number;
	object_name: string;
	action: string;
	created_at: string;
	updated_at: string;
}

export interface Principal {
	type: string;
	id: number;
	name: string;
}

export interface ApiObject {
	type: string;
	id: number;
	name: string;
}

export interface StatusSnapshot {
	running: boolean;
	peers: number;
	active_flows: number;
	started_at: string;
	uptime_sec: number;
	wireguard_udp?: string;
	socket_path?: string;
}

export interface Peer {
	name?: string;
	user_id?: number;
	ip_address?: string;
	public_key: string;
	preshared_key?: string;
	allowed_ips: string[];
	endpoint?: string;
	last_handshake: string;
	last_handshake_ago_sec?: number;
	keepalive_interval: number;
	rx_bytes: number;
	tx_bytes: number;
}

export interface Connection {
	id: number;
	src_user_id: number;
	src_name?: string;
	src_ip?: string;
	object_type: string;
	object_id: number;
	object_name?: string;
	object_ip?: string;
	port: number;
	protocol: string;
}

export interface ContainerSummary {
	id: string;
	name: string;
	networks?: string[];
	shared: boolean;
}
