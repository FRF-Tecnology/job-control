/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    // Fora do Wails (npm run dev), a API vem do servidor de desenvolvimento: go run ./cmd/central-dev
    proxy: { "/api": "http://127.0.0.1:4500" },
  },
  plugins: [react(), wails("./bindings")],
  test: {
    environment: "jsdom",
    env: { TZ: "America/Sao_Paulo" },
  },
});
