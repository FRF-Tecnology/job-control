// Dados fictícios da Fila e do envio para os testes de acessibilidade e teclado (jc-a2y.59). O pacote e a vaga são os
// mesmos dos testes da Fila, para a impressão calculada na tela bater com a do Go.
import type { Page } from '@playwright/test';

export const IMP = 'sha256:v1:2238b4657006a7556b71d4428ea8ca0b71e6158d730a2ed276d3d881f6198129';
const CONTEUDO = {
  curriculo: { arquivo: 'curriculo-exemplo.pdf', sha256: 'a'.repeat(64) }, idioma: 'pt',
  contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' }, pretensao: { valor: 5000.5, moeda: 'BRL', tipo: 'CLT' },
  respostas: [{ pergunta: 'Anos com React? "ação"\n', resposta: '3', origem: 'respostasPadrao' }], mensagem: null, // o mesmo pacote dos testes da Fila: a impressão é a do Go
};
const cand = (id: string, estado: string, extra: Record<string, unknown> = {}) => ({
  id, vagaId: 'ext-exemplo-1', plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/123', estado, conteudo: CONTEUDO,
  impressao: IMP, aprovacao: null, bloqueio: null, resultado: null, versao: 1,
  criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00', ...extra,
});
export const LISTA = [
  cand('cand-prep', 'preparada'),
  cand('cand-ruim', 'preparada', { impressao: 'sha256:v1:' + '0'.repeat(64) }),
  cand('cand-apro', 'aprovada', { aprovacao: { em: '2026-11-09T10:05:00-03:00', impressao: IMP, validaAte: '2026-11-09T23:59:59-03:00', fuso: 'America/Sao_Paulo' } }),
  cand('cand-envi', 'enviando'),
  cand('cand-bloq', 'bloqueada', { bloqueio: { motivo: 'cpf', pendencia: 'Digitar o número fiscal e enviar', em: '2026-11-09T11:00:00-03:00' } }),
  cand('cand-falh', 'falhou', { resultado: { em: '2026-11-09T11:00:00-03:00', tipo: 'falhou', detalhe: 'currículo mudou' } }),
  cand('cand-ince', 'incerta'),
  cand('cand-canc', 'cancelada'),
];
export const LIMITES = Object.fromEntries(['linkedin', 'gupy', 'inhire'].map(p => [p, { limite: 30, usadas: p === 'linkedin' ? 30 : 2, renovaEm: '2026-11-10T00:00:00-03:00' }]));
export const ENVIO = { ligado: true, pausado: false, janela: 'oculta', login: false, enviandoAgora: { id: 'cand-envi', desde: '2026-11-09T10:10:00-03:00' },
  desfechos: [{ id: 'cand-bloq', estado: 'bloqueada', motivo: 'cpf', em: '2026-11-09T11:00:00-03:00' }, { id: 'cand-falh', estado: 'falhou', motivo: null, em: '2026-11-09T10:50:00-03:00' }] };

// A vaga fictícia entra só na resposta do GET /estado: os testes não gravam nada na pasta de dados, que a jornada de
// usuário novo precisa vazia.
const VAGA = { id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', link: 'https://www.linkedin.com/jobs/view/123', plataforma: 'linkedin', etapa: 'aplicar', aderencia: 'A' };
export async function interceptar(page: Page) {
  await page.route('**/api/v1/estado', async r => {
    const res = await r.fetch();
    const estado = await res.json();
    r.fulfill({ response: res, json: { ...estado, vagas: [...estado.vagas, VAGA] } });
  });
  await page.route('**/api/v1/candidaturas', r => r.request().method() === 'GET' ? r.fulfill({ json: LISTA }) : r.continue());
  await page.route('**/api/v1/candidaturas/limites', r => r.fulfill({ json: LIMITES }));
  await page.route('**/api/v1/envio', r => r.fulfill({ json: ENVIO }));
}
// Espera as animações (ex.: o diálogo subindo com opacidade) terminarem; senão o axe mede o contraste no meio delas.
