import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

async function captura(page: Page, info: TestInfo, nome: string) {
  const path = info.outputPath(nome + '.png');
  await page.screenshot({ path, animations: 'disabled', fullPage: true });
  await info.attach(nome, { path });
}

// A suíte compartilhada só precisa do perfil. O projeto radar-* usa um banco próprio,
// a API de busca e uma fonte controlada para percorrer também a candidatura manual.
test('primeiro acesso, revisão do currículo e Radar de vagas', async ({ page, context }, info) => {
  const radar = info.project.name.startsWith('radar-');
  const estado = async () => (await page.request.get('/api/v1/estado')).json();
  expect((await estado()).onboardingConcluido).toBe(false);
  const documento = await context.newPage();
  await documento.setContent(`<h1>Currículo fictício</h1><p>Cargo: Desenvolvimento de software</p>
    <p>Nível: Pleno</p><p>Experiência profissional: desenvolvimento de serviços com React e Java.</p>
    <p>Ferramentas principais: React e Java.</p><p>Inglês avançado.</p>`);
  const pdf = await documento.pdf({ format: 'A4' });
  await documento.close();

  if (!radar) {
    // A configuração não sai da página e nenhum fornecedor recebe o PDF nesta suíte.
    await page.route('**/api/v1/ia', r => r.fulfill({ json: {
      configurada: true, provedor: 'claude-cli', chaves: {}, gratis: { disponivel: false },
      clis: [{ provedor: 'claude-cli', nome: 'Claude', instalada: true, pronta: true, logada: true }],
      uso: { chamadas: 0, falhas: 0, custoEstimadoUSD: 0 },
    } }));
    await page.route('**/api/v1/ia/config', r => r.fulfill({ json: { provedor: 'claude-cli' } }));
    await page.route('**/api/v1/ia/testar', r => r.fulfill({ json: { ok: true, ms: 12 } }));
    await page.route('**/api/v1/onboarding/sugerir', r => {
      expect(Buffer.from(r.request().postDataJSON().curriculoBase64, 'base64')).toEqual(pdf);
      return r.fulfill({ json: { sugestoes: [
        { campo: 'cargo', valor: 'Desenvolvimento de software', evidencia: 'Desenvolvimento de software' },
        { campo: 'stackPrincipal', valor: 'React', evidencia: 'React e Java' },
        { campo: 'stackPrincipal', valor: 'Java', evidencia: 'React e Java' },
      ] } });
    });
  }

  let buscas = 0;
  page.on('request', r => { if (r.method() === 'POST' && r.url().endsWith('/api/v1/busca')) buscas++; });
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /Sua IA e seu currículo/i })).toBeVisible();
  await page.setViewportSize({ width: 375, height: 812 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(375);
  await page.setViewportSize({ width: 1366, height: 850 });
  const claude = page.getByRole('button', { name: 'Claude', exact: true });
  await expect(claude).toBeVisible({ timeout: 45_000 });
  await claude.click();
  if (radar) await page.getByRole('button', { name: 'Minha assinatura' }).click();
  await captura(page, info, 'primeiro-acesso');
  if (!radar) {
    const tema = page.getByRole('button', { name: 'Alternar tema claro/escuro' });
    await tema.click();
    await expect(page.locator('html')).toHaveAttribute('data-tema', 'escuro');
    await captura(page, info, 'primeiro-acesso-escuro');
    await tema.click();
  }
  const teste = page.waitForResponse(r => r.url().endsWith('/api/v1/ia/testar'), { timeout: 130_000 });
  await page.getByRole('button', { name: /Conectar e continuar|Usar conexão atual|Verificar conexão|Testar conexão/ }).click();
  const retorno = await teste;
  expect(retorno.status(), 'verificação da IA').toBe(200);
  await expect(page.getByText('Claude conectado', { exact: true }).first()).toBeVisible();
  await page.getByLabel('Currículo em PDF').setInputFiles({ name: 'curriculo-teste.pdf', mimeType: 'application/pdf', buffer: pdf });
  await expect(page.getByText('curriculo-teste.pdf', { exact: true }).first()).toBeVisible();
  await captura(page, info, 'claude-curriculo');

  const extracao = page.waitForResponse(r => r.url().endsWith('/api/v1/onboarding/sugerir'), { timeout: 130_000 });
  await page.getByRole('button', { name: 'Preencher com IA' }).click();
  const extraida = await extracao;
  expect(extraida.status(), 'extração do PDF').toBe(200);
  const { sugestoes } = await extraida.json();
  expect(sugestoes.length).toBeGreaterThan(0);
  await expect(page.getByRole('heading', { name: /Que vagas você procura/i })).toBeVisible();
  const campoCargo = page.getByLabel('Cargo ou área de interesse');
  const stack = page.getByLabel(/Stack principal/);
  await expect(stack).toHaveValue(/React|Java/i);
  await campoCargo.fill('Desenvolvimento de software');
  await stack.fill('React, Java');
  await page.getByLabel('Pleno', { exact: true }).check();
  await page.getByLabel('Os dois', { exact: true }).check();
  await page.getByLabel('Remoto', { exact: true }).check();
  await page.getByLabel('Seu nível de inglês').selectOption('Avançado');
  await page.getByLabel('Moeda').selectOption('USD');
  await page.getByLabel('Mínimo').fill('4000');
  await page.getByLabel('Máximo').fill('6000');
  await captura(page, info, 'revisao');
  const salvamento = page.waitForResponse(r => r.url().endsWith('/api/v1/onboarding') && r.request().method() === 'POST');
  await page.getByRole('button', { name: 'Concluir perfil' }).click();
  expect((await salvamento).status()).toBe(200);
  await expect(page).toHaveURL(/#\/radar$/);
  await expect(page.getByRole('heading', { name: 'Radar de vagas' })).toBeVisible();
  const salvo = await estado();
  expect(salvo.onboardingConcluido).toBe(true);
  expect(salvo.vagas).toHaveLength(0);
  expect(salvo.perfil).toMatchObject({
    cargo: 'Desenvolvimento de software', stackPrincipal: ['React', 'Java'], filtros: { niveis: ['Pleno'], modelo: ['Remoto'] },
    foco: 'ambos', ingles: 'Avançado', faixaSalarial: { moeda: 'USD', min: 4000, max: 6000 },
  });
  expect(salvo.curriculos).toHaveLength(1);
  expect(await readFile(join(process.env.CENTRAL_E2E_PASTA!, salvo.perfil.curriculos.PT))).toEqual(pdf);
  await page.reload();
  await expect(page.getByRole('button', { name: 'Buscar vagas', exact: true })).toBeVisible();
  expect(buscas).toBe(0);
  if (!radar) return;

  const busca = page.waitForResponse(r => r.url().endsWith('/api/v1/busca'), { timeout: 130_000 });
  await page.getByRole('button', { name: 'Buscar vagas', exact: true }).click();
  const rodada = await busca;
  expect(rodada.status()).toBe(200);
  const resultado = await rodada.json();
  expect(resultado).toMatchObject({ encontradas: 1, novas: 1, pendentes: 0, puladas: 0 });
  expect(buscas).toBe(1);
  const aposBusca = await estado();
  expect(aposBusca.vagas).toHaveLength(1);
  expect(aposBusca.vagas[0].aderencia).toMatch(/^[ABC]$/);
  expect(aposBusca.vagas[0].etapa).toBe('descoberta');
  const ficha = page.getByRole('article', { name: 'Desenvolvimento de software com React' });
  await expect(ficha).toBeVisible();
  await page.setViewportSize({ width: 375, height: 812 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(375);
  await page.setViewportSize({ width: 1366, height: 850 });
  await captura(page, info, 'radar-classificacao');
  await ficha.getByRole('button', { name: 'Aplicar na vaga' }).click();
  await expect(ficha.getByText('Aguardando você', { exact: true })).toBeVisible();
  const anuncio = 'https://vaga.example.invalid/radar-qa-react';
  // A nova guia recebe somente o anúncio da fixture, sem formulário nem envio a terceiros.
  await context.route(anuncio, r => {
    expect(r.request().method()).toBe('GET');
    return r.fulfill({ contentType: 'text/html; charset=utf-8', body: '<h1>Vaga de teste do Applyer</h1>' });
  });
  const externa = context.waitForEvent('page');
  await ficha.getByRole('link', { name: 'Abrir candidatura no navegador' }).click();
  const paginaVaga = await externa;
  await expect(paginaVaga.getByRole('heading', { name: 'Vaga de teste do Applyer' })).toBeVisible();
  await paginaVaga.close();
  await page.reload();
  await expect(ficha.getByText('Aguardando você', { exact: true })).toBeVisible();
  await captura(page, info, 'radar-aguardando');
  expect((await estado()).vagas[0].etapa).not.toBe('enviada');
  await ficha.getByRole('button', { name: 'Já enviei' }).click();
  await expect(ficha.getByText('Candidatura enviada', { exact: true })).toBeVisible();
  await captura(page, info, 'radar-confirmado');
  expect((await estado()).vagas[0].etapa).toBe('enviada');
  const ia = await (await page.request.get('/api/v1/ia')).json();
  expect(ia.provedor).toBe('claude-cli');
  expect(ia.uso.chamadas).toBeGreaterThanOrEqual(3);
  expect(ia.uso.falhas).toBe(0);
  const resultadoPath = info.outputPath('resultado.json');
  await writeFile(resultadoPath, JSON.stringify({
    provedor: info.project.name, chamadas: ia.uso.chamadas, sugestoes: sugestoes.length,
    vaga: 'sintética', grupo: aposBusca.vagas[0].aderencia, confirmacao: 'manual', anuncio: 'guia headless controlada',
  }, null, 2));
  await info.attach('resultado', { contentType: 'application/json', path: resultadoPath });
});
