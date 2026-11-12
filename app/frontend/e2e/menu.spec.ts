// Com o envio ativo, a navegação principal permanece visível em ambas as larguras do desktop.
import { expect, test } from '@playwright/test';

test.skip(({ isMobile }) => isMobile, 'menu lateral permanente: no celular a navegação abre pelo botão');

for (const largura of [1266, 1440]) {
  test(`${largura} px com "Envio ligado": todos os itens do menu à vista`, async ({ page }) => {
    await page.setViewportSize({ width: largura, height: 813 });
    await page.route('**/api/v1/envio', r => r.fulfill({ json: { ligado: true, pausado: false, janela: 'oculta', login: false, enviandoAgora: null, desfechos: [] } }));
    await page.goto('/#/fila');
    await expect(page.getByRole('link', { name: 'Envio ligado' })).toBeVisible();
    const menu = page.getByRole('navigation', { name: 'Navegação principal' });
    const medida = await menu.evaluate(n => {
      const caixa = n.getBoundingClientRect();
      const fora = [...n.querySelectorAll('a')].filter(a => { const r = a.getBoundingClientRect(); return r.right > caixa.right + 1 || r.left < caixa.left - 1; }).map(a => a.textContent);
      return { cortado: n.scrollWidth > n.clientWidth + 1, fora };
    });
    expect(medida).toEqual({ cortado: false, fora: [] });
  });
}
