// Pequenos utilitários de interface: criação de elementos, datas e textos.
import { NOMES_ETAPA, normalizar } from '../nucleo/modelo.mjs';

// h('div.classe#id', {atributos, on:{click}}, ...filhos)
export function h(tag, props, ...filhos) {
  const [nome, ...resto] = tag.split(/(?=[.#])/);
  const el = document.createElement(nome || 'div');
  for (const r of resto) r[0] === '.' ? el.classList.add(r.slice(1)) : (el.id = r.slice(1));
  if (props && (typeof props !== 'object' || props instanceof Node || Array.isArray(props))) { filhos.unshift(props); props = null; }
  for (const [k, v] of Object.entries(props ?? {})) {
    if (v == null || v === false) continue;
    if (k === 'on') for (const [ev, fn] of Object.entries(v)) el.addEventListener(ev, fn);
    else if (k === 'class') el.className += ` ${v}`;
    else if (k === 'style' && typeof v === 'object') for (const [prop, val] of Object.entries(v)) prop.startsWith('--') ? el.style.setProperty(prop, val) : (el.style[prop] = val);
    else if (k in el && k !== 'list' && typeof v !== 'string') el[k] = v;
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const f of filhos.flat(Infinity)) {
    if (f == null || f === false) continue;
    el.append(f instanceof Node ? f : document.createTextNode(String(f)));
  }
  return el;
}

const MESES = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];
const DIAS = ['domingo', 'segunda', 'terça', 'quarta', 'quinta', 'sexta', 'sábado'];

export const paraData = iso => { const [a, m, d] = iso.split('-').map(Number); return new Date(a, m - 1, d); };
export const dataCurta = iso => (iso ? `${Number(iso.slice(8))} ${MESES[Number(iso.slice(5, 7)) - 1]}` : '');
export const dataLonga = iso => { const d = paraData(iso); return `${DIAS[d.getDay()]}, ${d.getDate()} de ${['janeiro', 'fevereiro', 'março', 'abril', 'maio', 'junho', 'julho', 'agosto', 'setembro', 'outubro', 'novembro', 'dezembro'][d.getMonth()]}`; };
export const diasEntre = (de, ate) => Math.round((paraData(ate) - paraData(de)) / 864e5);
export function haQuanto(iso, hoje) {
  const d = diasEntre(iso, hoje);
  if (d <= 0) return 'hoje';
  if (d === 1) return 'ontem';
  return `há ${d} dias`;
}

export const nomeEtapa = e => NOMES_ETAPA[e] ?? e;
export const NOMES_SUB = { teste: 'Teste online', 'entrevista-rh': 'Entrevista RH', 'entrevista-tecnica': 'Entrevista técnica', proposta: 'Proposta' };

export function nomePlataforma(id, plataformas = []) {
  return plataformas.find(p => p.id === id)?.nome?.replace(/ \(.*\)$/, '') ?? ({ linkedin: 'LinkedIn', gupy: 'Gupy', inhire: 'InHire', externo: 'Site da empresa', outra: 'Outra' }[id] ?? id);
}

export const contem = (vaga, termo) => {
  const t = normalizar(termo);
  return !t || normalizar([vaga.empresa, vaga.titulo, vaga.notas, vaga.motivoClaude, vaga.id].join(' ')).includes(t);
};

export function etapaChip(etapa) {
  return h(`span.etapa.e-${etapa}`, nomeEtapa(etapa));
}

export function plural(n, um, varios) {
  return `${n} ${n === 1 ? um : varios}`;
}
