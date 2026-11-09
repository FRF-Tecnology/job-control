// Teclado nas telas novas (jc-a2y.59): quando um botão some da tela depois da ação (o formulário fecha, Pausar vira
// Retomar), o foco vai para um lugar que faz sentido, e não para o começo da página. Esc fecha diálogo e formulário e
// devolve o foco ao botão que os abriu. Fila e envio interceptados com dados fictícios.
import { expect, test, type Locator, type Page } from '@playwright/test';
import { ENVIO, interceptar } from './dados-fila';

test.skip(({ isMobile }) => isMobile, 'teclado: só no PC');

const focado = (alvo: Locator) => expect(alvo).toBeFocused();


async function abrirFila(page: Page) {
  await interceptar(page);
  await page.goto('/#/fila');
  await page.getByTestId('cand-prep').getByText('Conferida nesta tela').waitFor();
}

test('limite: abrir leva o foco ao campo; Esc e Cancelar devolvem ao botão', async ({ page }) => {
  await abrirFila(page);
  const regiao = page.getByRole('region', { name: 'Limite de hoje' });
  const ajustar = regiao.getByRole('button', { name: 'Ajustar limite na Gupy' });
  await ajustar.focus();
  await page.keyboard.press('Enter');
  await focado(regiao.getByLabel('Limite por dia, de 1 a 40'));
  await page.keyboard.press('Escape');
  await focado(ajustar);
  await page.keyboard.press('Enter');
  await regiao.getByRole('button', { name: 'Cancelar' }).click();
  await focado(ajustar);
});

test('envio: depois de pausar, o foco fica no "Retomar envio"', async ({ page }) => {
  await abrirFila(page);
  await page.route('**/api/v1/envio/pausar', r => r.fulfill({ json: { ...ENVIO, pausado: true } }));
  const painel = page.locator('section.envio');
  await painel.getByRole('button', { name: 'Pausar envio' }).focus();
  await page.keyboard.press('Enter');
  await focado(painel.getByRole('button', { name: 'Retomar envio' }));
});

test('aprovar: Esc fecha o diálogo e o foco volta ao botão', async ({ page }) => {
  await abrirFila(page);
  const aprovar = page.getByTestId('cand-prep').getByRole('button', { name: 'Aprovar este pacote' });
  await aprovar.focus();
  await page.keyboard.press('Enter');
  const dlg = page.locator('dialog#dialogo');
  await expect(dlg).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dlg).toBeHidden();
  await focado(aprovar);
});

test('editar pacote: abrir leva o foco ao formulário; Cancelar devolve ao botão', async ({ page }) => {
  await abrirFila(page);
  const cartao = page.getByTestId('cand-prep');
  const editar = cartao.getByRole('button', { name: 'Editar pacote' });
  await editar.focus();
  await page.keyboard.press('Enter');
  const primeiro = cartao.locator('form select, form input, form textarea').first();
  await focado(primeiro);
  await cartao.getByRole('button', { name: 'Cancelar' }).click();
  await focado(cartao.getByRole('button', { name: 'Editar pacote' }));
});
