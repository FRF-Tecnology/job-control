// Jornada de quem acabou de instalar: conta vazia → perfil → vaga → triagem → quadro → envio → números.
// Roda no PC e no celular (playwright.config.ts). Cada projeto usa a mesma pasta de dados, então
// a jornada é feita uma vez (PC) e o celular confere as telas com os dados que ela deixou.
import { expect, test, type Page } from '@playwright/test';
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const ir = async (page: Page, rota: string) => {
  await page.goto(`/#/${rota}`);
  await expect(page.locator('main#conteudo')).not.toBeEmpty();
};

// Responde o diálogo genérico (campo de texto) com um valor.
async function responder(page: Page, valor: string) {
  const dlg = page.locator('dialog#dialogo');
  await expect(dlg).toBeVisible();
  await dlg.locator('input, textarea').fill(valor);
  await dlg.locator('button[type=submit]').click();
}

test.describe.configure({ mode: 'serial' });

test('PC: conta nova até a primeira candidatura', async ({ page }, info) => {
  test.skip(info.project.name !== 'pc', 'a jornada grava dados; roda só uma vez');

  // 1. Primeiro acesso: guia "Primeiros passos" com 0 de 3 e plataformas iniciais já cadastradas.
  await ir(page, 'hoje');
  await expect(page.getByRole('heading', { name: 'Olá.' })).toBeVisible();
  // O selo do topo é a conexão com os dados locais, não a IA (jc-a2y.38).
  await expect(page.locator('.status-conexao')).toHaveText('Dados locais');
  await expect(page.getByText('0 de 3 prontos')).toBeVisible();
  await ir(page, 'plataformas');
  await expect(page.locator('article.plataforma').first()).toBeVisible();

  // 2. Perfil: nome e salvar. O passo "Preencher o perfil" fica pronto.
  await ir(page, 'perfil');
  await page.locator('#perfil-nome').fill('Ana Teste');
  await page.locator('form.formulario button[type=submit]').first().click();
  await expect(page.getByText('Perfil salvo')).toBeVisible();
  await ir(page, 'hoje');
  await expect(page.getByRole('heading', { name: 'Olá, Ana.' })).toBeVisible();
  await expect(page.getByText('1 de 3 prontos')).toBeVisible();

  // 3. Pedido pela caixa "O que vamos fazer hoje?".
  await page.getByLabel('O que vamos fazer hoje?').fill('Buscar vagas de React remotas');
  await page.getByLabel('O que vamos fazer hoje?').press('Enter');
  await expect(page.locator('li.pedido strong', { hasText: 'Buscar vagas de React remotas' })).toBeVisible();

  // 4. Vaga adicionada à mão (3 perguntas).
  await ir(page, 'vagas');
  await page.getByRole('button', { name: 'Adicionar vaga' }).click();
  await responder(page, 'https://www.linkedin.com/jobs/view/123');
  await responder(page, 'Acme Remote');
  await responder(page, 'Senior React Developer');
  await expect(page.locator('aside.gaveta h2')).toHaveText('Senior React Developer');
  await page.keyboard.press('Escape');

  // 5. Triagem: tecla A aprova.
  await ir(page, 'triagem');
  await expect(page.locator('.ficha .empresa')).toHaveText('Acme Remote');
  await page.keyboard.press('a');
  await expect(page.getByText('Tudo decidido')).toBeVisible();

  // 6. Quadro: a vaga está em "Aplicar". Detalhe → Mover para Enviada.
  await ir(page, 'quadro');
  const coluna = page.locator('section.coluna.e-aplicar');
  await expect(coluna.locator('article.cartao')).toHaveCount(1);
  await coluna.locator('article.cartao').click();
  await page.locator('aside.gaveta select').first().selectOption('enviada');
  await expect(page.getByText('Movida para Enviada')).toBeVisible();
  await expect(page.locator('aside.gaveta .etapa')).toHaveText('Enviada');

  // 7. Hoje e Números contam o envio.
  await ir(page, 'hoje');
  await expect(page.locator('.cartao-num', { hasText: 'enviadas hoje' }).locator('.n')).toHaveText('1');
  await ir(page, 'numeros');
  await expect(page.getByText('1 enviadas, 0 com retorno (0%)')).toBeVisible();

  // 8. Triagem: "Aprovar todas do grupo A" mostra a lista e a contagem; cancelar não muda nada;
  //    confirmar aprova só as A, com autor "eu" (jc-a2y.31). Vagas fictícias criadas pela API.
  const novas: [string, string][] = [['ext-grupo-a-1', 'A'], ['ext-grupo-a-2', 'A'], ['ext-grupo-b-1', 'B']];
  for (const [id, aderencia] of novas) {
    const r = await page.request.post('/api/v1/vagas', { data: { id, empresa: `Empresa ${id}`, titulo: 'Pessoa Dev Exemplo', link: `https://vagas.exemplo.com/${id}`, plataforma: 'externo', aderencia } });
    expect(r.status(), await r.text()).toBe(201);
  }
  const etapas = async () => {
    const e = await (await page.request.get('/api/v1/estado')).json();
    return Object.fromEntries(novas.map(([id]) => [id, e.vagas.find((v: any) => v.id === id)?.etapa]));
  };
  await ir(page, 'triagem');
  const aprovarTodas = page.getByRole('button', { name: 'Aprovar todas do grupo A (2)' });
  const dlg = page.locator('dialog#dialogo');
  await aprovarTodas.click();
  await expect(dlg.getByRole('heading')).toHaveText('Aprovar 2 vagas do grupo A?');
  await expect(dlg.getByRole('listitem')).toHaveCount(2);
  await expect(dlg.getByRole('list')).toContainText('Empresa ext-grupo-a-1 — Pessoa Dev Exemplo');
  await expect(dlg.getByRole('list')).toContainText('Empresa ext-grupo-a-2 — Pessoa Dev Exemplo');
  await expect(dlg.getByRole('list')).not.toContainText('ext-grupo-b-1');
  await dlg.getByRole('button', { name: 'Cancelar' }).click();
  await expect(dlg).toBeHidden();
  expect(await etapas()).toEqual({ 'ext-grupo-a-1': 'descoberta', 'ext-grupo-a-2': 'descoberta', 'ext-grupo-b-1': 'descoberta' });

  await aprovarTodas.click();
  await dlg.getByRole('button', { name: 'Aprovar 2 vagas' }).click();
  await expect(page.getByText('2 vagas foram para a fila do Claude.')).toBeVisible();
  await expect(page.locator('.ficha .empresa')).toHaveText('Empresa ext-grupo-b-1');
  expect(await etapas()).toEqual({ 'ext-grupo-a-1': 'aplicar', 'ext-grupo-a-2': 'aplicar', 'ext-grupo-b-1': 'descoberta' });
  const eventos = (await (await page.request.get('/api/v1/estado')).json()).eventos.filter((e: any) => e.tipo === 'aplicar' && e.vagaId.startsWith('ext-grupo-'));
  expect(eventos.map((e: any) => `${e.vagaId}:${e.autor}`).sort()).toEqual(['ext-grupo-a-1:eu', 'ext-grupo-a-2:eu']);

  // 9. Fila (P5, jc-a2y.6.3.1): uma candidatura criada no servidor aparece com o pacote exato, e a impressão que a
  //    tela calcula bate com a do servidor ("Conferida nesta tela"). Currículo e dados fictícios.
  const pasta = process.env.CENTRAL_E2E_PASTA!;
  mkdirSync(join(pasta, 'curriculos'), { recursive: true });
  writeFileSync(join(pasta, 'curriculos', 'curriculo-exemplo.pdf'), '%PDF-1.4\n% curriculo ficticio para teste\n');
  const vagaFila = 'li-fila-exemplo-1';
  const nova = await page.request.post('/api/v1/vagas', { data: { id: vagaFila, empresa: 'Empresa Fila Exemplo', titulo: 'Pessoa Dev Fila', link: 'https://www.linkedin.com/jobs/view/987654321', plataforma: 'linkedin' } });
  expect(nova.status(), await nova.text()).toBe(201);
  const movida = await page.request.post(`/api/v1/vagas/${vagaFila}/mover`, { data: { etapa: 'aplicar' } });
  expect(movida.ok(), await movida.text()).toBe(true);
  const r = await page.request.post('/api/v1/candidaturas', { data: { vagaId: vagaFila, conteudo: {
    curriculo: { arquivo: 'curriculo-exemplo.pdf' }, idioma: 'pt', contato: { nome: 'Ana Teste', email: 'ana@exemplo.com' },
    pretensao: { valor: 5000, moeda: 'BRL', tipo: 'CLT' }, respostas: [{ pergunta: 'Anos com React?', resposta: '3', origem: 'pessoa' }], mensagem: null,
  } } });
  expect(r.status(), await r.text()).toBe(201);
  const criada = await r.json();
  await ir(page, 'fila');
  const naFila = page.getByTestId(criada.id);
  await expect(naFila.getByText('Para revisar')).toBeVisible();
  await expect(naFila.getByText('Empresa Fila Exemplo — Pessoa Dev Fila')).toBeVisible();
  await expect(naFila.getByText('Ana Teste · ana@exemplo.com')).toBeVisible();
  await expect(naFila.getByText(criada.impressao)).toBeVisible();
  await expect(naFila.getByText('Conferida nesta tela')).toBeVisible();
  await expect(page.getByRole('region', { name: 'Limite de hoje' }).getByText('LinkedIn')).toBeVisible();
});

test('celular: todas as telas cabem em 375 px e as abas funcionam', async ({ page }, info) => {
  test.skip(info.project.name !== 'celular', 'conferência de layout no celular');
  for (const rota of ['hoje', 'triagem', 'fila', 'quadro', 'vagas', 'numeros', 'plataformas', 'rodadas', 'perfil']) {
    await ir(page, rota);
    const rolaDeLado = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    expect(rolaDeLado, `${rota} rola para o lado`).toBe(false);
    // Nenhum texto passa da borda do seu bloco (ex.: caminho longo de pasta num card; jc-a2y.47).
    // Blocos com rolagem própria (overflow auto/hidden, como o Quadro) ficam de fora.
    const vazam = await page.evaluate(() => [...document.querySelectorAll<HTMLElement>('main#conteudo *')].flatMap(el => {
      const e = getComputedStyle(el);
      if (e.display === 'inline' || e.overflowX !== 'visible' || !el.clientWidth || el.closest('[hidden]')) return [];
      if ([...el.querySelectorAll<HTMLElement>('*')].some(f => getComputedStyle(f).overflowX !== 'visible')) return [];
      return el.scrollWidth > el.clientWidth + 1 ? [`${el.tagName.toLowerCase()}.${[...el.classList].join('.')} "${(el.textContent ?? '').trim().slice(0, 40)}" ${el.scrollWidth}>${el.clientWidth}`] : [];
    }));
    expect(vazam, `${rota}: texto vaza do bloco`).toEqual([]);
    // Rolando até o fim, o conteúdo termina acima da barra de abas fixa (a barra não cobre nada).
    const cobre = await page.evaluate(() => {
      scrollTo(0, document.documentElement.scrollHeight);
      const abas = document.querySelector('nav.abas')?.getBoundingClientRect();
      const ultimo = [...document.querySelectorAll('main#conteudo > *')].at(-1)?.getBoundingClientRect();
      return abas && ultimo ? ultimo.bottom > abas.top + 0.5 : false;
    });
    expect(cobre, `${rota}: a barra de abas cobre o fim do conteúdo`).toBe(false);
  }
  await ir(page, 'hoje');
  await page.locator('nav.abas').getByRole('button', { name: 'Mais' }).click();
  await page.locator('.folha-mais a', { hasText: 'Números' }).click();
  await expect(page).toHaveURL(/#\/numeros$/);
});
