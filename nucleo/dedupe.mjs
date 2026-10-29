import { normalizar } from './modelo.mjs';

// Palavras que variam entre plataformas e não distinguem uma vaga da outra.
const RUIDO = new Set(['pessoa', 'desenvolvedora', 'desenvolvedor', 'desenvolvedor a', 'dev', 'a', 'de', 'da', 'do', 'remoto', 'remota', 'vaga']);

export function chaveVaga(empresa, titulo) {
  const limpar = s => normalizar(s).replace(/\(a\)/g, ' ').replace(/[^a-z0-9+#]+/g, ' ').trim();
  const palavras = limpar(titulo).split(/\s+/).filter(p => p && !RUIDO.has(p));
  return `${limpar(empresa).replace(/\s+/g, '')}|${palavras.join(' ')}`;
}

export function acharDuplicata(vagas, nova) {
  const porId = vagas.find(v => v.id === nova.id);
  if (porId) return porId;
  const chave = chaveVaga(nova.empresa, nova.titulo);
  return vagas.find(v => chaveVaga(v.empresa, v.titulo) === chave) ?? null;
}
