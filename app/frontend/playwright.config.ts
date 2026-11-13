import { defineConfig, devices } from '@playwright/test';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const aqui = dirname(fileURLToPath(import.meta.url));

// Servidor compilado só para testes: API Go, SQLite real e provedor isolado.
// A prova de conexão é emitida pelo backend. Nenhuma chamada usa a conta do computador.
const pasta = (process.env.CENTRAL_E2E_PASTA ??= mkdtempSync(join(tmpdir(), 'central-e2e-')));
// A conta (Task 7) roda num segundo central-dev, com -tags contafalsa (nuvem falsa e as rotas de teste /_teste/*),
// noutra porta e noutra pasta: os outros specs continuam sem conta, como o app sem a nuvem (ruling A11).
const pastaConta = (process.env.CENTRAL_E2E_PASTA_CONTA ??= mkdtempSync(join(tmpdir(), 'central-e2e-conta-')));
// Pasta já em uso (um currículo antigo): a conta testa quem atualiza o app, sem o primeiro acesso (a ordem porta →
// primeiro acesso está em src/app/ordem-conta.test.tsx).
mkdirSync(join(pastaConta, 'curriculos'), { recursive: true });
writeFileSync(join(pastaConta, 'curriculos', 'curriculo-ficticio.pdf'), '%PDF-1.4\n%%EOF\n');
const raiz = resolve(aqui, '../..');
const SPEC_CONTA = /conta\.spec\.ts$/;

export default defineConfig({
  testDir: './e2e',
  // A Fila prepara vagas ao abrir a tela. Outra aba de teste pode gravar na conta antes da jornada.
  workers: 1,
  timeout: 60_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: { baseURL: 'http://localhost:4510', trace: 'retain-on-failure' },
  projects: [
    { name: 'primeiro-acesso', testMatch: 'primeiro-acesso.setup.ts', use: { ...devices['Desktop Chrome'] } },
    { name: 'pc', dependencies: ['primeiro-acesso'], use: { ...devices['Desktop Chrome'] }, testIgnore: SPEC_CONTA },
    { name: 'celular', dependencies: ['primeiro-acesso'], use: { ...devices['Pixel 7'] }, testIgnore: SPEC_CONTA },
    { name: 'conta', use: { ...devices['Desktop Chrome'], baseURL: 'http://localhost:4511' }, testMatch: SPEC_CONTA },
  ],
  webServer: [
    {
      command: 'go test -tags e2e_radar ./internal/api -run=^TestServidorRadarPlaywright$ -count=1 -timeout=30m',
      cwd: raiz,
      url: 'http://localhost:4510/api/v1/saude',
      env: { APPLYER_E2E_SUITE:'1', APPLYER_E2E_PROVEDOR:'controlado', APPLYER_PASTA: pasta, CENTRAL_E2E_PASTA: pasta, CENTRAL_TELAS: process.env.CENTRAL_E2E_TELAS ?? resolve(aqui, 'dist') },
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      command: 'go run -tags contafalsa ./cmd/central-dev',
      cwd: raiz,
      url: 'http://localhost:4511/api/v1/saude',
      env: { PORT: '4511', CENTRAL_PASTA: pastaConta, CENTRAL_E2E_PASTA: pastaConta, CENTRAL_TELAS: process.env.CENTRAL_E2E_TELAS ?? resolve(aqui, 'dist') },
      timeout: 180_000,
      reuseExistingServer: false,
    },
  ],
});
