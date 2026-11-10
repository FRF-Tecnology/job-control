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
  desfechos: [{ id: 'cand-bloq', estado: 'bloqueada', motivo: 'cpf', em: '2026-11-09T11:00:00-03:00' }, { id: 'cand-falh', estado: 'falhou', motivo: null, em: '2026-11-09T10:50:00-03:00' }],
  // jc-a2y.6.11.1: uma plataforma em cada situação que pede algo da pessoa.
  plataformas: [
    { plataforma: 'gupy', estado: 'pausada', motivo: 'falhas-seguidas', ate: null, voltasHoje: 2, paginasHoje: 12, tetoPaginas: 40 },
    { plataforma: 'inhire', estado: 'parada-hoje', motivo: 'aviso-conta', ate: '2026-11-10T00:00:00-03:00', voltasHoje: 0, paginasHoje: 3, tetoPaginas: 40 },
    { plataforma: 'linkedin', estado: 'precisa-entrar', motivo: 'login', ate: null, voltasHoje: 1, paginasHoje: 30, tetoPaginas: 40 },
  ] };

// A vaga fictícia entra só na resposta do GET /estado: os testes não gravam nada na pasta de dados, que a jornada de
// usuário novo precisa vazia.
const VAGA = { id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', link: 'https://www.linkedin.com/jobs/view/123', plataforma: 'linkedin', etapa: 'aplicar', aderencia: 'A' };
// GET /estado real com dados fictícios por cima. O handler busca o servidor (r.fetch) e pode ainda estar no meio
// quando o teste acaba: com a página fechada, "Response has been disposed" (jc-h39, run 42708747310). Por isso ele
// tolera a página fechar, e cada spec chama soltarRotas no afterEach.
export async function alterarEstado(page: Page, mudar: (estado: any) => any) {
  await page.route('**/api/v1/estado', async r => {
    try {
      const res = await r.fetch();
      const estado = await res.json();
      await r.fulfill({ response: res, json: mudar(estado) });
    } catch (e) {
      if (!page.isClosed()) throw e; // com a página aberta, o erro é de verdade
    }
  });
}
// Desfaz os routes no fim do teste sem falhar por handler ainda em voo (recomendação do Playwright para esse erro).
export const soltarRotas = (page: Page) => page.unrouteAll({ behavior: 'ignoreErrors' });

export async function interceptar(page: Page) {
  await alterarEstado(page, estado => ({ ...estado, vagas: [...estado.vagas, VAGA] }));
  // O POST também fica aqui: com a lista falsa, o preparo automático desta aba veria as vagas reais da jornada (que roda
  // em paralelo no mesmo servidor) sem candidatura e criaria uma de verdade antes dela, e a jornada não veria o aviso.
  await page.route('**/api/v1/candidaturas', r => r.request().method() === 'GET'
    ? r.fulfill({ json: LISTA })
    : r.fulfill({ status: 409, json: { erro: 'teste: esta aba não grava', existente: 'cand-prep' } }));
  await page.route('**/api/v1/candidaturas/limites', r => r.fulfill({ json: LIMITES }));
  await page.route('**/api/v1/envio', r => r.fulfill({ json: ENVIO }));
}
// Espera as animações (ex.: o diálogo subindo com opacidade) terminarem; senão o axe mede o contraste no meio delas.
