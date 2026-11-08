#!/usr/bin/env node
// Servidor MCP local da Central (T8): JSON-RPC 2.0 sobre stdio, sem dependências.
// Qualquer cliente MCP (Claude Desktop, Claude Code, a futura extensão) usa a Central com ferramentas tipadas.
// Grava pela mesma API v1 e com a mesma trava de arquivo da Central e da CLI: os três podem rodar juntos.
// stdout é só do protocolo; avisos vão para stderr.
import { join, dirname } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { readFileSync } from 'node:fs';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { hojeISO } from '../nucleo/modelo.mjs';
import { criarApi } from '../servidor/api.mjs';
import { criarFerramentas, ErroFerramenta } from './ferramentas.mjs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const VERSOES = ['2025-11-25', '2025-06-18', '2025-03-26', '2024-11-05'];

// As regras do CLAUDE.md que o cliente precisa saber (as de dados também são barradas na API).
export const INSTRUCOES = [
  'Central de Candidaturas: vagas, triagem, envios, follow-ups e retornos de uma pessoa.',
  'Leia perfil_ler antes de triar ou aplicar. Prioridade: vagas USD LATAM de empresas dos EUA, depois LATAM + Europa overseas, depois nacionais.',
  'Nunca peça, escreva ou guarde CPF, RG, senha ou dados bancários (a Central recusa). Nunca invente experiência: use resposta_padrao; sem resposta, 0 / Não.',
  'Login, captcha e criação de conta ficam com a pessoa: mova a vaga para "bloqueada" com a pendência.',
  'Não existe ferramenta de enviar mensagem. Follow-up e contato com recrutador são rascunhos até a pessoa dizer "pode mandar".',
  'LinkedIn: no máximo 30–40 candidaturas por dia. Antes de aplicar, use vaga_duplicada.',
].join('\n');

const resposta = (id, result) => ({ jsonrpc: '2.0', id, result });
const erro = (id, code, message) => ({ jsonrpc: '2.0', id, error: { code, message } });

// Confere os campos obrigatórios e os tipos básicos do inputSchema (o resto a API valida).
function faltando(schema, args) {
  const problemas = (schema.required ?? []).filter(k => args[k] == null || args[k] === '').map(k => `falta "${k}"`);
  for (const [k, s] of Object.entries(schema.properties ?? {})) {
    const v = args[k];
    if (v == null) continue;
    if (s.type === 'string' && typeof v !== 'string') problemas.push(`"${k}" deve ser texto`);
    if (s.type === 'number' && typeof v !== 'number') problemas.push(`"${k}" deve ser número`);
    if (s.type === 'array' && !Array.isArray(v)) problemas.push(`"${k}" deve ser uma lista`);
    if (s.enum && !s.enum.includes(v)) problemas.push(`"${k}" deve ser ${s.enum.join(', ')}`);
  }
  return problemas;
}

export function criarServidorMcp({ api, versao = '0.1.0' }) {
  const ferramentas = criarFerramentas(api);
  const esquemas = new Map(ferramentas.listar().map(f => [f.name, f.inputSchema]));

  async function tratar(msg) {
    if (!msg || msg.jsonrpc !== '2.0' || typeof msg.method !== 'string') return msg?.id != null ? erro(msg.id, -32600, 'pedido inválido') : null;
    const { id, method, params = {} } = msg;
    const notificacao = id === undefined;
    switch (method) {
      case 'initialize':
        return resposta(id, {
          protocolVersion: VERSOES.includes(params.protocolVersion) ? params.protocolVersion : VERSOES[1],
          capabilities: { tools: { listChanged: false } },
          serverInfo: { name: 'central-candidaturas', title: 'Central de Candidaturas', version: versao },
          instructions: INSTRUCOES,
        });
      case 'notifications/initialized':
      case 'notifications/cancelled':
        return null;
      case 'ping':
        return resposta(id, {});
      case 'tools/list':
        return resposta(id, { tools: ferramentas.listar() });
      case 'tools/call': {
        const { name, arguments: args = {} } = params;
        if (!ferramentas.existe(name)) return erro(id, -32602, `ferramenta desconhecida: ${name}`);
        const problemas = faltando(esquemas.get(name), args ?? {});
        if (problemas.length) return resposta(id, { content: [{ type: 'text', text: `Argumentos inválidos: ${problemas.join('; ')}` }], isError: true });
        try {
          const r = await ferramentas.rodar(name, args);
          return resposta(id, { content: [{ type: 'text', text: JSON.stringify(r, null, 1) }] });
        } catch (e) {
          // Erro de regra/validação volta para o modelo corrigir; erro inesperado também, sem derrubar o servidor.
          if (!(e instanceof ErroFerramenta)) console.error(`[central-mcp] ${name}: ${e.stack ?? e.message}`);
          return resposta(id, { content: [{ type: 'text', text: `Recusado: ${e.message}` }], isError: true });
        }
      }
      default:
        return notificacao ? null : erro(id, -32601, `método não suportado: ${method}`);
    }
  }

  // Recebe uma linha de texto (uma mensagem JSON-RPC ou uma lista) e devolve as respostas a escrever.
  async function receber(linha) {
    let msg;
    try { msg = JSON.parse(linha); } catch { return [erro(null, -32700, 'JSON inválido')]; }
    const lista = Array.isArray(msg) ? msg : [msg];
    const saidas = [];
    for (const m of lista) { const r = await tratar(m); if (r) saidas.push(r); }
    return saidas;
  }

  return { receber, tratar };
}

// Uso direto: node mcp/servidor.mjs (pasta de dados em CENTRAL_DADOS ou ./dados).
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const dir = process.env.CENTRAL_DADOS || join(raiz, 'dados');
  const armazem = criarArmazem(dir, { hoje: () => hojeISO() });
  const versao = JSON.parse(readFileSync(join(raiz, 'package.json'), 'utf8')).version;
  const { receber } = criarServidorMcp({ api: criarApi({ armazem, dir, hoje: () => process.env.CENTRAL_HOJE || hojeISO() }), versao });
  let resto = '';
  let fila = Promise.resolve();
  process.stdin.setEncoding('utf8');
  process.stdin.on('data', pedaco => {
    resto += pedaco;
    const linhas = resto.split('\n');
    resto = linhas.pop();
    for (const linha of linhas.map(l => l.trim()).filter(Boolean)) {
      // Uma de cada vez, na ordem em que chegaram.
      fila = fila.then(() => receber(linha)).then(saidas => { for (const s of saidas) process.stdout.write(`${JSON.stringify(s)}\n`); })
        .catch(e => console.error(`[central-mcp] ${e.stack ?? e.message}`));
    }
  });
  process.stdin.on('end', () => fila.then(() => process.exit(0)));
}
