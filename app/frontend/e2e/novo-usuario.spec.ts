// Jornada de quem acabou de instalar: conta vazia → perfil → vaga → triagem → quadro → envio → números.
// Roda no PC e no celular (playwright.config.ts). Cada projeto usa a mesma pasta de dados, então
// a jornada é feita uma vez (PC) e o celular confere as telas com os dados que ela deixou.
import { expect, test, type Page } from '@playwright/test';

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
});

test('celular: todas as telas cabem em 375 px e as abas funcionam', async ({ page }, info) => {
  test.skip(info.project.name !== 'celular', 'conferência de layout no celular');
  for (const rota of ['hoje', 'triagem', 'quadro', 'vagas', 'numeros', 'plataformas', 'rodadas', 'perfil']) {
    await ir(page, rota);
    const rolaDeLado = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
    expect(rolaDeLado, `${rota} rola para o lado`).toBe(false);
  }
  await ir(page, 'hoje');
  await page.locator('nav.abas').getByRole('button', { name: 'Mais' }).click();
  await page.locator('.folha-mais a', { hasText: 'Números' }).click();
  await expect(page).toHaveURL(/#\/numeros$/);
});
