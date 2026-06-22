import { useData } from "./useData";
import { get } from "../api";
import { StatusSnapshot } from "../types";

const POLL_MS = 10000;

export function LiveIndicator() {
	const { error, updatedAt } = useData<StatusSnapshot>(() => get("/status"), POLL_MS);

	let state: "live" | "stale" | "offline";
	let detail: string;
	if (error) {
		state = "offline";
		detail = "Offline - can't reach the daemon";
	} else if (updatedAt == null) {
		state = "stale";
		detail = "Connecting...";
	} else {
		const age = Date.now() - updatedAt;
		if (age > POLL_MS * 2) {
			state = "stale";
			detail = "Stale - last refresh is overdue";
		} else {
			state = "live";
			detail = "Live - connected to the daemon";
		}
	}

	return (
		<span className={"live-dot live-dot-" + state} role="status" aria-label={detail} title={detail} />
	);
}
