// Seus pedidos atendidos pelo app (jc-a2y.7.1.1): os pedidos fictícios entram só na resposta do GET /estado, e as
// rotas que gravariam (interpretar, POST /vagas) são interceptadas: este teste não grava nada no servidor. Confere axe
// nos dois temas, 375 px sem rolagem lateral, os cabeçalhos do Confirmar e "Abrir a Fila" até o cartão da vaga.
import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { interceptar } from './dados-fila';

const IMP = 'sha256:v1:' + 'ab'.repeat(32);
const CHAMADA = { metodo: 'POST', caminho: '/vagas', corpo: { link: 'https://vagas.exemplo.com/1', empresa: 'Empresa Nova Exemplo', titulo: 'Pessoa Dev' } };
const pedidos = (hoje: string) => [
  { id: 'ped-ab', texto: 'Quantas vagas estão em processo?', status: 'aberto', criadoEm: hoje },
  { id: 'ped-pr', texto: 'Adiciona https://vagas.exemplo.com/1 da Empresa Nova Exemplo', status: 'proposta', criadoEm: hoje,
    proposta: { acao: 'adicionar-vaga', resumo: 'Adicionar uma vaga em Descoberta com o link de vagas.exemplo.com (Outra plataforma).', chamada: CHAMADA, impressao: IMP, falta: ['A triagem da vaga continua com você.'], criadaEm: hoje } },
  { id: 'ped-re', texto: 'Aprova todas as vagas', status: 'recusado', criadoEm: hoje,
    proposta: { acao: 'recusar', resumo: 'Isso só você faz, na tela da Fila.', chamada: null, impressao: IMP, falta: [], criadaEm: hoje } },
  { id: 'ped-fa', texto: 'Prepara o pacote da Empresa Exemplo', status: 'feito', concluidoEm: hoje, resultado: { vagaId: 'ext-exemplo-1', abrir: '#/fila?preparar=ext-exemplo-1' },
    proposta: { acao: 'preparar-pacote', resumo: 'Abrir a Fila para preparar o pacote da vaga Empresa Exemplo — Pessoa Dev React.', chamada: null, impressao: IMP, falta: [], criadaEm: hoje } },
  { id: 'ped-rs', texto: 'Quem me respondeu esta semana?', status: 'feito', concluidoEm: hoje, resposta: 'Ninguém respondeu ainda nesta semana. <b>Calma</b>, https://exemplo.com/x',
    proposta: { acao: 'responder', resumo: 'Resposta da IA (só leitura).', chamada: null, impressao: IMP, falta: [], criadaEm: hoje } },
];

async function preparar(page: Page) {
  await interceptar(page); // a vaga ext-exemplo-1 (Aplicar) e a fila falsa, que tem a candidatura dela
  await page.route('**/api/v1/estado', async r => {
    const res = await r.fetch();
    const estado = await res.json();
    const vaga = { id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', link: 'https://www.linkedin.com/jobs/view/123', plataforma: 'linkedin', etapa: 'aplicar', aderencia: 'A' };
    r.fulfill({ response: res, json: { ...estado, vagas: [...estado.vagas, vaga], pedidos: [...(estado.pedidos ?? []), ...pedidos(estado.hoje)] } });
  });
  await page.route('**/api/v1/ia/config', r => r.fulfill({ json: { configurada: true, provedor: 'claude-cli' } }));
  await page.goto('/#/hoje');
  await page.getByRole('region', { name: 'Seus pedidos' }).getByText('Proposta da IA').waitFor();
}
const axe = async (page: Page) => {
  await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity));
  return new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).include('.pedidos');
};

for (const tema of ['light', 'dark'] as const) {
  test(`Seus pedidos: axe sem violações no tema ${tema === 'light' ? 'claro' : 'escuro'}`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: tema });
    await preparar(page);
    const v = (await (await axe(page)).analyze()).violations;
    expect(v.map(x => `${x.id}: ${x.nodes.map(n => n.target.join(' ')).join(' | ')}`)).toEqual([]);
  });
}

test('Seus pedidos: cabe na tela sem rolagem lateral, alvos de 44 px no celular e o texto da IA fica texto', async ({ page }, info) => {
  await preparar(page);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  if (info.project.name === 'celular') {
    const pequenos = await page.locator('.pedidos button, .pedidos a[href]').evaluateAll(els => els
      .map(el => ({ t: el.textContent, r: el.getBoundingClientRect() })).filter(x => x.r.height < 43.5 || x.r.width < 43.5).map(x => x.t));
    expect(pequenos).toEqual([]);
  }
  const resposta = page.getByRole('region', { name: 'Seus pedidos' }).locator('.resposta-ia');
  await expect(resposta).toContainText('<b>Calma</b>');
  expect(await resposta.locator('b, a').count()).toBe(0);
});

test('Confirmar manda o corpo exato com X-Pedido e X-Impressao (interceptado: não grava)', async ({ page }) => {
  await preparar(page);
  let pedido: { corpo: unknown; cabecalhos: Record<string, string> } | null = null;
  await page.route('**/api/v1/vagas', r => {
    if (r.request().method() !== 'POST') return r.fallback();
    pedido = { corpo: r.request().postDataJSON(), cabecalhos: r.request().headers() };
    return r.fulfill({ status: 201, json: { id: 'ext-nova-1' } });
  });
  await page.getByRole('region', { name: 'Seus pedidos' }).getByRole('button', { name: 'Confirmar' }).click();
  await expect.poll(() => pedido).not.toBeNull();
  expect(pedido!.corpo).toEqual(CHAMADA.corpo);
  expect(pedido!.cabecalhos['x-pedido']).toBe('ped-pr');
  expect(pedido!.cabecalhos['x-impressao']).toBe(IMP);
});

test('"Abrir a Fila" do pedido leva à Fila, no cartão da vaga, e limpa o endereço', async ({ page }) => {
  await preparar(page);
  await page.getByRole('region', { name: 'Seus pedidos' }).getByRole('link', { name: 'Abrir a Fila' }).click();
  await expect(page.getByTestId('cand-prep')).toBeInViewport();
  await expect(page).toHaveURL(/#\/fila$/);
});
