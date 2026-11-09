// CSP da janela do app (jc-a2y.29): script inline e de outro site não rodam, e as telas
// funcionam sem nenhuma violação. Roda contra dist/ (npm run build), como o app Wails.
import { expect, test } from '@playwright/test';

const TELAS = ['hoje', 'triagem', 'quadro', 'vagas', 'numeros', 'plataformas', 'rodadas', 'perfil', 'ia'];

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    (window as any).__violacoes = [];
    document.addEventListener('securitypolicyviolation', e => (window as any).__violacoes.push(`${e.violatedDirective} ${e.blockedURI}`));
  });
});

test('CSP restrita está na página', async ({ page }) => {
  await page.goto('/#/hoje');
  const csp = await page.locator('meta[http-equiv="Content-Security-Policy"]').getAttribute('content');
  expect(csp).toContain("script-src 'self'");
  expect(csp).toContain("object-src 'none'");
  expect(csp).toContain("base-uri 'none'");
  expect(csp).not.toContain('unsafe-inline');
  expect(csp).not.toContain('unsafe-eval');
});

// Código mandado pelo Playwright roda pela ferramenta de depuração, fora da CSP; por isso o teste
// injeta <script> na página (como faria um link ou texto malicioso) em vez de chamar eval direto.
// A falta de 'unsafe-eval' é conferida no teste acima.
test('script inline e script de fora do app são barrados', async ({ page }) => {
  await page.goto('/#/hoje');
  await expect(page.locator('main#conteudo')).not.toBeEmpty();
  const rodou = await page.evaluate(async () => {
    const inline = document.createElement('script');
    inline.textContent = 'window.__inline = true';
    document.head.appendChild(inline);
    const deFora = document.createElement('script');
    deFora.src = 'data:text/javascript,window.__deFora=true';
    document.head.appendChild(deFora);
    await new Promise(r => setTimeout(r, 200));
    return { inline: (window as any).__inline === true, deFora: (window as any).__deFora === true };
  });
  expect(rodou).toEqual({ inline: false, deFora: false });
  const violacoes: string[] = await page.evaluate(() => (window as any).__violacoes);
  expect(violacoes.some(v => v.startsWith('script-src'))).toBe(true);
});

test('todas as telas abrem sem violação de CSP nem erro no console', async ({ page }) => {
  const erros: string[] = [];
  page.on('console', m => { if (m.type() === 'error') erros.push(m.text()); });
  page.on('pageerror', e => erros.push(e.message));
  for (const rota of TELAS) {
    await page.goto(`/#/${rota}`);
    await expect(page.locator('main#conteudo')).not.toBeEmpty();
  }
  // O tema salvo continua sendo aplicado antes de desenhar (script do tema saiu do HTML).
  await page.evaluate(() => localStorage.setItem('tema', 'escuro'));
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-tema', 'escuro');
  expect(await page.evaluate(() => (window as any).__violacoes)).toEqual([]);
  expect(erros).toEqual([]);
});
