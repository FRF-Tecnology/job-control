// Acessibilidade das telas novas (jc-a2y.59): axe (WCAG 2.1 A/AA) em cada estado da Fila, do painel de envio, do
// preparar/editar pacote, do limite diário e de "Seus dados", nos temas claro e escuro, no PC e no celular. A fila e
// o envio vêm interceptados com dados fictícios, para cobrir os estados que o central-dev não produz.
import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { interceptar, soltarRotas } from './dados-fila';

test.afterEach(async ({ page }) => { await soltarRotas(page); });

const axe = async (page: Page) => {
  await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity));
  return new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']);
};
const resumo = (v: Awaited<ReturnType<AxeBuilder['analyze']>>['violations']) =>
  v.map(x => `${x.id} (${x.impact}): ${x.nodes.slice(0, 3).map(n => n.target.join(' ') + ' — ' + (n.any[0]?.message ?? '')).join(' | ')}`);


for (const tema of ['light', 'dark'] as const) {
  test.describe(`tema ${tema === 'light' ? 'claro' : 'escuro'}`, () => {
    test.use({ colorScheme: tema });

    test('Fila: todos os estados, painel de envio e limites', async ({ page }) => {
      await interceptar(page);
      await page.goto('/#/fila');
      await page.getByTestId('cand-prep').getByText('Conferida nesta tela').waitFor();
      await page.getByText('Últimos envios (2)').click();
      expect(resumo((await (await axe(page)).analyze()).violations)).toEqual([]);
    });

    test('Fila: editar pacote e ajustar limite abertos', async ({ page }) => {
      await interceptar(page);
      await page.goto('/#/fila');
      await page.getByTestId('cand-prep').getByRole('button', { name: 'Editar pacote' }).click();
      const regiao = page.getByRole('region', { name: 'Limite de hoje' });
      await regiao.getByRole('button', { name: 'Ajustar limite na Gupy' }).click();
      await regiao.getByLabel('Limite por dia, de 1 a 40').fill('41');
      await regiao.getByRole('button', { name: 'Salvar' }).click();
      await regiao.getByRole('alert').waitFor();
      expect(resumo((await (await axe(page)).analyze()).violations)).toEqual([]);
    });

    test('Perfil: Seus dados neste computador', async ({ page }) => {
      await page.goto('/#/perfil');
      await page.getByRole('region', { name: 'Seus dados neste computador' }).waitFor();
      expect(resumo((await (await axe(page)).include('.seus-dados').analyze()).violations)).toEqual([]);
    });

    // O central-dev não tem janela (disponivel false, a seção não aparece): o estado do app de PC vem interceptado.
    test('Perfil: Janela do app, com o segundo plano ligado e o aviso de recusa', async ({ page }) => {
      await page.route('**/api/v1/app/janela', r => r.request().method() === 'GET'
        ? r.fulfill({ json: { disponivel: true, segundoPlano: true } })
        : r.fulfill({ status: 409, json: { erro: 'fictício', motivo: 'sem-bandeja' } }));
      await page.goto('/#/perfil');
      const regiao = page.getByRole('region', { name: 'Janela do app' });
      await regiao.getByRole('checkbox', { name: 'Continuar aberto em segundo plano' }).click();
      await regiao.getByRole('alert').waitFor();
      expect(resumo((await (await axe(page)).include('.janela-app').analyze()).violations)).toEqual([]);
    });

    test('Perfil: fora do app de PC a seção Janela do app não aparece', async ({ page }) => {
      await page.goto('/#/perfil');
      await page.getByRole('region', { name: 'Seus dados neste computador' }).waitFor();
      await expect(page.getByRole('region', { name: 'Janela do app' })).toHaveCount(0);
    });

    test('diálogo de aprovar aberto', async ({ page }) => {
      await interceptar(page);
      await page.goto('/#/fila');
      const prep = page.getByTestId('cand-prep');
      await prep.getByText('Conferida nesta tela').waitFor();
      await prep.getByRole('button', { name: 'Aprovar este pacote' }).click();
      await page.locator('dialog#dialogo[open]').waitFor();
      expect(resumo((await (await axe(page)).include('dialog#dialogo').analyze()).violations)).toEqual([]);
    });
  });
}
