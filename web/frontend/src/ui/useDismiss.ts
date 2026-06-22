import { Dispatch, RefObject, SetStateAction, useEffect } from "react";

export function useDismiss(ref: RefObject<HTMLElement>, open: boolean, setOpen: Dispatch<SetStateAction<boolean>>) {
	useEffect(() => {
		if (!open) return;
		const onDown = (e: MouseEvent) => {
			if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
		};
		window.addEventListener("mousedown", onDown);
		return () => window.removeEventListener("mousedown", onDown);
	}, [ref, open, setOpen]);
}
