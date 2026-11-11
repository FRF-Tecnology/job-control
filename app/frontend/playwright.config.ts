import { defineConfig, devices } from '@playwright/test';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const aqui = dirname(fileURLToPath(import.meta.url));

// A jornada de "usuário novo" roda contra o servidor de desenvolvimento (API Go + telas compiladas)
// com uma pasta de dados vazia. Antes: npm run build (gera dist/).
// Exposta aos testes (ex.: currículo fictício na pasta de currículos); os workers herdam a variável e usam a mesma pasta.
const pasta = (process.env.CENTRAL_E2E_PASTA ??= mkdtempSync(join(tmpdir(), 'central-e2e-')));
// A conta (Task 7) roda num segundo central-dev, com -tags contafalsa (nuvem falsa e as rotas de teste /_teste/*),
// noutra porta e noutra pasta: os outros specs continuam sem conta, como o app sem a nuvem (ruling A11).
const pastaConta = (process.env.CENTRAL_E2E_PASTA_CONTA ??= mkdtempSync(join(tmpdir(), 'central-e2e-conta-')));
const raiz = resolve(aqui, '../..');
const SPEC_CONTA = /conta\.spec\.ts$/;

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: { baseURL: 'http://localhost:4510', trace: 'retain-on-failure' },
  projects: [
    { name: 'pc', use: { ...devices['Desktop Chrome'] }, testIgnore: SPEC_CONTA },
    { name: 'celular', use: { ...devices['Pixel 7'] }, testIgnore: SPEC_CONTA },
    { name: 'conta', use: { ...devices['Desktop Chrome'], baseURL: 'http://localhost:4511' }, testMatch: SPEC_CONTA },
  ],
  webServer: [
    {
      command: 'go run ./cmd/central-dev',
      cwd: raiz,
      url: 'http://localhost:4510/api/v1/saude',
      env: { PORT: '4510', CENTRAL_PASTA: pasta, CENTRAL_TELAS: resolve(aqui, 'dist') },
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      command: 'go run -tags contafalsa ./cmd/central-dev',
      cwd: raiz,
      url: 'http://localhost:4511/api/v1/saude',
      env: { PORT: '4511', CENTRAL_PASTA: pastaConta, CENTRAL_TELAS: resolve(aqui, 'dist') },
      timeout: 180_000,
      reuseExistingServer: false,
    },
  ],
});
