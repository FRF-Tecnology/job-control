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
const EDITAVEIS = ['titulo', 'empresa', 'link', 'plataforma', 'nivel', 'contrato', 'modelo', 'stack', 'ingles', 'idioma', 'aderencia', 'motivoClaude', 'notas', 'subEtapa', 'local', 'mercado'];

export function estadoVazio() {
  return { vagas: [], eventos: [], plataformas: [], rodadas: [], perfil: {}, pedidos: [] };
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

// Dono dos dados: no modo local é sempre "local"; na nuvem vem da conta logada (nunca do corpo do pedido).
export const contaDe = ctx => ctx?.contaId ?? 'local';

function novoEvento(estado, vagaId, tipo, texto, ctx) {
  const { autor, agora } = ctx;
  const n = estado.eventos.length + 1;
  return { id: `ev-${agora}-${String(n).padStart(6, '0')}-${Math.random().toString(36).slice(2, 6)}`, contaId: contaDe(ctx), vagaId, tipo, data: agora, autor, texto: texto ?? '' };
}

export function adicionarVaga(estado, dados, ctx, { forcar: forcarLote = false } = {}) {
  if (!dados || typeof dados !== 'object' || Array.isArray(dados)) throw new ErroValidacao(['vaga vazia ou inválida']);
  // "forcar" pode vir no próprio item (vaga republicada) ou valer para o lote inteiro.
  const { forcar: forcarItem, contaId: _ignorado, ...resto } = dados;
  const forcar = Boolean(forcarLote || forcarItem);
  const vaga = {
    plataforma: 'outra', etapa: 'descoberta', descobertaEm: ctx.agora, atualizadoEm: ctx.agora,
    ...resto,
    contaId: contaDe(ctx),
  };
  exigirValida(vaga);
  const mesmoId = estado.vagas.find(v => v.id === vaga.id);
  const dup = mesmoId ?? (forcar ? null : acharDuplicata(estado.vagas, vaga));
  if (dup) throw new ErroDuplicata(dup);
  const novo = clonar(estado);
  novo.vagas.push(vaga);
  const dataEvento = vaga.envio?.data ?? vaga.descobertaEm ?? ctx.agora;
  novo.eventos.push(novoEvento(novo, vaga.id, vaga.etapa, dados.motivoClaude ?? dados.notas, { ...ctx, agora: dataEvento }));
  return { estado: novo, vaga };
}

export function moverVaga(estado, id, etapa, extras = {}, ctx) {
  if (!ETAPAS.includes(etapa)) throw new ErroValidacao([`etapa desconhecida: ${etapa}`]);
  const novo = clonar(estado);
  const i = acharIndice(novo, id);
  const antes = novo.vagas[i];
  // Só estes campos podem vir junto de uma mudança de etapa.
  const v = { ...antes, etapa, atualizadoEm: ctx.agora };
  if (extras.motivo !== undefined) v.motivo = extras.motivo;
  if (extras.pendencia !== undefined) v.pendencia = extras.pendencia;
  if (extras.subEtapa !== undefined) v.subEtapa = extras.subEtapa;
  if (etapa !== 'bloqueada') v.pendencia = extras.pendencia ?? null;
  if (etapa !== 'pulada' && etapa !== 'encerrada' && !extras.motivo) v.motivo = null;
  if (etapa !== 'em-processo' && etapa !== 'oferta' && extras.subEtapa === undefined) v.subEtapa = null;
  if (extras.envio && typeof extras.envio === 'object') v.envio = { ...(antes.envio ?? {}), ...extras.envio };
  if (etapa === 'enviada' && !antes.envio?.data) {
    v.envio = { ...(v.envio ?? {}), data: extras.envio?.data ?? ctx.agora };
    v.followUp = { proximo: somarDias(v.envio.data, ctx.diasFollowUp ?? 7), feitos: 0 };
  }
  // Entrar num processo sem envio registrado (ex.: recrutador chamou) conta como candidatura nesse dia.
  if ((etapa === 'em-processo' || etapa === 'oferta') && !v.envio?.data) {
    v.envio = { ...(v.envio ?? {}), data: extras.envio?.data ?? ctx.agora };
  }
  if (contemDocumento(extras.texto)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
  exigirValida(v);
  novo.vagas[i] = v;
  const texto = extras.motivo || extras.pendencia || extras.texto || '';
  novo.eventos.push(novoEvento(novo, id, etapa, texto, ctx));
  return { estado: novo, vaga: v };
}

export function atualizarVaga(estado, id, patch, ctx) {
  const novo = clonar(estado);
  const i = acharIndice(novo, id);
  // Notas: se a tela mandou a versão que tinha, recusa quando alguém (ex.: o Claude) mudou no meio.
  if ('notasAntes' in patch && (novo.vagas[i].notas ?? '') !== (patch.notasAntes ?? '')) {
    throw new ErroValidacao(['as notas mudaram enquanto você editava — recarregue a vaga e junte as duas versões']);
  }
  const permitido = Object.fromEntries(Object.entries(patch ?? {}).filter(([k]) => EDITAVEIS.includes(k)));
  const v = { ...novo.vagas[i], ...permitido, atualizadoEm: ctx.agora };
  exigirValida(v);
  novo.vagas[i] = v;
  return { estado: novo, vaga: v };
}

export function registrarEvento(estado, id, tipo, texto, ctx) {
  if (typeof tipo !== 'string' || !tipo.trim()) throw new ErroValidacao(['tipo do evento deve ser texto']);
  if (texto != null && typeof texto !== 'string') throw new ErroValidacao(['texto do evento deve ser texto']);
  if (contemDocumento(texto)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
  const novo = clonar(estado);
  acharIndice(novo, id);
  const ev = novoEvento(novo, id, tipo, texto, ctx);
  novo.eventos.push(ev);
  return { estado: novo, evento: ev };
}

export function followUpFeito(estado, id, ctx) {
  if (contemDocumento(ctx.texto)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
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

// Pedidos: o que a pessoa digita na caixa "O que vamos fazer hoje?" e o assistente
// (Claude Code, hoje) ainda não fez. O assistente lista com /pedidos e conclui com uma resposta curta.
export function criarPedido(estado, texto, ctx) {
  const t = String(texto ?? '').trim();
  if (!t) throw new ErroValidacao(['escreva o pedido']);
  if (t.length > 500) throw new ErroValidacao(['pedido longo demais (máximo 500 caracteres)']);
  if (contemDocumento(t)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
  const novo = clonar(estado);
  novo.pedidos ??= [];
  const pedido = { id: `pd-${ctx.agora}-${String(novo.pedidos.length + 1).padStart(4, '0')}-${Math.random().toString(36).slice(2, 6)}`, contaId: contaDe(ctx), texto: t, autor: ctx.autor, criadoEm: ctx.agora, status: 'aberto' };
  novo.pedidos.push(pedido);
  return { estado: novo, pedido };
}

export function concluirPedido(estado, id, resposta, ctx) {
  if (resposta != null && typeof resposta !== 'string') throw new ErroValidacao(['resposta deve ser texto']);
  if (contemDocumento(resposta)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
  const novo = clonar(estado);
  const p = (novo.pedidos ?? []).find(x => x.id === id);
  if (!p) throw new Error(`pedido não encontrado: ${id}`);
  Object.assign(p, { status: 'feito', resposta: resposta ?? '', concluidoEm: ctx.agora });
  return { estado: novo, pedido: p };
}
