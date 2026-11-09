/// <reference types="vitest/config" />
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

// A CSP do index.html vale no build (app e e2e). No `vite`/`wails3 dev`, o recarregamento rápido
// injeta script inline, então só o servidor de desenvolvimento tira a meta.
const cspSoNoBuild: Plugin = {
  name: "csp-so-no-build",
  apply: "serve",
  transformIndexHtml: html => html.replace(/\s*<meta http-equiv="Content-Security-Policy"[^>]*>/, ""),
};

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    // Fora do Wails (npm run dev), a API vem do servidor de desenvolvimento: go run ./cmd/central-dev
    proxy: { "/api": "http://127.0.0.1:4500" },
  },
  plugins: [react(), wails("./bindings"), cspSoNoBuild],
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"], // e2e/ é do Playwright
    env: { TZ: "America/Sao_Paulo" },
  },
});
