import { useCallback, useEffect, useRef, useState } from "react";
import { errMsg } from "../api";

interface State<T> {
	data: T | null;
	error: string | null;
	loading: boolean;
	updatedAt: number | null;
	reload: () => void;
}

export function useData<T>(fetcher: () => Promise<T>, pollMs?: number): State<T> {
	const [data, setData] = useState<T | null>(null);
	const [error, setError] = useState<string | null>(null);
	const [loading, setLoading] = useState(true);
	const [updatedAt, setUpdatedAt] = useState<number | null>(null);
	const [tick, setTick] = useState(0);

	const fetcherRef = useRef(fetcher);
	fetcherRef.current = fetcher;

	const reload = useCallback(() => setTick((t) => t + 1), []);

	useEffect(() => {
		let cancelled = false;
		const run = () => {
			fetcherRef.current()
				.then((d) => {
					if (!cancelled) {
						setData(d);
						setError(null);
						setUpdatedAt(Date.now());
					}
				})
				.catch((e) => {
					if (!cancelled) {
						setError(errMsg(e));
					}
				})
				.finally(() => {
					if (!cancelled) setLoading(false);
				});
		};
		run();
		let id: number | undefined;
		if (pollMs) id = window.setInterval(() => {
			if (!document.hidden) run();
		}, pollMs);
		const onVisible = () => {
			if (!document.hidden) run();
		};
		document.addEventListener("visibilitychange", onVisible);
		return () => {
			cancelled = true;
			if (id) window.clearInterval(id);
			document.removeEventListener("visibilitychange", onVisible);
		};

	}, [tick, pollMs]);

	return { data, error, loading, updatedAt, reload };
}
