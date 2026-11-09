// Grava uma sessão de chamadas à API Node (servidor/api.mjs) para os testes da API em Go (internal/api).
// A API Node é a referência: o Go, partindo do zero, precisa dar o mesmo status e a mesma resposta em cada chamada.
// Uso: node ferramentas/gerar-golden-api.mjs   (GOLDEN_SAIDA troca o arquivo de saída)
process.env.TZ = 'America/Sao_Paulo';
import { writeFileSync, mkdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarApi, PREFIXO } from '../servidor/api.mjs';
import { criarArmazemMemoria } from '../nucleo/armazenamento.mjs';

const ALEATORIO = 0.123456789;
Math.random = () => ALEATORIO;
const sufixo = ALEATORIO.toString(36).slice(2, 6);

let dia = '2026-11-07';
const armazem = criarArmazemMemoria();
const api = criarApi({ armazem, dir: null, hoje: () => dia });
const chamadas = [];

async function chamar(metodo, caminho, corpo, autor = 'eu') {
  const pedido = { metodo, caminho: caminho.startsWith('/api/') ? caminho : PREFIXO + caminho, autor, hoje: dia };
  if (corpo !== undefined) pedido.corpo = JSON.parse(JSON.stringify(corpo));
  const r = await api.chamar({ metodo, caminho: pedido.caminho, corpo: corpo === undefined ? undefined : structuredClone(pedido.corpo), autor });
  chamadas.push({ ...pedido, resposta: { status: r.status, corpo: JSON.parse(JSON.stringify(r.corpo ?? null)), tipo: r.tipo ?? null, cabecalhos: r.cabecalhos ?? null } });
  return r;
}

// ---------- conta nova ----------
await chamar('GET', '/estado');
await chamar('GET', '/versao');
await chamar('GET', '/primeiros-passos');
await chamar('GET', '/perfil');

// ---------- perfil ----------
await chamar('PUT', '/perfil', { nome: 'Ana Teste', foco: 'internacional', diasFollowUp: 10, filtros: { niveis: ['Pleno'] }, respostasPadrao: [{ pergunta: 'Anos com React?', resposta: '4' }] });
await chamar('PATCH', '/perfil', { cargo: 'Desenvolvedora front-end React', stackPrincipal: ['React', 'TypeScript'], filtros: { modelo: ['Remoto'] }, respostasPadrao: [{ pergunta: 'anos com react?', resposta: '5' }, { pergunta: 'Inglês?', resposta: 'Avançado' }] });
await chamar('PUT', '/perfil', { nome: 'X', senha: '123' });
await chamar('PATCH', '/perfil', { respostasPadrao: [{ pergunta: 'Qual seu CPF?', resposta: '529.982.247-25' }] });
await chamar('PATCH', '/perfil', { foco: 'lua', pretensaoCLT: 'muito' });
await chamar('PATCH', '/perfil', 'texto');
await chamar('GET', '/perfil');

// ---------- vagas ----------
const vaga = (id, empresa, titulo, extra = {}) => ({ id, empresa, titulo, plataforma: 'linkedin', ...extra });
await chamar('POST', '/vagas', vaga('li-1', 'Acme Remote', 'Senior React Developer (LATAM, USD)', { idioma: 'en', contrato: 'Contractor USD', aderencia: 'A', motivoClaude: 'React + TS, USD' }), 'claude');
await chamar('POST', '/vagas', vaga('li-1', 'Acme Remote', 'Senior React Developer (LATAM, USD)'), 'claude');
await chamar('POST', '/vagas', vaga('li-1b', 'Acme Remote', 'Senior React Developer (LATAM, USD)'), 'claude');
await chamar('POST', '/vagas', { ...vaga('li-1b', 'Acme Remote', 'Senior React Developer (LATAM, USD)'), forcar: true }, 'claude');
await chamar('POST', '/vagas', [
  vaga('gupy-2', 'Banco Exemplo', 'Desenvolvedora Front-end React Pleno', { plataforma: 'gupy', contrato: 'CLT', aderencia: 'A' }),
  vaga('gupy-3', 'Varejo Teste', 'Dev Front-end Sênior', { plataforma: 'gupy', contrato: 'PJ', aderencia: 'C' }),
  vaga('gupy-2', 'Banco Exemplo', 'Desenvolvedora Front-end React Pleno'),
  { id: 'ruim', empresa: '', titulo: 'x' },
  { empresa: 'Sem id', titulo: 'y' },
  vaga('li-cpf', 'Fraude', 'Dev', { notas: 'cpf 529.982.247-25' }),
], 'claude');
await chamar('POST', '/vagas', { lista: [vaga('li-4', 'Initech', 'React Native Developer', { idioma: 'en' }), vaga('li-5', 'Acme Remote', 'Senior React Developer (LATAM, USD)')], forcar: true }, 'claude');
await chamar('POST', '/vagas/importar', [
  vaga('li-6', 'Hooli', 'Senior Frontend Engineer', { etapa: 'enviada', envio: { data: '2026-11-01' }, followUp: { proximo: '2026-11-08' }, descobertaEm: '2020-01-01', motivo: 'x', pendencia: 'y', subEtapa: 'teste' }),
  'não é objeto',
  vaga('li-1', 'Acme Remote', 'Senior React Developer (LATAM, USD)'),
]);
await chamar('POST', '/vagas/importar', vaga('li-7', 'Globex', 'Frontend Engineer - Next.js'));
await chamar('POST', '/vagas', 'texto');
await chamar('POST', '/vagas', ['x'], 'claude');
await chamar('GET', '/vagas');
await chamar('GET', '/vagas?etapa=descoberta&plataforma=gupy');
await chamar('GET', '/duplicatas?empresa=Acme%20Remote&titulo=Senior%20React%20Developer%20(LATAM%2C%20USD)&excluir=li-1');
await chamar('GET', '/duplicatas?empresa=&titulo=x');
await chamar('GET', '/vagas/li-1');
await chamar('GET', '/vagas/nao-existe');
await chamar('GET', '/vagas/%E0%A4%A');
await chamar('PATCH', '/vagas/li-1', { notas: 'falar com a recrutadora', etapa: 'oferta' });
await chamar('PATCH', '/vagas/li-1', { notas: 'minha versão', notasAntes: 'outra coisa' });
await chamar('PATCH', '/vagas/li-1', { link: 'acme.com' });
await chamar('PATCH', '/vagas/nao-existe', { notas: 'x' });

// ---------- etapas, eventos e follow-up ----------
await chamar('POST', '/vagas/li-1/mover', { etapa: 'aplicar' });
await chamar('POST', '/vagas/li-1/mover', { etapa: 'enviada', envio: { rodada: 1, curriculo: 'EN', pretensao: 6000 } }, 'claude');
await chamar('POST', '/vagas/gupy-2/mover', { etapa: 'bloqueada', pendencia: 'A Gupy pede CPF: preencha você' }, 'claude');
await chamar('POST', '/vagas/gupy-3/mover', { etapa: 'pulada' });
await chamar('POST', '/vagas/gupy-3/mover', { etapa: 'pulada', motivo: 'Stack' });
await chamar('POST', '/vagas/nao-existe/mover', { etapa: 'aplicar' });
await chamar('POST', '/vagas/li-4/mover', { etapa: 'voando' });
await chamar('POST', '/vagas/li-4/mover', { etapa: 'enviada', envio: { rodada: 1 } }, 'claude');
await chamar('POST', '/vagas/li-1/eventos', { texto: 'mandei mensagem no LinkedIn' });
await chamar('POST', '/vagas/li-1/eventos', { tipo: 'entrevista', texto: 'cpf 529.982.247-25' });
await chamar('POST', '/vagas/nao-existe/eventos', { texto: 'x' });
dia = '2026-11-17';
await chamar('GET', '/followups');
await chamar('POST', '/vagas/li-1/followup', { acao: 'feito', texto: 'mandei de novo' });
await chamar('POST', '/vagas/li-4/followup', { acao: 'adiar', dias: 'muitos' });
await chamar('POST', '/vagas/li-4/followup', { acao: 'adiar', dias: 5 });
await chamar('POST', '/vagas/li-4/followup', { acao: 'esquecer' });
await chamar('GET', '/followups');
await chamar('POST', '/vagas/li-1/acoes', {});

// ---------- pedidos ----------
const p = await chamar('POST', '/pedidos', { texto: 'Buscar vagas USD LATAM' });
await chamar('POST', '/pedidos', { texto: '' });
await chamar('GET', '/pedidos?status=aberto');
await chamar('POST', `/pedidos/${p.corpo.id}/concluir`, { resposta: 'Achei 5 vagas' }, 'claude');
await chamar('POST', '/pedidos/pd-x/concluir', { resposta: 'x' }, 'claude');
await chamar('GET', '/pedidos');
await chamar('GET', '/pedidos?status=aberto');

// ---------- emails ----------
const email = (idMensagem, de, assunto, texto = '') => ({ idMensagem, de, assunto, texto, data: 'Fri, 07 Nov 2026 14:00:00 +0000' });
const item = (e, tipo, extra = {}) => ({ email: e, classificacao: { tipo, vagaId: null, resumo: `${tipo} de ${e.de}`, ...extra } });
await chamar('POST', '/emails', { lista: [
  item(email('m1', 'talent@acmeremote.example', 'Acme Remote - next steps', 'corpo que não fica guardado'), 'entrevista', { proximoPasso: 'Escolher horário' }),
  item(email('m2', 'no-reply@initech.example', 'Your application to Initech'), 'rejeicao'),
  item(email('m3', 'jobs@misterio.example', 'Desafio técnico'), 'teste'),
  item(email('m4', 'news@loja.com', 'Promoção'), 'outro'),
  item(email('m5', 'a@b.com', 'x'), 'convite'),
  item(email('m1', 'talent@acmeremote.example', 'Acme Remote - next steps'), 'entrevista'),
] }, 'claude');
await chamar('POST', '/emails', item(email('m6', 'rh@x.com', 'Documentos'), 'outro', { resumo: 'pediu o CPF 529.982.247-25' }), 'claude');
await chamar('POST', '/emails', item(email('m7', 'jobs@acme.io', 'Application received'), 'confirmacao'), 'claude');
await chamar('POST', '/emails', { email: { de: 'x' }, classificacao: { tipo: 'outro' } }, 'claude');
await chamar('GET', '/emails?status=sem-vaga');
await chamar('GET', '/emails?vaga=li-1');
await chamar('POST', '/emails/m3/ligar', { vagaId: 'li-1' });
await chamar('POST', '/emails/m3/ligar', { vagaId: 'li-4' });
await chamar('POST', '/emails/m7/descartar', {});
await chamar('POST', '/emails/nao-existe/descartar', {});
await chamar('GET', '/emails');

// ---------- plataformas e rodadas ----------
await chamar('GET', '/plataformas');
await chamar('PUT', '/plataformas/gupy', { nome: 'Gupy', status: 'Automatizada', comoAplicar: 'Script de busca' });
await chamar('PATCH', '/plataformas/gupy', { limitacoes: 'Pede CPF no formulário' });
await chamar('PUT', '/plataformas/nova%20fonte', { url: 'https://exemplo.com' });
await chamar('PUT', '/plataformas/ruim', { nome: 3 });
await chamar('PUT', '/plataformas/ruim', { nome: 'cpf 529.982.247-25' });
await chamar('GET', '/plataformas');
await chamar('PUT', '/rodadas/2', { fontes: ['gupy'], resumo: 'Segunda' }, 'claude');
await chamar('PUT', '/rodadas/1', { fontes: ['linkedin', 'gupy'], resumo: 'Primeira rodada' }, 'claude');
await chamar('PUT', '/rodadas/1', { enviadas: 2 }, 'claude');
await chamar('PUT', '/rodadas/abc', {}, 'claude');
await chamar('PUT', '/rodadas/3', { resumo: 'rg 1234567' }, 'claude');
await chamar('GET', '/rodadas');

// ---------- números, exportações e checagens ----------
await chamar('GET', '/resumo');
await chamar('GET', '/historico.md');
await chamar('GET', '/exportar.csv');
await chamar('GET', '/validar');
await chamar('GET', '/curriculos');
await chamar('GET', '/primeiros-passos');
await chamar('POST', '/backup', {});
await chamar('POST', '/backup', {});
await chamar('GET', '/nada');
await chamar('DELETE', '/vagas/li-1', {});
await chamar('GET', '/api/outra');
await chamar('GET', '/versao');
await chamar('GET', '/estado');

const destino = process.env.GOLDEN_SAIDA || join(dirname(fileURLToPath(import.meta.url)), '..', 'internal', 'api', 'testdata', 'golden-api.json');
mkdirSync(dirname(destino), { recursive: true });
writeFileSync(destino, JSON.stringify({ fuso: process.env.TZ, sufixo, chamadas }, null, 1) + '\n');
console.log(`${chamadas.length} chamadas gravadas em ${destino}`);
