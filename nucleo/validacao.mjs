import { ETAPAS, PEDE_MOTIVO, ehDataISO } from './modelo.mjs';

function cpfValido(digitos) {
  if (!/^\d{11}$/.test(digitos) || /^(\d)\1{10}$/.test(digitos)) return false;
  const dv = n => {
    let soma = 0;
    for (let i = 0; i < n; i++) soma += Number(digitos[i]) * (n + 1 - i);
    const r = (soma * 10) % 11;
    return r === 10 ? 0 : r;
  };
  return dv(9) === Number(digitos[9]) && dv(10) === Number(digitos[10]);
}

// Procura CPF ou RG em um texto. Telefones (DDD + 9 dígitos) e ids de vaga
// não batem porque exigimos o dígito verificador do CPF ou a palavra "RG"/"CPF".
export function contemDocumento(texto) {
  const s = String(texto ?? '');
  if (/\b\d{3}\.\d{3}\.\d{3}-\d{2}\b/.test(s)) return true;
  if (/\b(cpf|rg)\b\s*(n[ºo°.]*)?\s*[:\-]?\s*[\d.\-]{6,}/i.test(s)) return true;
  // 11 dígitos soltos: só conta se o dígito verificador de CPF bater
  // (um celular qualquer passa nessa conta em ~1% dos casos; preferimos recusar a guardar um CPF).
  for (const m of s.matchAll(/(?<!\d)\d{11}(?!\d)/g)) {
    if (cpfValido(m[0])) return true;
  }
  return false;
}

function textos(obj, acc = []) {
  if (obj == null) return acc;
  if (typeof obj === 'string') acc.push(obj);
  else if (typeof obj === 'object') for (const v of Object.values(obj)) textos(v, acc);
  return acc;
}

export function validarVaga(v) {
  const erros = [];
  if (!v || typeof v !== 'object') return ['vaga vazia'];
  for (const campo of ['id', 'empresa', 'titulo']) {
    if (!String(v[campo] ?? '').trim()) erros.push(`campo obrigatório: ${campo}`);
  }
  if (!ETAPAS.includes(v.etapa)) erros.push(`etapa desconhecida: ${v.etapa}`);
  if (PEDE_MOTIVO.includes(v.etapa) && !String(v.motivo ?? '').trim()) erros.push('informe o motivo');
  if (v.etapa === 'bloqueada' && !String(v.pendencia ?? '').trim()) erros.push('informe a pendência (o que falta você fazer)');
  if (v.envio?.data && !ehDataISO(v.envio.data)) erros.push('data de envio deve ser AAAA-MM-DD');
  if (v.followUp?.proximo && !ehDataISO(v.followUp.proximo)) erros.push('data de follow-up deve ser AAAA-MM-DD');
  if (textos(v).some(contemDocumento)) erros.push('parece conter CPF/RG — documentos não são guardados na central');
  return erros;
}
