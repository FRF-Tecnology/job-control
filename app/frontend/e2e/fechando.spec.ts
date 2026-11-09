// Depois de "Apagar meus dados" (jc-a2y.49), o servidor responde 503 a todo pedido até o app sair. A tela mostra que
// o app está fechando e para de tentar; não mostra "problema nos dados" nem fica pedindo de novo.
import { expect, test } from '@playwright/test';

test('503 depois de apagar: mostra que o app está fechando e para de tentar', async ({ page }) => {
  await page.goto('/#/perfil');
  await expect(page.locator('main#conteudo')).not.toBeEmpty();
  let pedidos = 0;
  await page.route('**/api/v1/**', r => { pedidos++; return r.fulfill({ status: 503, json: { erro: 'os dados foram apagados; o app está fechando' } }); });
  await page.evaluate(() => window.dispatchEvent(new Event('focus'))); // força uma leitura agora
  await expect(page.getByText('Dados apagados. O app está fechando.')).toBeVisible();
  await expect(page.getByText('A Central encontrou um problema nos dados')).toHaveCount(0);
  const depois = pedidos;
  await page.waitForTimeout(6000); // mais que a atualização de 5 s
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  expect(pedidos).toBe(depois);
});
