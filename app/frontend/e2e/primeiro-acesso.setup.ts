import { expect, test } from '@playwright/test';
import { readFile } from 'node:fs/promises';
import { join } from 'node:path';

test('primeiro acesso grava o PDF e as preferências na conta de teste', async ({ page, context }, info) => {
  const estado = async () => (await page.request.get('/api/v1/estado')).json();
  expect((await estado()).onboardingConcluido).toBe(false);
  const documento = await context.newPage();
  await documento.setContent('<h1>Currículo fictício para teste do Applyer</h1><p>Desenvolvimento de software com React e Java.</p>');
  const pdf = await documento.pdf({ format: 'A4' });
  await documento.close();

  // A conexão e o provedor externos são dublês. O banco, o arquivo e o salvamento usam o central-dev.
  await page.route('**/api/v1/ia', rota => rota.fulfill({ json: {
    configurada: true, provedor: 'openai', chaves: { openai: '…teste' }, clis: [], gratis: { disponivel: false },
    uso: { chamadas: 0, falhas: 0, custoEstimadoUSD: 0 },
  } }));
  await page.route('**/api/v1/ia/testar', rota => rota.fulfill({ json: { ms: 12 } }));
  await page.route('**/api/v1/onboarding/sugerir', rota => {
    expect(Buffer.from(rota.request().postDataJSON().curriculoBase64, 'base64')).toEqual(pdf);
    return rota.fulfill({ json: { sugestoes: [
      { campo: 'cargo', valor: 'Desenvolvimento de software', evidencia: 'Desenvolvimento de software com React e Java' },
      { campo: 'stackPrincipal', valor: 'React', evidencia: 'React e Java' },
      { campo: 'stackPrincipal', valor: 'Java', evidencia: 'React e Java' },
    ] } });
  });
  await page.route('**/api/v1/busca', rota => rota.fulfill({ json: { encontradas: 0, novas: 0, grupos: {}, puladas: 0, pendentes: 0, avisos: [] } }));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Conecte sua IA.' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Grok', exact: true })).toBeVisible();
  await page.setViewportSize({ width: 375, height: 812 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(375);
  await page.setViewportSize({ width: 1280, height: 720 });
  const chamada = page.getByText('E se a vaga dos seus sonhos estiver', { exact: false });
  await expect(chamada).toBeInViewport({ ratio: 1 });
  await expect(page.getByRole('button', { name: 'Usar conexão atual' })).toBeInViewport({ ratio: 1 });
  const claro = info.outputPath('conexao-claro.png');
  await page.screenshot({ path: claro, animations: 'disabled' });
  await info.attach('Conexão com IA (claro)', { path: claro });
  await page.getByRole('button', { name: 'Alternar tema claro/escuro' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-tema', 'escuro');
  await page.setViewportSize({ width: 1366, height: 768 });
  await expect(chamada).toBeInViewport({ ratio: 1 });
  const inicio = info.outputPath('primeiro-acesso.png');
  await page.screenshot({ path: inicio, animations: 'disabled' });
  await info.attach('Primeiro acesso', { path: inicio });
  await page.getByRole('button', { name: 'Usar conexão atual' }).click();
  await page.getByLabel('Currículo em PDF').setInputFiles({ name: 'curriculo-teste.pdf', mimeType: 'application/pdf', buffer: pdf });
  await expect(page.getByText('curriculo-teste.pdf', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Preencher com IA' }).click();
  await expect(page.getByLabel('Cargo ou área de interesse')).toHaveValue('Desenvolvimento de software');
  await page.getByLabel('Pleno', { exact: true }).check();
  await expect(page.getByLabel(/Stack principal/)).toHaveValue('React, Java');
  await page.getByRole('button', { name: 'Continuar' }).click();
  await page.getByLabel('Os dois', { exact: true }).check();
  await page.getByLabel('Remoto', { exact: true }).check();
  await page.getByLabel('Seu nível de inglês').selectOption('Avançado');
  await page.getByLabel('Moeda').selectOption('USD');
  await page.getByLabel('Mínimo').fill('4000');
  await page.getByLabel('Máximo').fill('6000');
  const faixa = info.outputPath('faixa-salarial.png');
  await page.screenshot({ path: faixa, animations: 'disabled' });
  await info.attach('Condições e faixa salarial', { path: faixa });
  await page.getByRole('button', { name: 'Continuar' }).click();
  await expect(page.getByRole('heading', { name: 'Revise seu perfil de busca.' })).toBeVisible();
  const revisao = info.outputPath('revisao.png');
  await page.screenshot({ path: revisao, animations: 'disabled' });
  await info.attach('Revisão do perfil', { path: revisao });
  const resposta = page.waitForResponse(r => r.url().endsWith('/api/v1/onboarding') && r.request().method() === 'POST');
  await page.getByRole('button', { name: /Salvar e buscar vagas/ }).click();
  const enviado = await resposta;
  expect(enviado.status(), await enviado.text()).toBe(200);
  await expect(page).toHaveURL(/#\/triagem/);

  const salvo = await estado();
  expect(salvo.onboardingConcluido).toBe(true);
  expect(salvo.perfil).toMatchObject({
    cargo: 'Desenvolvimento de software', stackPrincipal: ['React', 'Java'], filtros: { niveis: ['Pleno'], modelo: ['Remoto'] },
    foco: 'ambos', ingles: 'Avançado', faixaSalarial: { moeda: 'USD', min: 4000, max: 6000 },
  });
  expect(salvo.curriculos).toHaveLength(1);
  expect(await readFile(join(process.env.CENTRAL_E2E_PASTA!, salvo.perfil.curriculos.PT))).toEqual(pdf);
});
