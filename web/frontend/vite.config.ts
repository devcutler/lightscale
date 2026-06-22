import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
	plugins: [react()],
	base: "/",
	build: {
		outDir: "../dist",
		emptyOutDir: true,
	},
	server: {
		proxy: {
			"/api": {
				target: process.env.LIGHTSCALE_WEB ?? "http://127.0.0.1:11687",
				changeOrigin: true,
			},
		},
	},
});
