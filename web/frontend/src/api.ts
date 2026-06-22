async function parseError(res: Response): Promise<never> {
	let msg = `${res.status} ${res.statusText}`;
	try {
		const body = await res.json();
		if (body && typeof body.error === "string") msg = body.error;
	} catch {

	}
	throw new Error(msg);
}

export async function get<T>(path: string): Promise<T> {
	const res = await fetch("/api" + path);
	if (!res.ok) await parseError(res);
	return res.json() as Promise<T>;
}

export async function getText(path: string): Promise<string> {
	const res = await fetch("/api" + path);
	if (!res.ok) await parseError(res);
	return res.text();
}

async function send<T>(method: string, path: string, body: unknown): Promise<T> {
	const res = await fetch("/api" + path, {
		method,
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(body),
	});
	if (!res.ok) await parseError(res);
	if (res.status === 204) return undefined as T;
	return res.json() as Promise<T>;
}

export const post = <T>(path: string, body: unknown) => send<T>("POST", path, body);
export const patch = <T>(path: string, body: unknown) => send<T>("PATCH", path, body);

export async function del(path: string): Promise<void> {
	const res = await fetch("/api" + path, { method: "DELETE" });
	if (!res.ok) await parseError(res);
}

export function errMsg(e: unknown): string {
	if (e instanceof Error) return e.message;
	return String(e);
}
