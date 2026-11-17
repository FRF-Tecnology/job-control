// Preparar o pacote sozinho (jc-a2y.6.12). Só dados fictícios.
import { describe, expect, it, vi } from 'vitest';
import { ErroApi } from './contexto';
import { liberouDepoisDaTrava, prepararProntas, vagasProntas } from './preparoAutomatico';

const PERFIL = { nome: 'Ana Exemplo', email: 'ana@exemplo.com', curriculos: { PT: 'curriculos/curriculo-exemplo.pdf' }, respostasPadrao: [{ pergunta: 'Anos com React?', resposta: '3' }, { pergunta: 'Seu CPF', resposta: 'nunca' }] };
const PDFS = [{ nome: 'curriculo-exemplo.pdf' }];
const vaga = (id: string, etapa: string, extra = {}) => ({ id, etapa, ...extra });

describe('vagasProntas', () => {
  it('Aplicar sem candidatura ativa; cancelada não conta; descoberta só no modo direto e grupo A', () => {
    const vagas = [vaga('a', 'aplicar'), vaga('b', 'aplicar'), vaga('c', 'aplicar'), vaga('d', 'descoberta', { aderencia: 'A' }), vaga('e', 'descoberta', { aderencia: 'B' })];
    const cands = [{ vagaId: 'b', estado: 'preparada' }, { vagaId: 'c', estado: 'cancelada' }];
    expect(vagasProntas(vagas, cands, {}).map(v => v.id)).toEqual(['a', 'c']);
    expect(vagasProntas(vagas, cands, { modo: 'direto' }).map(v => v.id)).toEqual(['a', 'c', 'd']);
  });
});

describe('prepararProntas', () => {
  it('usa o currículo e o nome da busca que guardou a vaga quando outro perfil está ativo', async () => {
    const api = vi.fn().mockResolvedValue({});
    const dados = { vagas: [vaga('v', 'aplicar', { perfilBuscaId: 'origem' })], perfilBuscaAtivo: 'outro', perfil: PERFIL,
      curriculos: [...PDFS, { nome: 'origem.pdf' }], perfisBusca: [{ id: 'origem', perfilPacote: { nome: 'Pessoa do perfil', curriculos: { PT: 'curriculos/origem.pdf' } } }] };
    await prepararProntas(api, dados, [], new Set());
    expect(api.mock.calls[0][2].conteudo).toMatchObject({ contato: { nome: 'Pessoa do perfil' }, curriculo: { arquivo: 'origem.pdf' } });
  });
  it('mantém a vaga para revisão se só houver PDF de outro perfil na pasta', async () => {
    const api = vi.fn();
    const dados = { vagas: [vaga('v', 'aplicar', { perfilBuscaId: 'java' })], perfilBuscaAtivo: 'java', perfil: { curriculos: { PT: 'curriculos/java.pdf' } },
      curriculos: [{ nome: 'ia.pdf' }], perfisBusca: [{ id: 'original', perfilPacote: PERFIL }, { id: 'java', perfilPacote: {} }] };
    expect(await prepararProntas(api, dados, [], new Set())).toEqual({ preparadas: 0, semPdf: 1 });
    expect(api).not.toHaveBeenCalled();
  });
  it('cria a candidatura com o pacote do perfil (sem pergunta sensível) e não repete a vaga', async () => {
    const api = vi.fn(async () => ({}));
    const tentadas = new Set<string>();
    const dados = { vagas: [vaga('a', 'aplicar')], perfil: PERFIL, curriculos: PDFS };
    expect(await prepararProntas(api, dados, [], tentadas)).toEqual({ preparadas: 1, semPdf: 0 });
    expect(api).toHaveBeenCalledWith('/api/v1/candidaturas', 'POST', {
      vagaId: 'a',
      conteudo: { curriculo: { arquivo: 'curriculo-exemplo.pdf' }, idioma: 'pt', contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' }, pretensao: null, respostas: [{ pergunta: 'Anos com React?', resposta: '3', origem: 'respostasPadrao' }], mensagem: null },
    });
    expect(await prepararProntas(api, dados, [], tentadas)).toEqual({ preparadas: 0, semPdf: 0 });
    expect(api).toHaveBeenCalledTimes(1);
  });

  it('sem PDF na pasta: não prepara e tenta de novo quando o PDF aparece', async () => {
    const api = vi.fn(async () => ({}));
    const tentadas = new Set<string>();
    const vagas = [vaga('a', 'aplicar')];
    expect(await prepararProntas(api, { vagas, perfil: PERFIL, curriculos: [] }, [], tentadas)).toEqual({ preparadas: 0, semPdf: 1 });
    expect(api).not.toHaveBeenCalled();
    expect(await prepararProntas(api, { vagas, perfil: PERFIL, curriculos: PDFS }, [], tentadas)).toEqual({ preparadas: 1, semPdf: 0 });
  });

  it('409 (já existe) e outros erros não quebram nem repetem', async () => {
    const api = vi.fn()
      .mockRejectedValueOnce(new ErroApi('já existe', 409, { erro: 'já existe', existente: 'cand-x' }))
      .mockRejectedValueOnce(new Error('sem conexão'));
    const tentadas = new Set<string>();
    const dados = { vagas: [vaga('a', 'aplicar'), vaga('b', 'aplicar')], perfil: PERFIL, curriculos: PDFS };
    expect(await prepararProntas(api, dados, [], tentadas)).toEqual({ preparadas: 0, semPdf: 0 });
    expect(await prepararProntas(api, dados, [], tentadas)).toEqual({ preparadas: 0, semPdf: 0 });
    expect(api).toHaveBeenCalledTimes(2);
  });
});

describe('prepararProntas com a trava das ações', () => {
  it('402: para na primeira, não marca a vaga e tenta de novo na próxima leitura', async () => {
    const api = vi.fn()
      .mockRejectedValueOnce(new ErroApi('Seu teste acabou.', 402, { erro: 'Seu teste acabou.', motivo: 'teste-acabou' }))
      .mockResolvedValue({});
    const tentadas = new Set<string>();
    const dados = { vagas: [vaga('a', 'aplicar'), vaga('b', 'aplicar')], perfil: PERFIL, curriculos: PDFS };
    expect(await prepararProntas(api, dados, [], tentadas)).toEqual({ preparadas: 0, semPdf: 0, travado: true });
    expect(api).toHaveBeenCalledTimes(1); // não tentou a "b"
    expect(tentadas.size).toBe(0);
    // A licença voltou (próxima mudança de estado): prepara as duas.
    expect(await prepararProntas(api, dados, [], tentadas)).toEqual({ preparadas: 2, semPdf: 0 });
  });
});

describe('liberouDepoisDaTrava', () => {
  it('sem 402 antes: liberado, sem ler a conta', async () => {
    const api = vi.fn();
    expect(await liberouDepoisDaTrava(api, null, 'v1')).toBe(true);
    expect(api).not.toHaveBeenCalled();
  });
  it('depois de um 402: só volta quando a conta deixa agir', async () => {
    const api = vi.fn()
      .mockResolvedValueOnce({ entrada: true, motivo: 'teste-acabou' })
      .mockResolvedValueOnce({ entrada: true, motivo: '' });
    expect(await liberouDepoisDaTrava(api, 'v1', 'v2')).toBe(false);
    expect(api).toHaveBeenCalledWith('/api/v1/conta');
    expect(await liberouDepoisDaTrava(api, 'v1', 'v3')).toBe(true);
  });
  it('sem a rota da conta (erro): espera a versão do estado mudar', async () => {
    const api = vi.fn().mockRejectedValue(new ErroApi('indisponível', 503, {}));
    expect(await liberouDepoisDaTrava(api, 'v1', 'v1')).toBe(false);
    expect(await liberouDepoisDaTrava(api, 'v1', 'v2')).toBe(true);
  });
});
