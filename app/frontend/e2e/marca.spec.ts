// O nome do produto é Applyer (jc-a2y.61): cabeçalho, título da aba e mensagens. Pacotes, rotas e pastas não mudam.
import { expect, test } from '@playwright/test';

test('a marca no cabeçalho e no título é Applyer', async ({ page }) => {
  await page.goto('/#/hoje');
  await expect(page.locator('main#conteudo')).not.toBeEmpty();
  await expect(page.getByRole('link', { name: 'Applyer — início' })).toBeVisible();
  await expect(page).toHaveTitle(/^Hoje · Applyer$/);
  // Nenhum texto visível chama o app de "Central" (a "Central antiga", o produto anterior, pode aparecer).
  const corpo = await page.locator('body').innerText();
  expect(corpo.replace(/Central antiga/g, '')).not.toMatch(/\bCentral\b/);
});
