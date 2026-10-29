import { normalizar } from './modelo.mjs';

// Palavras que variam entre plataformas e não distinguem uma vaga da outra.
const RUIDO = new Set(['pessoa', 'desenvolvedora', 'desenvolvedor', 'desenvolvedor a', 'dev', 'a', 'de', 'da', 'do', 'remoto', 'remota', 'vaga']);

const SUFIXOS = new Set(['group', 'grupo', 'global', 'software', 'tecnologia', 'technology', 'tech', 'ltda', 'sa', 's', 'brasil', 'brazil', 'inc', 'consultoria', 'solucoes', 'digital']);
const limpar = s => normalizar(s).replace(/\(a\)/g, ' ').replace(/back[\s-]+end/g, 'backend').replace(/front[\s-]+end/g, 'frontend').replace(/full[\s-]+stack/g, 'fullstack').replace(/[^a-z0-9+#]+/g, ' ').trim();

export function chaveEmpresa(empresa) {
  const partes = limpar(empresa).split(/\s+/).filter(Boolean);
  const nucleo = partes.filter((p, i) => i === 0 || !SUFIXOS.has(p));
  return nucleo.join('');
}

export function chaveVaga(empresa, titulo) {
  const palavras = limpar(titulo).split(/\s+/).filter(p => p && !RUIDO.has(p));
  return `${chaveEmpresa(empresa)}|${palavras.join(' ')}`;
}

// Todas as vagas com a mesma empresa+título (empresa desconhecida nunca casa).
export function duplicatasPorTitulo(vagas, nova, excluirId) {
  if (!chaveEmpresa(nova.empresa)) return [];
  const chave = chaveVaga(nova.empresa, nova.titulo);
  return vagas.filter(v => v.id !== excluirId && chaveVaga(v.empresa, v.titulo) === chave);
}

export function acharDuplicata(vagas, nova) {
  const porId = vagas.find(v => v.id === nova.id);
  if (porId) return porId;
  return duplicatasPorTitulo(vagas, nova)[0] ?? null;
}
