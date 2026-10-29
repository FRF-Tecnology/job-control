// Operações sobre o estado da central. Todas são puras: recebem o estado,
// devolvem um estado novo e nunca alteram o original. Tela e CLI usam as mesmas.
import { ETAPAS, somarDias } from './modelo.mjs';
import { validarVaga, contemDocumento } from './validacao.mjs';
import { acharDuplicata } from './dedupe.mjs';

export class ErroValidacao extends Error {
  constructor(erros) {
    super(erros.join('; '));
    this.name = 'ErroValidacao';
    this.erros = erros;
  }
}

export class ErroDuplicata extends Error {
  constructor(existente) {
    super(`já existe: ${existente.empresa} · ${existente.titulo} (${existente.id}, etapa ${existente.etapa})`);
    this.name = 'ErroDuplicata';
    this.existente = existente;
  }
}

// Campos que podem ser editados livremente pela tela.
const EDITAVEIS = ['titulo', 'empresa', 'link', 'plataforma', 'nivel', 'contrato', 'modelo', 'stack', 'ingles', 'idioma', 'aderencia', 'motivoClaude', 'notas', 'subEtapa', 'local'];

export function estadoVazio() {
  return { vagas: [], eventos: [], plataformas: [], rodadas: [], perfil: {} };
}

const clonar = o => structuredClone(o);

function exigirValida(v) {
  const erros = validarVaga(v);
  if (erros.length) throw new ErroValidacao(erros);
}

function acharIndice(estado, id) {
  const i = estado.vagas.findIndex(v => v.id === id);
  if (i < 0) throw new Error(`vaga não encontrada: ${id}`);
  return i;
}

function novoEvento(estado, vagaId, tipo, texto, { autor, agora }) {
  const n = estado.eventos.length + 1;
  return { id: `ev-${agora}-${n}-${Math.random().toString(36).slice(2, 6)}`, vagaId, tipo, data: agora, autor, texto: texto ?? '' };
}

export function adicionarVaga(estado, dados, ctx) {
  const vaga = {
    plataforma: 'outra', etapa: 'descoberta', descobertaEm: ctx.agora, atualizadoEm: ctx.agora,
    ...dados,
  };
  exigirValida(vaga);
  const dup = acharDuplicata(estado.vagas, vaga);
  if (dup) throw new ErroDuplicata(dup);
  const novo = clonar(estado);
  novo.vagas.push(vaga);
  novo.eventos.push(novoEvento(novo, vaga.id, vaga.etapa === 'descoberta' ? 'descoberta' : vaga.etapa, dados.motivoClaude, ctx));
  return { estado: novo, vaga };
}

export function moverVaga(estado, id, etapa, extras = {}, ctx) {
  if (!ETAPAS.includes(etapa)) throw new ErroValidacao([`etapa desconhecida: ${etapa}`]);
  const novo = clonar(estado);
  const i = acharIndice(novo, id);
  const v = { ...novo.vagas[i], ...extras, etapa, atualizadoEm: ctx.agora };
  if (etapa !== 'bloqueada') v.pendencia = extras.pendencia ?? null;
  if (etapa !== 'pulada' && etapa !== 'encerrada' && !extras.motivo) v.motivo = null;
  if (etapa === 'enviada' && !novo.vagas[i].envio?.data) {
    v.envio = { ...(extras.envio ?? {}), data: extras.envio?.data ?? ctx.agora };
    v.followUp = { proximo: somarDias(v.envio.data, ctx.diasFollowUp ?? 7), feitos: 0 };
  }
  exigirValida(v);
  novo.vagas[i] = v;
  const texto = extras.motivo || extras.pendencia || extras.texto || '';
  novo.eventos.push(novoEvento(novo, id, etapa, texto, ctx));
  return { estado: novo, vaga: v };
}

export function atualizarVaga(estado, id, patch, ctx) {
  const novo = clonar(estado);
  const i = acharIndice(novo, id);
  const permitido = Object.fromEntries(Object.entries(patch).filter(([k]) => EDITAVEIS.includes(k)));
  const v = { ...novo.vagas[i], ...permitido, atualizadoEm: ctx.agora };
  exigirValida(v);
  novo.vagas[i] = v;
  return { estado: novo, vaga: v };
}

export function registrarEvento(estado, id, tipo, texto, ctx) {
  if (contemDocumento(texto)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
  const novo = clonar(estado);
  acharIndice(novo, id);
  const ev = novoEvento(novo, id, tipo, texto, ctx);
  novo.eventos.push(ev);
  return { estado: novo, evento: ev };
}

export function followUpFeito(estado, id, ctx) {
  const novo = clonar(estado);
  const i = acharIndice(novo, id);
  const v = novo.vagas[i];
  v.followUp = { proximo: somarDias(ctx.agora, ctx.diasFollowUp ?? 7), feitos: (v.followUp?.feitos ?? 0) + 1 };
  v.atualizadoEm = ctx.agora;
  novo.eventos.push(novoEvento(novo, id, 'follow-up', ctx.texto || 'follow-up feito', ctx));
  return { estado: novo, vaga: v };
}

export function adiarFollowUp(estado, id, dias, ctx) {
  const novo = clonar(estado);
  const i = acharIndice(novo, id);
  const v = novo.vagas[i];
  v.followUp = { feitos: v.followUp?.feitos ?? 0, proximo: somarDias(ctx.agora, dias) };
  v.atualizadoEm = ctx.agora;
  return { estado: novo, vaga: v };
}
