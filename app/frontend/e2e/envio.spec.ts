// Envio ligado e não pausado aparece no cabeçalho em todas as telas (jc-a2y.51, nota do CyberSec no #60), no PC e no
// celular, e leva à Fila. Pausado ou desligado, nada aparece. GET /envio interceptado.
import { expect, test } from '@playwright/test';

const estado = (ligado: boolean, pausado: boolean) => ({ ligado, pausado, enviandoAgora: null, desfechos: [] });

test('marca "Envio ligado" no cabeçalho quando o envio está ligado e não pausado', async ({ page }) => {
  let atual = estado(true, false);
  await page.route('**/api/v1/envio', r => r.fulfill({ json: atual }));
  await page.goto('/#/perfil');
  const marca = page.getByRole('link', { name: 'Envio ligado' });
  await expect(marca).toBeVisible();
  const caixa = await marca.boundingBox();
  if (test.info().project.name === 'celular') expect(caixa!.height).toBeGreaterThanOrEqual(44);
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
  await marca.click();
  await expect(page).toHaveURL(/#\/fila$/);
  // Pausado: a marca some na próxima leitura (até 5 s).
  atual = estado(true, true);
  await expect(marca).toHaveCount(0, { timeout: 8000 });
});

test('leitura falhou: a marca fica com o último valor conhecido', async ({ page }) => {
  let falhar = false;
  await page.route('**/api/v1/envio', r => falhar ? r.fulfill({ status: 500, json: { erro: 'erro interno do app; tente de novo' } }) : r.fulfill({ json: estado(true, false) }));
  await page.goto('/#/hoje');
  const marca = page.getByRole('link', { name: 'Envio ligado' });
  await expect(marca).toBeVisible();
  falhar = true;
  await page.waitForTimeout(6000); // mais que a leitura de 5 s
  await expect(marca).toBeVisible();
});

test('desligado no build: nenhuma marca', async ({ page }) => {
  await page.route('**/api/v1/envio', r => r.fulfill({ json: estado(false, false) }));
  await page.goto('/#/hoje');
  await expect(page.locator('main#conteudo')).not.toBeEmpty();
  await expect(page.getByRole('link', { name: 'Envio ligado' })).toHaveCount(0);
});
