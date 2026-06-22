import { createContext, useCallback, useContext, useState, ReactNode } from "react";
import { errMsg } from "../api";

type ToastKind = "ok" | "err";
interface Toast {
	id: number;
	kind: ToastKind;
	msg: ReactNode;
}

interface ToastCtx {
	ok: (msg: ReactNode) => void;
	err: (msg: ReactNode) => void;
}

const Ctx = createContext<ToastCtx>({ ok: () => { }, err: () => { } });

let nextId = 1;

export function ToastProvider({ children }: { children: ReactNode; }) {
	const [toasts, setToasts] = useState<Toast[]>([]);

	const push = useCallback((kind: ToastKind, msg: ReactNode) => {
		const id = nextId++;
		setToasts((t) => [...t, { id, kind, msg }]);
		setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4000);
	}, []);

	const ok = useCallback((msg: ReactNode) => push("ok", msg), [push]);
	const err = useCallback((msg: ReactNode) => push("err", msg), [push]);

	return (
		<Ctx.Provider value={{ ok, err }}>
			{children}
			<div className="toasts">
				{toasts.map((t) => (
					<div key={t.id} className={`toast toast-${t.kind}`}>
						{t.msg}
					</div>
				))}
			</div>
		</Ctx.Provider>
	);
}

export function useToast() {
	return useContext(Ctx);
}

export function useAction() {
	const toast = useToast();
	return useCallback(
		async (fn: () => Promise<void>) => {
			try {
				await fn();
			} catch (e) {
				toast.err(errMsg(e));
			}
		},
		[toast],
	);
}
