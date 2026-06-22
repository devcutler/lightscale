function prefersReducedMotion(): boolean {
	return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
}

function radiusToFarthestCorner(x: number, y: number): number {
	const w = window.innerWidth;
	const h = window.innerHeight;
	const dx = Math.max(x, w - x);
	const dy = Math.max(y, h - y);
	return Math.hypot(dx, dy);
}

export function toggleThemeWithTransition(
	x: number,
	y: number,
	flip: () => void,
): void {
	if (!document.startViewTransition || prefersReducedMotion()) {
		flip();
		return;
	}

	const r = radiusToFarthestCorner(x, y);

	const transition = document.startViewTransition(flip);

	transition.ready.then(() => {
		document.documentElement.animate(
			{
				clipPath: [
					`circle(0px at ${x}px ${y}px)`,
					`circle(${r}px at ${x}px ${y}px)`,
				],
			},
			{
				duration: 650,
				easing: "cubic-bezier(0.4, 0, 0.2, 1)",
				pseudoElement: "::view-transition-new(root)",
			},
		);
	});
}
