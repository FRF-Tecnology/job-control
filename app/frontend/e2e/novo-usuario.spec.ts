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

// Os avisos somem sozinhos em ~3 s. Numa máquina lenta, conferir o aviso na tela pode chegar tarde e falhar à toa
// (jc-a2y.55). Cada aviso que aparece fica registrado, e a jornada confere o registro: continua provando que a pessoa
// viu a mensagem, sem depender do relógio.
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    const w = window as any;
    w.__avisos = [];
    new MutationObserver(ms => {
      for (const m of ms) for (const n of m.addedNodes) if (n instanceof HTMLElement && n.classList.contains('aviso')) w.__avisos.push(n.textContent ?? '');
    }).observe(document, { childList: true, subtree: true });
  });
});
const avisou = (page: Page, texto: string) => expect.poll(() => page.evaluate(() => (window as any).__avisos as string[])).toContain(texto);

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
  await avisou(page, 'Perfil salvo');
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
  await avisou(page, 'Movida para Enviada');
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
  await avisou(page, '2 vagas foram para a fila do Claude.');
  await expect(page.locator('.ficha .empresa')).toHaveText('Empresa ext-grupo-b-1');
  expect(await etapas()).toEqual({ 'ext-grupo-a-1': 'aplicar', 'ext-grupo-a-2': 'aplicar', 'ext-grupo-b-1': 'descoberta' });
  const eventos = (await (await page.request.get('/api/v1/estado')).json()).eventos.filter((e: any) => e.tipo === 'aplicar' && e.vagaId.startsWith('ext-grupo-'));
  expect(eventos.map((e: any) => `${e.vagaId}:${e.autor}`).sort()).toEqual(['ext-grupo-a-1:eu', 'ext-grupo-a-2:eu']);

  // 9. Fila (P5, jc-a2y.6.3.1 e .6.3.3): a pessoa prepara a candidatura pela tela, com o pacote vindo do perfil e
  //    da pasta de currículos; a impressão que a tela calcula bate com a do servidor ("Conferida nesta tela");
  //    depois edita o pacote. Currículo e dados fictícios.
  const pasta = process.env.CENTRAL_E2E_PASTA!;
  mkdirSync(join(pasta, 'curriculos'), { recursive: true });
  writeFileSync(join(pasta, 'curriculos', 'curriculo-exemplo.pdf'), '%PDF-1.4\n% curriculo ficticio para teste\n');
  const vagaFila = 'li-fila-exemplo-1';
  const nova = await page.request.post('/api/v1/vagas', { data: { id: vagaFila, empresa: 'Empresa Fila Exemplo', titulo: 'Pessoa Dev Fila', link: 'https://www.linkedin.com/jobs/view/987654321', plataforma: 'linkedin' } });
  expect(nova.status(), await nova.text()).toBe(201);
  const movida = await page.request.post(`/api/v1/vagas/${vagaFila}/mover`, { data: { etapa: 'aplicar' } });
  expect(movida.ok(), await movida.text()).toBe(true);
  await ir(page, 'fila');
  const pronta = page.getByRole('region', { name: 'Prontas para preparar' }).getByRole('listitem').filter({ hasText: 'Empresa Fila Exemplo — Pessoa Dev Fila' });
  await pronta.getByRole('button', { name: 'Preparar candidatura' }).click();
  const form = pronta.getByRole('form', { name: 'Pacote da candidatura' });
  await expect(form.getByLabel('Currículo')).toHaveValue('curriculo-exemplo.pdf');
  await expect(form.getByLabel('Nome')).toHaveValue('Ana Teste'); // do perfil (passo 2)
  await form.getByRole('button', { name: 'Adicionar resposta' }).click();
  await form.getByLabel('Pergunta 1', { exact: true }).fill('Anos com React?');
  await form.getByLabel('Resposta 1', { exact: true }).fill('3');
  await form.getByRole('button', { name: 'Preparar candidatura' }).click();
  await avisou(page, 'Candidatura preparada. Revise o pacote e aprove.');
  const cands = await (await page.request.get('/api/v1/candidaturas')).json();
  const criada = cands.find((c: any) => c.vagaId === vagaFila);
  expect(criada.conteudo.curriculo.arquivo).toBe('curriculo-exemplo.pdf');
  expect(criada.conteudo.respostas).toEqual([{ pergunta: 'Anos com React?', resposta: '3', origem: 'pessoa' }]);
  const naFila = page.getByTestId(criada.id);
  await expect(naFila.getByText('Para revisar')).toBeVisible();
  // O selo fica à vista; o código, recolhido em "Detalhes técnicos" (jc-a2y.6.3.5).
  await expect(naFila.getByText('Conferida nesta tela')).toBeVisible();
  await expect(naFila.getByText(criada.impressao)).toBeHidden();
  await naFila.getByText('Detalhes técnicos').click();
  await expect(naFila.getByText(criada.impressao)).toBeVisible();
  // Editar: a mensagem muda, a impressão muda e continua conferida.
  await naFila.getByRole('button', { name: 'Editar pacote' }).click();
  await naFila.getByLabel('Mensagem').fill('Olá, tenho interesse na vaga.');
  await naFila.getByRole('button', { name: 'Salvar pacote' }).click();
  await avisou(page, 'Pacote atualizado.');
  const editada = (await (await page.request.get(`/api/v1/candidaturas/${criada.id}`)).json());
  expect(editada.conteudo.mensagem).toBe('Olá, tenho interesse na vaga.');
  expect(editada.impressao).not.toBe(criada.impressao);
  await expect(naFila.getByText('Conferida nesta tela')).toBeVisible();
  await naFila.getByText('Detalhes técnicos').click();
  await expect(naFila.getByText(editada.impressao)).toBeVisible();
  await expect(page.getByRole('region', { name: 'Limite de hoje' }).getByText('LinkedIn')).toBeVisible();

  // 10. Aprovar (jc-a2y.6.3.2): a tela manda a impressão que ela calculou; o servidor só aceita se for a atual.
  //     Depois tira a aprovação e cancela, cada um com o efeito conferido na API.
  const estadoNaApi = async () => (await (await page.request.get(`/api/v1/candidaturas/${criada.id}`)).json()).estado;
  await naFila.getByRole('button', { name: 'Aprovar este pacote' }).click();
  const dlgFila = page.locator('dialog#dialogo');
  await expect(dlgFila.getByRole('heading')).toHaveText('Aprovar e enviar este pacote?');
  await dlgFila.getByRole('button', { name: 'Aprovar e enviar' }).click();
  await avisou(page, 'Aprovado. O app envia este pacote hoje, uma vez.');
  expect(await estadoNaApi()).toBe('aprovada');
  await expect(naFila.getByText(/Aprovada até/)).toBeVisible();
  await naFila.getByRole('button', { name: 'Tirar aprovação' }).click();
  await avisou(page, 'Aprovação retirada. O pacote voltou para revisão.');
  expect(await estadoNaApi()).toBe('preparada');
  await naFila.getByRole('button', { name: 'Cancelar candidatura' }).click();
  await dlgFila.getByRole('button', { name: 'Cancelar candidatura' }).click();
  await avisou(page, 'Candidatura cancelada.');
  expect(await estadoNaApi()).toBe('cancelada');
  await expect(naFila.getByText('Cancelada', { exact: true })).toBeVisible();
});

test('celular: todas as telas cabem em 375 px e as abas funcionam', async ({ page }, info) => {
  test.skip(info.project.name !== 'celular', 'conferência de layout no celular');
  // O projeto celular é um Pixel 7 (412 px); a meta é a tela menor, 375 px (jc-a2y.53).
  await page.setViewportSize({ width: 375, height: 812 });
  for (const rota of ['hoje', 'triagem', 'fila', 'quadro', 'vagas', 'numeros', 'plataformas', 'rodadas', 'perfil']) {
    await ir(page, rota);
    const rolaDeLado = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    expect(rolaDeLado, `${rota} rola para o lado`).toBe(false);
    // No celular, uma página mais larga que a tela não rola: o navegador reduz o zoom e innerWidth cresce junto,
    // então a conta acima não vê. A largura de layout tem de ser a da tela (jc-a2y.53).
    expect(await page.evaluate(() => innerWidth), `${rota} é mais larga que a tela`).toBe(375);
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
