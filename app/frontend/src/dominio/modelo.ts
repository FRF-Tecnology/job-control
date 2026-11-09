// Porte de nucleo/modelo.mjs (o mesmo código; conferido contra o golden do Node em dominio.test.ts).

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

// Mercado da vaga e foco da busca (seletor Internacional / Nacional / Os dois).
export const MERCADOS = ['internacional', 'nacional'];
export const FOCOS = ['internacional', 'nacional', 'ambos'];

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

// Vaga sem mercado anotado: palpite pelas anotações (CLT/nacional pesa mais) e pelo idioma.
export function mercadoDe(v) {
  if (MERCADOS.includes(v?.mercado)) return v.mercado;
  const texto = normalizar(`${v?.motivoClaude ?? ''} ${v?.notas ?? ''} ${v?.contrato ?? ''}`);
  if (/\bclt\b|nacional|\bbrl\b|reais/.test(texto)) return 'nacional';
  if (/\busd\b|contractor|overseas|dolar/.test(texto) || v?.idioma === 'en') return 'internacional';
  return 'nacional';
}
export const noFoco = (v, foco) => !foco || foco === 'ambos' || mercadoDe(v) === foco;

export function foiEnviada(vaga) {
  return Boolean(vaga?.envio?.data);
}

// Valor em dinheiro digitado à brasileira: "8.500", "8.500,50", "R$ 8.500" ou "8500.5".
export function lerValor(texto) {
  const s = String(texto ?? '').replace(/[R$\s]/g, '');
  if (/^\d{1,3}(\.\d{3})+(,\d+)?$/.test(s)) return Number(s.replace(/\./g, '').replace(',', '.'));
  if (/^\d+(,\d+)?$/.test(s)) return Number(s.replace(',', '.'));
  if (/^\d+(\.\d+)?$/.test(s)) return Number(s);
  return NaN;
}
