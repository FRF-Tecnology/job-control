// Landing (site/): a página é HTML pronto e um script que só troca textos e classes. Estes testes pegam o que
// quebra sem ninguém ver: âncora para um id que não existe, texto ligado por script sem lugar na página,
// recurso de fora do site e resto do protótipo esquecido no HTML.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const pasta = join(dirname(fileURLToPath(import.meta.url)), '..', 'site');
const ler = nome => readFileSync(join(pasta, nome), 'utf8');
const html = ler('index.html');
const js = ler('app.js');
const css = ler('estilo.css');
const todos = (texto, re) => [...texto.matchAll(re)].map(m => m[1]);

test('site: toda âncora e todo aria-controls apontam para um id que existe', () => {
  const ids = new Set(todos(html, /\sid="([^"]+)"/g));
  const faltam = [...todos(html, /href="#([^"]+)"/g), ...todos(html, /aria-controls="([^"]+)"/g)].filter(id => !ids.has(id));
  assert.deepEqual([...new Set(faltam)], []);
});

test('site: ids não se repetem', () => {
  const ids = todos(html, /\sid="([^"]+)"/g).filter(id => !id.startsWith('lg')); // gradientes do SVG também têm id único
  assert.equal(new Set(ids).size, ids.length);
});

test('site: o script não tem erro de sintaxe', () => {
  assert.doesNotThrow(() => new vm.Script(js, { filename: 'app.js' }));
  assert.doesNotThrow(() => new vm.Script(ler('tema.js'), { filename: 'tema.js' }));
});

test('site: a primeira pintura é clara, exceto quando a pessoa já escolheu o tema escuro', () => {
  for (const temaSalvo of [null, 'inválido', 'claro', 'escuro']) {
    const meta = {};
    let classeEscura;
    const document = {
      documentElement: { classList: { toggle: (_, ativo) => { classeEscura = ativo; } } },
      querySelector: seletor => seletor === 'meta[name="theme-color"]' ? meta : null,
      addEventListener() {},
    };
    const window = { matchMedia: () => ({ matches: true, addEventListener() {} }), addEventListener() {} };
    vm.runInNewContext(ler('tema.js'), { window, document, localStorage: { getItem: () => temaSalvo } });
    assert.equal(classeEscura, temaSalvo === 'escuro');
    assert.equal(meta.content, temaSalvo === 'escuro' ? '#0C0E2C' : '#EEF2FB');
  }
});

test('site: todo texto que o script liga tem lugar na página', () => {
  const naPagina = new Set(todos(html, /data-liga="([^"]+)"/g));
  const fixas = todos(js, /ligar\('([^'`$]+)'/g);
  assert.ok(fixas.length >= 10, 'achou as chamadas ligar(...)');
  assert.deepEqual(fixas.filter(k => !naPagina.has(k)), []);
  // As chaves montadas no script: os motivos da triagem e os campos da IA escolhida.
  for (const k of ['tri.p0', 'tri.p1', 'tri.p2', ...['nome', 'curto', 'empresa', 'estado', 'detalhe'].map(c => `prov.${c}`)]) {
    assert.ok(naPagina.has(k), `falta data-liga="${k}"`);
  }
});

test('site: todo seletor data-* que o script procura existe na página', () => {
  const usados = new Set(todos(js, /\[(data-[a-z-]+)[\]=]/g));
  const faltam = [...usados].filter(a => !html.includes(`${a}=`) && !html.includes(`${a} `) && !html.includes(`${a}>`));
  assert.deepEqual(faltam, []);
});

test('site: nada do protótipo ficou no HTML', () => {
  for (const resto of ['{{', '<sc-if', '<sc-for', 'onClick=', '<x-dc', 'support.js', '[PREÇO]', '[TERMOS]', '[PRIVACIDADE]', '[E-MAIL DE CONTATO]']) assert.ok(!html.includes(resto), `sobrou ${resto}`);
});

test('site: os três preços aprovados informam o total e o intervalo mesmo sem JavaScript', () => {
  const precos = [...html.matchAll(/class="preco">R\$ ([\d,]+)<small>\/([^<]+)<\/small>/g)];
  assert.deepEqual(precos.map(([, total, periodo]) => [total, periodo]), [
    ['29,90', 'mês'], ['161,46', 'semestre'], ['287,04', 'ano']
  ]);
  assert.match(html, /mailto:suporte@applyer\.dev/);
});

test('site: só carrega arquivos do próprio site', () => {
  const enderecos = [...todos(html, /(?:src|href)="(https?:\/\/[^"]+)"/g), ...todos(css, /url\((https?:\/\/[^)]+)\)/g)];
  assert.deepEqual(enderecos, []);
  for (const arquivo of todos(html, /(?:src|href)="([^"#:]+\.(?:js|css|svg))"/g)) assert.ok(existsSync(join(pasta, arquivo)), `falta ${arquivo}`);
  assert.match(html, /Content-Security-Policy[^>]+script-src 'self'/);
  assert.ok(!/<script(?![^>]*\ssrc=)/.test(html), 'script inline: a CSP do site não deixa rodar');
});

test('site: os cinco sistemas aparecem no menu de baixar e nos cartões', () => {
  for (const k of ['mac', 'win', 'linux', 'ios', 'android']) {
    assert.equal(todos(html, new RegExp(`data-plat="(${k})"`, 'g')).length, 2, `menu: ${k}`);
    assert.equal(todos(html, new RegExp(`data-baixar="(${k})"`, 'g')).length, 1, `cartão: ${k}`);
    assert.match(js, new RegExp(`k: '${k}'`));
    const disponivel = k === 'mac' || k === 'win';
    const status = disponivel ? 'Em lançamento' : 'Em breve';
    const opcoes = todos(html, new RegExp(`data-plat="${k}"[^>]*>.*?<small[^>]*>([^<]+)`, 'g'));
    assert.deepEqual(opcoes, [status, status], `status nos menus: ${k}`);
    const [acao] = todos(html, new RegExp(`data-baixar="${k}"><span data-so-texto>([^<]+)`, 'g'));
    assert.match(acao, disponivel ? /^Baixar para / : /^Avisar quando sair$/);
    assert.match(html, new RegExp(`<button[^>]* disabled data-baixar="${k}"`));
  }
});

test('site: opções de plataforma e de IA usam o logo correspondente e preservam o rótulo acessível', () => {
  const opcoes = [...html.matchAll(/<button [^>]*data-(plat|prov)="([^"]+)"[^>]*>(.*?)<\/button>/g)];
  assert.equal(opcoes.length, 16); // dois menus com cinco plataformas e seis provedores
  for (const [, tipo, id, conteudo] of opcoes) {
    const marca = id === 'ios' ? 'mac' : id;
    const imagem = conteudo.match(/<img\b[^>]*>/)?.[0] ?? '';
    assert.ok(imagem.includes(`src="marcas/${marca}.svg"`), `${tipo}: ${id}`);
    assert.match(imagem, /\salt=""/);
    assert.ok(conteudo.replace(/<[^>]+>/g, '').trim(), `falta rótulo para ${id}`);
  }
});
