// Nomes dos tipos de email (de nucleo/emails.mjs).
export const TIPOS_EMAIL = ['confirmacao', 'rejeicao', 'entrevista', 'teste', 'oferta', 'outro'];
export const NOMES_EMAIL: Record<string, string> = { confirmacao: 'Candidatura recebida', rejeicao: 'Recusa', entrevista: 'Convite para entrevista', teste: 'Teste', oferta: 'Oferta', outro: 'Email' };

// E-mails que pedem para a pessoa dizer de qual vaga são (ou criar a vaga). "Candidatura recebida" e recusa sem vaga
// não mudam etapa que importe (em geral são de candidaturas feitas fora do app), então não entram na lista; o app novo
// já guarda esses como "ignorado".
export const pedeVaga = (e: any) => e?.status === 'sem-vaga' && e?.tipo !== 'confirmacao' && e?.tipo !== 'rejeicao';
