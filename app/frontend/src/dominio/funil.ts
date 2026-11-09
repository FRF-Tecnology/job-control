// Porte de nucleo/funil.mjs (conferido contra o golden do Node em dominio.test.ts).
import { foiEnviada, somarDias } from './modelo';
import { pendentesFollowUp } from './followup';

// "Retorno" = a empresa respondeu de alguma forma (andou no processo ou reprovou).
export function teveRetorno(v) {
  return foiEnviada(v) && (v.etapa === 'em-processo' || v.etapa === 'oferta' || (v.etapa === 'encerrada' && v.motivo === 'Reprovada'));
}

function segundaFeira(iso) {
  const [a, m, d] = iso.split('-').map(Number);
  const dia = new Date(a, m - 1, d).getDay(); // 0 = domingo
  return somarDias(iso, -((dia + 6) % 7));
}

export function resumo(vagas, hoje) {
  const enviadas = vagas.filter(foiEnviada);
  const retornos = enviadas.filter(teveRetorno).length;
  const contar = etapa => vagas.filter(v => v.etapa === etapa).length;
  const inicioSemana = segundaFeira(hoje);
  return {
    total: vagas.length,
    descoberta: contar('descoberta'),
    aplicar: contar('aplicar'),
    bloqueadas: contar('bloqueada'),
    emProcesso: contar('em-processo'),
    ofertas: contar('oferta'),
    enviadas: enviadas.length,
    enviadasHoje: enviadas.filter(v => v.envio.data === hoje).length,
    enviadasSemana: enviadas.filter(v => v.envio.data >= inicioSemana).length,
    retornos,
    taxaRetorno: enviadas.length ? Math.round((retornos / enviadas.length) * 100) : 0,
    followUps: pendentesFollowUp(vagas, hoje).length,
  };
}

export function funil(vagas) {
  const env = vagas.filter(foiEnviada);
  return [
    { etapa: 'Enviadas', total: env.length },
    { etapa: 'Com retorno', total: env.filter(teveRetorno).length },
    { etapa: 'Em processo', total: env.filter(v => v.etapa === 'em-processo').length },
    { etapa: 'Oferta', total: env.filter(v => v.etapa === 'oferta').length },
  ];
}

export function porPlataforma(vagas) {
  const mapa = new Map();
  for (const v of vagas) {
    const p = v.plataforma || 'outra';
    const g = mapa.get(p) ?? { plataforma: p, achadas: 0, enviadas: 0, retornos: 0 };
    g.achadas++;
    if (foiEnviada(v)) g.enviadas++;
    if (teveRetorno(v)) g.retornos++;
    mapa.set(p, g);
  }
  return [...mapa.values()].sort((a, b) => b.enviadas - a.enviadas || b.achadas - a.achadas);
}

export function porSemana(vagas) {
  const mapa = new Map();
  for (const v of vagas.filter(foiEnviada)) {
    const s = segundaFeira(v.envio.data);
    mapa.set(s, (mapa.get(s) ?? 0) + 1);
  }
  return [...mapa.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([semana, total]) => ({ semana, total }));
}

export function motivosPulo(vagas) {
  const mapa = new Map();
  for (const v of vagas.filter(v => v.etapa === 'pulada')) {
    const m = v.motivo || 'Outro';
    mapa.set(m, (mapa.get(m) ?? 0) + 1);
  }
  return [...mapa.entries()].map(([motivo, total]) => ({ motivo, total })).sort((a, b) => b.total - a.total || a.motivo.localeCompare(b.motivo));
}
