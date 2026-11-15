// Nomes dos tipos de email (de nucleo/emails.mjs).
export const TIPOS_EMAIL = ['confirmacao', 'rejeicao', 'entrevista', 'teste', 'oferta', 'outro'];
export const NOMES_EMAIL: Record<string, string> = { confirmacao: 'Candidatura recebida', rejeicao: 'Recusa', entrevista: 'Convite para entrevista', teste: 'Teste', oferta: 'Oferta', outro: 'Email' };

// E-mails que pedem para a pessoa dizer de qual vaga são. "Candidatura recebida" sem vaga não muda etapa nenhuma (em
// geral é de uma candidatura feita fora do app), então não entra na lista; o app novo já guarda esses como "ignorado".
export const pedeVaga = (e: any) => e?.status === 'sem-vaga' && e?.tipo !== 'confirmacao';
