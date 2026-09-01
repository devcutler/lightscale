import { Dispatch, RefObject, SetStateAction, useEffect } from "react";

const HANDLED = "__escapehamdled__";

export function escapeHandled(e: KeyboardEvent): boolean {
	return (e as KeyboardEvent & { [HANDLED]?: boolean; })[HANDLED] === true;
}

function claimEscape(e: KeyboardEvent) {
	(e as KeyboardEvent & { [HANDLED]?: boolean; })[HANDLED] = true;
}

export function useDismiss(ref: RefObject<HTMLElement>, open: boolean, setOpen: Dispatch<SetStateAction<boolean>>) {
	useEffect(() => {
		if (!open) return;
		const onDown = (e: MouseEvent) => {
			const target = e.target as Node;
			if (ref.current?.contains(target)) return;
			if (target instanceof Element && target.closest(".popover")) return;
			setOpen(false);
		};
		const onKey = (e: KeyboardEvent) => {
			if (e.key !== "Escape" || escapeHandled(e)) return;
			claimEscape(e);
			setOpen(false);
		};
		window.addEventListener("mousedown", onDown);
		window.addEventListener("keydown", onKey, true);
		return () => {
			window.removeEventListener("mousedown", onDown);
			window.removeEventListener("keydown", onKey, true);
		};
	}, [ref, open, setOpen]);
}
