// Casca da interface: estado, rotas, atualização automática, avisos e diálogos.
import { h } from './util.mjs';
import { resumo } from '../nucleo/funil.mjs';
import { telaHoje } from './telas/hoje.mjs';
import { telaQuadro } from './telas/quadro.mjs';
import { telaTriagem } from './telas/triagem.mjs';
import { telaVagas } from './telas/vagas.mjs';
import { telaPlataformas } from './telas/plataformas.mjs';
import { telaRodadas } from './telas/rodadas.mjs';
import { telaNumeros } from './telas/numeros.mjs';
import { telaPerfil } from './telas/perfil.mjs';
import { abrirDetalhe, fecharDetalhe, detalheAberto } from './telas/detalhe.mjs';

const TELAS = [
  { rota: 'hoje', nome: 'Hoje', tela: telaHoje, contador: r => r.bloqueadas + r.followUps, alerta: true },
  { rota: 'triagem', nome: 'Triagem', tela: telaTriagem, contador: r => r.descoberta },
  { rota: 'quadro', nome: 'Quadro', tela: telaQuadro },
  { rota: 'vagas', nome: 'Vagas', tela: telaVagas, contador: r => r.total },
  { rota: 'numeros', nome: 'Números', tela: telaNumeros },
  { rota: 'plataformas', nome: 'Plataformas', tela: telaPlataformas },
  { rota: 'rodadas', nome: 'Rodadas', tela: telaRodadas },
  { rota: 'perfil', nome: 'Perfil e regras', tela: telaPerfil },
];

const app = {
  estado: null,
  versao: null,
  get hoje() { return this.estado?.hoje; },

  async api(caminho, metodo = 'GET', corpo) {
    const r = await fetch(caminho, { method: metodo, headers: { 'content-type': 'application/json' }, body: corpo ? JSON.stringify(corpo) : undefined });
    const dados = r.headers.get('content-type')?.includes('json') ? await r.json() : await r.text();
    if (!r.ok) {
      const msg = dados?.erros?.join('; ') || dados?.erro || `erro ${r.status}`;
      const err = new Error(msg);
      err.dados = dados;
      err.status = r.status;
      throw err;
    }
    return dados;
  },

  // Executa uma alteração, avisa e recarrega. Devolve true se deu certo.
  async agir(fn, mensagem) {
    try {
      await fn();
      if (mensagem) aviso(mensagem);
      await carregar(true);
      return true;
    } catch (e) {
      aviso(e.message, true);
      return false;
    }
  },

  mover(id, etapa, extras = {}, mensagem) {
    return this.agir(() => this.api(`/api/v1/vagas/${encodeURIComponent(id)}/mover`, 'POST', { etapa, ...extras }), mensagem);
  },

  abrirVaga(id) { location.hash = `#/${rotaAtual().rota}/${encodeURIComponent(id)}`; },
  navegar(rota) { location.hash = `#/${rota}`; },
  aviso: (...a) => aviso(...a),
  perguntar: (...a) => perguntar(...a),
  renderizar: () => renderizar(),
  recarregar: () => carregar(true),
};

// ---------- avisos ----------
function aviso(texto, erro = false) {
  const el = h(`div.aviso${erro ? '.erro' : ''}`, { role: erro ? 'alert' : 'status' }, texto);
  document.getElementById('avisos').append(el);
  setTimeout(() => el.remove(), erro ? 7000 : 3200);
}

// ---------- diálogo genérico ----------
// perguntar({ titulo, texto, opcoes: ['A','B'], campo: 'placeholder', valor, confirmar })
// → Promise<string|null>
function perguntar({ titulo, texto, opcoes = [], campo, valor = '', confirmar = 'Confirmar', multilinha = false }) {
  const dlg = document.getElementById('dialogo');
  return new Promise(resolver => {
    let escolhido = null;
    const entrada = campo ? h(multilinha ? 'textarea' : 'input', { type: 'text', placeholder: campo, value: valor }) : null;
    if (entrada && multilinha) entrada.value = valor;
    const fechar = v => { dlg.close(); resolver(v); };
    const form = h('form', { method: 'dialog', on: { submit: ev => { ev.preventDefault(); const v = entrada ? entrada.value.trim() : (escolhido ?? 'ok'); if (!v) return entrada?.focus(); fechar(v); } } },
      h('h2', titulo),
      texto && h('p.mudo', { style: { margin: 0 } }, texto),
      opcoes.length > 0 && h('div.motivos', opcoes.map((o, i) => h('button.botao-fantasma', {
        type: 'button', on: { click: () => { if (entrada) { entrada.value = o; entrada.focus(); } else fechar(o); } },
      }, i < 9 ? h('kbd', String(i + 1)) : null, o))),
      entrada,
      h('div.acoes', { style: { justifyContent: 'flex-end' } },
        h('button.botao-fantasma', { type: 'button', on: { click: () => fechar(null) } }, 'Cancelar'),
        (entrada || !opcoes.length) && h('button.botao', { type: 'submit' }, confirmar)),
    );
    dlg.replaceChildren(form);
    dlg.onkeydown = ev => {
      if (ev.key === 'Escape') { ev.preventDefault(); fechar(null); }
      const n = Number(ev.key);
      if (!entrada && n >= 1 && n <= opcoes.length && !ev.target.matches('input,textarea')) fechar(opcoes[n - 1]);
    };
    dlg.showModal();
    (entrada ?? form.querySelector('button'))?.focus();
  });
}

// ---------- rotas ----------
function rotaAtual() {
  const [, rota = 'hoje', vaga] = location.hash.split('/');
  return { ...(TELAS.find(t => t.rota === rota) ?? TELAS[0]), vaga: vaga ? decodeURIComponent(vaga) : null };
}

function montarMenu(r) {
  const atual = rotaAtual().rota;
  document.getElementById('menu').replaceChildren(...TELAS.map(t => {
    const n = t.contador?.(r);
    return h('li', h('a', { href: `#/${t.rota}`, 'aria-current': t.rota === atual ? 'page' : null },
      h('span', t.nome), n ? h(`span.contador${t.alerta ? '.alerta' : ''}`, String(n)) : null));
  }));
}

let ultimaRota = null;
function renderizar() {
  if (!app.estado) return;
  const r = resumo(app.estado.vagas, app.hoje);
  const { rota, tela, vaga } = rotaAtual();
  montarMenu(r);
  const main = document.getElementById('conteudo');
  const rolagem = rota === ultimaRota ? window.scrollY : 0;
  const foco = document.activeElement?.id;
  main.replaceChildren(tela(app));
  if (foco && document.getElementById(foco)) document.getElementById(foco).focus();
  if (rota !== ultimaRota) { main.focus({ preventScroll: true }); document.title = `${TELAS.find(t => t.rota === rota).nome} · Central de Candidaturas`; }
  window.scrollTo(0, rolagem);
  ultimaRota = rota;
  if (vaga && app.estado.vagas.some(v => v.id === vaga)) abrirDetalhe(app, vaga);
  else if (detalheAberto()) fecharDetalhe();
}

// ---------- dados ----------
// Há texto digitado e ainda não salvo na tela? (perfil, nota nova, notas da vaga…)
// Enquanto houver, a atualização automática não redesenha nada.
app.sujo = false;
document.addEventListener('input', ev => { if (ev.target.matches('input:not([type=search]), textarea, form select')) app.sujo = true; });

async function carregar(forcar = false) {
  const conexao = document.getElementById('conexao');
  let estado;
  try {
    if (!forcar && app.estado) {
      const { versao } = await app.api('/api/v1/versao');
      if (versao === app.versao) { conexao.textContent = 'Conectado'; conexao.classList.remove('off'); return; }
    }
    if (app.sujo && !forcar) { conexao.textContent = 'Há mudanças novas — salve o que está editando para ver'; return; }
    estado = await app.api('/api/v1/estado');
  } catch (e) {
    conexao.classList.add('off');
    if (e.status) {
      // O servidor respondeu, mas com erro (ex.: arquivo de dados corrompido).
      conexao.textContent = 'Erro nos dados';
      mostrarFalha('A Central encontrou um problema nos dados', e.message);
    } else {
      conexao.textContent = 'Desconectado — rode iniciar.bat';
      if (!app.estado) mostrarFalha('A Central não está rodando', 'Abra o arquivo iniciar.bat (Windows) ou iniciar.sh na pasta do projeto e recarregue esta página.');
    }
    return;
  }
  app.estado = estado;
  conexao.textContent = 'Conectado';
  conexao.classList.remove('off');
  try {
    renderizar();
    app.versao = estado.versao;
    app.sujo = false;
  } catch (e) {
    console.error(e);
    mostrarFalha('Esta tela não conseguiu ser desenhada', `${e.message}. Os dados estão salvos; tente outra seção ou peça ao Claude para conferir.`);
  }
}

function mostrarFalha(titulo, texto) {
  document.getElementById('conteudo').replaceChildren(h('div.convite', h('div', h('strong', titulo), h('p', texto))));
}

// ---------- início ----------
document.getElementById('alternar-tema').addEventListener('click', () => {
  const raiz = document.documentElement;
  const escuroAgora = raiz.dataset.tema ? raiz.dataset.tema === 'escuro' : matchMedia('(prefers-color-scheme: dark)').matches;
  raiz.dataset.tema = escuroAgora ? 'claro' : 'escuro';
  try { localStorage.setItem('tema', raiz.dataset.tema); } catch {}
});
document.getElementById('veu').addEventListener('click', () => { location.hash = `#/${rotaAtual().rota}`; });
document.addEventListener('keydown', ev => {
  if (ev.key === 'Escape' && detalheAberto() && !document.getElementById('dialogo').open) location.hash = `#/${rotaAtual().rota}`;
});
window.addEventListener('hashchange', () => { app.sujo = false; carregar(true); });
window.addEventListener('focus', () => carregar());
setInterval(() => { if (document.visibilityState === 'visible') carregar(); }, 5000);
carregar(true);
