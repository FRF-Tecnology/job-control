// Pacote inicial da candidatura (P5, jc-a2y.6.3.3): só dado do perfil e da pasta de currículos, sem inventar
// resposta. E o corpo que vai para a API: só o nome do currículo (o servidor calcula o sha256). Só dados fictícios.
import { describe, expect, it } from 'vitest';
import { conteudoParaEnviar, pacoteInicial } from './pacote';

const PERFIL = {
  nome: 'Ana Exemplo', email: 'ana@exemplo.com', telefone: '', cidade: 'Cidade Exemplo',
  pretensaoCLT: '6000', pretensaoPJ: 9000, pretensaoUSD: '3500',
  curriculos: { PT: 'curriculos/cv-pt.pdf', EN: 'curriculos/cv-en.pdf' },
  respostasPadrao: [{ pergunta: 'Anos com React?', resposta: '3' }, { pergunta: '', resposta: 'sem pergunta' }, { pergunta: 'Inglês?', resposta: '' }],
};
const PDFS = [{ nome: 'cv-pt.pdf' }, { nome: 'cv-en.pdf' }];

describe('pacoteInicial', () => {
  it('vaga nacional CLT: currículo PT, contato só com o que está preenchido, pretensão CLT e respostas padrão', () => {
    const p = pacoteInicial({ idioma: 'pt', contrato: 'CLT', mercado: 'nacional' }, PERFIL, PDFS);
    expect(p).toEqual({
      curriculo: { arquivo: 'cv-pt.pdf' }, idioma: 'pt',
      contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com', cidade: 'Cidade Exemplo' },
      pretensao: { valor: 6000, moeda: 'BRL', tipo: 'CLT' },
      respostas: [{ pergunta: 'Anos com React?', resposta: '3', origem: 'respostasPadrao' }],
      mensagem: null,
    });
  });

  it('vaga em inglês e internacional: currículo EN e pretensão em USD como contractor', () => {
    const p = pacoteInicial({ idioma: 'en', mercado: 'internacional' }, PERFIL, PDFS);
    expect(p.curriculo).toEqual({ arquivo: 'cv-en.pdf' });
    expect(p.idioma).toBe('en');
    expect(p.pretensao).toEqual({ valor: 3500, moeda: 'USD', tipo: 'contractor' });
  });

  it('contrato PJ usa a pretensão PJ', () => {
    expect(pacoteInicial({ contrato: 'PJ', mercado: 'nacional' }, PERFIL, PDFS).pretensao).toEqual({ valor: 9000, moeda: 'BRL', tipo: 'PJ' });
  });

  it('perfil vazio: nada inventado (sem contato, sem pretensão, sem respostas) e o primeiro PDF da pasta', () => {
    expect(pacoteInicial({}, {}, [{ nome: 'unico.pdf' }])).toEqual({
      curriculo: { arquivo: 'unico.pdf' }, idioma: 'pt', contato: null, pretensao: null, respostas: [], mensagem: null,
    });
  });

  it('currículo do perfil que não está mais na pasta não é usado', () => {
    expect(pacoteInicial({ idioma: 'en' }, { curriculos: { EN: 'curriculos/sumiu.pdf' } }, PDFS).curriculo.arquivo).toBe('cv-pt.pdf');
  });

  it('sem PDF na pasta, o arquivo fica vazio (a tela pede para colocar um)', () => {
    expect(pacoteInicial({}, PERFIL, []).curriculo.arquivo).toBe('');
  });
});

describe('conteudoParaEnviar', () => {
  it('manda só o nome do currículo (sem sha256), tira campos vazios e respostas em branco', () => {
    expect(conteudoParaEnviar({
      curriculo: { arquivo: 'cv-pt.pdf', sha256: 'a'.repeat(64) }, idioma: 'pt',
      contato: { nome: ' Ana Exemplo ', email: '', telefone: '', cidade: '' },
      pretensao: { valor: Number.NaN, moeda: 'BRL', tipo: 'CLT' },
      respostas: [{ pergunta: 'Anos com React?', resposta: ' 3 ', origem: 'pessoa' }, { pergunta: ' ', resposta: '', origem: 'pessoa' }],
      mensagem: '   ',
    })).toEqual({
      curriculo: { arquivo: 'cv-pt.pdf' }, idioma: 'pt', contato: { nome: 'Ana Exemplo' }, pretensao: null,
      respostas: [{ pergunta: 'Anos com React?', resposta: '3', origem: 'pessoa' }], mensagem: null,
    });
  });

  it('contato todo vazio vira null; mensagem com texto é mantida', () => {
    const c = conteudoParaEnviar({ curriculo: { arquivo: 'cv.pdf' }, idioma: 'en', contato: { nome: '', email: '' }, pretensao: null, respostas: [], mensagem: 'Olá, tenho interesse.' });
    expect(c.contato).toBeNull();
    expect(c.mensagem).toBe('Olá, tenho interesse.');
  });
});
