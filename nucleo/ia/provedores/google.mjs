// Google Gemini generateContent (POST /v1beta/models/{modelo}:generateContent), JSON via responseJsonSchema.
import { ErroIA } from '../erro.mjs';
import { postarJSON, exigirChave } from './http.mjs';

export const nome = 'google';
export const variavelChave = 'GEMINI_API_KEY';

// Lista fechada dos motivos de bloqueio do Gemini para textos nossos (jc-a2y.21).
// Com URL própria o servidor controla o motivo; valor fora da lista não aparece.
const MOTIVOS_BLOQUEIO = { SAFETY: 'segurança', BLOCKLIST: 'termos proibidos', PROHIBITED_CONTENT: 'conteúdo proibido', IMAGE_SAFETY: 'segurança de imagem' };
export const motivoBloqueio = (m) => (typeof m === 'string' && Object.hasOwn(MOTIVOS_BLOQUEIO, m) ? MOTIVOS_BLOQUEIO[m] : 'motivo não informado');

export async function chamar({ fetch, chave, modelo, sistema, mensagem, esquema, opcoes = {} }) {
  exigirChave(chave, nome, variavelChave);
  const r = await postarJSON({
    fetch, chave, provedor: nome,
    url: `${opcoes.url ?? 'https://generativelanguage.googleapis.com'}/v1beta/models/${encodeURIComponent(modelo)}:generateContent`,
    headers: { 'x-goog-api-key': chave },
    corpo: {
      systemInstruction: { parts: [{ text: sistema }] },
      contents: [{ role: 'user', parts: [{ text: mensagem }] }],
      generationConfig: { responseMimeType: 'application/json', responseJsonSchema: esquema },
    },
  });
  if (r.promptFeedback?.blockReason) throw new ErroIA('provedor', `${nome}: bloqueado (${motivoBloqueio(r.promptFeedback.blockReason)})`, { provedor: nome, reintentavel: false });
  const partes = r.candidates?.[0]?.content?.parts ?? [];
  const u = r.usageMetadata ?? {};
  return {
    texto: partes.filter(p => !p.thought && typeof p.text === 'string').map(p => p.text).join(''),
    tokensEntrada: u.promptTokenCount ?? null,
    // pensamento é cobrado como saída
    tokensSaida: u.candidatesTokenCount == null ? null : u.candidatesTokenCount + (u.thoughtsTokenCount ?? 0),
  };
}
