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
  await page.getByRole('button', { name: 'Minha assinatura' }).click();
  await captura(page, info, 'primeiro-acesso');
  if (!radar) {
    const tema = page.getByRole('button', { name: 'Alternar tema claro/escuro' });
    await expect(page.locator('html')).toHaveAttribute('data-tema', 'escuro');
    await tema.click();
    await expect(page.locator('html')).toHaveAttribute('data-tema', 'claro');
    await captura(page, info, 'primeiro-acesso-claro');
    await tema.click();
    await expect(page.locator('html')).toHaveAttribute('data-tema', 'escuro');
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
  const stack = page.getByLabel(/Competências principais/);
  await expect(stack).toHaveValue(/React|Java/i);
  await campoCargo.fill('Desenvolvimento de software');
  await stack.fill('React, Java');
  await page.getByLabel('Pleno', { exact: true }).check();
  await page.getByLabel('Os dois', { exact: true }).check();
  await page.getByLabel('Remoto', { exact: true }).check();
  await page.getByLabel('País de trabalho').selectOption('BR');
  await page.getByLabel('Seu nível de inglês').selectOption('Avançado');
  await page.getByLabel('Moeda').selectOption('USD');
  await page.getByLabel('Mínimo').fill('4000');
  await page.getByLabel('Máximo').fill('6000');
  if (!radar) {
    await page.getByLabel('Máximo').blur();
    await expect(page.getByText('Salvo neste computador', {exact:true})).toBeVisible();
    await page.reload();
    await expect(page.getByText('curriculo-teste.pdf', {exact:true})).toBeVisible();
    await expect(page.getByText('Claude conectado', {exact:true})).toHaveCount(0);
    await page.getByRole('button', {name:'Usar conexão atual'}).click();
    await expect(campoCargo).toHaveValue('Desenvolvimento de software');
    await expect(page.getByLabel('Mínimo')).toHaveValue('4000');
    await expect(page.getByLabel('Máximo')).toHaveValue('6000');
  }
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
    foco: 'ambos', paisTrabalho:'BR', ingles: 'Avançado', faixaSalarial: { moeda: 'USD', min: 4000, max: 6000 },
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
  expect(aposBusca.vagas[0]).toMatchObject({aderencia:'A',radarVersao:1,radarClasse:'forte'});
  expect(aposBusca.vagas[0].etapa).toBe('descoberta');
  const ficha = page.getByRole('article', { name: 'Desenvolvimento de software com React' });
  const listaRadar = page.getByRole('list', { name: 'Vagas para revisão' });
  const linha = listaRadar.getByRole('button', { name: /Desenvolvimento de software com React/ });
  await expect(linha).toHaveAttribute('aria-current', 'true');
  await page.getByRole('button', { name: 'A conferir', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Nenhuma vaga a conferir' })).toBeVisible();
  await page.getByRole('button', { name: 'Recomendadas', exact: true }).click();
  await expect(ficha).toBeVisible();
  await page.setViewportSize({ width: 375, height: 812 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(375);
  await expect(linha).toBeVisible();
  await expect(ficha).toBeHidden();
  await linha.click();
  await expect(ficha).toBeVisible();
  await expect(listaRadar).toBeHidden();
  await ficha.getByRole('button', { name: 'Voltar às vagas' }).click();
  await expect(linha).toBeFocused();
  await page.setViewportSize({ width: 1366, height: 850 });
  await captura(page, info, 'radar-classificacao');
  await ficha.getByRole('button', { name: 'Aplicar na vaga' }).click();
  await expect(ficha.getByText('Aguardando você', { exact: true })).toBeVisible();
  const anuncio = 'https://vaga.example.invalid/radar-qa-react';
  const nomeTeste = 'Pessoa de Teste';
  let recebimentos = 0;
  // A empresa fictícia vive no interceptador. O Chromium abre a vaga e submete o formulário
  // multipart como faria no site de uma empresa, mas os bytes só chegam a este teste.
  await context.route(anuncio, r => {
    expect(r.request().method()).toBe('GET');
    return r.fulfill({ contentType: 'text/html; charset=utf-8', body: `<!doctype html>
      <html lang="pt-BR"><meta charset="utf-8"><title>Candidatura de teste</title>
      <h1>Vaga de teste do Applyer</h1><p>Oficina de Software Fictícia</p>
      <form method="post" action="/recibo" enctype="multipart/form-data">
        <p><label>Nome <input name="nome" required></label></p>
        <p><label>Currículo <input type="file" name="curriculo" accept="application/pdf" required></label></p>
        <button>Enviar candidatura de teste</button>
      </form></html>` });
  });
  await context.route('https://vaga.example.invalid/recibo', async r => {
    const pedido = r.request();
    expect(pedido.method()).toBe('POST');
    const corpo = pedido.postDataBuffer();
    if (!corpo) throw new Error('O formulário chegou sem conteúdo.');
    const formulario = await new Response(new Uint8Array(corpo), { headers: {
      'Content-Type': pedido.headers()['content-type'],
    } }).formData();
    expect(formulario.get('nome')).toBe(nomeTeste);
    const arquivo = formulario.get('curriculo');
    if (!(arquivo instanceof File)) throw new Error('O formulário não recebeu o currículo.');
    expect(arquivo.name).toBe('curriculo-teste.pdf');
    expect(arquivo.type).toBe('application/pdf');
    expect(Buffer.from(await arquivo.arrayBuffer()).equals(pdf), 'PDF recebido idêntico ao anexado').toBe(true);
    recebimentos++;
    await r.fulfill({ contentType: 'text/html; charset=utf-8', body: `<!doctype html>
      <html lang="pt-BR"><meta charset="utf-8"><title>Recibo de teste</title>
      <h1>Candidatura de teste recebida</h1><p>Oficina de Software Fictícia</p>
      <p>Protocolo QA-001</p><p>Nome: Pessoa de Teste</p><p>Currículo: curriculo-teste.pdf</p></html>` });
  });
  const externa = context.waitForEvent('page');
  await ficha.getByRole('link', { name: 'Abrir candidatura no navegador' }).click();
  const paginaVaga = await externa;
  await expect(paginaVaga.getByRole('heading', { name: 'Vaga de teste do Applyer' })).toBeVisible();
  await paginaVaga.getByLabel('Nome').fill(nomeTeste);
  await paginaVaga.getByLabel('Currículo').setInputFiles({ name: 'curriculo-teste.pdf', mimeType: 'application/pdf', buffer: pdf });
  await paginaVaga.getByRole('button', { name: 'Enviar candidatura de teste' }).click();
  await expect(paginaVaga.getByRole('heading', { name: 'Candidatura de teste recebida' })).toBeVisible();
  await expect(paginaVaga.getByText('Protocolo QA-001')).toBeVisible();
  expect(recebimentos).toBe(1);
  await captura(paginaVaga, info, 'recibo-candidatura');
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
    formulario: { recebimentos, protocolo: 'QA-001', pdfConferido: true, bytesPDF: pdf.length, metodo: 'POST multipart/form-data' },
  }, null, 2));
  await info.attach('resultado', { contentType: 'application/json', path: resultadoPath });
});
