// OpenAI Chat Completions (POST /v1/chat/completions), saída em JSON via response_format json_schema.
// strict:false porque os contratos têm campos opcionais; quem garante o formato é a guarda.
import { ErroIA } from '../erro.mjs';
import { postarJSON, exigirChave } from './http.mjs';

export const nome = 'openai';
export const variavelChave = 'OPENAI_API_KEY';

export async function chamar({ fetch, chave, modelo, sistema, mensagem, esquema, nomeEsquema, opcoes = {} }) {
  exigirChave(chave, nome, variavelChave);
  const r = await postarJSON({
    fetch, chave, provedor: nome,
    url: `${opcoes.url ?? 'https://api.openai.com'}/v1/chat/completions`,
    headers: { authorization: `Bearer ${chave}` },
    corpo: {
      model: modelo,
      messages: [{ role: 'system', content: sistema }, { role: 'user', content: mensagem }],
      response_format: { type: 'json_schema', json_schema: { name: nomeEsquema, schema: esquema, strict: false } },
    },
  });
  const msg = r.choices?.[0]?.message ?? {};
  if (msg.refusal) throw new ErroIA('provedor', `${nome}: o modelo recusou`, { provedor: nome, reintentavel: false });
  return {
    texto: msg.content ?? '',
    tokensEntrada: r.usage?.prompt_tokens ?? null,
    tokensSaida: r.usage?.completion_tokens ?? null,
  };
}
