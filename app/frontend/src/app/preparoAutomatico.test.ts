// Preparar o pacote sozinho (jc-a2y.6.12). Só dados fictícios.
import { describe, expect, it, vi } from 'vitest';
import { ErroApi } from './contexto';
import { prepararProntas, vagasProntas } from './preparoAutomatico';

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
