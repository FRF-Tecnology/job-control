// Regras como código: toda saída de IA passa por aqui antes de voltar ao núcleo,
// venha de qual provedor vier. O prompt pede; a guarda garante.
import { contemDocumentoEm } from '../validacao.mjs';
import { normalizar } from '../modelo.mjs';
import { FUNCOES } from './funcoes.mjs';
import { ErroIA, validarEsquema } from './erro.mjs';

const SENSIVEL = /\b(cpf|rg|senha|password|passcode|pin)\b|dados banc|conta banc|ag[eê]ncia banc|bank (account|details)|routing number|\biban\b|\bpix\b|cri(e|ar) (uma |sua )?conta|create (an |your )?account|sign ?up|cadastre-se|\blog ?in\b|fazer login/i;
const EXPERIENCIA = /experi[eê]ncia|experience|\banos\b|\byears?\b|profici|familiar|conhecimento|knowledge|dom[ií]nio|skill|habilidade|n[ií]vel (de|em)|level (of|in)|trabalhou com|worked with|j[aá] (usou|utilizou|trabalhou)|have you (used|worked)|how (long|many)/i;
const IDADE = /idade|\bage\b|old are you|nasceu|born/i;
const EM_INGLES = s => !/[ãõçáéíóúâêô]|\b(você|voce|tem|possui|qual|quantos)\b/i.test(s) && /\b(do|does|have|are|is|can|will|you|your)\b/i.test(s);
const CONSERVADORA = /^(nao|no|none|nenhum|nenhuma|sem experiencia|0|zero|never|nunca|less than|menos de)\b/;

const bloquear = (regra, mensagem) => { throw new ErroIA('regra', mensagem, { regra }); };

// Resposta conservadora para experiência sem fonte: nunca inventar.
function conservadora(e) {
  const en = EM_INGLES(e.pergunta);
  if (e.tipoCampo === 'numero') return '0';
  if (e.tipoCampo === 'sim-nao') return en ? 'No' : 'Não';
  if (e.tipoCampo === 'opcao') return (e.opcoes ?? []).find(o => CONSERVADORA.test(normalizar(o))) ?? null;
  return en ? "I don't have experience with this." : 'Não tenho experiência com isso.';
}

// Fonte declarada precisa existir: "curriculo" sem currículo ou "respostasPadrao" sem respostas é o mesmo que "nenhuma".
function fonteReal(s, e) {
  if (s.fonte === 'curriculo' && !String(e.curriculo ?? '').trim()) return 'nenhuma';
  if (s.fonte === 'respostasPadrao' && !(e.perfil?.respostasPadrao ?? []).length) return 'nenhuma';
  return s.fonte ?? 'nenhuma';
}

// Ordem importa: parar vem antes das correções; documento é a última barreira.
export const REGRAS = [
  {
    id: 'parar-dado-sensivel', funcoes: ['responderPergunta'],
    descricao: 'Pergunta pede CPF, RG, senha, dados bancários ou criar conta/login: parar, sem resposta.',
    aplicar: (s, e) => SENSIVEL.test(e.pergunta)
      ? { resposta: '', fonte: 'nenhuma', confianca: 1, parar: true, motivoParar: 'Pergunta pede dado sensível ou conta/login: fica com a pessoa.' }
      : s,
  },
  {
    id: 'nunca-inventar', funcoes: ['responderPergunta'],
    descricao: 'Experiência sem fonte real (currículo, respostasPadrao, perfil) vira a resposta conservadora.',
    aplicar: (s, e) => {
      if (s.parar || fonteReal(s, e) !== 'nenhuma' || !EXPERIENCIA.test(e.pergunta) || IDADE.test(e.pergunta)) return s;
      const resposta = conservadora(e);
      if (resposta == null) return { ...s, resposta: '', fonte: 'nenhuma', parar: true, motivoParar: 'Sem experiência e nenhuma opção conservadora na lista.' };
      return { ...s, resposta, fonte: 'nenhuma' };
    },
  },
  {
    id: 'numero-so-digitos', funcoes: ['responderPergunta'],
    descricao: 'Campo numérico: só dígitos. "R$ 5.000" vira "5000"; texto ou faixa é bloqueado.',
    aplicar: (s, e) => {
      if (s.parar || e.tipoCampo !== 'numero' || /^\d+$/.test(s.resposta)) return s;
      const numeros = String(s.resposta).match(/\d[\d.,]*/g) ?? [];
      if (numeros.length === 1 && /^\d{1,3}([.,]\d{3})+$/.test(numeros[0])) return { ...s, resposta: numeros[0].replace(/\D/g, '') };
      if (numeros.length === 1 && /^\d+$/.test(numeros[0])) return { ...s, resposta: numeros[0] };
      return bloquear('numero-so-digitos', `campo numérico com resposta não numérica: "${s.resposta}"`);
    },
  },
  {
    id: 'opcao-da-lista', funcoes: ['responderPergunta'],
    descricao: 'Campo de opção: a resposta tem de ser uma das opções (devolvida com o texto exato da lista).',
    aplicar: (s, e) => {
      if (s.parar || e.tipoCampo !== 'opcao') return s;
      const achada = (e.opcoes ?? []).find(o => normalizar(o) === normalizar(s.resposta));
      return achada != null ? { ...s, resposta: achada } : bloquear('opcao-da-lista', `opção fora da lista: "${s.resposta}"`);
    },
  },
  {
    id: 'sim-nao-valido', funcoes: ['responderPergunta'],
    descricao: 'Campo sim/não: só Sim, Não, Yes ou No.',
    aplicar: (s, e) => {
      if (s.parar || e.tipoCampo !== 'sim-nao') return s;
      const r = normalizar(s.resposta);
      const mapa = { sim: 'Sim', nao: 'Não', yes: 'Yes', no: 'No' };
      return mapa[r] ? { ...s, resposta: mapa[r] } : bloquear('sim-nao-valido', `campo sim/não com resposta "${s.resposta}"`);
    },
  },
  {
    id: 'pulo-com-motivo', funcoes: ['triarVaga'],
    descricao: 'Triagem "pular" sempre tem motivoPulo (padrão "Outro").',
    aplicar: s => (s.decisao === 'pular' && !s.motivoPulo ? { ...s, motivoPulo: 'Outro' } : s),
  },
  {
    id: 'revisao-coerente', funcoes: ['revisarAntesDeEnviar'],
    descricao: 'Revisão com problemas nunca sai como ok.',
    aplicar: s => ({ ...s, ok: s.problemas.length === 0 }),
  },
  {
    id: 'vaga-conhecida', funcoes: ['classificarEmail'],
    descricao: 'vagaId do email tem de ser uma das vagas enviadas; id inventado vira null.',
    aplicar: (s, e) => (s.vagaId != null && !e.vagas.some(v => v.id === s.vagaId) ? { ...s, vagaId: null } : s),
  },
  {
    id: 'mensagem-rascunho', funcoes: ['escreverMensagem'],
    descricao: 'Mensagem é sempre rascunho: nunca é enviada sem aprovação da pessoa.',
    aplicar: s => ({ ...s, rascunho: true }),
  },
  {
    id: 'sem-documento', funcoes: '*',
    descricao: 'Qualquer saída com CPF ou RG é bloqueada.',
    aplicar: s => (contemDocumentoEm(s) ? bloquear('sem-documento', 'a saída da IA contém CPF/RG') : s),
  },
];

// Lê o texto do provedor como JSON (tolera cerca ```json).
export function lerJSON(textoBruto) {
  if (textoBruto && typeof textoBruto === 'object') return textoBruto;
  const t = String(textoBruto ?? '').trim().replace(/^```(?:json)?\s*/i, '').replace(/\s*```$/, '');
  try { return JSON.parse(t); } catch { throw new ErroIA('json', 'a IA não devolveu JSON válido'); }
}

export function validarEntrada(funcao, e) {
  const f = FUNCOES[funcao];
  if (!f) throw new ErroIA('entrada', `função desconhecida: ${funcao}`);
  const erros = validarEsquema(e, f.entrada);
  if (funcao === 'responderPergunta' && e?.tipoCampo === 'opcao' && !e.opcoes?.length) erros.push('$.opcoes: obrigatório para tipoCampo "opcao"');
  if (erros.length) throw new ErroIA('entrada', `entrada inválida para ${funcao}: ${erros.join('; ')}`, { erros });
}

// Valida o esquema e aplica as regras. Devolve { saida, aplicadas } (ids das regras que mudaram algo).
export function guardar(funcao, entradaFn, bruto) {
  const f = FUNCOES[funcao];
  let saida = lerJSON(bruto);
  const erros = [...validarEsquema(saida, f.saida), ...(f.extra && saida && typeof saida === 'object' ? f.extra(saida) : [])];
  if (erros.length) throw new ErroIA('esquema', `saída fora do contrato de ${funcao}: ${erros.join('; ')}`, { erros });
  const aplicadas = [];
  for (const r of REGRAS) {
    if (r.funcoes !== '*' && !r.funcoes.includes(funcao)) continue;
    const antes = JSON.stringify(saida);
    saida = r.aplicar(saida, entradaFn);
    if (JSON.stringify(saida) !== antes) aplicadas.push(r.id);
  }
  return { saida, aplicadas };
}
