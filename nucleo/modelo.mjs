// Vocabulário comum da central: etapas, motivos e utilitários de data/texto.

export const ETAPAS = ['descoberta', 'aplicar', 'enviada', 'bloqueada', 'em-processo', 'oferta', 'encerrada', 'pulada'];

export const NOMES_ETAPA = {
  descoberta: 'Descoberta',
  aplicar: 'Aplicar',
  enviada: 'Enviada',
  bloqueada: 'Precisa de você',
  'em-processo': 'Em processo',
  oferta: 'Oferta',
  encerrada: 'Encerrada',
  pulada: 'Pulada',
};

export const SUB_ETAPAS = ['teste', 'entrevista-rh', 'entrevista-tecnica', 'proposta'];

export const MOTIVOS_PULO = ['Sênior', 'Inglês', 'Diploma', 'Stack', 'Presencial/Local', 'Documento', 'Duplicada', 'Vaga fechada', 'Outro'];
export const MOTIVOS_ENCERRAMENTO = ['Reprovada', 'Sem retorno', 'Vaga fechada', 'Desisti', 'Outro'];

// Etapas que exigem um texto explicando o porquê.
export const PEDE_MOTIVO = ['pulada', 'encerrada'];

export function normalizar(texto) {
  return String(texto ?? '')
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .trim();
}

// Datas sempre como 'AAAA-MM-DD' no fuso local.
export function hojeISO(data = new Date()) {
  const p = n => String(n).padStart(2, '0');
  return `${data.getFullYear()}-${p(data.getMonth() + 1)}-${p(data.getDate())}`;
}

export function somarDias(iso, dias) {
  const [a, m, d] = iso.split('-').map(Number);
  return hojeISO(new Date(a, m - 1, d + dias));
}

export function ehDataISO(v) {
  return typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v);
}

export function foiEnviada(vaga) {
  return Boolean(vaga?.envio?.data);
}
