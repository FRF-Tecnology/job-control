// API v1 da Central: tudo que a CLI, a tela, a extensão e o MCP podem fazer passa por aqui.
// Não depende de HTTP: o servidor chama `chamar` para cada pedido e a CLI chama direto, no mesmo processo.
// Resposta: { status, corpo, tipo?, cabecalhos? }.
import { existsSync, statSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { adicionarVaga, moverVaga, atualizarVaga, registrarEvento, followUpFeito, adiarFollowUp, criarPedido, concluirPedido, registrarEmail, ligarEmail, descartarEmail, ErroValidacao, ErroDuplicata } from '../nucleo/servico.mjs';
import { perfilSensivel, contemDocumentoEm, validarPerfil, validarVaga } from '../nucleo/validacao.mjs';
import { gerarHistoricoMd } from '../nucleo/historico.mjs';
import { resumo } from '../nucleo/funil.mjs';
import { duplicatasPorTitulo } from '../nucleo/dedupe.mjs';
import { pendentesFollowUp, sugerirMensagem } from '../nucleo/followup.mjs';
import { hojeISO } from '../nucleo/modelo.mjs';

export const PREFIXO = '/api/v1';
const ARQ_DADOS = ['vagas.json', 'eventos.json', 'plataformas.json', 'rodadas.json', 'perfil.json', 'pedidos.json', 'emails.json'];

export class ErroApi extends Error { constructor(status, corpo) { super(corpo.erro); this.status = status; this.corpo = corpo; } }

function csv(vagas) {
  const cols = ['id', 'empresa', 'titulo', 'plataforma', 'etapa', 'subEtapa', 'aderencia', 'contrato', 'motivo', 'pendencia', 'dataEnvio', 'rodada', 'pretensao', 'proximoFollowUp', 'link', 'notas'];
  const valor = (v, c) => ({ dataEnvio: v.envio?.data, rodada: v.envio?.rodada, pretensao: v.envio?.pretensao, proximoFollowUp: v.followUp?.proximo }[c] ?? v[c] ?? '');
  // Apóstrofo na frente de =, +, -, @ impede o Excel de tratar o texto como fórmula.
  const esc = x => `"${String(x).replace(/^[=+\-@\t\r]/, "'$&").replace(/"/g, '""')}"`;
  return '﻿' + [cols.join(';'), ...vagas.map(v => cols.map(c => esc(valor(v, c))).join(';'))].join('\n');
}

// Perfil parcial: filtros mesclados e respostasPadrao mescladas pela pergunta.
export function mesclarPerfil(atual = {}, p = {}) {
  const respostas = [...(atual.respostasPadrao ?? [])];
  for (const r of p.respostasPadrao ?? []) {
    const i = respostas.findIndex(x => String(x.pergunta).toLowerCase() === String(r.pergunta).toLowerCase());
    if (i >= 0) respostas[i] = r; else respostas.push(r);
  }
  return { ...atual, ...p, filtros: { ...(atual.filtros ?? {}), ...(p.filtros ?? {}) }, respostasPadrao: respostas };
}

function exigirPerfilOk(perfil) {
  const erros = validarPerfil(perfil);
  if (erros.length) throw new ErroValidacao(erros);
  if (perfilSensivel(perfil)) throw new ErroValidacao(['o perfil não pode guardar CPF, RG, senha ou dados bancários']);
}

const decodificar = s => { try { return decodeURIComponent(s); } catch { throw new ErroApi(404, { erro: 'não encontrado' }); } };
const objeto = c => c !== null && typeof c === 'object' && !Array.isArray(c);

// Erros conhecidos viram status HTTP; o resto sobe.
export function respostaDeErro(e) {
  if (e instanceof ErroApi) return { status: e.status, corpo: e.corpo };
  if (e instanceof ErroValidacao) return { status: 400, corpo: { erro: e.message, erros: e.erros } };
  if (e instanceof ErroDuplicata) return { status: 409, corpo: { erro: e.message, existente: e.existente } };
  if (/não encontrad[ao]/.test(e.message)) return { status: 404, corpo: { erro: e.message } };
  return null;
}

export function criarApi({ armazem, dir = armazem.dir, hoje = () => hojeISO(), contaId = 'local' } = {}) {
  let gravacoes = 0;
  const gravar = fn => { const r = armazem.transacao(fn); gravacoes++; return r; };
  const ctx = (autor, extra) => ({ autor, contaId, agora: hoje(), diasFollowUp: Number(armazem.carregar().perfil?.diasFollowUp) || 7, ...extra });

  function curriculos() {
    const pasta = dir && join(dir, 'curriculos');
    if (!pasta || !existsSync(pasta)) return [];
    return readdirSync(pasta).filter(n => /\.pdf$/i.test(n)).sort().map(nome => ({ nome, tamanho: statSync(join(pasta, nome)).size }));
  }

  // Muda sempre que alguém grava (a tela, a CLI em outro processo ou um currículo novo).
  const versao = () => !dir ? String(gravacoes)
    : `${gravacoes}-${ARQ_DADOS.map(a => { const p = join(dir, a); return existsSync(p) ? `${statSync(p).mtimeMs}:${statSync(p).size}` : '0'; }).join('|')}|${curriculos().map(c => c.nome).join(',')}`;

  // Checklist da primeira vez (mostrada na tela Hoje até tudo estar feito).
  const primeirosPassos = e => [
    { id: 'perfil', feito: Boolean(String(e.perfil?.nome ?? '').trim()), titulo: 'Preencher o perfil', texto: 'Nome, pretensões, inglês e filtros. O Claude usa em todas as candidaturas.', rota: '#/perfil' },
    { id: 'curriculo', feito: curriculos().length > 0, titulo: 'Colocar o currículo', texto: `Copie o PDF (em português e, se tiver, em inglês) para ${dir ? join(dir, 'curriculos') : 'a pasta de currículos'}.`, rota: '#/perfil' },
    { id: 'rodada', feito: e.rodadas.length > 0 || e.vagas.length > 0, titulo: 'Fazer a primeira rodada', texto: 'Abra o Claude Code nesta pasta e digite /rodada. Ele busca, filtra e mostra as vagas aqui para você decidir.', rota: '#/plataformas', botao: 'Ver plataformas' },
  ];

  // Adiciona uma lista; cada item falha sozinho (duplicata ou erro) sem derrubar os outros.
  function adicionarLote(lista, c, { forcar = false, soDescoberta = false } = {}) {
    const r = { adicionadas: [], duplicadas: [], erros: [] };
    gravar(estado => {
      for (const item of lista) {
        let dados = item;
        // Importação de fora só cria vagas novas em Descoberta: etapa, envio e datas são ignorados.
        if (soDescoberta) { const { etapa, envio, followUp, descobertaEm, atualizadoEm, motivo, pendencia, subEtapa, ...resto } = item; dados = { ...resto, etapa: 'descoberta' }; }
        try {
          ({ estado } = adicionarVaga(estado, dados, c, { forcar }));
          r.adicionadas.push(dados.id);
        } catch (e) {
          if (e instanceof ErroDuplicata) r.duplicadas.push({ id: dados.id, existente: e.existente.id, etapa: e.existente.etapa });
          else r.erros.push({ id: dados.id ?? null, erro: e.message });
        }
      }
      return { estado };
    });
    return r;
  }

  const ok = (corpo, status = 200, tipo) => ({ status, corpo, ...(tipo ? { tipo } : {}) });

  // metodo: GET/POST/PUT/PATCH; caminho: '/api/v1/...' (com ?consulta); autor: 'eu' (tela) ou 'claude' (CLI, extensão, MCP).
  async function rotear({ metodo, caminho, corpo = {}, autor = 'eu' }) {
    const url = new URL(caminho, 'http://local');
    if (!url.pathname.startsWith(PREFIXO + '/')) throw new ErroApi(404, { erro: 'rota não encontrada' });
    const p = url.pathname.slice(PREFIXO.length);
    const q = Object.fromEntries(url.searchParams);
    const m = metodo.toUpperCase();
    if (m !== 'GET' && !objeto(corpo) && !(m === 'POST' && (p === '/vagas' || p === '/vagas/importar') && Array.isArray(corpo))) throw new ErroApi(400, { erro: 'envie um objeto JSON' });

    if (m === 'GET') {
      if (p === '/estado') { const e = armazem.carregar(); return ok({ ...e, hoje: hoje(), versao: versao(), curriculos: curriculos(), primeirosPassos: primeirosPassos(e) }); }
      if (p === '/versao') return ok({ versao: versao() });
      if (p === '/resumo') return ok(resumo(armazem.carregar().vagas, hoje()));
      if (p === '/vagas') return ok(armazem.carregar().vagas.filter(v => (!q.etapa || v.etapa === q.etapa) && (!q.plataforma || v.plataforma === q.plataforma)));
      if (p === '/duplicatas') return ok(duplicatasPorTitulo(armazem.carregar().vagas, { empresa: q.empresa ?? '', titulo: q.titulo ?? '' }, q.excluir));
      if (p === '/followups') { const e = armazem.carregar(); return ok(pendentesFollowUp(e.vagas, hoje()).map(v => ({ ...v, mensagem: sugerirMensagem(v, e.perfil) }))); }
      if (p === '/perfil') return ok(armazem.carregar().perfil ?? {});
      if (p === '/plataformas') return ok(armazem.carregar().plataformas);
      if (p === '/rodadas') return ok(armazem.carregar().rodadas);
      if (p === '/pedidos') return ok((armazem.carregar().pedidos ?? []).filter(x => !q.status || x.status === q.status));
      if (p === '/emails') return ok((armazem.carregar().emails ?? []).filter(x => (!q.status || x.status === q.status) && (!q.vaga || x.vagaId === q.vaga)));
      if (p === '/curriculos') return ok({ pasta: dir ? join(dir, 'curriculos') : null, arquivos: curriculos() });
      if (p === '/primeiros-passos') return ok({ passos: primeirosPassos(armazem.carregar()) });
      if (p === '/historico.md') return ok(gerarHistoricoMd(armazem.carregar(), hoje()), 200, 'text/markdown; charset=utf-8');
      if (p === '/exportar.csv') return { ...ok(csv(armazem.carregar().vagas), 200, 'text/csv; charset=utf-8'), cabecalhos: { 'content-disposition': `attachment; filename="candidaturas-${hoje()}.csv"` } };
      if (p === '/validar') {
        const e = armazem.carregar();
        const problemas = [];
        const ids = new Set();
        for (const v of e.vagas) {
          if (ids.has(v.id)) problemas.push(`${v.id}: id repetido`);
          ids.add(v.id);
          for (const erro of validarVaga(v)) problemas.push(`${v.id}: ${erro}`);
        }
        return ok({ ok: !problemas.length, problemas, vagas: e.vagas.length, eventos: e.eventos.length });
      }
      const r = p.match(/^\/vagas\/([^/]+)$/);
      if (r) {
        const id = decodificar(r[1]);
        const e = armazem.carregar();
        const v = e.vagas.find(x => x.id === id);
        if (!v) throw new ErroApi(404, { erro: `vaga não encontrada: ${id}` });
        return ok({ ...v, eventos: e.eventos.filter(x => x.vagaId === id) });
      }
    }

    if (m === 'POST' && p === '/vagas') {
      // Lista → resultado por item; objeto → a vaga criada (409 se duplicada).
      if (Array.isArray(corpo)) return ok(adicionarLote(corpo, ctx(autor)));
      if (Array.isArray(corpo.lista)) return ok(adicionarLote(corpo.lista, ctx(autor), { forcar: Boolean(corpo.forcar) }));
      const { forcar, ...dados } = corpo;
      return ok(gravar(e => adicionarVaga(e, dados, ctx(autor), { forcar: Boolean(forcar) })).vaga, 201);
    }
    if (m === 'POST' && p === '/vagas/importar') {
      const lista = [].concat(corpo).filter(objeto);
      return ok(adicionarLote(lista, ctx(autor), { soDescoberta: true }));
    }

    if (p === '/perfil' && (m === 'PUT' || m === 'PATCH')) {
      const novo = m === 'PUT' ? corpo : mesclarPerfil(armazem.carregar().perfil ?? {}, corpo);
      exigirPerfilOk(novo);
      gravar(e => ({ estado: { ...e, perfil: novo } }));
      return ok(novo);
    }

    if (m === 'POST' && p === '/pedidos') return ok(gravar(e => criarPedido(e, corpo.texto, ctx(autor))).pedido, 201);
    const rp = p.match(/^\/pedidos\/([^/]+)\/concluir$/);
    if (m === 'POST' && rp) return ok(gravar(e => concluirPedido(e, decodificar(rp[1]), corpo.resposta, ctx(autor))).pedido);

    // Emails (T1): { email, classificacao } ou { lista: [...] }. Cada item falha sozinho; repetido não duplica.
    if (m === 'POST' && p === '/emails') {
      const lote = Array.isArray(corpo.lista);
      const resultados = [];
      gravar(estado => {
        for (const item of lote ? corpo.lista : [corpo]) {
          try {
            const r = registrarEmail(estado, item ?? {}, ctx(autor));
            estado = r.estado;
            resultados.push(r.resultado);
          } catch (e) {
            if (!(e instanceof ErroValidacao)) throw e;
            resultados.push({ idMensagem: item?.email?.idMensagem ?? null, status: 'erro', erro: e.message });
          }
        }
        return { estado };
      });
      if (lote) return ok(resultados);
      return ok(resultados[0], resultados[0].status === 'erro' ? 400 : 200);
    }
    const re = p.match(/^\/emails\/([^/]+)\/(ligar|descartar)$/);
    if (m === 'POST' && re) {
      const id = decodificar(re[1]);
      return ok(gravar(e => (re[2] === 'ligar' ? ligarEmail(e, id, corpo.vagaId, ctx(autor)) : descartarEmail(e, id))).resultado);
    }

    if (m === 'POST' && p === '/backup') return ok({ criado: armazem.backupDiario(hoje()), data: hoje() });

    let r = p.match(/^\/plataformas\/([^/]+)$/);
    if (r && (m === 'PUT' || m === 'PATCH')) {
      const id = decodificar(r[1]);
      const errosPlat = ['nome', 'url', 'status', 'comoAplicar', 'limitacoes', 'ultimaVarredura'].filter(k => corpo[k] != null && typeof corpo[k] !== 'string');
      if (errosPlat.length) throw new ErroValidacao(errosPlat.map(k => `${k} deve ser texto`));
      if (contemDocumentoEm(corpo)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
      gravar(e => {
        const atual = e.plataformas.find(x => x.id === id);
        const nova = { ...(atual ?? { nome: id }), ...corpo, id };
        return { estado: { ...e, plataformas: atual ? e.plataformas.map(x => (x.id === id ? nova : x)) : [...e.plataformas, nova] } };
      });
      return ok({ ok: true, id });
    }

    r = p.match(/^\/rodadas\/([^/]+)$/);
    if (r && m === 'PUT') {
      const n = Number(decodificar(r[1]));
      if (!Number.isFinite(n)) throw new ErroValidacao(['número da rodada inválido']);
      if (contemDocumentoEm(corpo)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
      const { estado } = gravar(e => {
        const rodadas = e.rodadas.filter(x => x.n !== n);
        rodadas.push({ data: hoje(), ...e.rodadas.find(x => x.n === n), ...corpo, n });
        rodadas.sort((a, b) => a.n - b.n);
        return { estado: { ...e, rodadas } };
      });
      return ok(estado.rodadas.find(x => x.n === n));
    }

    r = p.match(/^\/vagas\/([^/]+)(\/[a-z]+)?$/);
    if (r) {
      const id = decodificar(r[1]);
      const acao = r[2];
      const c = ctx(autor);
      if (m === 'PATCH' && !acao) return ok(gravar(e => atualizarVaga(e, id, corpo, c)).vaga);
      if (m === 'POST' && acao === '/mover') {
        const { etapa, ...extras } = corpo;
        return ok(gravar(e => moverVaga(e, id, etapa, extras, c)).vaga);
      }
      if (m === 'POST' && acao === '/eventos') return ok(gravar(e => registrarEvento(e, id, corpo.tipo || 'nota', corpo.texto, c)).evento, 201);
      if (m === 'POST' && acao === '/followup') {
        if (corpo.acao === 'feito') return ok(gravar(e => followUpFeito(e, id, { ...c, texto: corpo.texto })).vaga);
        if (corpo.acao === 'adiar') return ok(gravar(e => adiarFollowUp(e, id, Number(corpo.dias) || 3, c)).vaga);
        throw new ErroApi(400, { erro: 'acao deve ser "feito" ou "adiar"' });
      }
    }
    throw new ErroApi(404, { erro: 'rota não encontrada' });
  }

  async function chamar(pedido) {
    try {
      return await rotear(pedido);
    } catch (e) {
      const r = respostaDeErro(e);
      if (r) return r;
      throw e;
    }
  }

  return { chamar, versao, curriculos };
}
