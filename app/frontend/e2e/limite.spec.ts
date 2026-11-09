// Limite diário pela tela (jc-a2y.54), contra a API real: valor fora de 1 a 40 dá texto claro e não grava; o ajuste
// confirmado aparece no GET /candidaturas/limites e no cartão. Usa a Gupy (a jornada usa o LinkedIn) e volta a 30.
import { expect, test } from '@playwright/test';

test('ajustar o limite da Gupy pela tela', async ({ page }, info) => {
  test.skip(info.project.name !== 'pc', 'grava dados; roda só uma vez');
  const limiteGupy = async () => (await (await page.request.get('/api/v1/candidaturas/limites')).json()).gupy.limite;
  const antes = await limiteGupy();
  try {
    await page.goto('/#/fila');
    const regiao = page.getByRole('region', { name: 'Limite de hoje' });
    await regiao.getByRole('button', { name: 'Ajustar limite na Gupy' }).click();
    const campo = regiao.getByLabel('Limite por dia, de 1 a 40');
    await campo.fill('41');
    await regiao.getByRole('button', { name: 'Salvar' }).click();
    await expect(regiao.getByRole('alert')).toHaveText('O limite vai de 1 a 40 por dia.');
    expect(await limiteGupy()).toBe(antes);

    await campo.fill('25');
    await regiao.getByRole('button', { name: 'Salvar' }).click();
    const dlg = page.locator('dialog#dialogo');
    await expect(dlg.getByRole('heading')).toHaveText('Mudar o limite na Gupy para 25 por dia?');
    await dlg.getByRole('button', { name: 'Mudar limite' }).click();
    await expect(regiao.getByText('de 25')).toBeVisible();
    expect(await limiteGupy()).toBe(25);
  } finally {
    await page.request.put('/api/v1/candidaturas/limites/gupy', { data: { limite: antes } });
  }
});
