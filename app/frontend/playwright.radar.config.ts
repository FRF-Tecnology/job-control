import { defineConfig, devices } from '@playwright/test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// npx playwright test -c playwright.radar.config.ts
// APPLYER_E2E_CLAUDE=1 usa Claude Code e a assinatura autorizada no computador.
const aqui = dirname(fileURLToPath(import.meta.url));
const pasta = process.env.CENTRAL_E2E_PASTA ??= mkdtempSync(join(tmpdir(), 'applyer-radar-'));
const modo = process.env.APPLYER_E2E_CLAUDE === '1' ? 'claude' : 'controlado';
const baseURL = 'http://127.0.0.1:4511';

export default defineConfig({
  testDir: './e2e',
  testMatch: 'primeiro-acesso.setup.ts',
  timeout: 300_000,
  retries: 0,
  use: { ...devices['Desktop Chrome'], baseURL, headless: true },
  projects: [{ name: 'radar-' + modo }],
  webServer: {
    command: 'go test -tags e2e_radar ./internal/api -run=^TestServidorRadarPlaywright$ -count=1 -timeout=10m',
    cwd: resolve(aqui, '../..'),
    url: baseURL + '/api/v1/saude',
    env: { APPLYER_E2E_PROVEDOR: modo, APPLYER_PASTA: pasta, CENTRAL_E2E_PASTA: pasta, CENTRAL_TELAS: resolve(aqui, 'dist') },
    timeout: 180_000,
    reuseExistingServer: false,
  },
});
