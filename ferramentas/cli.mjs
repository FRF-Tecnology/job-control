#!/usr/bin/env node
// CLI da Central. É por aqui que o Claude grava tudo (nunca editando os JSON à mão).
// Todo comando de dados passa pela API v1 (servidor/api.mjs), chamada no mesmo processo: não precisa da Central aberta.
// Pasta de dados: CENTRAL_DADOS (padrão: ./dados ao lado do projeto).
import { readFileSync, writeFileSync, mkdirSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { criarApi, PREFIXO } from '../servidor/api.mjs';
import { randomBytes } from 'node:crypto';
import { hojeISO, somarDias, lerValor } from '../nucleo/modelo.mjs';
import { filaAplicar } from '../nucleo/servico.mjs';
import { existsSync } from 'node:fs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const dirDados = process.env.CENTRAL_DADOS || join(raiz, 'dados');
const hoje = process.env.CENTRAL_HOJE || hojeISO();
const armazem = criarArmazem(dirDados, { hoje: () => hoje });
const api = criarApi({ armazem, dir: dirDados, hoje: () => hoje });

class ErroCli extends Error {}
// Chama a API como "claude". Erro de validação vira "recusado: ...", o resto "erro: ...".
async function chamar(metodo, caminho, corpo) {
  const r = await api.chamar({ metodo, caminho: PREFIXO + caminho, corpo, autor: 'claude' });
  if (r.status >= 400) throw new ErroCli(r.corpo.erros ? `recusado: ${r.corpo.erros.join('; ')}` : `erro: ${r.corpo.erro}`);
  return r.corpo;
}
const id_ = id => encodeURIComponent(id ?? '');
const lerJson = (texto, nome = '--json') => { try { return JSON.parse(texto); } catch { sair(`JSON inválido em ${nome}`); } };

const AJUDA = `Uso: node ferramentas/cli.mjs <comando> [opções]

  resumo                                   números do dia (JSON)
  listar [--etapa e] [--plataforma p]      uma vaga por linha
  fila-aplicar                             vagas que a pessoa aprovou (e o grupo A no modo direto): a fila do /aplicar
  ver <id>                                 vaga completa + eventos (JSON)
  dup <empresa> <título>                   avisa se a vaga já existe
  add --json '{...}' | --arquivo x.json | --uri <lista codificada>
                                           adiciona uma vaga ou uma lista
      [--forcar]                           aceita mesmo empresa+título se o id for outro (vaga republicada)
  (qualquer opção também aceita --opcao=valor, útil quando o valor começa com "--")
  mover <id> <etapa> [--motivo t] [--pendencia t] [--sub s]
        [--rodada n] [--curriculo PT|EN] [--pretensao n] [--respostas t] [--data AAAA-MM-DD]
  editar <id> --json '{campo: valor}'      campos livres (notas, aderência, link...)
  evento <id> <tipo> <texto>               linha do tempo (retorno, teste, entrevista, nota...)
  followup                                 lista os vencidos com mensagem sugerida
  followup <id> feito | adiar <dias>
  rodada --json '{"n":16,"fontes":[...],"resumo":"..."}'   registra/atualiza uma rodada
  plataforma --json '{"id":"..."}'         cria/atualiza uma plataforma
  perfil                                   mostra o perfil (JSON)
  perfil --json '{campo: valor}'           atualiza só os campos enviados (filtros mesclados;
                                           respostasPadrao mescladas pela pergunta)
  pedidos                                  pedidos abertos da caixa "O que vamos fazer hoje?"
  pedido <id> feito [resposta curta]       conclui um pedido (a resposta aparece na Central)
  email --json '{"email":{...},"classificacao":{...}}' | --arquivo lista.json
                                           registra email(s) de processo seletivo: acha a vaga e muda a etapa
                                           (recusa → encerrada, convite → entrevista, teste, oferta). Repetido não duplica.
  emails [--status sem-vaga|ligado|ignorado|descartado] [--ultimo]
                                           lista os emails registrados (--ultimo: data do mais recente)
  email-ligar <idMensagem> <vagaId>        liga um email sem vaga à vaga certa (aplica a mudança de etapa)
  email-descartar <idMensagem>             tira um email da lista "sem vaga"
  validar                                  confere todos os dados
  exportar-md [arquivo]                    histórico em Markdown (stdout ou arquivo)
  backup                                   cópia de hoje em dados/backups/
  avaliar --provedor anthropic|openai|google --modelo <id> [--funcao f1,f2] [--limite n]
                                           roda o conjunto de avaliacao/ no modelo (chave da variável de ambiente)
  avaliar --provedor assistente-local      gera os pedidos para o assistente responder
  avaliar --respostas <arquivo> --modelo <nome>
                                           pontua as respostas do assistente com a mesma guarda
  avaliar --comparar                       tabela com todas as avaliações salvas em dados/avaliacoes/
  chave                                    chave local usada pelos scripts de busca
  script <nome> [--palavras '["java"]'] [--dias 7] [--geo 106057199] [--central url]
                                           imprime automacao/<nome>.js com os marcadores preenchidos
                                           (busca-linkedin, busca-gupy, busca-inhire, inhire-IH, inhire-Q, linkedin-LI, linkedin-auto)

Saída 0 = tudo certo; 1 = o comando falhou; 2 = add/email rodou, mas algum item foi recusado (veja as linhas "erro").`;

function opcoes(args) {
  const o = { _: [] };
  for (let i = 0; i < args.length; i++) {
    const igual = args[i].match(/^--([\w-]+)=(.*)$/s);
    if (igual) o[igual[1]] = igual[2];
    else if (args[i].startsWith('--')) o[args[i].slice(2)] = args[i + 1]?.startsWith('--') || args[i + 1] === undefined ? true : args[++i];
    else o._.push(args[i]);
  }
  return o;
}

const sair = (msg, cod = 1) => { (cod ? console.error : console.log)(msg); process.exit(cod); };

async function comandos(cmd, o) {
  switch (cmd) {
    case 'resumo':
      return console.log(JSON.stringify(await chamar('GET', '/resumo'), null, 2));

    case 'listar': {
      const q = new URLSearchParams(Object.entries({ etapa: o.etapa, plataforma: o.plataforma }).filter(([, v]) => typeof v === 'string'));
      const vs = await chamar('GET', `/vagas${q.size ? `?${q}` : ''}`);
      for (const v of vs) console.log([v.id, v.etapa, v.empresa, v.titulo, v.plataforma, v.aderencia ?? '', v.link ?? ''].join(' | '));
      return console.log(`(${vs.length})`);
    }

    case 'fila-aplicar': {
      // Só o que a pessoa aprovou na Triagem (jc-a2y.28). Vaga em aplicar sem aprovação dela não entra.
      const vs = filaAplicar(await chamar('GET', '/estado'));
      for (const v of vs) console.log([v.id, v.etapa, v.empresa, v.titulo, v.plataforma, v.aderencia ?? '', v.link ?? ''].join(' | '));
      return console.log(`(${vs.length})`);
    }

    case 'ver': {
      const r = await api.chamar({ metodo: 'GET', caminho: `${PREFIXO}/vagas/${id_(o._[0])}` });
      if (r.status === 404) sair(`vaga não encontrada: ${o._[0]}`);
      return console.log(JSON.stringify(r.corpo, null, 2));
    }

    case 'dup': {
      // Lista todas as vagas com a mesma empresa+título, menos a indicada em --excluir (a própria vaga).
      const q = new URLSearchParams({ empresa: o._[0] ?? '', titulo: o._[1] ?? '', ...(typeof o.excluir === 'string' ? { excluir: o.excluir } : {}) });
      const ds = await chamar('GET', `/duplicatas?${q}`);
      if (!ds.length) return console.log('nova');
      for (const d of ds) console.log(`JÁ EXISTE: ${d.id} | ${d.etapa} | ${d.empresa} | ${d.titulo}${d.envio?.data ? ` | enviada ${d.envio.data}` : ''}`);
      return;
    }

    case 'add': {
      const bruto = o.arquivo ? readFileSync(o.arquivo, 'utf8') : o.uri ? decodeURIComponent(o.uri) : o.json;
      if (!bruto) sair('informe --json ou --arquivo');
      const lista = [].concat(JSON.parse(bruto));
      const r = await chamar('POST', '/vagas', { lista, forcar: Boolean(o.forcar) });
      console.log(`adicionadas: ${r.adicionadas.length}\nduplicadas: ${r.duplicadas.length}\ncom erro: ${r.erros.length}`);
      for (const d of r.duplicadas) console.log(`  dup  ${d.id} → ${d.existente}`);
      for (const e of r.erros) console.log(`  erro ${e.id ?? '?'}: ${e.erro}`);
      // Código 2: rodou, mas alguma vaga foi recusada. Quem chama (o Claude) precisa ver isso.
      if (r.erros.length) process.exitCode = 2;
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
          ...(o.pretensao ? { pretensao: lerValor(o.pretensao) } : {}),
          ...(o.respostas ? { respostas: o.respostas } : {}),
        };
      }
      if (Number.isNaN(extras.envio?.pretensao)) sair(`pretensão inválida: ${o.pretensao} (use 8500 ou 8.500)`);
      const vaga = await chamar('POST', `/vagas/${id_(id)}/mover`, { etapa, ...extras });
      return console.log(`ok: ${vaga.empresa} · ${vaga.titulo} → ${vaga.etapa}${vaga.followUp?.proximo ? ` (follow-up ${vaga.followUp.proximo})` : ''}`);
    }

    case 'editar': {
      const vaga = await chamar('PATCH', `/vagas/${id_(o._[0])}`, lerJson(o.json));
      return console.log(`ok: ${vaga.id}`);
    }

    case 'evento': {
      const [id, tipo, ...texto] = o._;
      await chamar('POST', `/vagas/${id_(id)}/eventos`, { tipo, texto: texto.join(' ') });
      return console.log('ok');
    }

    case 'followup': {
      const [id, acao, dias] = o._;
      if (!id) {
        for (const v of await chamar('GET', '/followups')) {
          console.log(`\n## ${v.id} | ${v.empresa} · ${v.titulo} | enviada ${v.envio.data} | vence ${v.followUp.proximo}${v.link ? `\n${v.link}` : ''}\n${v.mensagem}`);
        }
        return;
      }
      if (acao === 'feito') await chamar('POST', `/vagas/${id_(id)}/followup`, { acao: 'feito' });
      else if (acao === 'adiar') await chamar('POST', `/vagas/${id_(id)}/followup`, { acao: 'adiar', dias: Number(dias) || 3 });
      else sair('use: followup <id> feito | adiar <dias>');
      return console.log('ok');
    }

    case 'rodada': {
      const r = lerJson(o.json);
      if (r.n == null) sair('a rodada precisa de "n"');
      await chamar('PUT', `/rodadas/${id_(String(r.n))}`, r);
      return console.log(`ok: rodada ${r.n}`);
    }

    case 'plataforma': {
      const p = lerJson(o.json);
      if (!p.id) sair('a plataforma precisa de "id"');
      await chamar('PATCH', `/plataformas/${id_(p.id)}`, p);
      return console.log(`ok: ${p.id}`);
    }

    case 'perfil': {
      if (!o.json || o.json === true) return console.log(JSON.stringify(await chamar('GET', '/perfil'), null, 2));
      const p = lerJson(o.json);
      if (!p || typeof p !== 'object' || Array.isArray(p)) sair('--json deve ser um objeto');
      await chamar('PATCH', '/perfil', p);
      return console.log(`ok: perfil atualizado (${Object.keys(p).join(', ')})`);
    }

    case 'pedidos': {
      const ps = await chamar('GET', '/pedidos?status=aberto');
      for (const p of ps) console.log(`${p.id} | ${p.criadoEm} | ${p.texto}`);
      return console.log(`(${ps.length})`);
    }

    case 'pedido': {
      const [id, acao, ...resposta] = o._;
      if (acao !== 'feito') sair('use: pedido <id> feito [resposta curta]');
      await chamar('POST', `/pedidos/${id_(id)}/concluir`, { resposta: resposta.join(' ') });
      return console.log('ok');
    }

    case 'email': {
      const bruto = o.arquivo ? readFileSync(o.arquivo, 'utf8') : o.json;
      if (!bruto || bruto === true) sair('informe --json ou --arquivo');
      const dados = lerJson(bruto, o.arquivo ? o.arquivo : '--json');
      const lista = Array.isArray(dados) ? dados : [dados];
      const rs = await chamar('POST', '/emails', { lista });
      const NOMES = { ligado: 'ligado', 'sem-vaga': 'SEM VAGA', ignorado: 'ignorado', duplicado: 'já registrado', erro: 'ERRO' };
      for (const r of rs) console.log(`${NOMES[r.status] ?? r.status} | ${r.idMensagem}${r.vagaId ? ` | ${r.vagaId}` : ''}${r.transicao ? ` → ${r.transicao}` : ''}${r.erro ? ` | ${r.erro}` : ''}`);
      const conta = s => rs.filter(r => r.status === s).length;
      if (conta('erro')) process.exitCode = 2;
      return console.log(`(${rs.length}: ${conta('ligado')} ligados, ${rs.filter(r => r.transicao).length} mudaram de etapa, ${conta('sem-vaga')} sem vaga, ${conta('duplicado')} repetidos, ${conta('erro')} com erro)`);
    }

    case 'emails': {
      const todos = await chamar('GET', `/emails${typeof o.status === 'string' ? `?status=${encodeURIComponent(o.status)}` : ''}`);
      if (o.ultimo) return console.log(todos.map(e => e.data).sort().at(-1) ?? 'nenhum email registrado ainda');
      for (const e of todos) console.log([e.idMensagem, e.data, e.status, e.tipo ?? '', e.vagaId ?? '', e.de ?? '', e.assunto ?? ''].join(' | '));
      return console.log(`(${todos.length})`);
    }

    case 'email-ligar': {
      const [id, vagaId] = o._;
      const r = await chamar('POST', `/emails/${id_(id)}/ligar`, { vagaId });
      return console.log(`ok: ${r.idMensagem} → ${r.vagaId}${r.transicao ? ` (vaga foi para ${r.transicao})` : ''}`);
    }

    case 'email-descartar':
      await chamar('POST', `/emails/${id_(o._[0])}/descartar`, {});
      return console.log('ok');

    case 'validar': {
      const r = await chamar('GET', '/validar');
      if (!r.ok) sair(r.problemas.join('\n'));
      return console.log(`OK: ${r.vagas} vagas, ${r.eventos} eventos`);
    }

    case 'exportar-md': {
      const md = await chamar('GET', '/historico.md');
      if (o._[0]) { writeFileSync(o._[0], md, 'utf8'); return console.log(`ok: ${o._[0]}`); }
      return console.log(md);
    }

    case 'chave': {
      const arq = join(dirDados, '.chave');
      if (o.nova) { writeFileSync(arq, randomBytes(18).toString('hex')); return console.log('chave trocada — reinicie a Central (iniciar.bat) para valer'); }
      return console.log(existsSync(arq) ? readFileSync(arq, 'utf8').trim() : 'ainda não existe: abra a Central uma vez (iniciar.bat)');
    }

    case 'script': {
      const nome = String(o._[0] ?? '').replace(/\.js$/, '');
      const arq = join(raiz, 'automacao', `${nome}.js`);
      if (!/^[a-z-]+$/i.test(nome) || !existsSync(arq)) sair(`script não encontrado: ${nome}`);
      const perfil = await chamar('GET', '/perfil');
      const arqChave = join(dirDados, '.chave');
      const empresas = readFileSync(join(raiz, 'automacao', 'inhire-empresas.txt'), 'utf8').split(/\r?\n/).filter(l => !l.startsWith('#')).join(' ').split(/\s+/).filter(Boolean);
      const valores = {
        central: o.central || `http://localhost:${process.env.PORT || (existsSync(join(dirDados, '.porta')) ? readFileSync(join(dirDados, '.porta'), 'utf8').trim() : 4321)}`,
        chave: existsSync(arqChave) ? readFileSync(arqChave, 'utf8').trim() : '',
        palavras: o.palavras || JSON.stringify(['java', 'spring boot', 'desenvolvedor java', 'backend java', 'full stack java', 'desenvolvedor full stack', 'desenvolvedor backend', 'react node']),
        dias: String(o.dias || 7),
        geo: String(o.geo || '106057199'),
        desde: somarDias(hoje, -(Number(o.dias) || 7)),
        empresas: JSON.stringify(empresas),
        nome: perfil.nome ?? '', email: perfil.email ?? '', linkedin: perfil.linkedin ?? '',
        telefoneSoNumeros: String(perfil.telefone ?? '').replace(/\D/g, ''),
        pretensaoCLT: String(perfil.pretensaoCLT ?? ''), pretensaoPJ: String(perfil.pretensaoPJ ?? ''),
        // Início do nome do arquivo de currículo como aparece no LinkedIn (perfil.curriculosLinkedin).
        cvPT: perfil.curriculosLinkedin?.PT ?? '', cvEN: perfil.curriculosLinkedin?.EN ?? '',
        // Filtros de título da busca (regex; vazio = padrão do script) e regras de senioridade/inglês vindas do perfil.
        tituloFora: perfil.buscaTituloFora ?? '', tituloDev: perfil.buscaTituloDev ?? '',
        aceitaSenior: (perfil.filtros?.niveis ?? []).some(n => /s[eê]nior/i.test(n)) ? '1' : '',
        inglesOk: /fluente|avan[cç]ad|proficient|fluent|advanced|c1|c2/i.test(perfil.ingles ?? '') ? '1' : '',
      };
      // Valores de texto entram como literais JSON (aspas e apóstrofos não quebram o script);
      // palavras e empresas já são listas JSON.
      const literal = k => (['palavras', 'empresas'].includes(k) ? valores[k] : JSON.stringify(valores[k]));
      const js = readFileSync(arq, 'utf8').replace(/'?\{\{(\w+)\}\}'?/g, (m, k) => (k in valores ? literal(k) : m));
      return console.log(js);
    }

    case 'avaliar': {
      const { criarIA } = await import('../nucleo/ia/index.mjs');
      const av = await import('../nucleo/ia/avaliacao.mjs');
      const dirAval = join(dirDados, 'avaliacoes');
      mkdirSync(dirAval, { recursive: true });
      const conjunto = av.carregarConjunto(join(raiz, 'avaliacao'));
      const funcoes = typeof o.funcao === 'string' ? o.funcao.split(',') : [];
      const carimbo = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
      const salvar = (provedor, modelo, resultados) => {
        const exec = { provedor, modelo, data: new Date().toISOString(), casos: resultados.length, resumo: av.resumir(resultados), resultados };
        const arq = join(dirAval, `${carimbo}-${provedor}-${String(modelo).replace(/[^\w.-]/g, '_')}.json`);
        writeFileSync(arq, JSON.stringify(exec, null, 2));
        console.log(av.tabelaResumo(exec.resumo));
        const falhas = resultados.filter(r => !r.ok || r.violacoes.length);
        if (falhas.length) console.log('\nCasos com erro ou violação:');
        for (const r of falhas) console.log(`  ✗ ${r.id}: ${[r.erro, ...(r.divergencias ?? []), ...r.violacoes.map(v => `violou ${v}`)].filter(Boolean).join('; ')}`);
        return console.log(`\nsalvo em ${arq}`);
      };

      if (o.comparar) {
        const execs = readdirSync(dirAval).filter(n => /^\d{4}-.*\.json$/.test(n)).sort().map(n => JSON.parse(readFileSync(join(dirAval, n), 'utf8')));
        if (!execs.length) sair('nenhuma avaliação salva ainda: rode avaliar --provedor ...');
        return console.log(av.tabelaComparacao(execs));
      }
      if (o.respostas) {
        const resultados = av.pontuarRespostas(conjunto, JSON.parse(readFileSync(o.respostas, 'utf8')));
        if (!resultados.length) sair('nenhuma resposta bate com os ids dos casos');
        return salvar('assistente-local', typeof o.modelo === 'string' ? o.modelo : 'assistente', resultados);
      }
      if (o.provedor === 'assistente-local') {
        const pedidos = av.pedidosParaAssistente(conjunto, funcoes);
        const arq = join(dirAval, `pedidos-${carimbo}.json`);
        writeFileSync(arq, JSON.stringify(pedidos, null, 2));
        return console.log(`${pedidos.length} pedidos em ${arq}\nResponda cada um com o JSON pedido, num arquivo {"<id>": {...}}, e rode:\n  avaliar --respostas <arquivo> --modelo <nome do assistente>`);
      }
      const VARIAVEL = { anthropic: 'ANTHROPIC_API_KEY', openai: 'OPENAI_API_KEY', google: 'GEMINI_API_KEY' };
      if (!VARIAVEL[o.provedor] || typeof o.modelo !== 'string') sair('use: avaliar --provedor anthropic|openai|google --modelo <id>  (ou --provedor assistente-local, --respostas, --comparar)');
      if (!process.env[VARIAVEL[o.provedor]]) sair(`defina a variável ${VARIAVEL[o.provedor]} neste terminal antes (a chave fica só com você; a Central não grava)`);
      const usos = [];
      const ia = criarIA({ config: { provedores: { [o.provedor]: {} }, rotas: { padrao: { provedor: o.provedor, modelo: o.modelo } } }, registrarUso: u => usos.push(u) });
      const { resultados } = await av.avaliar({ ia, usos, conjunto, funcoes, limite: Number(o.limite) || 0, aoProgresso: (i, n) => process.stderr.write(`\r${i}/${n} casos`) });
      process.stderr.write('\n');
      return salvar(o.provedor, o.modelo, resultados);
    }

    case 'backup':
      return console.log((await chamar('POST', '/backup')).criado ? `ok: dados/backups/${hoje}` : 'backup de hoje já existe');

    default:
      return console.log(AJUDA);
  }
}

const [cmd, ...resto] = process.argv.slice(2);
try {
  await comandos(cmd, opcoes(resto));
} catch (e) {
  sair(e instanceof ErroCli ? e.message : `erro: ${e.message}`);
}
