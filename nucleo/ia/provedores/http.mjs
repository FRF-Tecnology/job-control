// POST JSON comum aos provedores. Rede, 5xx e 429 viram erro reintentável (aciona a rota reserva).
// A mensagem de erro é sempre um texto fixo nosso: o corpo do provedor e o texto da falha de rede nunca entram,
// porque podem ecoar a chave (inteira, escapada, codificada ou em partes). Só passa um código de erro curto
// do provedor (ex.: authentication_error), e só se for letras e "_" e não fizer parte da chave.
import { ErroIA } from '../erro.mjs';

const TEMPO_LIMITE_MS = 120_000;

export function mensagemHttp(status) {
  if (status === 400) return 'pedido recusado (confira o modelo e as opções)';
  if (status === 401) return 'chave recusada (confira a chave de API)';
  if (status === 403) return 'acesso negado (confira a chave e o plano da conta)';
  if (status === 404) return 'modelo ou endereço não encontrado';
  if (status === 408 || status === 504) return 'o provedor demorou demais';
  if (status === 413) return 'pedido grande demais';
  if (status === 429) return 'limite de uso atingido; tente mais tarde';
  if (status >= 500) return 'o provedor está com problema; tente mais tarde';
  return 'o provedor recusou o pedido';
}

const RE_CODIGO = /^[A-Za-z_]{3,48}$/;

// codigoErro tira do corpo de erro só um código seguro: error.code, error.type ou error.status (Anthropic, OpenAI, Google).
export function codigoErro(bruto, chave) {
  let j;
  try { j = JSON.parse(bruto); } catch { return ''; }
  const e = j && typeof j === 'object' ? j.error : null;
  if (!e || typeof e !== 'object') return '';
  for (const c of [e.code, e.type, e.status]) {
    if (typeof c === 'string' && RE_CODIGO.test(c) && !(chave && chave.includes(c))) return c;
  }
  return '';
}

// motivoRede descreve a falha sem repetir o texto dela (que traz endereço e pode trazer dados do pedido).
export function motivoRede(e) {
  if (e?.name === 'TimeoutError' || e?.name === 'AbortError') return 'tempo esgotado';
  const codigo = String(e?.cause?.code ?? e?.code ?? '');
  if (codigo === 'ENOTFOUND' || codigo === 'EAI_AGAIN') return 'endereço não encontrado';
  if (codigo === 'ECONNREFUSED') return 'conexão recusada';
  if (/CERT|SELF_SIGNED|UNABLE_TO_VERIFY/.test(codigo)) return 'certificado inválido';
  return 'sem conexão com o provedor';
}

export async function postarJSON({ fetch, url, headers, corpo, chave, provedor }) {
  let resp;
  try {
    resp = await fetch(url, {
      method: 'POST',
      headers: { 'content-type': 'application/json', ...headers },
      body: JSON.stringify(corpo),
      signal: AbortSignal.timeout(TEMPO_LIMITE_MS),
    });
  } catch (e) {
    throw new ErroIA('provedor', `${provedor}: falha de rede (${motivoRede(e)})`, { provedor, reintentavel: true });
  }
  const bruto = await resp.text();
  if (!resp.ok) {
    const reintentavel = resp.status === 429 || resp.status >= 500;
    const codigo = codigoErro(bruto, chave);
    const msg = `${provedor}: HTTP ${resp.status} ${mensagemHttp(resp.status)}${codigo ? ` [${codigo}]` : ''}`;
    throw new ErroIA('provedor', msg, { provedor, status: resp.status, reintentavel });
  }
  try { return JSON.parse(bruto); } catch { throw new ErroIA('provedor', `${provedor}: resposta não é JSON`, { provedor, reintentavel: true }); }
}

export const exigirChave = (chave, provedor, variavel) => {
  if (!chave) throw new ErroIA('config', `${provedor}: sem chave de API (configure provedores.${provedor}.chave ou ${variavel})`, { provedor });
};
