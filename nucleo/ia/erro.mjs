// Erro único da camada de IA. `tipo` diz de onde veio:
// entrada | config | provedor | json | esquema | regra.
// Só erros de provedor com `reintentavel` (rede, 5xx, 429) acionam a rota reserva.
export class ErroIA extends Error {
  constructor(tipo, mensagem, extra = {}) {
    super(mensagem);
    this.name = 'ErroIA';
    this.tipo = tipo;
    Object.assign(this, extra);
  }
}

const PALAVRAS_TRATADAS = new Set(['type', 'enum', 'properties', 'required', 'additionalProperties', 'items']);
const TIPOS = new Set(['string', 'number', 'integer', 'boolean', 'object', 'array', 'null']);

// conferirEsquema: o contrato só pode pedir o que validarEsquema aplica. Outra palavra-chave (minLength, pattern,
// format, description...), additionalProperties diferente de false, ou required/additionalProperties sem properties
// seriam ignorados em silêncio, e o contrato pareceria mais forte do que é (jc-a2y.60). Mesmas regras do Go (lerEsquema).
export function conferirEsquema(esquema, caminho = '$') {
  const recusar = motivo => { throw new Error(`esquema ${caminho}: ${motivo}`); };
  if (!esquema || typeof esquema !== 'object' || Array.isArray(esquema)) recusar('não é objeto');
  for (const k of Object.keys(esquema).sort()) if (!PALAVRAS_TRATADAS.has(k)) recusar(`palavra-chave não tratada: ${k}`);
  if ('type' in esquema) {
    const tipos = [].concat(esquema.type);
    for (const t of tipos) if (typeof t !== 'string') recusar('campo type malformado');
    for (const t of tipos) if (!TIPOS.has(t)) recusar(`tipo desconhecido: ${t}`);
  }
  if ('enum' in esquema && !Array.isArray(esquema.enum)) recusar('campo enum malformado');
  const temProps = 'properties' in esquema;
  if (temProps) {
    const p = esquema.properties;
    if (!p || typeof p !== 'object' || Array.isArray(p)) recusar('campo properties malformado');
    for (const [k, v] of Object.entries(p)) conferirEsquema(v, `${caminho}.${k}`);
  }
  if ('required' in esquema) {
    const r = esquema.required;
    if (!Array.isArray(r) || r.some(k => typeof k !== 'string')) recusar('campo required malformado');
    if (!temProps) recusar('required sem properties');
    for (const k of r) if (!Object.hasOwn(esquema.properties, k)) recusar(`required com campo que não está em properties: ${k}`);
  }
  if ('additionalProperties' in esquema) {
    if (esquema.additionalProperties !== false) recusar('additionalProperties só pode ser false');
    if (!temProps) recusar('additionalProperties sem properties');
  }
  if ('items' in esquema) conferirEsquema(esquema.items, `${caminho}[]`);
}

// Validador de um subconjunto de JSON Schema (type, enum, properties, required,
// additionalProperties:false, items). O mesmo esquema vai para o provedor como formato de saída.
// O contrato é conferido antes, ao carregar (conferirEsquema em funcoes.mjs).
export function validarEsquema(valor, esquema, caminho = '$') {
  const erros = [];
  const tipos = [].concat(esquema.type ?? []);
  const tipoDe = v => v === null ? 'null' : Array.isArray(v) ? 'array' : Number.isInteger(v) ? 'integer' : typeof v;
  const t = tipoDe(valor);
  if (tipos.length && !tipos.some(x => x === t || (x === 'number' && t === 'integer'))) {
    return [`${caminho}: esperado ${tipos.join('|')}, veio ${t}`];
  }
  if (esquema.enum && !esquema.enum.includes(valor)) erros.push(`${caminho}: valor fora da lista`); // sem o valor: pode vir da vaga ou do email (jc-a2y.23)
  if (t === 'object' && esquema.properties) {
    for (const k of esquema.required ?? []) if (valor[k] === undefined) erros.push(`${caminho}.${k}: obrigatório`);
    for (const [k, v] of Object.entries(valor)) {
      if (esquema.properties[k]) erros.push(...validarEsquema(v, esquema.properties[k], `${caminho}.${k}`));
      else if (esquema.additionalProperties === false) erros.push(`${caminho}: campo não previsto`); // sem o nome: a chave vem da saída da IA (jc-a2y.23)
    }
  }
  if (t === 'array' && esquema.items) valor.forEach((v, i) => erros.push(...validarEsquema(v, esquema.items, `${caminho}[${i}]`)));
  return erros;
}
