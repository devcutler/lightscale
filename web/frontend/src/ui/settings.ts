import { useSyncExternalStore } from "react";

export type DataFormat = "binary" | "decimal";

export interface Settings {
	dataFormat: DataFormat;
}

const KEY = "lightscale-settings";

const DEFAULTS: Settings = {
	dataFormat: "binary",
};

function load(): Settings {
	try {
		const raw = localStorage.getItem(KEY);
		if (!raw) return DEFAULTS;
		const parsed = JSON.parse(raw) as Partial<Settings>;
		return { ...DEFAULTS, ...parsed };
	} catch {
		return DEFAULTS;
	}
}

let current: Settings = load();
const listeners = new Set<() => void>();

function emit() {
	for (const l of listeners) l();
}

export function getSettings(): Settings {
	return current;
}

export function setSetting<K extends keyof Settings>(key: K, value: Settings[K]) {
	if (current[key] === value) return;
	current = { ...current, [key]: value };
	try {
		localStorage.setItem(KEY, JSON.stringify(current));
	} catch { }
	emit();
}

function subscribe(cb: () => void): () => void {
	listeners.add(cb);
	return () => listeners.delete(cb);
}

export function useSettings(): Settings {
	return useSyncExternalStore(subscribe, getSettings, getSettings);
}
