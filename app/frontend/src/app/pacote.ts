// Pacote da candidatura (P5, jc-a2y.6.3.3). O pacote inicial sai só do perfil e da pasta de currículos: nada é
// inventado (sem resposta que a pessoa não deu). O corpo que vai para a API leva o currículo só pelo nome (o
// servidor calcula o sha256 do arquivo) e tira campos vazios. Nunca CPF, RG, senha ou dado bancário.
import { perguntaSensivel } from './sensivel';

export type Resposta = { pergunta: string; resposta: string; origem: 'respostasPadrao' | 'pessoa' | 'ia' | string };
export type Conteudo = {
  curriculo: { arquivo: string; sha256?: string }; idioma: 'pt' | 'en' | string;
  contato: { nome?: string; email?: string; telefone?: string; cidade?: string } | null;
  pretensao: { valor?: number; moeda?: string; tipo?: string } | null;
  respostas: Resposta[]; mensagem: string | null;
};
type VagaBase = { idioma?: string; contrato?: string; mercado?: string };
type PDF = { nome: string };

const CAMPOS_CONTATO = ['nome', 'email', 'telefone', 'cidade'] as const;
const texto = (v: unknown) => (typeof v === 'string' ? v.trim() : typeof v === 'number' ? String(v) : '');
const numero = (v: unknown) => { const n = typeof v === 'number' ? v : Number(texto(v).replace(',', '.')); return Number.isFinite(n) && n > 0 ? n : null; };
const nomeDoArquivo = (caminho: unknown) => texto(caminho).split(/[\\/]/).pop() ?? '';

const respostasDoPerfil = (perfil: any) => (Array.isArray(perfil?.respostasPadrao) ? perfil.respostasPadrao : [])
  .filter((r: any) => texto(r?.pergunta) && texto(r?.resposta));

// Quantas respostas padrão ficaram de fora do pacote por serem perguntas que só a pessoa responde.
export const perguntasSensiveisDoPerfil = (perfil: any) => respostasDoPerfil(perfil).filter((r: any) => perguntaSensivel(texto(r.pergunta))).length;

export function pacoteInicial(vaga: VagaBase, perfil: any, pdfs: PDF[]): Conteudo {
  const idioma = vaga.idioma === 'en' ? 'en' : 'pt';
  const naPasta = (caminho: unknown) => { const n = nomeDoArquivo(caminho); return n && pdfs.some(p => p.nome === n) ? n : ''; };
  const arquivo = naPasta(perfil?.curriculos?.[idioma.toUpperCase()]) || naPasta(perfil?.curriculos?.PT) || pdfs[0]?.nome || '';

  const contato: Record<string, string> = {};
  for (const c of CAMPOS_CONTATO) if (texto(perfil?.[c])) contato[c] = texto(perfil[c]);

  let pretensao: Conteudo['pretensao'] = null;
  const usd = numero(perfil?.pretensaoUSD), pj = numero(perfil?.pretensaoPJ), clt = numero(perfil?.pretensaoCLT);
  if (vaga.mercado === 'internacional' && usd) pretensao = { valor: usd, moeda: 'USD', tipo: 'contractor' };
  else if (/\bPJ\b/i.test(vaga.contrato ?? '') && pj) pretensao = { valor: pj, moeda: 'BRL', tipo: 'PJ' };
  else if (clt) pretensao = { valor: clt, moeda: 'BRL', tipo: 'CLT' };

  // Pergunta que só a pessoa responde na página (CPF, senha, dado sensível…) não entra, mesmo vinda do perfil.
  const respostas: Resposta[] = respostasDoPerfil(perfil)
    .filter((r: any) => !perguntaSensivel(texto(r.pergunta)))
    .map((r: any) => ({ pergunta: texto(r.pergunta), resposta: texto(r.resposta), origem: 'respostasPadrao' }));

  return { curriculo: { arquivo }, idioma, contato: Object.keys(contato).length ? contato : null, pretensao, respostas, mensagem: null };
}

export function conteudoParaEnviar(c: Conteudo): Conteudo {
  const contato: Record<string, string> = {};
  for (const k of CAMPOS_CONTATO) if (texto(c.contato?.[k])) contato[k] = texto(c.contato![k]);
  const valor = numero(c.pretensao?.valor);
  return {
    curriculo: { arquivo: texto(c.curriculo.arquivo) },
    idioma: c.idioma === 'en' ? 'en' : 'pt',
    contato: Object.keys(contato).length ? contato : null,
    pretensao: c.pretensao && valor && c.pretensao.tipo ? { valor, moeda: c.pretensao.moeda ?? 'BRL', tipo: c.pretensao.tipo } : null,
    respostas: c.respostas.filter(r => texto(r.pergunta) && texto(r.resposta)).map(r => ({ pergunta: texto(r.pergunta), resposta: texto(r.resposta), origem: r.origem })),
    mensagem: texto(c.mensagem) || null,
  };
}
