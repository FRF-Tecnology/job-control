// Assistente do próprio cliente (hoje o Claude Code) faz o papel da IA: nenhuma API é chamada.
// Devolve um pedido pendente; a CLI/MCP entrega o prompt ao assistente e depois chama ia.concluir(pendente, resposta),
// que passa a resposta pela mesma guarda dos outros provedores.
export const nome = 'assistente-local';

export async function chamar({ funcao, entrada, sistema, mensagem, esquema }) {
  return { pendente: true, funcao, entrada, prompt: { sistema, mensagem, esquema } };
}
