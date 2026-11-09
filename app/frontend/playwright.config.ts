import { defineConfig, devices } from '@playwright/test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const aqui = dirname(fileURLToPath(import.meta.url));

// A jornada de "usuário novo" roda contra o servidor de desenvolvimento (API Go + telas compiladas)
// com uma pasta de dados vazia. Antes: npm run build (gera dist/).
const pasta = mkdtempSync(join(tmpdir(), 'central-e2e-'));
const raiz = resolve(aqui, '../..');

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: { baseURL: 'http://localhost:4510', trace: 'retain-on-failure' },
  projects: [
    { name: 'pc', use: { ...devices['Desktop Chrome'] } },
    { name: 'celular', use: { ...devices['Pixel 7'] } },
  ],
  webServer: {
    command: 'go run ./cmd/central-dev',
    cwd: raiz,
    url: 'http://localhost:4510/api/v1/saude',
    env: { PORT: '4510', CENTRAL_PASTA: pasta, CENTRAL_TELAS: resolve(aqui, 'dist') },
    timeout: 180_000,
    reuseExistingServer: false,
  },
});
