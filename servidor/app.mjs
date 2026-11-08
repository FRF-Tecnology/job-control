// Servidor HTTP da Central: casca fina em volta da API v1 (servidor/api.mjs) + arquivos da interface. Sem dependências.
import { createServer } from 'node:http';
import { readFileSync, writeFileSync, existsSync, statSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { join, resolve, extname, sep, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { hojeISO } from '../nucleo/modelo.mjs';
import { criarApi, PREFIXO, ErroApi, respostaDeErro } from './api.mjs';

const raiz = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const PASTAS = { web: join(raiz, 'web'), nucleo: join(raiz, 'nucleo') };
const TIPOS = { '.html': 'text/html; charset=utf-8', '.css': 'text/css; charset=utf-8', '.mjs': 'text/javascript; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.svg': 'image/svg+xml', '.json': 'application/json; charset=utf-8', '.ico': 'image/x-icon', '.webmanifest': 'application/manifest+json; charset=utf-8' };

function lerCorpo(req) {
  return new Promise((ok, falha) => {
    let dados = '';
    req.on('data', c => { dados += c; if (dados.length > 2e6) falha(new ErroApi(413, { erro: 'corpo grande demais' })); });
    req.on('end', () => {
      if (!dados) return ok({});
      try { ok(JSON.parse(dados)); } catch { falha(new ErroApi(400, { erro: 'JSON inválido' })); }
    });
  });
}

export function criarServidor({ dir, hoje = () => hojeISO(), armazem = criarArmazem(dir, { hoje }) } = {}) {
  const api = criarApi({ armazem, dir, hoje });
  // Chave local: só quem lê dados/.chave (o Claude, nesta máquina) consegue importar de outros sites.
  const arqChave = join(dir, '.chave');
  if (!existsSync(arqChave)) writeFileSync(arqChave, randomBytes(18).toString('hex'));
  const chave = readFileSync(arqChave, 'utf8').trim();
  const CORS = { 'access-control-allow-origin': '*', 'access-control-allow-methods': 'POST, OPTIONS', 'access-control-allow-headers': 'content-type, x-central-chave', 'access-control-allow-private-network': 'true' };

  function servirArquivo(res, caminho) {
    let decodificado;
    try { decodificado = decodeURIComponent(caminho); } catch { return enviar(res, 404, { erro: 'não encontrado' }); }
    let base, rel;
    if (decodificado === '/' || decodificado === '/index.html') [base, rel] = [PASTAS.web, 'index.html'];
    // PWA: o service worker e o manifesto moram na raiz para valer no site inteiro.
    else if (decodificado === '/sw.js' || decodificado === '/manifest.webmanifest') [base, rel] = [PASTAS.web, decodificado.slice(1)];
    else if (decodificado.startsWith('/web/')) [base, rel] = [PASTAS.web, decodificado.slice(5)];
    // A tela só importa módulos soltos do núcleo (modelo, funil...). Armazéns e IA ficam de fora.
    else if (/^\/nucleo\/[\w-]+\.mjs$/.test(decodificado) && !decodificado.includes('armazenamento')) [base, rel] = [PASTAS.nucleo, decodificado.slice(8)];
    else return enviar(res, 404, { erro: 'não encontrado' });
    const alvo = resolve(base, rel);
    if (!alvo.startsWith(base + sep) || !existsSync(alvo) || !statSync(alvo).isFile()) return enviar(res, 404, { erro: 'não encontrado' });
    res.writeHead(200, { 'content-type': TIPOS[extname(alvo)] ?? 'application/octet-stream', 'cache-control': 'no-store' });
    res.end(readFileSync(alvo));
  }

  function enviar(res, status, corpo, tipo = 'application/json; charset=utf-8', cabecalhos = {}) {
    res.writeHead(status, { 'content-type': tipo, 'cache-control': 'no-store', ...cabecalhos });
    res.end(typeof corpo === 'string' ? corpo : JSON.stringify(corpo));
  }

  async function rotear(req, res) {
    const url = new URL(req.url, 'http://local');
    // Rotas antigas (/api/x) continuam valendo: viram /api/v1/x.
    let p = url.pathname;
    if (p.startsWith('/api/') && !p.startsWith(PREFIXO + '/')) p = PREFIXO + p.slice(4);
    const m = req.method;

    // Só atende quem chama pelo próprio computador (evita DNS rebinding).
    const host = String(req.headers.host ?? '').replace(/:\d+$/, '');
    if (!['localhost', '127.0.0.1', '[::1]'].includes(host)) return enviar(res, 403, { erro: 'acesso só pelo próprio computador' });

    const importar = p === PREFIXO + '/importar';
    // Gravações só da própria Central: JSON obrigatório (força preflight) e nada vindo de outro site.
    if (!['GET', 'HEAD', 'OPTIONS'].includes(m) && !importar) {
      const origem = req.headers['sec-fetch-site'];
      if (origem && !['same-origin', 'none'].includes(origem)) return enviar(res, 403, { erro: 'gravação recusada: pedido veio de outro site' });
      if (!String(req.headers['content-type'] ?? '').includes('application/json')) return enviar(res, 415, { erro: 'envie JSON (content-type: application/json)' });
    }

    // Scripts de busca rodando no LinkedIn/Gupy mandam vagas para cá com a chave local.
    if (importar) {
      if (m === 'OPTIONS') { res.writeHead(204, CORS); return res.end(); }
      for (const [k, v] of Object.entries(CORS)) res.setHeader(k, v);
      if (m !== 'POST') return enviar(res, 405, { erro: 'use POST' });
      if (req.headers['x-central-chave'] !== chave) return enviar(res, 403, { erro: 'chave inválida' });
      const r = await api.chamar({ metodo: 'POST', caminho: PREFIXO + '/vagas/importar', corpo: [].concat(await lerCorpo(req)), autor: 'claude' });
      return enviar(res, r.status, r.corpo);
    }
    if (m === 'GET' && p === PREFIXO + '/chave') return enviar(res, 200, { chave });

    if (!p.startsWith(PREFIXO + '/')) return m === 'GET' ? servirArquivo(res, p) : enviar(res, 405, { erro: 'método não permitido' });

    const corpo = m === 'GET' ? {} : await lerCorpo(req);
    const r = await api.chamar({ metodo: m, caminho: p + url.search, corpo, autor: 'eu' });
    return enviar(res, r.status, r.corpo, r.tipo, r.cabecalhos);
  }

  return createServer(async (req, res) => {
    try {
      await rotear(req, res);
    } catch (e) {
      const r = respostaDeErro(e);
      if (r) return enviar(res, r.status, r.corpo);
      console.error(e);
      enviar(res, 500, { erro: e.message });
    }
  });
}
