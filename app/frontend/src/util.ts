// Pequenos utilitários de interface: datas, nomes e textos (porte de web/util.mjs).
import { NOMES_ETAPA, normalizar } from './dominio/modelo';

const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
const DIAS = ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado'];
const MESES_LONGOS = ['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro'];

// Quem fez (jc-a2y.7.4.1): a pessoa pela tela, ou o assistente pelo MCP ou pela linha de comando. Canal fora da lista
// fechada vira só "assistente"; nada do pedido aparece. Ex.: "assistente (MCP)".
export type Quem = { autor?: string; canal?: string } | null | undefined;
const CANAIS: Record<string, string> = { mcp: 'MCP', cli: 'linha de comando' };
export const doAssistente = (q: Quem) => q?.autor === 'claude';
export const nomeAssistente = (canal?: string) => (canal && Object.prototype.hasOwnProperty.call(CANAIS, canal) ? `assistente (${CANAIS[canal]})` : 'assistente');

export const paraData = (iso: string) => { const [a, m, d] = iso.split('-').map(Number); return new Date(a, m - 1, d); };
export const dataCurta = (iso?: string | null) => (iso ? `${Number(iso.slice(8))} ${MESES[Number(iso.slice(5, 7)) - 1]}` : '');
export const dataLonga = (iso: string) => { const d = paraData(iso); return `${DIAS[d.getDay()]}, ${d.getDate()} de ${MESES_LONGOS[d.getMonth()]}`; };
export const diasEntre = (de: string, ate: string) => Math.round((+paraData(ate) - +paraData(de)) / 864e5);
export function haQuanto(iso: string, hoje: string) {
  const d = diasEntre(iso, hoje);
  if (d <= 0) return 'hoje';
  if (d === 1) return 'ontem';
  return `há ${d} dias`;
}

// Etapas e outros tipos de evento da linha do tempo.
const NOMES_EVENTO: Record<string, string> = { email: 'Email', 'follow-up': 'Follow-up', nota: 'Nota', retorno: 'Retorno', teste: 'Teste', entrevista: 'Entrevista' };
export const nomeEtapa = (e: string) => (NOMES_ETAPA as Record<string, string>)[e] ?? NOMES_EVENTO[e] ?? e;
export const NOMES_SUB: Record<string, string> = { teste: 'Teste online', 'entrevista-rh': 'Entrevista RH', 'entrevista-tecnica': 'Entrevista técnica', proposta: 'Proposta' };

export function nomePlataforma(id: any, plataformas: any[] = []): string {
  return plataformas.find(p => p.id === id)?.nome?.replace(/ \(.*\)$/, '') ?? ({ linkedin: 'LinkedIn', gupy: 'Gupy', inhire: 'InHire', externo: 'Site da empresa', outra: 'Outra' } as Record<string, string>)[id] ?? id;
}

export const contem = (vaga: any, termo: string) => {
  const t = normalizar(termo);
  return !t || normalizar([vaga.empresa, vaga.titulo, vaga.notas, vaga.motivoClaude, vaga.id].join(' ')).includes(t);
};

export function plural(n: number, um: string, varios: string) {
  return `${n} ${n === 1 ? um : varios}`;
}

export const encode = encodeURIComponent;

// Único jeito de um endereço de fora (vaga, plataforma, instalação) virar href: só http(s).
// javascript:, data:, file: e o resto ficam sem link (jc-a2y.29).
export function safeHref(url: unknown): string | undefined {
  if (typeof url !== 'string') return undefined;
  const t = url.trim();
  if (!/^https?:\/\//i.test(t)) return undefined;
  try {
    const u = new URL(t);
    return u.protocol === 'http:' || u.protocol === 'https:' ? u.href : undefined;
  } catch {
    return undefined;
  }
}
