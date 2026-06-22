import { useEffect, useState } from "react";
import { toggleThemeWithTransition } from "./viewTransition";

export type Theme = "dark" | "light";

const KEY = "lightscale-theme";

function getTheme(): Theme {
	const stored = localStorage.getItem(KEY);
	if (stored === "dark" || stored === "light") return stored;
	return window.matchMedia?.("(prefers-color-scheme: light)").matches
		? "light"
		: "dark";
}

function applyTheme(theme: Theme) {
	document.documentElement.dataset.theme = theme;
}

export type ToggleOrigin = { x: number; y: number; };

export function useTheme(): [Theme, (origin?: ToggleOrigin) => void] {
	const [theme, setTheme] = useState<Theme>(() => getTheme());

	useEffect(() => {
		applyTheme(theme);
		localStorage.setItem(KEY, theme);
	}, [theme]);

	const toggle = (origin?: ToggleOrigin) => {
		const next: Theme = getTheme() === "dark" ? "light" : "dark";
		const flip = () => {
			applyTheme(next);
			setTheme(next);
		};
		if (origin) {
			toggleThemeWithTransition(origin.x, origin.y, flip);
		} else {
			flip();
		}
	};

	return [theme, toggle];
}
