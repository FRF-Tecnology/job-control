// "Ao vivo" (jc-a2y.6.9.1) contra dist/ (como o app). O central-dev não tem a rota (404); os estados com quadro vêm
// interceptados, com um JPEG de verdade gerado num canvas. Confere: a CSP deixa o <img> com data: e nada mais (nenhuma
// violação), o quadro aparece, axe nos dois temas, 375 px, fechar desliga, e o 404 real vira frase fixa.
import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { ENVIO, interceptar } from './dados-fila';

async function jpeg(page: Page) {
  return page.evaluate(() => {
    const c = document.createElement('canvas'); c.width = 320; c.height = 200;
    const g = c.getContext('2d')!; g.fillStyle = '#dde3ea'; g.fillRect(0, 0, 320, 200); g.fillStyle = '#0066cc'; g.fillRect(20, 20, 280, 40);
    return c.toDataURL('image/jpeg', 0.4);
  });
}
async function preparar(page: Page) {
  await page.addInitScript(() => {
    (window as any).__violacoes = [];
    document.addEventListener('securitypolicyviolation', e => (window as any).__violacoes.push(`${e.violatedDirective} ${e.blockedURI}`));
  });
  await interceptar(page);
  await page.route('**/api/v1/envio', r => r.fulfill({ json: { ...ENVIO, enviandoAgora: null } }));
}

for (const tema of ['light', 'dark'] as const) {
  test(`ao vivo com quadro (${tema === 'light' ? 'claro' : 'escuro'}): imagem aparece sem violar a CSP, axe limpo, fechar desliga`, async ({ page }) => {
    await page.emulateMedia({ colorScheme: tema });
    await preparar(page);
    await page.goto('/#/fila');
    const imagem = await jpeg(page);
    const posts: unknown[] = [];
    await page.route('**/api/v1/envio/ao-vivo', r => {
      if (r.request().method() === 'POST') posts.push(r.request().postDataJSON());
      return r.fulfill({ json: { ligado: true, etapa: 'preenchendo', candidatura: 'cand-envi', quadro: { imagem, em: '2026-11-09T10:10:00-03:00', largura: 320, altura: 200 } } });
    });
    await page.getByRole('button', { name: 'Ver ao vivo' }).click();
    const img = page.getByRole('img', { name: /^Tela do envio de / });
    await expect(img).toBeVisible();
    expect(await img.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth > 0)).toBe(true);
    await expect(page.locator('.ao-vivo-etapa')).toContainText('Preenchendo o formulário…');
    expect(await page.evaluate(() => (window as any).__violacoes)).toEqual([]);
    await page.waitForFunction(() => document.getAnimations().every(a => a.playState !== 'running' || a.effect?.getTiming().iterations === Infinity));
    const v = (await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).include('.ao-vivo').analyze()).violations;
    expect(v.map(x => x.id)).toEqual([]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.getByRole('button', { name: 'Fechar o ao vivo' }).click();
    await expect.poll(() => posts.at(-1)).toEqual({ ligado: false });
    expect(posts[0]).toEqual({ ligado: true });
  });
}

test('rota real do central-dev (404): frase fixa, painel fechado', async ({ page }) => {
  await preparar(page);
  await page.goto('/#/fila');
  await page.getByRole('button', { name: 'Ver ao vivo' }).click();
  await expect(page.locator('.ao-vivo [role="alert"]')).toHaveText('Este app não traz o envio: não há o que ver.');
  await expect(page.getByRole('button', { name: 'Ver ao vivo' })).toBeVisible();
});

test('a CSP libera só data: a mais em img-src', async ({ page }) => {
  await page.goto('/#/hoje');
  const csp = await page.locator('meta[http-equiv="Content-Security-Policy"]').getAttribute('content');
  expect(csp).toContain("img-src 'self' data:;");
  expect(csp).toContain("script-src 'self';");
  expect(csp).toContain("default-src 'self';");
});
