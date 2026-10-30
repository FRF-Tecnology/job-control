import { ETAPAS, PEDE_MOTIVO, SUB_ETAPAS, ehDataISO } from './modelo.mjs';

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
  if (/\b(cpf|rg)\b[^\d\n]{0,15}\d[\d.\-\s]{5,}/i.test(s)) return true;
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

const TEXTOS_VAGA = ['id', 'empresa', 'titulo', 'link', 'plataforma', 'nivel', 'contrato', 'modelo', 'ingles', 'idioma', 'aderencia', 'motivoClaude', 'notas', 'motivo', 'pendencia', 'local'];

export function validarVaga(v) {
  const erros = [];
  if (!v || typeof v !== 'object') return ['vaga vazia'];
  for (const campo of ['id', 'empresa', 'titulo']) {
    if (!String(v[campo] ?? '').trim()) erros.push(`campo obrigatório: ${campo}`);
  }
  if (!ETAPAS.includes(v.etapa)) erros.push(`etapa desconhecida: ${v.etapa}`);
  if (PEDE_MOTIVO.includes(v.etapa) && !String(v.motivo ?? '').trim()) erros.push('informe o motivo');
  if (v.etapa === 'bloqueada' && !String(v.pendencia ?? '').trim()) erros.push('informe a pendência (o que falta você fazer)');
  for (const campo of TEXTOS_VAGA) {
    if (v[campo] != null && typeof v[campo] !== 'string' && typeof v[campo] !== 'number') erros.push(`${campo} deve ser texto`);
  }
  if (v.stack != null && !(Array.isArray(v.stack) && v.stack.every(x => typeof x === 'string'))) erros.push('stack deve ser uma lista de textos');
  if (v.subEtapa != null && !SUB_ETAPAS.includes(v.subEtapa)) erros.push(`subetapa desconhecida: ${v.subEtapa}`);
  if (v.envio?.data && !ehDataISO(v.envio.data)) erros.push('data de envio deve ser AAAA-MM-DD');
  if (v.followUp?.proximo && !ehDataISO(v.followUp.proximo)) erros.push('data de follow-up deve ser AAAA-MM-DD');
  if (v.link && !/^https?:\/\//i.test(String(v.link))) erros.push('link deve começar com http:// ou https://');
  if (textos(v).some(contemDocumento)) erros.push('parece conter CPF/RG — documentos não são guardados na central');
  return erros;
}

// Formato do perfil: a tela de Perfil e o Claude dependem destes tipos.
const TEXTOS_PERFIL = ['nome', 'email', 'telefone', 'cidade', 'linkedin', 'github', 'cargo', 'ingles', 'formacao', 'inicio', 'salarioAtual', 'modo'];
const LISTAS_PERFIL = ['stackPrincipal', 'stackSecundaria'];
const LISTAS_FILTRO = ['niveis', 'modelo', 'pularSe'];
export function validarPerfil(p) {
  if (!p || typeof p !== 'object' || Array.isArray(p)) return ['perfil deve ser um objeto'];
  const erros = [];
  const ehLista = x => Array.isArray(x) && x.every(i => typeof i === 'string');
  for (const k of TEXTOS_PERFIL) if (p[k] != null && !['string', 'number'].includes(typeof p[k])) erros.push(`${k} deve ser texto`);
  for (const k of LISTAS_PERFIL) if (p[k] != null && !ehLista(p[k])) erros.push(`${k} deve ser uma lista de textos`);
  if (p.filtros != null) {
    if (typeof p.filtros !== 'object' || Array.isArray(p.filtros)) erros.push('filtros deve ser um objeto');
    else for (const k of LISTAS_FILTRO) if (p.filtros[k] != null && !ehLista(p.filtros[k])) erros.push(`filtros.${k} deve ser uma lista de textos`);
  }
  if (p.respostasPadrao != null && !(Array.isArray(p.respostasPadrao) && p.respostasPadrao.every(r => r && typeof r === 'object' && ['string', 'undefined'].includes(typeof r.pergunta) && ['string', 'undefined'].includes(typeof r.resposta)))) {
    erros.push('respostasPadrao deve ser uma lista de {pergunta, resposta}');
  }
  for (const k of ['pretensaoCLT', 'pretensaoPJ', 'experienciaAnos', 'diasFollowUp']) {
    if (p[k] != null && p[k] !== '' && !Number.isFinite(Number(p[k]))) erros.push(`${k} deve ser número`);
  }
  return erros;
}

// Perfil: além de CPF/RG nos textos, recusa respostas padrão sobre documentos/senha e campos de senha.
const PALAVRAS_PROIBIDAS = /\b(cpf|rg|senha|password|passwd)\b/i;
export function perfilSensivel(perfil) {
  const chaves = [];
  const varrer = o => {
    if (o && typeof o === 'object') for (const [k, v] of Object.entries(o)) { chaves.push(k); varrer(v); }
  };
  varrer(perfil);
  if (chaves.some(k => PALAVRAS_PROIBIDAS.test(k))) return true;
  if ((perfil?.respostasPadrao ?? []).some(r => PALAVRAS_PROIBIDAS.test(String(r?.pergunta ?? '')) && String(r?.resposta ?? '').trim() && !/^n[aã]o\b/i.test(String(r.resposta).trim()))) return true;
  return textos(perfil).some(contemDocumento);
}

// Qualquer objeto livre (plataforma, rodada, evento): só checa CPF/RG nos textos.
export function contemDocumentoEm(obj) {
  return textos(obj).some(contemDocumento);
}
