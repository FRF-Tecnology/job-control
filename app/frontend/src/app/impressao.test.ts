// A tela calcula a impressão da candidatura igual ao servidor (internal/dominio/canonico.go): JSON canônico da
// RFC 8785 + sha256, prefixo "sha256:v1:" (P5, R3). Só dados fictícios.
import { describe, expect, it } from 'vitest';
import { impressaoCandidatura, jsonCanonico } from './impressao';

describe('JSON canônico (RFC 8785)', () => {
  it('exemplo da seção 3.2.2 (o mesmo do canonico_test.go)', () => {
    const entrada = '{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"string":"\\u20ac$\\u000F\\u000aA\'\\u0042\\u0022\\u005c\\\\\\"\\/","literals":[null,true,false]}';
    const quer = '{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\\u000f\\nA\'B\\"\\\\\\\\\\"/"}';
    expect(jsonCanonico(JSON.parse(entrada))).toBe(quer);
  });

  it('ordem das chaves por unidades UTF-16 (seção 3.2.3)', () => {
    // U+FB33 (dalet com dagesh, precomposto) como na RFC; escrito como escape para não virar a forma decomposta.
    const o = JSON.parse('{"€":1,"\\r":2,"\\uFB33":3,"1":4,"😀":5,"\\u0080":6,"ö":7}');
    const chaves = Object.keys(JSON.parse(jsonCanonico(o)));
    // A ordem textual do canônico é a que importa (JSON.parse devolve a chave numérica "1" primeiro):
    const texto = jsonCanonico(o);
    const posicoes = ['\\r', '"1"', '\u0080', 'ö', '€', '😀', 'דּ'].map(k => texto.indexOf(k));
    expect(posicoes).toEqual([...posicoes].sort((a, b) => a - b));
    expect(chaves).toHaveLength(7);
  });

  it('-0 vira 0; NaN e Infinity são recusados', () => {
    expect(jsonCanonico({ a: -0 })).toBe('{"a":0}');
    expect(() => jsonCanonico({ a: Number.NaN })).toThrow();
    expect(() => jsonCanonico([Infinity])).toThrow();
  });
});

describe('impressão da candidatura', () => {
  // Valor gerado pelo ImpressaoCandidatura do Go com este mesmo pacote fictício.
  const IMPRESSAO_DO_GO = 'sha256:v1:2238b4657006a7556b71d4428ea8ca0b71e6158d730a2ed276d3d881f6198129';
  const pacote = {
    vagaId: 'ext-exemplo-1', plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/123',
    conteudo: {
      curriculo: { arquivo: 'curriculo-exemplo.pdf', sha256: 'a'.repeat(64) }, idioma: 'pt',
      contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' },
      pretensao: { valor: 5000.5, moeda: 'BRL', tipo: 'CLT' },
      respostas: [{ pergunta: 'Anos com React? "ação"\n', resposta: '3', origem: 'respostasPadrao' }],
      mensagem: null,
    },
  };

  it('bate com o servidor (Go)', async () => {
    expect(await impressaoCandidatura(pacote)).toBe(IMPRESSAO_DO_GO);
  });

  // Formulário (jc-a2y.7.2.4): a impressão cobre também a estrutura, como impressaoDe em internal/dominio/formulario.go.
  // Valor gerado pelo impressaoDe do Go com esta mesma candidatura fictícia (opcoes vazias não entram).
  const FORMULARIO_DO_GO = 'sha256:v1:33be9501e5ffff806d5334abce6f820ba9a5d7b3d124ceebefe7e58eb87bd82b';
  const formulario = {
    vagaId: 'ext-form-1', plataforma: 'formulario', url: 'https://docs.google.com/forms/d/e/abc/viewform',
    conteudo: { curriculo: { arquivo: 'cv.pdf' }, idioma: 'pt', respostas: [{ pergunta: 'Nível de inglês', resposta: 'Avançado', origem: 'respostasPadrao', perguntaId: 'q3' }] },
    estrutura: { hash: 'sha256:v1:ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd', urlFinal: 'https://docs.google.com/forms/d/e/abc/viewform',
      perguntas: [{ id: 'q1', texto: 'Qual o seu e-mail?', tipo: 'texto-curto', obrigatoria: true, opcoes: [] as string[] }, { id: 'q3', texto: 'Nível de inglês', tipo: 'escolha-unica', obrigatoria: true, opcoes: ['Básico', 'Avançado'] }] },
  };

  it('formulário com estrutura lida bate com o servidor (Go)', async () => {
    expect(await impressaoCandidatura(formulario)).toBe(FORMULARIO_DO_GO);
  });

  it('a estrutura só conta em formulário, e mudar uma pergunta muda a impressão', async () => {
    expect(await impressaoCandidatura({ ...pacote, estrutura: formulario.estrutura })).toBe(IMPRESSAO_DO_GO);
    const outra = { ...formulario.estrutura, perguntas: [{ ...formulario.estrutura.perguntas[0], texto: 'Qual o seu e-mail? ' }, formulario.estrutura.perguntas[1]] };
    expect(await impressaoCandidatura({ ...formulario, estrutura: outra })).not.toBe(FORMULARIO_DO_GO);
  });

  it('não depende da ordem das chaves e muda com qualquer mudança no pacote', async () => {
    const invertido = { url: pacote.url, conteudo: { ...pacote.conteudo }, plataforma: pacote.plataforma, vagaId: pacote.vagaId };
    expect(await impressaoCandidatura(invertido)).toBe(IMPRESSAO_DO_GO);
    expect(await impressaoCandidatura({ ...pacote, conteudo: { ...pacote.conteudo, idioma: 'en' } })).not.toBe(IMPRESSAO_DO_GO);
  });
});
