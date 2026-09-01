import { ReactNode, RefObject, useLayoutEffect, useState } from "react";
import { createPortal } from "react-dom";

interface Props {
	anchor: RefObject<HTMLElement>;
	open: boolean;
	align?: "left" | "right";
	className?: string;
	children: ReactNode;
}

export function Popover({ anchor, open, align = "left", className, children }: Props) {
	const [pos, setPos] = useState<{ top: number; left: number; width: number; maxHeight: number; } | null>(null);

	useLayoutEffect(() => {
		if (!open) return;

		const place = () => {
			const el = anchor.current;
			if (!el) return;
			const r = el.getBoundingClientRect();
			const GAP = 4;
			const MARGIN = 8;

			const below = window.innerHeight - r.bottom - GAP - MARGIN;
			const above = r.top - GAP - MARGIN;
			const flip = below < 160 && above > below;
			setPos({
				top: flip ? Math.max(MARGIN, r.top - GAP) : r.bottom + GAP,
				left: align === "right" ? r.right : r.left,
				width: r.width,
				maxHeight: Math.max(120, flip ? above : below),
			});
		};
		place();

		window.addEventListener("scroll", place, true);
		window.addEventListener("resize", place);
		return () => {
			window.removeEventListener("scroll", place, true);
			window.removeEventListener("resize", place);
		};
	}, [open, anchor, align]);

	if (!open || !pos) return null;

	const flipped = pos.top < (anchor.current?.getBoundingClientRect().top ?? 0);
	const vertical = flipped
		? { bottom: window.innerHeight - pos.top }
		: { top: pos.top };
	const horizontal =
		align === "right"
			? { right: Math.max(0, window.innerWidth - pos.left) }
			: { left: pos.left, minWidth: pos.width };
	const style: React.CSSProperties = {
		...vertical,
		...horizontal,
		maxHeight: pos.maxHeight,
	};

	return createPortal(
		<div className={"popover " + (className ?? "")} style={style}>
			{children}
		</div>,
		document.body,
	);
}
