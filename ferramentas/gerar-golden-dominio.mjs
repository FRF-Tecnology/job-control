// Grava as respostas do domínio Node (nucleo/) para os testes do domínio em Go (internal/dominio).
// O Node é a referência: o Go precisa dar a mesma resposta para cada caso.
// Uso: node ferramentas/gerar-golden-dominio.mjs (o fuso fica fixo em São Paulo, o mesmo que o teste Go usa).
import { writeFileSync, mkdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import * as modelo from '../nucleo/modelo.mjs';
import * as validacao from '../nucleo/validacao.mjs';
import * as dedupe from '../nucleo/dedupe.mjs';
import * as followup from '../nucleo/followup.mjs';
import * as funil from '../nucleo/funil.mjs';
import * as emails from '../nucleo/emails.mjs';
import * as servico from '../nucleo/servico.mjs';
import * as historico from '../nucleo/historico.mjs';

// As datas dependem do fuso; o Node aplica a troca na hora, e nenhum módulo do núcleo calcula data ao ser importado.
process.env.TZ = 'America/Sao_Paulo';

// Ids de eventos e pedidos levam 4 letras aleatórias. Aqui elas ficam fixas; o Go usa o mesmo sufixo.
const ALEATORIO = 0.123456789;
Math.random = () => ALEATORIO;
const sufixo = ALEATORIO.toString(36).slice(2, 6);

const F = { ...modelo, ...validacao, ...dedupe, ...followup, ...funil, ...emails, ...servico, ...historico };
const json = v => (v === undefined ? null : JSON.parse(JSON.stringify(v)));
const casos = [];

// Roda F[funcao](...args) com os argumentos já passados por JSON (o Go recebe exatamente o mesmo).
function rodar(nome, funcao, ...argsBrutos) {
  // Argumento ausente no fim fica ausente (o JS aplica o valor padrão); no meio, vira null.
  while (argsBrutos.length && argsBrutos.at(-1) === undefined) argsBrutos.pop();
  const args = argsBrutos.map(json);
  const entrada = structuredClone(args);
  try {
    const r = F[funcao](...entrada);
    casos.push({ nome, funcao, args, resultado: json(r) });
    if (JSON.stringify(entrada) !== JSON.stringify(args)) throw new Error(`${nome}: a função alterou a entrada`);
    return json(r);
  } catch (e) {
    if (/alterou a entrada/.test(e.message)) throw e;
    casos.push({ nome, funcao, args, erro: { tipo: e.name, mensagem: e.message, erros: e.erros ?? null } });
    return null;
  }
}

// ---------- dados de apoio (mesmos dos testes Node) ----------
const ctx = (agora = '2026-10-29', extra = {}) => ({ autor: 'claude', agora, diasFollowUp: 7, ...extra });
const base = (extra = {}) => ({ id: 'li-1', empresa: 'Compass UOL', titulo: 'Desenvolvedor Quarkus', plataforma: 'linkedin', etapa: 'descoberta', ...extra });
const vazio = servico.estadoVazio();
const amostra = () => [
  base({ id: 'a', etapa: 'enviada', plataforma: 'linkedin', envio: { data: '2026-10-28', rodada: 15, pretensao: 5200 }, followUp: { proximo: '2026-10-29', feitos: 0 } }),
  base({ id: 'b', empresa: 'B', etapa: 'enviada', plataforma: 'gupy', envio: { data: '2026-10-20', rodada: 14, respostas: 'Java 3 anos' }, followUp: { proximo: '2026-11-30', feitos: 0 } }),
  base({ id: 'c', empresa: 'C', etapa: 'em-processo', plataforma: 'gupy', contrato: 'CLT', envio: { data: '2026-10-21', rodada: 14 } }),
  base({ id: 'd', empresa: 'D', etapa: 'encerrada', motivo: 'Reprovada', plataforma: 'gupy', envio: { data: '2026-10-22' } }),
  base({ id: 'e', empresa: 'E', etapa: 'pulada', motivo: 'Sênior' }),
  base({ id: 'e2', empresa: 'E2', etapa: 'pulada', motivo: 'Stack' }),
  base({ id: 'e3', empresa: 'E3', etapa: 'pulada', motivo: 'Stack' }),
  base({ id: 'e4', empresa: 'E4', etapa: 'pulada', motivo: 'Inglês' }),
  base({ id: 'e5', empresa: 'E5', etapa: 'pulada', motivo: 'Sênior' }),
  base({ id: 'e6', empresa: 'E6', etapa: 'pulada', motivo: '' }),
  base({ id: 'f', empresa: 'F', etapa: 'bloqueada', pendencia: 'CPF', link: 'https://x.com/vaga' }),
  base({ id: 'g', empresa: 'G', etapa: 'descoberta', plataforma: '' }),
  base({ id: 'h', empresa: 'H', etapa: 'oferta', plataforma: 'inhire', envio: { data: '2026-10-27', rodada: '15' } }),
];

// ---------- modelo ----------
for (const s of ['  São PAULO  ', 'Ação ÚNICA', '', null, 42, 'ﬁ ligadura', ' nbsp ', 'Ñandú']) rodar(`normalizar ${JSON.stringify(s)}`, 'normalizar', s);
for (const [d, n] of [['2026-10-29', 7], ['2026-12-28', 7], ['2028-02-27', 2], ['2026-03-01', -1], ['2026-10-29', 0], ['2026-01-31', 30]]) rodar(`somarDias ${d} ${n}`, 'somarDias', d, n);
for (const v of ['2026-10-29', '2026-1-29', '2026-10-29T10:00', 20261029, null]) rodar(`ehDataISO ${JSON.stringify(v)}`, 'ehDataISO', v);
for (const v of [
  { mercado: 'internacional' }, { mercado: 'xx', contrato: 'CLT' }, { contrato: 'Contractor USD' }, { idioma: 'en' },
  { idioma: 'en', notas: 'paga em reais' }, { motivoClaude: 'vaga nacional remota' }, { notas: 'pagamento em dólar' }, {}, null,
]) {
  rodar(`mercadoDe ${JSON.stringify(v)}`, 'mercadoDe', v);
  for (const foco of ['internacional', 'nacional', 'ambos', '', null]) rodar(`noFoco ${JSON.stringify(v)} ${foco}`, 'noFoco', v, foco);
}
for (const v of [{ envio: { data: '2026-10-01' } }, { envio: {} }, {}, null]) rodar(`foiEnviada ${JSON.stringify(v)}`, 'foiEnviada', v);
for (const s of ['8.500', '8.500,50', 'R$ 8.500', '8500.5', '8500', '8500,5', '1.234.567', 'abc', '', null, '12 000', '0,5']) rodar(`lerValor ${JSON.stringify(s)}`, 'lerValor', s);

// ---------- validação ----------
for (const s of ['meu cpf 123.456.789-09', '52998224725', '84996617957', '4484521064', 'RG: 1234567', 'cpf: 529.982.247-25 ok',
  'rg 12', 'tel 11999998888', 'id 529982247251', 'a52998224725b', '', null, 123, 'CPF\n123456789']) rodar(`contemDocumento ${JSON.stringify(s)}`, 'contemDocumento', s);
for (const [nome, v] of [
  ['válida', base()], ['pulada sem motivo', base({ etapa: 'pulada' })], ['bloqueada sem pendência', base({ etapa: 'bloqueada' })],
  ['etapa desconhecida', base({ etapa: 'xyz' })], ['nota com CPF', base({ notas: 'cpf 123.456.789-09' })], ['sem campos', {}],
  ['não é objeto', 'texto'], ['nula', null], ['texto numérico aceito', base({ nivel: 3 })], ['texto como objeto', base({ notas: { a: 1 } })],
  ['stack inválida', base({ stack: ['Java', 3] })], ['stack ok', base({ stack: ['Java'] })], ['subetapa errada', base({ subEtapa: 'x' })],
  ['mercado errado', base({ mercado: 'lua' })], ['envio data errada', base({ envio: { data: '29/10/2026' } })],
  ['followUp data errada', base({ followUp: { proximo: 'amanhã' } })], ['link sem http', base({ link: 'www.x.com' })],
  ['link ok', base({ link: 'HTTPS://x.com' })], ['empresa só espaços', base({ empresa: '   ' })], ['id numérico', base({ id: 7 })],
  ['CPF dentro de envio', base({ envio: { respostas: 'cpf 529.982.247-25' } })],
]) rodar(`validarVaga ${nome}`, 'validarVaga', v);
for (const [nome, p] of [
  ['vazio', {}], ['não objeto', []], ['tipos errados', { nome: 3, email: {}, stackPrincipal: 'Java', filtros: [], foco: 'lua', pretensaoCLT: 'abc' }],
  ['filtros listas', { filtros: { niveis: 'Pleno', modelo: ['Remoto'], pularSe: [1] } }], ['respostas ruins', { respostasPadrao: [{ pergunta: 1 }] }],
  ['números como texto', { pretensaoCLT: '9000', pretensaoPJ: '', pretensaoUSD: '5k', experienciaAnos: 4, diasFollowUp: true }],
  ['ok', { nome: 'Ana', foco: 'ambos', respostasPadrao: [{ pergunta: 'Anos?', resposta: '4' }], filtros: { niveis: ['Pleno'] } }],
]) rodar(`validarPerfil ${nome}`, 'validarPerfil', p);
for (const [nome, p] of [
  ['limpo', { nome: 'Ana', respostasPadrao: [{ pergunta: 'Qual seu CPF?', resposta: 'Não informo' }] }],
  ['resposta de CPF', { respostasPadrao: [{ pergunta: 'Qual seu CPF?', resposta: '123' }] }], ['campo senha', { linkedin: { senha: 'x' } }],
  ['CPF num texto', { notas: '529.982.247-25' }], ['nao sem acento', { respostasPadrao: [{ pergunta: 'Senha do portal', resposta: 'nao tenho' }] }],
  ['resposta vazia', { respostasPadrao: [{ pergunta: 'RG', resposta: '  ' }] }],
]) rodar(`perfilSensivel ${nome}`, 'perfilSensivel', p);
for (const o of [{ a: ['x', { b: 'cpf 529.982.247-25' }] }, { a: 'ok' }, null]) rodar(`contemDocumentoEm ${JSON.stringify(o)}`, 'contemDocumentoEm', o);

// ---------- dedupe ----------
for (const e of ['V.tal', 'Zup Innovation', 'Compass UOL Group', 'Tech Global', 'S.A.', '', 'Ação Digital Ltda', 'BairesDev']) rodar(`chaveEmpresa ${e}`, 'chaveEmpresa', e);
for (const [e, t] of [['V.tal', 'Pessoa Desenvolvedora Full Stack - Pleno'], ['V.tal', 'Desenvolvedor Full Stack Pleno'], ['Compass', 'Desenvolvedor(a) Back-End Java'],
  ['X', 'Dev Front End React Remoto'], ['Y', 'C++ / C# Developer']]) rodar(`chaveVaga ${e} ${t}`, 'chaveVaga', e, t);
rodar('duplicatas: mesma empresa+título', 'duplicatasPorTitulo', [base(), base({ id: 'li-2' }), base({ id: 'x', empresa: 'Outra' })], base({ id: 'novo' }));
rodar('duplicatas: excluir a própria', 'duplicatasPorTitulo', [base(), base({ id: 'li-2' })], base(), 'li-1');
rodar('duplicatas: empresa vazia nunca casa', 'duplicatasPorTitulo', [base({ empresa: '' })], base({ empresa: '' }));
rodar('acharDuplicata por id', 'acharDuplicata', [base()], base({ titulo: 'outra' }));
rodar('acharDuplicata por empresa+título', 'acharDuplicata', [base()], base({ id: 'gupy-9', titulo: 'Desenvolvedor(a) Quarkus' }));
rodar('acharDuplicata nenhuma', 'acharDuplicata', [base()], base({ id: 'gupy-9', empresa: 'Outra' }));

// ---------- follow-up ----------
rodar('pendentesFollowUp', 'pendentesFollowUp', amostra(), '2026-10-29');
rodar('pendentesFollowUp todos vencidos em ordem', 'pendentesFollowUp', amostra(), '2026-12-31');
const perfis = [{}, { nome: 'Aldenor' }, { nome: 'Ana', cargo: 'Desenvolvedora front-end React', stackPrincipal: ['React', 'TypeScript'] },
  { nome: 'Bia', stackPrincipal: ['Go', 'Postgres', 'AWS', 'Kafka'] }, { cargo: 'Éngenheira de dados' }, { stackPrincipal: ['', 'Go'] }];
for (const p of perfis) for (const idioma of [undefined, 'en']) rodar(`sugerirMensagem ${JSON.stringify(p)} ${idioma ?? 'pt'}`, 'sugerirMensagem', base({ idioma }), p);

// ---------- funil ----------
for (const v of amostra()) rodar(`teveRetorno ${v.id}`, 'teveRetorno', v);
rodar('resumo amostra', 'resumo', amostra(), '2026-10-29');
rodar('resumo domingo', 'resumo', amostra(), '2026-11-01');
rodar('resumo vazio', 'resumo', [], '2026-10-29');
rodar('funil', 'funil', amostra());
rodar('porPlataforma', 'porPlataforma', amostra());
rodar('porSemana', 'porSemana', amostra());
rodar('motivosPulo', 'motivosPulo', amostra());
rodar('motivosPulo acentos', 'motivosPulo', [base({ etapa: 'pulada', motivo: 'Stack' }), base({ id: '2', etapa: 'pulada', motivo: 'Sênior' }), base({ id: '3', etapa: 'pulada', motivo: 'Diploma' })]);

// ---------- emails ----------
const enviada = (id, empresa, titulo, extra = {}) => ({ id, empresa, titulo, etapa: 'enviada', plataforma: 'linkedin', descobertaEm: '2026-11-01', atualizadoEm: '2026-11-01', envio: { data: '2026-11-01' }, followUp: { proximo: '2026-11-08', feitos: 0 }, ...extra });
const VAGAS = [
  enviada('li-1', 'Oscilar', 'Senior Software Engineer'),
  enviada('li-2', 'Sezzle', 'Backend Engineer'),
  enviada('li-3', 'Zup Innovation', 'Pessoa Desenvolvedora Backend Java Sênior'),
  enviada('li-4', 'BairesDev', 'Senior Java Developer'),
  enviada('li-5', 'BairesDev', 'Senior AI Engineer'),
  { id: 'li-6', empresa: 'Nortal', titulo: 'Java Developer', etapa: 'pulada', motivo: 'Stack', plataforma: 'linkedin' },
  { id: 'li-7', empresa: 'CI&T', titulo: 'Desenvolvedor Java AWS', etapa: 'aplicar', plataforma: 'linkedin' },
];
const email = (idMensagem, de, assunto, texto = '') => ({ idMensagem, de, assunto, texto, data: 'Fri, 07 Nov 2026 14:00:00 +0000' });
for (const e of [email('1', 'no-reply@greenhouse.io', 'Hello'), email('2', 'maria@empresa.com', 'Próximas etapas do processo seletivo'),
  email('3', 'rh@x.com', 'Update', 'Unfortunately we decided to move forward with other candidates'), email('4', 'news@loja.com', 'Promoção de sexta'),
  email('5', 'x@y.com', 'PROXIMOS PASSOS'), email('6', 'x@y.com', 'Desafio Técnico'), { de: null }]) rodar(`passaPreFiltro ${e.assunto}`, 'passaPreFiltro', e);
for (const [nome, e, vagaId] of [
  ['assunto', email('a', 'no-reply@greenhouse.io', 'Thank you for applying to Oscilar')],
  ['domínio', email('b', 'Talent <talent@sezzle.com>', 'Your application')],
  ['texto', email('c', 'recrutamento@zup.com.br', 'Atualização', 'Obrigado pelo interesse na Zup')],
  ['título desempata', email('d', 'jobs@bairesdev.com', 'Senior AI Engineer — next steps')],
  ['empate', email('e', 'jobs@bairesdev.com', 'Your application')],
  ['vaga pulada', email('f', 'jobs@nortal.com', 'Interview')],
  ['id da classificação', email('g', 'x@y.com', 'oi'), 'li-4'],
  ['id inexistente cai na busca', email('g2', 'no-reply@greenhouse.io', 'Oscilar'), 'li-99'],
  ['nenhuma', email('h', 'jobs@acme.io', 'Application received')],
  ['empresa com &', email('i', 'rh@ciandt.com', 'CI&T: próximas etapas')],
]) rodar(`acharVagaDoEmail ${nome}`, 'acharVagaDoEmail', VAGAS, e, vagaId);
const v0 = VAGAS[0];
for (const [tipo, vaga, e] of [['confirmacao', v0], ['outro', v0], ['rejeicao', v0], ['entrevista', v0, { assunto: 'Interview invitation' }],
  ['entrevista', v0, { assunto: 'Technical interview' }], ['entrevista', v0, { resumo: 'live code amanhã' }], ['teste', v0],
  ['teste', v0, { proximoPasso: '  Fazer a entrevista com IA no DigAÍ  ' }], ['oferta', v0], ['entrevista', { ...v0, etapa: 'oferta' }],
  ['rejeicao', { ...v0, etapa: 'encerrada' }], ['oferta', { ...v0, etapa: 'pulada' }], ['teste', v0, { proximoPasso: 'x'.repeat(200) }]])
  rodar(`transicaoDoEmail ${tipo} ${vaga.etapa} ${JSON.stringify(e ?? {}).slice(0, 40)}`, 'transicaoDoEmail', tipo, vaga, e);
for (const [v, p] of [['Fri, 07 Nov 2026 14:00:00 +0000', 'x'], ['Fri, 07 Nov 2026 01:30:00 +0000', 'x'], ['2026-11-05T10:00:00Z', 'x'],
  [null, '2026-11-07'], ['lixo', '2026-11-07'], ['Thu, 6 Nov 2026 23:59:00 -0300', 'x'], ['Fri, 07 Nov 2026 14:00:00 GMT', 'x']]) rodar(`dataDoEmail ${v}`, 'dataDoEmail', v, p);

// ---------- serviço (cenários de testes/nucleo.test.mjs e emails.test.mjs) ----------
rodar('estadoVazio', 'estadoVazio');
const e1 = rodar('adicionar grava a vaga e um evento de descoberta', 'adicionarVaga', vazio, base(), ctx()).estado;
rodar('adicionar duplicada lança ErroDuplicata', 'adicionarVaga', e1, base({ id: 'x-2' }), ctx());
rodar('adicionar inválida lança ErroValidacao', 'adicionarVaga', vazio, base({ empresa: '' }), ctx());
rodar('adicionar não-objeto', 'adicionarVaga', vazio, ['x'], ctx());
rodar('adicionar ignora contaId do corpo e guarda campos extras', 'adicionarVaga', vazio, base({ contaId: 'outra', resumoDescricao: 'texto', stack: ['Java'] }), ctx('2026-10-29', { contaId: 'conta-1' }));
rodar('forçar aceita vaga republicada', 'adicionarVaga', e1, base({ id: 'inhire-novo' }), ctx(), { forcar: true });
rodar('forçar no próprio item', 'adicionarVaga', e1, base({ id: 'inhire-novo', forcar: true }), ctx());
rodar('forçar nunca aceita o mesmo id', 'adicionarVaga', e1, base(), ctx(), { forcar: true });
rodar('evento inicial usa a data do envio', 'adicionarVaga', vazio, base({ etapa: 'enviada', descobertaEm: '2026-10-20', envio: { data: '2026-10-26' } }), ctx('2026-10-29', { autor: 'eu' }));
// jc-a2y.28: aprovar (aplicar) e adicionar fora de descoberta são só da pessoa (autor eu).
rodar('assistente não adiciona em aplicar', 'adicionarVaga', vazio, base({ etapa: 'aplicar' }), ctx());
rodar('assistente não adiciona em enviada', 'adicionarVaga', vazio, base({ etapa: 'enviada', envio: { data: '2026-10-26' } }), ctx());
rodar('assistente adiciona em descoberta explícita', 'adicionarVaga', vazio, base({ etapa: 'descoberta' }), ctx());
rodar('pessoa adiciona direto em aplicar', 'adicionarVaga', vazio, base({ etapa: 'aplicar' }), ctx('2026-10-29', { autor: 'eu' }));
rodar('plataforma nula sobrescreve o padrão', 'adicionarVaga', vazio, base({ plataforma: null }), ctx());
const e2 = rodar('mover para enviada grava envio e follow-up', 'moverVaga', e1, 'li-1', 'enviada', { envio: { rodada: 15, curriculo: 'PT', pretensao: 5200 } }, ctx()).estado;
rodar('mover para pulada sem motivo', 'moverVaga', e1, 'li-1', 'pulada', {}, ctx());
rodar('mover para pulada com motivo', 'moverVaga', e1, 'li-1', 'pulada', { motivo: 'Stack' }, ctx());
rodar('mover vaga inexistente', 'moverVaga', vazio, 'nao-existe', 'aplicar', {}, ctx('2026-10-29', { autor: 'eu' }));
rodar('assistente não aprova (aplicar)', 'moverVaga', e1, 'li-1', 'aplicar', {}, ctx());
rodar('assistente não aprova nem vaga inexistente', 'moverVaga', vazio, 'nao-existe', 'aplicar', {}, ctx());
const eAprov = rodar('pessoa aprova (aplicar)', 'moverVaga', e1, 'li-1', 'aplicar', {}, ctx('2026-10-29', { autor: 'eu' })).estado;
rodar('fila do aplicar: aprovada pela pessoa', 'filaAplicar', eAprov);
rodar('fila do aplicar: vazia sem aprovação', 'filaAplicar', e1);
rodar('fila do aplicar: evento de aplicar do assistente não conta', 'filaAplicar', { ...eAprov, eventos: eAprov.eventos.map(ev => (ev.tipo === 'aplicar' ? { ...ev, autor: 'claude' } : ev)) });
rodar('fila do aplicar: modo direto inclui grupo A em descoberta', 'filaAplicar', { ...e1, perfil: { modo: 'direto' }, vagas: e1.vagas.map(v => ({ ...v, aderencia: 'A' })) });
rodar('fila do aplicar: modo revisar não inclui descoberta', 'filaAplicar', { ...e1, perfil: { modo: 'revisar' }, vagas: e1.vagas.map(v => ({ ...v, aderencia: 'A' })) });
rodar('mover etapa desconhecida', 'moverVaga', e1, 'li-1', 'voando', {}, ctx());
const e3 = rodar('bloquear com pendência', 'moverVaga', e1, 'li-1', 'bloqueada', { pendencia: 'digitar CPF' }, ctx()).estado;
rodar('sair de bloqueada limpa a pendência', 'moverVaga', e3, 'li-1', 'enviada', {}, ctx());
rodar('enviada de novo não refaz o envio', 'moverVaga', e2, 'li-1', 'enviada', { envio: { pretensao: 6000 } }, ctx('2026-10-30'));
rodar('em processo sem envio conta como candidatura', 'moverVaga', e1, 'li-1', 'em-processo', { subEtapa: 'entrevista-rh' }, ctx());
rodar('oferta mantém subetapa', 'moverVaga', e2, 'li-1', 'oferta', { subEtapa: 'proposta' }, ctx());
rodar('motivo nulo explícito', 'moverVaga', e1, 'li-1', 'aplicar', { motivo: null }, ctx());
rodar('texto com CPF no mover', 'moverVaga', e1, 'li-1', 'aplicar', { texto: 'cpf 529.982.247-25' }, ctx());
rodar('envio com data própria', 'moverVaga', e1, 'li-1', 'enviada', { envio: { data: '2026-10-25' } }, ctx());
rodar('envio com data inválida', 'moverVaga', e1, 'li-1', 'enviada', { envio: { data: '25/10' } }, ctx());
rodar('encerrar com motivo vira texto do evento', 'moverVaga', e2, 'li-1', 'encerrada', { motivo: 'Reprovada' }, ctx('2026-11-02'));
rodar('dias de follow-up do perfil', 'moverVaga', e1, 'li-1', 'enviada', {}, ctx('2026-10-29', { diasFollowUp: 10 }));
const e4 = rodar('follow-up feito conta e reagenda', 'followUpFeito', e2, 'li-1', ctx('2026-11-05')).estado;
rodar('follow-up feito com texto', 'followUpFeito', e2, 'li-1', ctx('2026-11-05', { texto: 'mandei no LinkedIn' }));
rodar('follow-up com CPF', 'followUpFeito', e2, 'li-1', ctx('2026-11-05', { texto: 'cpf 529.982.247-25' }));
rodar('adiar empurra a data', 'adiarFollowUp', e4, 'li-1', 3, ctx('2026-11-12'));
rodar('adiar vaga sem follow-up', 'adiarFollowUp', e1, 'li-1', 2, ctx());
rodar('atualizar só aceita campos editáveis', 'atualizarVaga', e1, 'li-1', { notas: 'falar com recrutador', etapa: 'oferta', contaId: 'x' }, ctx());
rodar('atualizar notas com conflito', 'atualizarVaga', rodar('notas', 'atualizarVaga', e1, 'li-1', { notas: 'do Claude' }, ctx()).estado, 'li-1', { notas: 'minha', notasAntes: '' }, ctx());
rodar('atualizar notas sem conflito', 'atualizarVaga', e1, 'li-1', { notas: 'minha', notasAntes: null }, ctx());
rodar('atualizar inválido', 'atualizarVaga', e1, 'li-1', { link: 'x.com' }, ctx());
rodar('atualizar mercado', 'atualizarVaga', e1, 'li-1', { mercado: 'internacional' }, ctx());
rodar('evento de nota', 'registrarEvento', e1, 'li-1', 'nota', 'ligar amanhã', ctx());
rodar('evento de nota com CPF', 'registrarEvento', e1, 'li-1', 'nota', 'cpf 123.456.789-09', ctx());
rodar('evento sem tipo', 'registrarEvento', e1, 'li-1', '  ', 'x', ctx());
rodar('evento com texto numérico', 'registrarEvento', e1, 'li-1', 'nota', 5, ctx());
rodar('evento sem texto', 'registrarEvento', e1, 'li-1', 'nota', null, ctx());
const p1 = rodar('criar pedido', 'criarPedido', e1, '  Buscar vagas USD LATAM  ', ctx('2026-11-08', { autor: 'eu' })).estado;
rodar('pedido vazio', 'criarPedido', e1, '   ', ctx());
rodar('pedido longo', 'criarPedido', e1, 'x'.repeat(501), ctx());
rodar('pedido com CPF', 'criarPedido', e1, 'meu cpf 529.982.247-25', ctx());
rodar('concluir pedido', 'concluirPedido', p1, p1.pedidos[0].id, 'Achei 5 vagas', ctx('2026-11-09'));
rodar('concluir pedido sem resposta', 'concluirPedido', p1, p1.pedidos[0].id, null, ctx('2026-11-09'));
rodar('concluir pedido inexistente', 'concluirPedido', p1, 'pd-x', 'ok', ctx());
rodar('concluir pedido resposta não texto', 'concluirPedido', p1, p1.pedidos[0].id, 3, ctx());

const eV = { ...servico.estadoVazio(), vagas: VAGAS };
const ctxE = ctx('2026-11-07');
const item = (e, tipo, extra = {}) => ({ email: e, classificacao: { tipo, vagaId: null, resumo: `${tipo} de ${e.de}`, ...extra } });
let eE = eV;
for (const [nome, it] of [
  ['confirmação', item(email('m1', 'no-reply@greenhouse.io', 'Thank you for applying to Oscilar', 'corpo longo'), 'confirmacao')],
  ['recusa', item(email('m2', 'talent@sezzle.com', 'Your application to Sezzle'), 'rejeicao')],
  ['entrevista técnica', item(email('m3', 'recrutamento@zup.com.br', 'Convite para entrevista técnica na Zup'), 'entrevista')],
  ['teste', item(email('m4', 'no-reply@hackerrank.com', 'BairesDev invited you to a test for Senior AI Engineer'), 'teste', { proximoPasso: 'Fazer o teste até sexta' })],
  ['oferta', item(email('m5', 'people@bairesdev.com', 'Offer letter – Senior Java Developer'), 'oferta')],
  ['repetido', item(email('m2', 'talent@sezzle.com', 'Your application to Sezzle'), 'rejeicao')],
  ['sem vaga', item(email('m6', 'jobs@acme.io', 'Interview invitation'), 'entrevista')],
  ['newsletter ignorada', item(email('m7', 'news@loja.com', 'Promoção'), 'outro')],
  ['outro com cara de processo', item(email('m8', 'rh@y.com', 'Processo seletivo'), 'outro')],
  ['data em ISO', item({ ...email('m9', 'x@oscilar.com', 'Oscilar update'), data: '2026-11-03T12:00:00Z' }, 'confirmacao')],
]) {
  const r = rodar(`registrarEmail ${nome}`, 'registrarEmail', eE, it, ctxE);
  if (r) eE = r.estado;
}
rodar('registrarEmail sem idMensagem', 'registrarEmail', eV, { email: { de: 'x' }, classificacao: { tipo: 'outro' } }, ctxE);
rodar('registrarEmail tipo inválido', 'registrarEmail', eV, item(email('z1', 'a@b.com', 'x'), 'convite'), ctxE);
rodar('registrarEmail CPF no resumo', 'registrarEmail', eV, item(email('z2', 'rh@x.com', 'Documentos'), 'outro', { resumo: 'pediu o CPF 529.982.247-25' }), ctxE);
rodar('registrarEmail campos longos', 'registrarEmail', eV, item(email('z3', 'a'.repeat(201), 'b'.repeat(301)), 'outro', { resumo: 3 }), ctxE);
rodar('registrarEmail email vazio', 'registrarEmail', eV, { email: null, classificacao: { tipo: 'outro' } }, ctxE);
rodar('registrarEmail vagaId da classificação', 'registrarEmail', eV, item(email('z4', 'x@y.com', 'oi'), 'entrevista', { vagaId: 'li-7' }), ctxE);
rodar('registrarEmail vaga já na etapa', 'registrarEmail', eV, item(email('z5', 'x@oscilar.com', 'Oscilar'), 'entrevista', { vagaId: 'li-1' }), ctxE);
const eL = rodar('ligar email sem vaga', 'ligarEmail', eE, 'm6', 'li-4', ctxE);
rodar('ligar email já ligado', 'ligarEmail', eL.estado, 'm6', 'li-5', ctxE);
rodar('ligar email inexistente', 'ligarEmail', eE, 'nao-existe', 'li-4', ctxE);
rodar('ligar a vaga inexistente', 'ligarEmail', eE, 'm6', 'li-99', ctxE);
rodar('descartar email', 'descartarEmail', eE, 'm6');
rodar('descartar email inexistente', 'descartarEmail', eE, 'nao-existe');

// ---------- histórico ----------
rodar('histórico amostra', 'gerarHistoricoMd', { ...vazio, vagas: amostra(), rodadas: [{ n: 15, data: '2026-10-28', resumo: 'Rodada boa' }, { n: 14 }] }, '2026-10-29');
rodar('histórico vazio', 'gerarHistoricoMd', vazio, '2026-10-29');

// GOLDEN_SAIDA troca o arquivo de saída (o teste testes/golden.test.mjs usa para conferir se o golden está em dia).
const destino = process.env.GOLDEN_SAIDA || join(dirname(fileURLToPath(import.meta.url)), '..', 'internal', 'dominio', 'testdata', 'golden.json');
mkdirSync(dirname(destino), { recursive: true });
writeFileSync(destino, JSON.stringify({ fuso: process.env.TZ, sufixo, casos }, null, 1) + '\n');
console.log(`${casos.length} casos gravados em ${destino} (sufixo aleatório: ${sufixo})`);
