// POST JSON comum aos provedores. Rede, 5xx e 429 viram erro reintentável (aciona a rota reserva).
// A mensagem de erro é sempre um texto fixo nosso: nenhum valor do corpo do provedor nem o texto da falha de rede
// entram, porque podem ecoar a chave (inteira, escapada, codificada, em partes ou com a caixa trocada).
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
    throw new ErroIA('provedor', `${provedor}: HTTP ${resp.status} ${mensagemHttp(resp.status)}`, { provedor, status: resp.status, reintentavel });
  }
  try { return JSON.parse(bruto); } catch { throw new ErroIA('provedor', `${provedor}: resposta não é JSON`, { provedor, reintentavel: true }); }
}

export const exigirChave = (chave, provedor, variavel) => {
  if (!chave) throw new ErroIA('config', `${provedor}: sem chave de API (configure provedores.${provedor}.chave ou ${variavel})`, { provedor });
};
