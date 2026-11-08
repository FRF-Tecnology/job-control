// Emails de processos seletivos (T1): pré-filtro, qual vaga é, e o que muda nela.
// Puro: recebe dados, devolve decisões. Quem grava é o servico.mjs.
import { normalizar, foiEnviada } from './modelo.mjs';
import { chaveEmpresa } from './dedupe.mjs';

export const TIPOS_EMAIL = ['confirmacao', 'rejeicao', 'entrevista', 'teste', 'oferta', 'outro'];
export const NOMES_EMAIL = { confirmacao: 'Candidatura recebida', rejeicao: 'Recusa', entrevista: 'Convite para entrevista', teste: 'Teste', oferta: 'Oferta', outro: 'Email' };

// Remetentes de ATS/testes e palavras de processo seletivo. O que não passa aqui nem chega à IA.
const ATS = /greenhouse|lever\.co|ashbyhq|gupy|inhire|workable|smartrecruiters|jobs-noreply@linkedin|myworkday|workday|icims|teamtailor|recruitee|bamboohr|breezy|jobvite|taleo|successfactors|kenoby|solides|pandape|hackerrank|codility|coderbyte|testgorilla|codesignal/i;
const PALAVRAS = /\bapplication\b|\bapplying\b|candidatura|unfortunately|infelizmente|next steps?|pr[oó]ximas? etapas?|pr[oó]ximos passos|entrevista|interview|assessment|coding (test|challenge)|teste t[eé]cnico|desafio t[eé]cnico|job offer|offer letter|proposta de (trabalho|emprego)|processo seletivo/i;
export const passaPreFiltro = email => ATS.test(email?.de ?? '') || PALAVRAS.test(`${email?.assunto ?? ''}\n${email?.texto ?? ''}`);

const espacado = s => ` ${normalizar(s).replace(/[^a-z0-9]+/g, ' ').trim()} `;
const ETAPAS_COM_EMAIL = ['aplicar', 'enviada', 'bloqueada', 'em-processo', 'oferta', 'encerrada'];

// Quanto o email "fala" desta vaga: empresa no domínio, no nome do remetente, no assunto ou no texto; título desempata.
function pontos(vaga, email) {
  const chave = chaveEmpresa(vaga.empresa);
  const primeira = normalizar(vaga.empresa).replace(/[^a-z0-9 ]+/g, ' ').trim().split(/\s+/)[0] ?? '';
  // "Maria <maria@nortal.com.br>" → ['nortal', 'com', 'br']
  const rotulos = normalizar(String(email.de ?? '').match(/@([a-z0-9.-]+)/i)?.[1] ?? '').split('.').filter(Boolean);
  const texto = espacado(`${email.de ?? ''} ${email.assunto ?? ''} ${email.texto ?? ''}`);
  const colado = texto.replace(/\s+/g, '');
  const temEmpresa = (chave.length >= 4 && (colado.includes(chave) || rotulos.some(r => r.includes(chave))))
    || (primeira.length >= 3 && (texto.includes(` ${primeira} `) || rotulos.includes(primeira)));
  if (!temEmpresa) return 0;
  const palavrasTitulo = espacado(vaga.titulo).trim().split(' ').filter(p => p.length >= 4);
  return 10 + palavrasTitulo.filter(p => texto.includes(` ${p} `)).length;
}

// Vaga do email: o id da classificação (se existir), senão a única vaga com mais pontos. Empate ou nada = null.
export function acharVagaDoEmail(vagas, email, vagaId) {
  if (vagaId) {
    const v = vagas.find(x => x.id === vagaId);
    if (v) return v;
  }
  const candidatas = vagas
    .filter(v => ETAPAS_COM_EMAIL.includes(v.etapa) || foiEnviada(v))
    .map(v => [v, pontos(v, email)])
    .filter(([, p]) => p > 0)
    .sort((a, b) => b[1] - a[1]);
  if (!candidatas.length || (candidatas[1] && candidatas[1][1] === candidatas[0][1])) return null;
  return candidatas[0][0];
}

// O que muda na vaga. Nunca anda para trás e não mexe em vaga encerrada ou pulada.
export function transicaoDoEmail(tipo, vaga, email = {}) {
  if (['encerrada', 'pulada'].includes(vaga.etapa)) return null;
  const tecnica = /t[eé]cnic|technical|coding|pair|system design|live code/i.test(`${email.assunto ?? ''} ${email.resumo ?? ''}`);
  // O próximo passo do email (ex.: "Fazer a entrevista com IA no DigAÍ") vira a pendência; sem ele, um texto padrão.
  const passo = padrao => String(email.proximoPasso ?? '').trim().slice(0, 140) || padrao;
  switch (tipo) {
    case 'rejeicao': return { etapa: 'encerrada', extras: { motivo: 'Reprovada' } };
    case 'oferta': return { etapa: 'oferta', extras: { subEtapa: 'proposta' } };
    case 'entrevista': return vaga.etapa === 'oferta' ? null : { etapa: 'em-processo', extras: { subEtapa: tecnica ? 'entrevista-tecnica' : 'entrevista-rh', pendencia: passo('Agendar a entrevista') } };
    case 'teste': return vaga.etapa === 'oferta' ? null : { etapa: 'em-processo', extras: { subEtapa: 'teste', pendencia: passo('Fazer o teste') } };
    default: return null; // confirmação e outro só ficam registrados
  }
}

// Data do email como AAAA-MM-DD (aceita ISO, RFC 2822 ou vazio).
export function dataDoEmail(valor, padrao) {
  if (/^\d{4}-\d{2}-\d{2}/.test(String(valor ?? ''))) return String(valor).slice(0, 10);
  const d = valor ? new Date(valor) : null;
  if (!d || Number.isNaN(d.getTime())) return padrao;
  const p = n => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}
