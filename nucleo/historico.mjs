// Gera o histórico em Markdown a partir dos dados (substitui a edição manual).
import { foiEnviada, NOMES_ETAPA } from './modelo.mjs';
import { resumo } from './funil.mjs';

const fmt = iso => (iso ? iso.split('-').reverse().join('/') : '');

export function gerarHistoricoMd(estado, hoje) {
  const { vagas, rodadas = [] } = estado;
  const r = resumo(vagas, hoje);
  const linhas = [
    '# Histórico de candidaturas',
    '',
    `> Gerado pela Central em ${fmt(hoje)}. Não edite à mão: use a Central ou \`node ferramentas/cli.mjs\`.`,
    `> Enviadas: **${r.enviadas}** · com retorno: ${r.retornos} (${r.taxaRetorno}%) · precisa de você: ${r.bloqueadas} · esperando triagem: ${r.descoberta}`,
    '',
  ];
  const porRodada = new Map();
  for (const v of vagas.filter(foiEnviada)) {
    const k = v.envio.rodada ?? 'sem rodada';
    if (!porRodada.has(k)) porRodada.set(k, []);
    porRodada.get(k).push(v);
  }
  const chaves = [...porRodada.keys()].sort((a, b) => (Number(b) || -1) - (Number(a) || -1));
  for (const k of chaves) {
    const info = rodadas.find(x => String(x.n) === String(k));
    const lista = porRodada.get(k).sort((a, b) => a.envio.data.localeCompare(b.envio.data));
    linhas.push(`## Rodada ${k}${info?.data ? ` (${fmt(info.data)})` : ''} – ${lista.length} enviadas`);
    if (info?.resumo) linhas.push(`_${info.resumo}_`);
    for (const v of lista) {
      const extra = [v.plataforma, v.contrato, v.envio.pretensao ? `pretensão ${v.envio.pretensao}` : '', v.etapa !== 'enviada' ? NOMES_ETAPA[v.etapa] + (v.motivo ? `: ${v.motivo}` : '') : '']
        .filter(Boolean).join(' · ');
      linhas.push(`- **${v.empresa}** – ${v.titulo} (${fmt(v.envio.data)}; ${extra})${v.envio.respostas ? ` — ${v.envio.respostas}` : ''}`);
    }
    linhas.push('');
  }
  const bloq = vagas.filter(v => v.etapa === 'bloqueada');
  if (bloq.length) {
    linhas.push('## Precisa de você');
    for (const v of bloq) linhas.push(`- **${v.empresa}** – ${v.titulo}: ${v.pendencia}${v.link ? ` — ${v.link}` : ''}`);
    linhas.push('');
  }
  const pul = vagas.filter(v => v.etapa === 'pulada');
  if (pul.length) {
    linhas.push('## Puladas');
    for (const v of pul) linhas.push(`- ${v.empresa} – ${v.titulo}: ${v.motivo}`);
    linhas.push('');
  }
  return linhas.join('\n');
}
