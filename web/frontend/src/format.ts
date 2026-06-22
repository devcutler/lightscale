import { getSettings } from "./ui/settings";

export const NEVER = Number.MAX_SAFE_INTEGER;

//for natural sorting
export function ipKey(ip?: string): number {
	if (!ip) return NEVER;
	const parts = ip.split(".");
	if (parts.length !== 4) return NEVER;
	let n = 0;
	for (const p of parts) {
		const o = Number(p);
		if (!Number.isInteger(o) || o < 0 || o > 255) return NEVER;
		n = n * 256 + o;
	}
	return n;
}

const BINARY_UNITS = ["KiB", "MiB", "GiB", "TiB", "PiB"];
const DECIMAL_UNITS = ["KB", "MB", "GB", "TB", "PB"];

export function bytes(n: number): string {
	const binary = getSettings().dataFormat === "binary";
	const base = binary ? 1024 : 1000;
	const units = binary ? BINARY_UNITS : DECIMAL_UNITS;
	if (n < base) return `${n} B`;
	let v = n / base;
	let i = 0;
	while (v >= base && i < units.length - 1) {
		v /= base;
		i++;
	}
	return `${v.toFixed(v < 10 ? 1 : 0)} ${units[i]}`;
}

export function rate(bytesPerSec: number): string {
	return `${bytes(Math.round(bytesPerSec))}/s`;
}

export function ago(seconds?: number): string {
	if (seconds == null) return "never";
	if (seconds < 5) return "just now";
	if (seconds < 60) return `${seconds}s ago`;
	if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
	if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
	return `${Math.floor(seconds / 86400)}d ago`;
}

export function portsLabel(ports: { port: number; protocol: string; }[]): string {
	return ports.map((p) => `${p.port}/${p.protocol}`).join(", ");
}
