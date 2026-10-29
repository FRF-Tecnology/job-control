// Follow-up: só vagas ainda "Enviada" (sem retorno) com a data vencida.
export function pendentesFollowUp(vagas, hoje) {
  return vagas
    .filter(v => v.etapa === 'enviada' && v.followUp?.proximo && v.followUp.proximo <= hoje)
    .sort((a, b) => a.followUp.proximo.localeCompare(b.followUp.proximo));
}

export function sugerirMensagem(vaga, perfil = {}) {
  const nome = perfil.nome || '';
  if (vaga.idioma === 'en') {
    return `Hi! I applied for the ${vaga.titulo} position at ${vaga.empresa} and wanted to reinforce my interest. ` +
      `I work as a Java backend developer (Spring Boot, microservices, Kafka) and would be glad to talk about how I can help the team. ` +
      `Is there any update on the process?${nome ? `\n\nThanks,\n${nome}` : ''}`;
  }
  return `Olá! Tudo bem? Me candidatei para a vaga de ${vaga.titulo} na ${vaga.empresa} e queria reforçar meu interesse. ` +
    `Atuo como desenvolvedor backend Java (Spring Boot, microsserviços, Kafka) e ficaria feliz em conversar sobre como posso contribuir com o time. ` +
    `Há alguma novidade sobre o processo?${nome ? `\n\nObrigado,\n${nome}` : ''}`;
}
