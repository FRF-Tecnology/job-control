// Aviso da pasta de dados na Hoje (jc-mub). No central-dev a rota real responde aviso null (o aviso vem do app), então
// os dois avisos vêm interceptados; a rota real confere que nada aparece. Axe nos dois temas e "Entendi".
import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

test('rota real (central-dev): aviso null, nada aparece', async ({ page }) => {
  const resposta = page.waitForResponse(r => r.url().endsWith('/api/v1/dados/pasta'));
  await page.goto('/#/hoje');
  const r = await resposta;
  expect(r.status()).toBe(200);
  expect((await r.json()).aviso).toBeNull();
  await expect(page.getByText('A pasta antiga continua no computador')).toHaveCount(0);
  await expect(page.getByText('A pasta de dados ainda não mudou de nome')).toHaveCount(0);
});

for (const [aviso, titulo] of [['pasta-antiga-ficou', 'A pasta antiga continua no computador'], ['migracao-adiada', 'A pasta de dados ainda não mudou de nome']] as const) {
  for (const tema of ['light', 'dark'] as const) {
    test(`${aviso} (${tema === 'light' ? 'claro' : 'escuro'}): aparece com frase fixa, axe limpo e "Entendi" esconde`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: tema });
      await page.route('**/api/v1/dados/pasta', r => r.fulfill({ json: { aviso, migradaEm: null } }));
      await page.goto('/#/hoje');
      const titulo_ = page.getByText(titulo);
      await expect(titulo_).toBeVisible();
      await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity));
      const v = (await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).include('section[aria-labelledby="aviso-pasta-titulo"]').analyze()).violations;
      expect(v.map(x => x.id)).toEqual([]);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await page.getByRole('button', { name: 'Entendi' }).click();
      await expect(titulo_).toHaveCount(0);
      await page.goto('/#/fila'); await page.goto('/#/hoje');
      await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
      await expect(titulo_).toHaveCount(0);
    });
  }
}
