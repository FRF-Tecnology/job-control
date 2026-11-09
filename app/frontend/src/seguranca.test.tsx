// Links de vaga vêm de fora (importação, automações, IA). Um link javascript: não pode virar href:
// na janela do app ele rodaria script com acesso à API local (jc-a2y.29). Só dados fictícios.
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Contexto, type App } from './app/contexto';
import { Detalhe } from './telas/Detalhe';
import { safeHref } from './util';

afterEach(cleanup);

describe('safeHref', () => {
  it.each([
    'javascript:alert(1)', ' JavaScript:alert(1)', 'java\tscript:alert(1)', 'jav&#x61;script:alert(1)',
    'data:text/html,<script>alert(1)</script>', 'vbscript:msgbox(1)', 'file:///C:/Windows/win.ini',
    '//vagas.exemplo.com/1', '/api/v1/perfil', 'https:vagas.exemplo.com', '', null, undefined, 42, {},
  ])('recusa %j', url => expect(safeHref(url)).toBeUndefined());

  it.each([
    ['https://vagas.exemplo.com/1?x=1#y', 'https://vagas.exemplo.com/1?x=1#y'],
    ['HTTP://vagas.exemplo.com', 'http://vagas.exemplo.com/'],
    ['  https://vagas.exemplo.com/2  ', 'https://vagas.exemplo.com/2'],
  ])('aceita %j', (url, esperado) => expect(safeHref(url)).toBe(esperado));
});

const nada = async () => {};
function appCom(vaga: Record<string, unknown>): App {
  return {
    estado: { vagas: [vaga], eventos: [], plataformas: [] },
    hoje: '2026-11-09', foco: 'ambos', rota: 'quadro', vaga: String(vaga.id),
    mudarFoco: nada, api: async () => ({}), agir: async () => true, mover: async () => true,
    abrirVaga: () => {}, navegar: () => {}, aviso: () => {}, perguntar: async () => null,
    recarregar: nada, limpar: () => {},
  };
}
const vaga = (link: string) => ({ id: 'vaga-teste-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', etapa: 'descoberta', plataforma: 'linkedin', descobertaEm: '2026-11-01', link });

function desenharDetalhe(link: string) {
  const app = appCom(vaga(link));
  return render(<Contexto.Provider value={app}><Detalhe id="vaga-teste-1" fechar={() => {}} /></Contexto.Provider>);
}

describe('link da vaga no Detalhe', () => {
  it('javascript:alert(1) não vira href', () => {
    const { container, queryByRole } = desenharDetalhe('javascript:alert(1)');
    const hrefs = [...container.querySelectorAll('a[href]')].map(a => a.getAttribute('href') ?? '');
    expect(hrefs.filter(h => !/^https?:\/\//i.test(h))).toEqual([]);
    expect(queryByRole('link', { name: 'Abrir vaga' })).toBeNull();
  });

  it('link https continua abrindo em outra janela', () => {
    const { getByRole } = desenharDetalhe('https://vagas.exemplo.com/123');
    const a = getByRole('link', { name: 'Abrir vaga' });
    expect(a.getAttribute('href')).toBe('https://vagas.exemplo.com/123');
    expect(a.getAttribute('target')).toBe('_blank');
    expect(a.getAttribute('rel')).toContain('noopener');
  });
});

describe('telas', () => {
  // Trava contra regressão: nenhum href recebe direto um campo que vem de fora.
  it('nenhuma tela põe link, url ou instalar cru num href', () => {
    const pasta = join(__dirname, 'telas');
    const cru = /href=\{\s*[\w.?]+\.(link|url|instalar)\s*\}/;
    const achados = readdirSync(pasta).filter(f => f.endsWith('.tsx'))
      .flatMap(f => readFileSync(join(pasta, f), 'utf8').split('\n').map((l, i) => (cru.test(l) ? `${f}:${i + 1}` : '')).filter(Boolean));
    expect(achados).toEqual([]);
  });
});
