// Anthropic Messages API (POST /v1/messages), saída em JSON via output_config.format (json_schema).
import { ErroIA } from '../erro.mjs';
import { postarJSON, exigirChave } from './http.mjs';

export const nome = 'anthropic';
export const variavelChave = 'ANTHROPIC_API_KEY';

export async function chamar({ fetch, chave, modelo, sistema, mensagem, esquema, opcoes = {} }) {
  exigirChave(chave, nome, variavelChave);
  const r = await postarJSON({
    fetch, chave, provedor: nome,
    url: `${opcoes.url ?? 'https://api.anthropic.com'}/v1/messages`,
    headers: { 'x-api-key': chave, 'anthropic-version': '2026-10-21' },
    corpo: {
      model: modelo,
      max_tokens: opcoes.maxTokens ?? 16000,
      system: sistema,
      messages: [{ role: 'user', content: mensagem }],
      output_config: { format: { type: 'json_schema', schema: esquema } },
    },
  });
  if (r.stop_reason === 'refusal') throw new ErroIA('provedor', `${nome}: o modelo recusou`, { provedor: nome, reintentavel: false });
  return {
    texto: (r.content ?? []).filter(b => b.type === 'text').map(b => b.text).join(''),
    tokensEntrada: r.usage?.input_tokens ?? null,
    tokensSaida: r.usage?.output_tokens ?? null,
  };
}
