#!/usr/bin/env node
// CLI da Central. É por aqui que o Claude grava tudo (nunca editando os JSON à mão).
// Pasta de dados: CENTRAL_DADOS (padrão: ./dados ao lado do projeto).
import { readFileSync, writeFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { adicionarVaga, moverVaga, registrarEvento, followUpFeito, adiarFollowUp, atualizarVaga, ErroDuplicata, ErroValidacao } from '../nucleo/servico.mjs';
import { resumo } from '../nucleo/funil.mjs';
import { acharDuplicata } from '../nucleo/dedupe.mjs';
import { validarVaga } from '../nucleo/validacao.mjs';
import { pendentesFollowUp, sugerirMensagem } from '../nucleo/followup.mjs';
import { gerarHistoricoMd } from '../nucleo/historico.mjs';
import { hojeISO, somarDias } from '../nucleo/modelo.mjs';
import { existsSync } from 'node:fs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const dirDados = process.env.CENTRAL_DADOS || join(raiz, 'dados');
const armazem = criarArmazem(dirDados);
const hoje = process.env.CENTRAL_HOJE || hojeISO();

const AJUDA = `Uso: node ferramentas/cli.mjs <comando> [opções]

  resumo                                   números do dia (JSON)
  listar [--etapa e] [--plataforma p]      uma vaga por linha
  ver <id>                                 vaga completa + eventos (JSON)
  dup <empresa> <título>                   avisa se a vaga já existe
  add --json '{...}' | --arquivo x.json | --uri <lista codificada>
                                           adiciona uma vaga ou uma lista
      [--forcar]                           aceita mesmo empresa+título se o id for outro (vaga republicada)
  mover <id> <etapa> [--motivo t] [--pendencia t] [--sub s]
        [--rodada n] [--curriculo PT|EN] [--pretensao n] [--respostas t] [--data AAAA-MM-DD]
  editar <id> --json '{campo: valor}'      campos livres (notas, aderência, link...)
  evento <id> <tipo> <texto>               linha do tempo (retorno, teste, entrevista, nota...)
  followup                                 lista os vencidos com mensagem sugerida
  followup <id> feito | adiar <dias>
  rodada --json '{"n":16,"fontes":[...],"resumo":"..."}'   registra/atualiza uma rodada
  plataforma --json '{"id":"..."}'         cria/atualiza uma plataforma
  validar                                  confere todos os dados
  exportar-md [arquivo]                    histórico em Markdown (stdout ou arquivo)
  backup                                   cópia de hoje em dados/backups/
  chave                                    chave local usada pelos scripts de busca
  script <nome> [--palavras '["java"]'] [--dias 7] [--geo 106057199] [--central url]
                                           imprime automacao/<nome>.js com os marcadores preenchidos
                                           (busca-linkedin, busca-gupy, busca-inhire, inhire-IH, inhire-Q, linkedin-LI)`;

function opcoes(args) {
  const o = { _: [] };
  for (let i = 0; i < args.length; i++) {
    if (args[i].startsWith('--')) o[args[i].slice(2)] = args[i + 1]?.startsWith('--') || args[i + 1] === undefined ? true : args[++i];
    else o._.push(args[i]);
  }
  return o;
}

const ctx = () => ({ autor: 'claude', agora: hoje, diasFollowUp: Number(armazem.carregar().perfil?.diasFollowUp) || 7 });
const sair = (msg, cod = 1) => { (cod ? console.error : console.log)(msg); process.exit(cod); };

function comandos(cmd, o) {
  switch (cmd) {
    case 'resumo':
      return console.log(JSON.stringify(resumo(armazem.carregar().vagas, hoje), null, 2));

    case 'listar': {
      const vs = armazem.carregar().vagas.filter(v => (!o.etapa || v.etapa === o.etapa) && (!o.plataforma || v.plataforma === o.plataforma));
      for (const v of vs) console.log([v.id, v.etapa, v.empresa, v.titulo, v.plataforma, v.aderencia ?? '', v.link ?? ''].join(' | '));
      return console.log(`(${vs.length})`);
    }

    case 'ver': {
      const e = armazem.carregar();
      const v = e.vagas.find(x => x.id === o._[0]);
      if (!v) sair(`vaga não encontrada: ${o._[0]}`);
      return console.log(JSON.stringify({ ...v, eventos: e.eventos.filter(x => x.vagaId === v.id) }, null, 2));
    }

    case 'dup': {
      const d = acharDuplicata(armazem.carregar().vagas, { id: '\0', empresa: o._[0], titulo: o._[1] });
      return console.log(d ? `JÁ EXISTE: ${d.id} | ${d.etapa} | ${d.empresa} | ${d.titulo}` : 'nova');
    }

    case 'add': {
      const bruto = o.arquivo ? readFileSync(o.arquivo, 'utf8') : o.uri ? decodeURIComponent(o.uri) : o.json;
      if (!bruto) sair('informe --json ou --arquivo');
      const lista = [].concat(JSON.parse(bruto));
      const r = { adicionadas: [], duplicadas: [], erros: [] };
      armazem.transacao(estado => {
        for (const dados of lista) {
          try {
            ({ estado } = adicionarVaga(estado, dados, ctx(), { forcar: Boolean(o.forcar) }));
            r.adicionadas.push(dados.id);
          } catch (e) {
            if (e instanceof ErroDuplicata) r.duplicadas.push(`${dados.id} → ${e.existente.id}`);
            else r.erros.push(`${dados.id ?? '?'}: ${e.message}`);
          }
        }
        return { estado };
      });
      console.log(`adicionadas: ${r.adicionadas.length}\nduplicadas: ${r.duplicadas.length}\ncom erro: ${r.erros.length}`);
      for (const d of r.duplicadas) console.log(`  dup  ${d}`);
      for (const e of r.erros) console.log(`  erro ${e}`);
      return;
    }

    case 'mover': {
      const [id, etapa] = o._;
      const extras = {};
      if (o.motivo) extras.motivo = o.motivo;
      if (o.pendencia) extras.pendencia = o.pendencia;
      if (o.sub) extras.subEtapa = o.sub;
      if (o.texto) extras.texto = o.texto;
      if (etapa === 'enviada' && (o.rodada || o.curriculo || o.pretensao || o.respostas || o.data)) {
        extras.envio = {
          ...(o.data ? { data: o.data } : {}),
          ...(o.rodada ? { rodada: Number(o.rodada) } : {}),
          ...(o.curriculo ? { curriculo: o.curriculo } : {}),
          ...(o.pretensao ? { pretensao: Number(o.pretensao) } : {}),
          ...(o.respostas ? { respostas: o.respostas } : {}),
        };
      }
      const { vaga } = armazem.transacao(e => moverVaga(e, id, etapa, extras, ctx()));
      return console.log(`ok: ${vaga.empresa} · ${vaga.titulo} → ${vaga.etapa}${vaga.followUp?.proximo ? ` (follow-up ${vaga.followUp.proximo})` : ''}`);
    }

    case 'editar': {
      const { vaga } = armazem.transacao(e => atualizarVaga(e, o._[0], JSON.parse(o.json), ctx()));
      return console.log(`ok: ${vaga.id}`);
    }

    case 'evento': {
      const [id, tipo, ...texto] = o._;
      armazem.transacao(e => registrarEvento(e, id, tipo, texto.join(' '), ctx()));
      return console.log('ok');
    }

    case 'followup': {
      const [id, acao, dias] = o._;
      if (!id) {
        const e = armazem.carregar();
        for (const v of pendentesFollowUp(e.vagas, hoje)) {
          console.log(`\n## ${v.id} | ${v.empresa} · ${v.titulo} | enviada ${v.envio.data} | vence ${v.followUp.proximo}${v.link ? `\n${v.link}` : ''}\n${sugerirMensagem(v, e.perfil)}`);
        }
        return;
      }
      if (acao === 'feito') armazem.transacao(e => followUpFeito(e, id, ctx()));
      else if (acao === 'adiar') armazem.transacao(e => adiarFollowUp(e, id, Number(dias) || 3, ctx()));
      else sair('use: followup <id> feito | adiar <dias>');
      return console.log('ok');
    }

    case 'rodada': {
      const r = JSON.parse(o.json);
      if (r.n == null) sair('a rodada precisa de "n"');
      armazem.transacao(e => {
        const rodadas = e.rodadas.filter(x => x.n !== r.n);
        rodadas.push({ data: hoje, ...e.rodadas.find(x => x.n === r.n), ...r });
        rodadas.sort((a, b) => a.n - b.n);
        return { estado: { ...e, rodadas } };
      });
      return console.log(`ok: rodada ${r.n}`);
    }

    case 'plataforma': {
      const p = JSON.parse(o.json);
      if (!p.id) sair('a plataforma precisa de "id"');
      armazem.transacao(e => {
        const atual = e.plataformas.find(x => x.id === p.id) ?? {};
        return { estado: { ...e, plataformas: [...e.plataformas.filter(x => x.id !== p.id), { ...atual, ...p }] } };
      });
      return console.log(`ok: ${p.id}`);
    }

    case 'validar': {
      const e = armazem.carregar();
      const problemas = [];
      const ids = new Set();
      for (const v of e.vagas) {
        if (ids.has(v.id)) problemas.push(`${v.id}: id repetido`);
        ids.add(v.id);
        for (const erro of validarVaga(v)) problemas.push(`${v.id}: ${erro}`);
      }
      if (problemas.length) sair(problemas.join('\n'));
      return console.log(`OK: ${e.vagas.length} vagas, ${e.eventos.length} eventos`);
    }

    case 'exportar-md': {
      const md = gerarHistoricoMd(armazem.carregar(), hoje);
      if (o._[0]) { writeFileSync(o._[0], md, 'utf8'); return console.log(`ok: ${o._[0]}`); }
      return console.log(md);
    }

    case 'chave': {
      const arq = join(dirDados, '.chave');
      return console.log(existsSync(arq) ? readFileSync(arq, 'utf8').trim() : 'ainda não existe: abra a Central uma vez (iniciar.bat)');
    }

    case 'script': {
      const nome = String(o._[0] ?? '').replace(/\.js$/, '');
      const arq = join(raiz, 'automacao', `${nome}.js`);
      if (!/^[a-z-]+$/i.test(nome) || !existsSync(arq)) sair(`script não encontrado: ${nome}`);
      const perfil = armazem.carregar().perfil ?? {};
      const arqChave = join(dirDados, '.chave');
      const empresas = readFileSync(join(raiz, 'automacao', 'inhire-empresas.txt'), 'utf8').split(/\r?\n/).filter(l => !l.startsWith('#')).join(' ').split(/\s+/).filter(Boolean);
      const valores = {
        central: o.central || `http://localhost:${process.env.PORT || 4321}`,
        chave: existsSync(arqChave) ? readFileSync(arqChave, 'utf8').trim() : '',
        palavras: o.palavras || JSON.stringify(['java', 'spring boot', 'desenvolvedor java', 'backend java', 'full stack java', 'desenvolvedor full stack', 'desenvolvedor backend', 'react node']),
        dias: String(o.dias || 7),
        geo: String(o.geo || '106057199'),
        desde: somarDias(hoje, -(Number(o.dias) || 7)),
        empresas: JSON.stringify(empresas),
        nome: perfil.nome ?? '', email: perfil.email ?? '', linkedin: perfil.linkedin ?? '',
        telefoneSoNumeros: String(perfil.telefone ?? '').replace(/\D/g, ''),
        pretensaoCLT: String(perfil.pretensaoCLT ?? ''), pretensaoPJ: String(perfil.pretensaoPJ ?? ''),
      };
      const js = readFileSync(arq, 'utf8').replace(/\{\{(\w+)\}\}/g, (m, k) => (k in valores ? valores[k] : m));
      return console.log(js);
    }

    case 'backup':
      return console.log(armazem.backupDiario(hoje) ? `ok: dados/backups/${hoje}` : 'backup de hoje já existe');

    default:
      return console.log(AJUDA);
  }
}

const [cmd, ...resto] = process.argv.slice(2);
try {
  comandos(cmd, opcoes(resto));
} catch (e) {
  if (e instanceof ErroValidacao) sair(`recusado: ${e.erros.join('; ')}`);
  sair(`erro: ${e.message}`);
}
