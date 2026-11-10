// Aplica o tema antes da primeira pintura e acompanha o sistema até uma escolha manual.
(() => {
  const sistema = window.matchMedia('(prefers-color-scheme: dark)');
  const temaValido = valor => ['claro', 'escuro'].includes(valor) ? valor : null;
  let escolha = null;
  try {
    escolha = temaValido(localStorage.getItem('tema'));
  } catch { /* Sem armazenamento, a escolha vale enquanto esta página estiver aberta. */ }

  function aplicar() {
    const escuro = escolha === null ? sistema.matches : escolha === 'escuro';
    document.documentElement.classList.toggle('escuro', escuro);
    const botao = document.querySelector('[data-tema]');
    if (botao) botao.setAttribute('aria-pressed', String(escuro));
    for (const meta of document.querySelectorAll('meta[name="theme-color"]')) {
      meta.content = escuro ? '#0C0E2C' : '#EEF2FB';
    }
  }

  aplicar();
  document.addEventListener('DOMContentLoaded', aplicar, { once: true });
  sistema.addEventListener('change', aplicar);
  window.addEventListener('storage', evento => {
    if (evento.key !== 'tema' && evento.key !== null) return;
    escolha = temaValido(evento.newValue);
    aplicar();
  });
  document.addEventListener('click', evento => {
    if (!evento.target.closest('[data-tema]')) return;
    escolha = document.documentElement.classList.contains('escuro') ? 'claro' : 'escuro';
    aplicar();
    try { localStorage.setItem('tema', escolha); } catch { /* Preferência em memória. */ }
  });
})();
