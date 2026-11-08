// POST JSON comum aos provedores. Rede, 5xx e 429 viram erro reintentável (aciona a rota reserva).
// A chave nunca aparece em mensagem de erro.
import { ErroIA } from '../erro.mjs';

const TEMPO_LIMITE_MS = 120_000;

export async function postarJSON({ fetch, url, headers, corpo, chave, provedor }) {
  const limpar = s => (chave ? String(s).split(chave).join('***') : String(s));
  let resp;
  try {
    resp = await fetch(url, {
      method: 'POST',
      headers: { 'content-type': 'application/json', ...headers },
      body: JSON.stringify(corpo),
      signal: AbortSignal.timeout(TEMPO_LIMITE_MS),
    });
  } catch (e) {
    throw new ErroIA('provedor', `${provedor}: falha de rede (${limpar(e.message)})`, { provedor, reintentavel: true });
  }
  const bruto = await resp.text();
  if (!resp.ok) {
    const reintentavel = resp.status === 429 || resp.status >= 500;
    throw new ErroIA('provedor', `${provedor}: HTTP ${resp.status} ${limpar(bruto).slice(0, 300)}`, { provedor, status: resp.status, reintentavel });
  }
  try { return JSON.parse(bruto); } catch { throw new ErroIA('provedor', `${provedor}: resposta não é JSON`, { provedor, reintentavel: true }); }
}

export const exigirChave = (chave, provedor, variavel) => {
  if (!chave) throw new ErroIA('config', `${provedor}: sem chave de API (configure provedores.${provedor}.chave ou ${variavel})`, { provedor });
};
