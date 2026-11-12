// Conta, teste e trava de ponta a ponta (Task 7), contra o central-dev com -tags contafalsa (projeto "conta" do
// playwright.config.ts, porta 4511): abrir → criar a conta → código 123456 → Hoje com a faixa do teste → a rota de
// teste avança 25 h → buscar dá 402 e a faixa de bloqueio aparece → assinar (a rota de teste faz o papel do Checkout)
// → liberado → sair volta à porta. Só dados fictícios; sem janela (headless).
import { expect, test } from '@playwright/test';

test.describe.configure({ mode: 'serial' });

const BLOQUEIO = 'Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.';

test('conta nova: teste de 24 h, trava depois dele e assinatura libera', async ({ page }) => {
  // 1. Sem conta: só a porta de entrada, antes de tudo (sem menu).
  await page.goto('/#/hoje');
  await expect(page.getByRole('heading', { name: 'Entre na sua conta' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Seções' })).toHaveCount(0);
  await expect(page.getByLabel('Email')).toBeFocused();

  // 2. Criar a conta e confirmar com o código da nuvem falsa.
  await page.getByRole('button', { name: 'Criar uma conta' }).click();
  await page.getByLabel('Email').fill('ana@exemplo.com');
  await page.getByLabel('Senha', { exact: true }).fill('senha-forte-1');
  await page.getByLabel(/Li e aceito/).check();
  await page.getByRole('button', { name: 'Criar conta', exact: true }).click();
  await expect(page.getByText('Pode pedir outro código em', { exact: false })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Pedir outro código' })).toBeDisabled();
  await page.getByLabel('Código de confirmação').fill('123456');
  await page.getByRole('button', { name: 'Confirmar' }).click();

  // 3. Primeiro acesso: Hoje, com a faixa do teste e o menu com a Assinatura.
  await expect(page.getByRole('heading', { name: 'Olá.' })).toBeVisible();
  const faixa = page.locator('.faixa-conta');
  await expect(faixa).toHaveText(/^Seu teste acaba em (23 h 59 min|24 h 0 min) · Assinar$/);
  await expect(page.getByRole('navigation', { name: 'Seções' }).getByRole('link', { name: 'Assinatura' })).toBeVisible();
  const pedido = page.getByLabel('O que vamos fazer hoje?');
  await expect(pedido).toBeEnabled();

  // 4. A rota de teste adianta 25 h: a nuvem falsa diz que o teste acabou.
  const avancou = await page.request.post('/api/v1/_teste/avancar', { data: { horas: 25 } });
  expect(avancou.status()).toBe(200);
  expect((await avancou.json()).motivo).toBe('teste-acabou');

  // 5. A tela ainda não releu a conta: buscar vai ao servidor, que responde 402; a faixa de bloqueio aparece e as
  // ações de escrita ficam desabilitadas (ver continua).
  const resposta = page.waitForResponse(r => r.url().endsWith('/api/v1/pedidos') && r.request().method() === 'POST');
  await pedido.fill('Buscar vagas de React remotas');
  await pedido.press('Enter');
  expect((await resposta).status()).toBe(402);
  await expect(faixa).toContainText(BLOQUEIO);
  // Ruling A13: só a escrita trava (enviar o pedido); a caixa de texto e a navegação continuam.
  await expect(page.getByRole('button', { name: 'Enviar pedido' })).toBeDisabled();
  await expect(pedido).toBeEnabled();
  // No Perfil, sair e excluir a conta continuam; salvar o perfil trava (o servidor bloqueia PUT /perfil).
  await page.goto('/#/perfil');
  await expect(page.getByRole('button', { name: 'Sair da conta' })).toBeEnabled();
  await expect(page.locator('#perfil-nome')).toBeEnabled();
  await expect(page.getByRole('button', { name: 'Salvar perfil' }).first()).toBeDisabled();

  // 6. Assinar: o central-dev não abre o navegador; a rota de teste faz o papel do pagamento no Checkout.
  await page.goto('/#/assinatura');
  await page.getByRole('button', { name: 'Assinar', exact: true }).click();
  await expect(page.getByText('Conclua o pagamento no navegador; o app confere sozinho')).toBeVisible();
  expect((await page.request.post('/api/v1/_teste/pagar', { data: {} })).status()).toBe(200);
  await expect(page.getByText('Assinatura ativa. Obrigado!')).toBeVisible({ timeout: 15_000 });
  await expect(faixa).toHaveCount(0);

  // 7. Liberado: buscar volta a funcionar.
  await page.goto('/#/hoje');
  await expect(page.getByRole('button', { name: 'Enviar pedido' })).toBeEnabled();
  const depois = page.waitForResponse(r => r.url().endsWith('/api/v1/pedidos') && r.request().method() === 'POST');
  await pedido.fill('Buscar vagas de React remotas');
  await pedido.press('Enter');
  expect((await depois).status()).toBeLessThan(300);

  // 8. Sair da conta (Perfil): volta para a porta de entrada.
  await page.goto('/#/perfil');
  await page.getByRole('button', { name: 'Sair da conta' }).click();
  await expect(page.getByRole('heading', { name: 'Entre na sua conta' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Seções' })).toHaveCount(0);
});
