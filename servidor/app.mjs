// Servidor HTTP da Central: API JSON + arquivos da interface. Sem dependências.
import { createServer } from 'node:http';
import { readFileSync, writeFileSync, existsSync, statSync, readdirSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { join, resolve, extname, sep, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { adicionarVaga, moverVaga, atualizarVaga, registrarEvento, followUpFeito, adiarFollowUp, ErroValidacao, ErroDuplicata } from '../nucleo/servico.mjs';
import { perfilSensivel, contemDocumentoEm, validarPerfil } from '../nucleo/validacao.mjs';
import { gerarHistoricoMd } from '../nucleo/historico.mjs';
import { hojeISO } from '../nucleo/modelo.mjs';

const raiz = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const PASTAS = { web: join(raiz, 'web'), nucleo: join(raiz, 'nucleo') };
const TIPOS = { '.html': 'text/html; charset=utf-8', '.css': 'text/css; charset=utf-8', '.mjs': 'text/javascript; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.svg': 'image/svg+xml', '.json': 'application/json; charset=utf-8', '.ico': 'image/x-icon' };
const ARQ_DADOS = ['vagas.json', 'eventos.json', 'plataformas.json', 'rodadas.json', 'perfil.json'];

// Id na URL com codificação quebrada não é uma vaga que existe.
function decodificar(s) {
  try { return decodeURIComponent(s); } catch { throw new ErroHttp(404, { erro: 'vaga não encontrada' }); }
}

class ErroHttp extends Error { constructor(status, corpo) { super(corpo.erro); this.status = status; this.corpo = corpo; } }

function lerCorpo(req) {
  return new Promise((ok, falha) => {
    let dados = '';
    req.on('data', c => { dados += c; if (dados.length > 2e6) falha(new ErroHttp(413, { erro: 'corpo grande demais' })); });
    req.on('end', () => {
      if (!dados) return ok({});
      try { ok(JSON.parse(dados)); } catch { falha(new ErroHttp(400, { erro: 'JSON inválido' })); }
    });
  });
}

function csv(vagas) {
  const cols = ['id', 'empresa', 'titulo', 'plataforma', 'etapa', 'subEtapa', 'aderencia', 'contrato', 'motivo', 'pendencia', 'dataEnvio', 'rodada', 'pretensao', 'proximoFollowUp', 'link', 'notas'];
  const valor = (v, c) => ({ dataEnvio: v.envio?.data, rodada: v.envio?.rodada, pretensao: v.envio?.pretensao, proximoFollowUp: v.followUp?.proximo }[c] ?? v[c] ?? '');
  // Apóstrofo na frente de =, +, -, @ impede o Excel de tratar o texto como fórmula.
  const esc = x => `"${String(x).replace(/^[=+\-@\t\r]/, "'$&").replace(/"/g, '""')}"`;
  return '﻿' + [cols.join(';'), ...vagas.map(v => cols.map(c => esc(valor(v, c))).join(';'))].join('\n');
}

export function criarServidor({ dir, hoje = () => hojeISO() } = {}) {
  const armazem = criarArmazem(dir, { hoje });
  let gravacoes = 0;

  const ctx = extra => ({ autor: 'eu', agora: hoje(), diasFollowUp: Number(armazem.carregar().perfil?.diasFollowUp) || 7, ...extra });
  const gravar = fn => { const r = armazem.transacao(fn); gravacoes++; return r; };
  // Chave local: só quem lê dados/.chave (o Claude, nesta máquina) consegue importar de outros sites.
  const arqChave = join(dir, '.chave');
  if (!existsSync(arqChave)) writeFileSync(arqChave, randomBytes(18).toString('hex'));
  const chave = readFileSync(arqChave, 'utf8').trim();
  const CORS = { 'access-control-allow-origin': '*', 'access-control-allow-methods': 'POST, OPTIONS', 'access-control-allow-headers': 'content-type, x-central-chave', 'access-control-allow-private-network': 'true' };

  const versao = () => `${gravacoes}-${ARQ_DADOS.map(a => { const p = join(dir, a); return existsSync(p) ? `${statSync(p).mtimeMs}:${statSync(p).size}` : '0'; }).join('|')}|${curriculos().map(c => c.nome).join(',')}`;

  function servirArquivo(res, caminho) {
    let decodificado;
    try { decodificado = decodeURIComponent(caminho); } catch { return enviar(res, 404, { erro: 'não encontrado' }); }
    let base, rel;
    if (decodificado === '/' || decodificado === '/index.html') [base, rel] = [PASTAS.web, 'index.html'];
    else if (decodificado.startsWith('/web/')) [base, rel] = [PASTAS.web, decodificado.slice(5)];
    else if (decodificado.startsWith('/nucleo/') && decodificado.endsWith('.mjs') && !decodificado.includes('armazenamento')) [base, rel] = [PASTAS.nucleo, decodificado.slice(8)];
    else return enviar(res, 404, { erro: 'não encontrado' });
    const alvo = resolve(base, rel);
    if (!alvo.startsWith(base + sep) || !existsSync(alvo) || !statSync(alvo).isFile()) return enviar(res, 404, { erro: 'não encontrado' });
    res.writeHead(200, { 'content-type': TIPOS[extname(alvo)] ?? 'application/octet-stream', 'cache-control': 'no-store' });
    res.end(readFileSync(alvo));
  }

  function curriculos() {
    const pasta = join(dir, 'curriculos');
    if (!existsSync(pasta)) return [];
    return readdirSync(pasta).filter(n => /\.pdf$/i.test(n)).sort().map(nome => ({ nome, tamanho: statSync(join(pasta, nome)).size }));
  }

  // Checklist da primeira vez (mostrada na tela Hoje até tudo estar feito).
  function primeirosPassos(e) {
    return [
      { id: 'perfil', feito: Boolean(String(e.perfil?.nome ?? '').trim()), titulo: 'Preencher o perfil', texto: 'Nome, pretensões, inglês e filtros. O Claude usa em todas as candidaturas.', rota: '#/perfil' },
      { id: 'curriculo', feito: curriculos().length > 0, titulo: 'Colocar o currículo', texto: `Copie o PDF (em português e, se tiver, em inglês) para ${join(dir, 'curriculos')}.`, rota: '#/perfil' },
      { id: 'rodada', feito: e.rodadas.length > 0 || e.vagas.length > 0, titulo: 'Fazer a primeira rodada', texto: 'Abra o Claude Code nesta pasta e digite /rodada. Ele busca, filtra e mostra as vagas aqui para você decidir.', rota: '#/plataformas', botao: 'Ver plataformas' },
    ];
  }

  function enviar(res, status, corpo, tipo = 'application/json; charset=utf-8') {
    res.writeHead(status, { 'content-type': tipo, 'cache-control': 'no-store' });
    res.end(typeof corpo === 'string' ? corpo : JSON.stringify(corpo));
  }

  async function rotear(req, res) {
    const url = new URL(req.url, 'http://local');
    const p = url.pathname;
    const m = req.method;

    // Só atende quem chama pelo próprio computador (evita DNS rebinding).
    const host = String(req.headers.host ?? '').replace(/:\d+$/, '');
    if (!['localhost', '127.0.0.1', '[::1]'].includes(host)) return enviar(res, 403, { erro: 'acesso só pelo próprio computador' });

    // Gravações só da própria Central: JSON obrigatório (força preflight) e nada vindo de outro site.
    if (!['GET', 'HEAD', 'OPTIONS'].includes(m) && p !== '/api/importar') {
      const origem = req.headers['sec-fetch-site'];
      if (origem && !['same-origin', 'none'].includes(origem)) return enviar(res, 403, { erro: 'gravação recusada: pedido veio de outro site' });
      if (!String(req.headers['content-type'] ?? '').includes('application/json')) return enviar(res, 415, { erro: 'envie JSON (content-type: application/json)' });
    }

    if (p === '/api/importar') {
      if (m === 'OPTIONS') { res.writeHead(204, CORS); return res.end(); }
      for (const [k, v] of Object.entries(CORS)) res.setHeader(k, v);
      if (m !== 'POST') return enviar(res, 405, { erro: 'use POST' });
      if (req.headers['x-central-chave'] !== chave) return enviar(res, 403, { erro: 'chave inválida' });
      // Importação só cria vagas novas em Descoberta: etapa, envio e datas vindos de fora são ignorados.
      const lista = [].concat(await lerCorpo(req)).filter(x => x && typeof x === 'object' && !Array.isArray(x));
      const r = { adicionadas: [], duplicadas: [], erros: [] };
      gravar(estado => {
        const c = ctx({ autor: 'claude' });
        for (const item of lista) {
          const { etapa, envio, followUp, descobertaEm, atualizadoEm, motivo, pendencia, subEtapa, ...dados } = item;
          try {
            ({ estado } = adicionarVaga(estado, { ...dados, etapa: 'descoberta' }, c));
            r.adicionadas.push(dados.id);
          } catch (e) {
            if (e instanceof ErroDuplicata) r.duplicadas.push({ id: dados.id, existente: e.existente.id, etapa: e.existente.etapa });
            else r.erros.push({ id: dados.id ?? null, erro: e.message });
          }
        }
        return { estado };
      });
      return enviar(res, 200, r);
    }
    if (m === 'GET' && p === '/api/chave') return enviar(res, 200, { chave });

    if (!p.startsWith('/api/')) return m === 'GET' ? servirArquivo(res, p) : enviar(res, 405, { erro: 'método não permitido' });

    if (m === 'GET' && p === '/api/estado') { const e = armazem.carregar(); return enviar(res, 200, { ...e, hoje: hoje(), versao: versao(), curriculos: curriculos(), primeirosPassos: primeirosPassos(e) }); }
    if (m === 'GET' && p === '/api/versao') return enviar(res, 200, { versao: versao() });
    if (m === 'GET' && p === '/api/exportar.csv') {
      res.setHeader('content-disposition', `attachment; filename="candidaturas-${hoje()}.csv"`);
      return enviar(res, 200, csv(armazem.carregar().vagas), 'text/csv; charset=utf-8');
    }
    if (m === 'GET' && p === '/api/curriculos') return enviar(res, 200, { pasta: join(dir, 'curriculos'), arquivos: curriculos() });
    if (m === 'GET' && p === '/api/primeiros-passos') return enviar(res, 200, { passos: primeirosPassos(armazem.carregar()) });
    if (m === 'GET' && p === '/api/historico.md') return enviar(res, 200, gerarHistoricoMd(armazem.carregar(), hoje()), 'text/markdown; charset=utf-8');

    const corpo = m === 'GET' ? {} : await lerCorpo(req);
    if (corpo === null || typeof corpo !== 'object' || Array.isArray(corpo)) return enviar(res, 400, { erro: 'envie um objeto JSON' });

    if (m === 'POST' && p === '/api/vagas') {
      const { forcar, ...dados } = corpo;
      const { vaga } = gravar(e => adicionarVaga(e, dados, ctx(), { forcar: Boolean(forcar) }));
      return enviar(res, 201, vaga);
    }
    if (m === 'PUT' && p === '/api/perfil') {
      const errosPerfil = validarPerfil(corpo);
      if (errosPerfil.length) throw new ErroValidacao(errosPerfil);
      if (perfilSensivel(corpo)) throw new ErroValidacao(['o perfil não pode guardar CPF, RG ou senha (nem respostas padrão sobre eles)']);
      gravar(e => ({ estado: { ...e, perfil: corpo } }));
      return enviar(res, 200, corpo);
    }
    let r = p.match(/^\/api\/plataformas\/([^/]+)$/);
    if (m === 'PUT' && r) {
      const id = decodificar(r[1]);
      const errosPlat = ['nome', 'url', 'status', 'comoAplicar', 'limitacoes', 'ultimaVarredura'].filter(k => corpo[k] != null && typeof corpo[k] !== 'string');
      if (errosPlat.length) throw new ErroValidacao(errosPlat.map(k => `${k} deve ser texto`));
      if (contemDocumentoEm(corpo)) throw new ErroValidacao(['parece conter CPF/RG — documentos não são guardados na central']);
      gravar(e => ({ estado: { ...e, plataformas: e.plataformas.some(x => x.id === id) ? e.plataformas.map(x => (x.id === id ? { ...x, ...corpo, id } : x)) : [...e.plataformas, { ...corpo, id }] } }));
      return enviar(res, 200, { ok: true });
    }

    r = p.match(/^\/api\/vagas\/([^/]+)(\/[a-z]+)?$/);
    if (r) {
      const id = decodificar(r[1]);
      const acao = r[2];
      if (m === 'PATCH' && !acao) return enviar(res, 200, gravar(e => atualizarVaga(e, id, corpo, ctx())).vaga);
      if (m === 'POST' && acao === '/mover') {
        const { etapa, ...extras } = corpo;
        return enviar(res, 200, gravar(e => moverVaga(e, id, etapa, extras, ctx())).vaga);
      }
      if (m === 'POST' && acao === '/eventos') return enviar(res, 201, gravar(e => registrarEvento(e, id, corpo.tipo || 'nota', corpo.texto, ctx())).evento);
      if (m === 'POST' && acao === '/followup') {
        if (corpo.acao === 'feito') return enviar(res, 200, gravar(e => followUpFeito(e, id, ctx({ texto: corpo.texto }))).vaga);
        if (corpo.acao === 'adiar') return enviar(res, 200, gravar(e => adiarFollowUp(e, id, Number(corpo.dias) || 3, ctx())).vaga);
        return enviar(res, 400, { erro: 'acao deve ser "feito" ou "adiar"' });
      }
    }
    return enviar(res, 404, { erro: 'rota não encontrada' });
  }

  return createServer(async (req, res) => {
    try {
      await rotear(req, res);
    } catch (e) {
      if (e instanceof ErroHttp) return enviar(res, e.status, e.corpo);
      if (e instanceof ErroValidacao) return enviar(res, 400, { erro: e.message, erros: e.erros });
      if (e instanceof ErroDuplicata) return enviar(res, 409, { erro: e.message, existente: e.existente });
      if (/não encontrada/.test(e.message)) return enviar(res, 404, { erro: e.message });
      console.error(e);
      enviar(res, 500, { erro: e.message });
    }
  });
}
