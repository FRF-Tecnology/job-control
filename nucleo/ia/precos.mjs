// Tabela editável de preços em USD por 1 milhão de tokens. Modelo fora da tabela: custo null (desconhecido).
// Anthropic: preços públicos de 2026-10 (Haiku 5.5: até 100K tokens de prompt; acima disso $0.50/$2.50).
// OpenAI e Google: preencher com a página de preços de cada um antes de usar em produção.
export const PRECOS = {
  'claude-opus-5-5': { entrada: 4, saida: 20 },
  'claude-sonnet-5-5': { entrada: 2, saida: 10 },
  'claude-haiku-5-5': { entrada: 0.1, saida: 0.5 },
  'claude-haiku-4-5': { entrada: 1, saida: 5 },
};

export function custoEstimado(modelo, tokensEntrada, tokensSaida, precos = PRECOS) {
  const p = precos[modelo];
  if (!p || tokensEntrada == null || tokensSaida == null) return null;
  return Number(((tokensEntrada * p.entrada + tokensSaida * p.saida) / 1e6).toFixed(6));
}
