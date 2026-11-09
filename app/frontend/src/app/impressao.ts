// Impressão da candidatura (P5, regra R3 da proposta da fila): sha256 do JSON canônico da RFC 8785 de
// {vagaId, plataforma, url, conteudo}, com o prefixo "sha256:v1:". Igual a internal/dominio/canonico.go: a tela
// recalcula a partir do pacote que mostra, para a pessoa aprovar exatamente o que vê.

// JSON canônico (RFC 8785): chaves em ordem de unidades UTF-16 (a ordem padrão do sort do JavaScript), sem espaços,
// números e strings como o JSON.stringify do ECMAScript (que é o que a RFC adota).
export function jsonCanonico(v: unknown): string {
  if (v === null || typeof v === 'boolean' || typeof v === 'string') return JSON.stringify(v);
  if (typeof v === 'number') {
    if (!Number.isFinite(v)) throw new Error('JSON canônico: número inválido');
    return JSON.stringify(v); // -0 vira "0"
  }
  if (Array.isArray(v)) return `[${v.map(jsonCanonico).join(',')}]`;
  if (typeof v === 'object') {
    const o = v as Record<string, unknown>;
    const chaves = Object.keys(o).filter(k => o[k] !== undefined).sort();
    return `{${chaves.map(k => `${JSON.stringify(k)}:${jsonCanonico(o[k])}`).join(',')}}`;
  }
  throw new Error(`JSON canônico: tipo não suportado (${typeof v})`);
}

export type PacoteCandidatura = { vagaId: string; plataforma: string; url: string; conteudo: unknown };

export async function impressaoCandidatura({ vagaId, plataforma, url, conteudo }: PacoteCandidatura): Promise<string> {
  const bytes = new TextEncoder().encode(jsonCanonico({ vagaId, plataforma, url, conteudo }));
  const resumo = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
  return `sha256:v1:${[...resumo].map(b => b.toString(16).padStart(2, '0')).join('')}`;
}
