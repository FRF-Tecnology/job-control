// Contraste das cores do app nos temas claro e escuro (jc-a2y.20). Lê os tokens de estilo.css e confere
// toda regra que declara cor de texto e fundo, mais os estados (hover, foco) que herdam a cor do texto.
// WCAG 2.2 AA: texto normal 4,5:1; ícone e anel de foco 3:1.
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

const css = readFileSync(join(__dirname, 'estilo.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');

function tokens(bloco: string) {
  const t: Record<string, string> = {};
  for (const m of bloco.matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) t[m[1]] = m[2].trim();
  return t;
}
function bloco(abertura: RegExp) {
  const m = css.match(abertura);
  if (!m || m.index == null) throw new Error(`bloco não achado: ${abertura}`);
  const inicio = m.index + m[0].length;
  return css.slice(inicio, css.indexOf('}', inicio));
}
const CLARO = tokens(bloco(/:root\s*\{/));
const ESCURO_SISTEMA = tokens(bloco(/:root:not\(\[data-tema="claro"\]\)\s*\{/));
const ESCURO_ESCOLHIDO = tokens(bloco(/:root\[data-tema="escuro"\]\s*\{/));
const TEMAS = { claro: CLARO, escuro: { ...CLARO, ...ESCURO_ESCOLHIDO } } as const;

const hex = (v: string) => { const h = v.replace('#', ''); return h.length === 3 ? h.split('').map(c => c + c).join('') : h; };
const canal = (c: number) => { const s = c / 255; return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4; };
const luminancia = (v: string) => { const h = hex(v); const [r, g, b] = [0, 2, 4].map(i => canal(parseInt(h.slice(i, i + 2), 16))); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
const razao = (a: string, b: string) => { const [x, y] = [luminancia(a), luminancia(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };

// var(--x) ou #hex; o resto (transparent, color-mix, gradiente) não entra na conta.
function resolver(valor: string, tema: Record<string, string>): string | null {
  const v = valor.trim().split(/\s+!important/)[0];
  const ref = v.match(/^var\((--[\w-]+)\)$/);
  const final = ref ? tema[ref[1]] : v;
  return final && /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.test(final) ? final : null;
}

// Regras com cor de texto e fundo declarados juntos.
const ICONE = /caixa-pedido button|\.redondo/; // só ícone: basta 3:1
const pares: { regra: string; texto: string; fundo: string; minimo: number }[] = [];
for (const m of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
  const regra = m[1].trim().replace(/\s+/g, ' ');
  if (regra.startsWith('@') || regra.startsWith(':root')) continue;
  const fundo = m[2].match(/(?:^|;)\s*background(?:-color)?\s*:\s*([^;]+)/)?.[1];
  const texto = m[2].match(/(?:^|;)\s*color\s*:\s*([^;]+)/)?.[1];
  if (fundo && texto) pares.push({ regra, texto, fundo, minimo: ICONE.test(regra) ? 3 : 4.5 });
}
// Estados que trocam só o fundo e herdam a cor do texto, e o anel de foco sobre fundo de destaque.
const herdados = [
  { regra: '.botao:hover', texto: 'var(--sobre-destaque)', fundo: 'var(--destaque-forte)', minimo: 4.5 },
  { regra: '.caixa-pedido button:hover', texto: 'var(--sobre-destaque)', fundo: 'var(--destaque-forte)', minimo: 3 },
  { regra: '.convite.azul :focus-visible (anel)', texto: 'var(--sobre-destaque)', fundo: 'var(--destaque)', minimo: 3 },
  { regra: '.primeiros li.feito .marca-passo (✓)', texto: 'var(--sobre-destaque)', fundo: 'var(--destaque)', minimo: 3 },
];

describe('contraste nos temas claro e escuro', () => {
  it('os dois blocos do tema escuro (sistema e escolhido) têm os mesmos tokens', () => {
    expect(ESCURO_SISTEMA).toEqual(ESCURO_ESCOLHIDO);
  });

  it('achou as regras de cor (sanidade do leitor)', () => {
    expect(pares.length).toBeGreaterThan(20);
    expect(pares.some(p => p.regra === '.botao')).toBe(true);
  });

  for (const [nome, tema] of Object.entries(TEMAS)) {
    it(`tema ${nome}: texto e fundo declarados atingem AA`, () => {
      const falhas = [...pares, ...herdados].flatMap(p => {
        const [t, f] = [resolver(p.texto, tema), resolver(p.fundo, tema)];
        if (!t || !f) return [];
        const r = razao(t, f);
        return r + 1e-9 < p.minimo ? [`${p.regra}: ${t} sobre ${f} = ${r.toFixed(2)}:1 (mínimo ${p.minimo})`] : [];
      });
      expect(falhas).toEqual([]);
    });
  }

  it('o anel de foco aparece sobre o fundo de destaque (quadro azul)', () => {
    expect(css).toMatch(/\.convite\.azul :focus-visible\s*\{[^}]*outline-color:\s*var\(--sobre-destaque\)/);
  });

  it('nenhuma cor fixa branca como texto ou fundo: use --sobre-destaque', () => {
    const fixos = [...css.matchAll(/([^{}]+)\{([^{}]*)\}/g)]
      .filter(m => /(?:^|;)\s*(?:color|background)\s*:\s*(#fff\b|#ffffff\b|white\b)/i.test(m[2]) && !m[1].trim().startsWith(':root'))
      .map(m => m[1].trim());
    expect(fixos).toEqual([]);
  });
});
