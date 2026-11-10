// Aplica o tema antes de desenhar a página (evita piscar claro no modo escuro).
// Vale a escolha salva; sem escolha, segue o sistema.
(function () {
  var salvo = null;
  try { salvo = localStorage.getItem('tema'); } catch (e) { /* sem armazenamento: segue o sistema */ }
  var escuro = salvo ? salvo === 'escuro' : matchMedia('(prefers-color-scheme: dark)').matches;
  document.documentElement.classList.toggle('escuro', escuro);
})();
