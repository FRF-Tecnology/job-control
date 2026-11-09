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

// Validador de um subconjunto de JSON Schema (type, enum, properties, required,
// additionalProperties:false, items). O mesmo esquema vai para o provedor como formato de saída.
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
