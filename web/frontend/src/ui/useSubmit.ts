import { useState } from "react";
import { errMsg } from "../api";

interface Result {
	submit: () => void;
	busy: boolean;
	err: string | null;
}

export function useSubmit(fn: () => Promise<void>): Result {
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState<string | null>(null);

	const submit = async () => {
		setBusy(true);
		setErr(null);
		try {
			await fn();
		} catch (e) {
			setErr(errMsg(e));
		} finally {
			setBusy(false);
		}
	};

	return { submit, busy, err };
}
