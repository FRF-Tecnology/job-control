// Follow-up: só vagas ainda "Enviada" (sem retorno) com a data vencida.
export function pendentesFollowUp(vagas, hoje) {
  return vagas
    .filter(v => v.etapa === 'enviada' && v.followUp?.proximo && v.followUp.proximo <= hoje)
    .sort((a, b) => a.followUp.proximo.localeCompare(b.followUp.proximo));
}

const juntar = (itens, e) => itens.length > 1 ? `${itens.slice(0, -1).join(', ')} ${e} ${itens.at(-1)}` : itens[0] ?? '';

// A apresentação vem só do perfil da pessoa. Sem perfil, a frase sai — nunca um texto pronto com experiência de outra pessoa.
function apresentacao(perfil, ingles) {
  const stack = (perfil.stackPrincipal ?? []).filter(Boolean).slice(0, 3);
  if (ingles) return stack.length ? `I work with ${juntar(stack, 'and')} and would be glad to talk about how I can help the team. ` : '';
  const cargo = String(perfil.cargo ?? '').trim();
  if (cargo) return `Atuo como ${cargo.charAt(0).toLowerCase()}${cargo.slice(1)} e ficaria feliz em conversar sobre como posso contribuir com o time. `;
  return stack.length ? `Trabalho com ${juntar(stack, 'e')} e ficaria feliz em conversar sobre como posso contribuir com o time. ` : '';
}

export function sugerirMensagem(vaga, perfil = {}) {
  const nome = perfil.nome || '';
  if (vaga.idioma === 'en') {
    return `Hi! I applied for the ${vaga.titulo} position at ${vaga.empresa} and wanted to reinforce my interest. ` +
      apresentacao(perfil, true) +
      `Is there any update on the process?${nome ? `\n\nThanks,\n${nome}` : ''}`;
  }
  return `Olá! Tudo bem? Me candidatei para a vaga de ${vaga.titulo} na ${vaga.empresa} e queria reforçar meu interesse. ` +
    apresentacao(perfil, false) +
    `Há alguma novidade sobre o processo?${nome ? `\n\nUm abraço,\n${nome}` : ''}`;
}
