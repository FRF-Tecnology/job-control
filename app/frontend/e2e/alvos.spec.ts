// Alvos de toque (jc-a2y.39): no celular, todo controle visível tem pelo menos 44 px de altura e de largura.
// Fora da conta: links dentro de um parágrafo (exceção da WCAG para link em texto) e o "Ir para o conteúdo".
import { expect, test } from '@playwright/test';

const TELAS = ['hoje', 'triagem', 'fila', 'quadro', 'vagas', 'numeros', 'plataformas', 'rodadas', 'perfil', 'ia'];

test('celular: controles com alvo de toque de 44 px', async ({ page }, info) => {
  test.skip(info.project.name !== 'celular', 'alvo de toque é do celular (ponteiro grosso)');
  const pequenos: string[] = [];
  for (const rota of TELAS) {
    await page.goto(`/#/${rota}`);
    await expect(page.locator('main#conteudo')).not.toBeEmpty();
    // A lista de CLIs da Conectar IA chega depois (o back procura as CLIs): mede só quando ela aparece.
    if (rota === 'ia') await expect(page.getByRole('heading', { name: 'Usar minha assinatura' })).toBeVisible({ timeout: 15_000 });
    pequenos.push(...await page.evaluate(r => {
      const sel = 'button, a[href], select, input:not([type=hidden]):not([type=checkbox]):not([type=radio]), textarea, [role=button]';
      return [...document.querySelectorAll<HTMLElement>(sel)].flatMap(el => {
        const caixa = el.getBoundingClientRect();
        const estilo = getComputedStyle(el);
        if (!caixa.width || !caixa.height || estilo.visibility === 'hidden' || el.closest('[hidden], dialog:not([open])')) return [];
        if (el.matches('.pular-link') || (el.tagName === 'A' && el.closest('p'))) return [];
        if (caixa.height >= 44 - 0.5 && caixa.width >= 44 - 0.5) return [];
        const nome = (el.getAttribute('aria-label') || el.textContent || el.tagName).trim().replace(/\s+/g, ' ').slice(0, 30);
        return [`${r}: ${el.tagName.toLowerCase()}.${[...el.classList].join('.')} "${nome}" ${Math.round(caixa.width)}x${Math.round(caixa.height)}`];
      });
    }, rota));
  }
  expect(pequenos).toEqual([]);
});

test('PC com mouse: botões compactos seguem compactos', async ({ page }, info) => {
  test.skip(info.project.name !== 'pc', 'tamanho compacto é do ponteiro fino');
  await page.goto('/#/hoje');
  await expect(page.locator('main#conteudo')).not.toBeEmpty();
  const altura = await page.locator('.seletor button').first().evaluate(el => el.getBoundingClientRect().height);
  expect(altura).toBeLessThan(44);
  // O menu do topo cabe inteiro no desktop (um item novo não pode esconder o último atrás do esmaecido).
  const menu = await page.locator('.topo-app nav').evaluate(n => ({ total: n.scrollWidth, visivel: n.clientWidth }));
  expect(menu.total, 'menu do topo passa da largura').toBeLessThanOrEqual(menu.visivel + 1);
});
