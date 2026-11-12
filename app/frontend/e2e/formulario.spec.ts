// Formulário externo na Fila (jc-a2y.7.2.4) contra dist/. A leitura só existe no app com o envio (404 no central-dev),
// então a candidatura de formulário vem interceptada, e o PUT /conteudo também (este teste não grava nada). Confere:
// axe nos dois temas, 375 px sem rolagem lateral e alvos de 44 px, o corpo do responder e o aprovar travado.
import AxeBuilder from '@axe-core/playwright';
import { temaDeTeste } from './tema';
import { expect, test, type Page } from '@playwright/test';
import { alterarEstado, soltarRotas } from './dados-fila';

test.afterEach(async ({ page }) => { await soltarRotas(page); });

const URL_FORM = 'https://docs.google.com/forms/d/e/abc/viewform';
const CONTEUDO = { curriculo: { arquivo: 'cv.pdf' }, idioma: 'pt', respostas: [{ pergunta: 'Nível de inglês', resposta: 'Avançado', origem: 'respostasPadrao', perguntaId: 'q3' }] };
const ESTRUTURA = { hash: 'sha256:v1:ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd', urlFinal: URL_FORM, perguntas: [
  { id: 'q1', texto: 'Qual o seu e-mail?', tipo: 'texto-curto', obrigatoria: true },
  { id: 'q3', texto: 'Nível de inglês', tipo: 'escolha-unica', obrigatoria: true, opcoes: ['Básico', 'Intermediário', 'Avançado'] },
  { id: 'q5', texto: 'Estado', tipo: 'lista', obrigatoria: true, opcoes: ['SP', 'RJ', 'MG'] },
  { id: 'q7', texto: 'Comentários', tipo: 'texto-longo', obrigatoria: false },
] };
const CAND = {
  id: 'cand-form', vagaId: 'ext-form-1', plataforma: 'formulario', url: URL_FORM, estado: 'preparada', conteudo: CONTEUDO,
  impressao: 'sha256:v1:' + '1'.repeat(64), aprovacao: null, bloqueio: null, resultado: null, versao: 4,
  criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00',
  estrutura: ESTRUTURA, pendentes: ['q1', 'q5'], preenchimentoIgnorado: true,
};

async function preparar(page: Page) {
  await alterarEstado(page, e => ({ ...e, vagas: [...e.vagas, { id: 'ext-form-1', empresa: 'Empresa Teste', titulo: 'Dev Go', link: URL_FORM, plataforma: 'formulario', etapa: 'aplicar' }] }));
  await page.route('**/api/v1/candidaturas', r => r.request().method() === 'GET' ? r.fulfill({ json: [CAND] }) : r.fulfill({ status: 409, json: { erro: 'teste: não grava' } }));
  await page.goto('/#/fila');
  await page.getByTestId('cand-form').getByRole('heading', { name: 'Perguntas do formulário (4)' }).waitFor();
}

for (const tema of ['light', 'dark'] as const) {
  test(`formulário lido com pendentes (${tema === 'light' ? 'claro' : 'escuro'}): axe limpo, cabe na tela, aprovar travado`, async ({ page }, info) => {
    await temaDeTeste(page, tema);
    await preparar(page);
    const c = page.getByTestId('cand-form');
    await expect(c.getByText('O link trazia respostas pré-preenchidas; o app ignorou.')).toBeVisible();
    await expect(c.getByText('Faltam responder 2 perguntas obrigatórias para aprovar.')).toBeVisible();
    await expect(c.getByRole('button', { name: 'Aprovar este pacote' })).toBeDisabled();
    await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity));
    const v = (await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).include('[data-testid="cand-form"] .form-ext').analyze()).violations;
    expect(v.map(x => `${x.id}: ${x.nodes.map(n => n.target.join(' ')).join(' | ')}`)).toEqual([]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    if (info.project.name === 'celular') {
      const pequenos = await c.locator('.form-ext').locator('button, input, select').evaluateAll(els => els.filter(e => e.getBoundingClientRect().height < 43.5).map(e => e.outerHTML.slice(0, 60)));
      expect(pequenos).toEqual([]);
    }
  });
}

test('responder as pendentes manda o pacote com perguntaId, origem pessoa e a versão (interceptado: não grava)', async ({ page }) => {
  await preparar(page);
  let corpo: any = null;
  await page.route('**/api/v1/candidaturas/cand-form/conteudo', r => { corpo = r.request().postDataJSON(); return r.fulfill({ json: CAND }); });
  const c = page.getByTestId('cand-form');
  await c.getByRole('textbox', { name: 'Resposta: Qual o seu e-mail?' }).fill('pessoa.teste@exemplo.com');
  await c.getByRole('combobox', { name: 'Resposta: Estado' }).selectOption('SP');
  await c.getByRole('button', { name: 'Salvar 2 respostas' }).click();
  await expect.poll(() => corpo).not.toBeNull();
  expect(corpo.versao).toBe(4);
  expect(corpo.conteudo.respostas).toEqual([
    CONTEUDO.respostas[0],
    { pergunta: 'Qual o seu e-mail?', resposta: 'pessoa.teste@exemplo.com', origem: 'pessoa', perguntaId: 'q1' },
    { pergunta: 'Estado', resposta: 'SP', origem: 'pessoa', perguntaId: 'q5' },
  ]);
});
