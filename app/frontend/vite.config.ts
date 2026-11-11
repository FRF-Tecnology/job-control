/// <reference types="vitest/config" />
import { defineConfig, searchForWorkspaceRoot, type Plugin } from "vite";
import { fileURLToPath } from "node:url";
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
    // Os termos e a política (docs/legal, na raiz do repositório) entram no app como texto (?raw), para ler sem internet.
    fs: { allow: [searchForWorkspaceRoot(process.cwd()), fileURLToPath(new URL("../../docs/legal", import.meta.url))] },
  },
  plugins: [react(), wails("./bindings"), cspSoNoBuild],
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"], // e2e/ é do Playwright
    env: { TZ: "America/Sao_Paulo" },
    // No CI, o runner às vezes para uns 10 s ao mesmo tempo em vários arquivos (visto no #77: 3 testes de ~100 ms
    // levaram 10 a 11 s, e o rerun do mesmo commit passou com o mais lento em 1 s). Folga só lá; aqui, 5 s continua
    // pegando teste lento de verdade (jc-a2y.62).
    testTimeout: process.env.CI ? 20_000 : 5_000,
  },
});
