// Aplica o tema claro na primeira visita e restaura escolhas explícitas antes da pintura.
(() => {
  let escuro = false;
  try {
    escuro = localStorage.getItem('tema') === 'escuro';
  } catch { /* Sem armazenamento, a escolha vale enquanto esta página estiver aberta. */ }

  function aplicar() {
    document.documentElement.classList.toggle('escuro', escuro);
    const botao = document.querySelector('[data-tema]');
    if (botao) botao.setAttribute('aria-pressed', String(escuro));
    document.querySelector('meta[name="theme-color"]').content = escuro ? '#0C0E2C' : '#EEF2FB';
  }

  aplicar();
  document.addEventListener('DOMContentLoaded', aplicar, { once: true });
  window.addEventListener('storage', evento => {
    if (evento.key !== 'tema' && evento.key !== null) return;
    escuro = evento.newValue === 'escuro';
    aplicar();
  });
  document.addEventListener('click', evento => {
    if (!evento.target.closest('[data-tema]')) return;
    escuro = !escuro;
    aplicar();
    try { localStorage.setItem('tema', escuro ? 'escuro' : 'claro'); } catch { /* Preferência em memória. */ }
  });
})();
