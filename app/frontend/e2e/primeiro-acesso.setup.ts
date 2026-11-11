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

  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Comece pelo seu currículo.' })).toBeVisible();
  const inicio = info.outputPath('primeiro-acesso.png');
  await page.screenshot({ path: inicio });
  await info.attach('Primeiro acesso', { path: inicio });
  await page.getByLabel('Currículo em PDF').setInputFiles({ name: 'curriculo-teste.pdf', mimeType: 'application/pdf', buffer: pdf });
  await expect(page.getByText('curriculo-teste.pdf', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Continuar' }).click();
  await page.getByLabel('Cargo ou área de interesse').fill('Desenvolvimento de software');
  await page.getByLabel('Pleno', { exact: true }).check();
  await page.getByLabel(/Stack principal/).fill('React, Java');
  await page.getByRole('button', { name: 'Continuar' }).click();
  await page.getByLabel('Os dois', { exact: true }).check();
  await page.getByLabel('Remoto', { exact: true }).check();
  await page.getByLabel('Seu nível de inglês').selectOption('Avançado');
  await page.getByLabel(/Pretensão internacional/).fill('3500');
  await page.getByRole('button', { name: 'Continuar' }).click();
  await expect(page.getByRole('heading', { name: 'Revise seu perfil de busca.' })).toBeVisible();
  const revisao = info.outputPath('revisao.png');
  await page.screenshot({ path: revisao });
  await info.attach('Revisão do perfil', { path: revisao });
  const resposta = page.waitForResponse(r => r.url().endsWith('/api/v1/onboarding') && r.request().method() === 'POST');
  await page.getByRole('button', { name: /Salvar e conectar IA/ }).click();
  const enviado = await resposta;
  expect(enviado.status(), await enviado.text()).toBe(200);
  await expect(page.getByRole('heading', { name: 'Conectar IA' })).toBeVisible();

  const salvo = await estado();
  expect(salvo.onboardingConcluido).toBe(true);
  expect(salvo.perfil).toMatchObject({
    cargo: 'Desenvolvimento de software', stackPrincipal: ['React', 'Java'], filtros: { niveis: ['Pleno'], modelo: ['Remoto'] },
    foco: 'ambos', ingles: 'Avançado', pretensaoUSD: 3500,
  });
  expect(salvo.curriculos).toHaveLength(1);
  expect(await readFile(join(process.env.CENTRAL_E2E_PASTA!, salvo.perfil.curriculos.PT))).toEqual(pdf);
});
