// Aplica o tema salvo antes de desenhar a tela (evita piscar claro no modo escuro).
// Fica num arquivo, e não inline no index.html, porque a CSP só deixa rodar script do próprio app.
document.documentElement.dataset.tema = 'escuro';
try { if (localStorage.getItem('tema') === 'claro') document.documentElement.dataset.tema = 'claro'; } catch (e) {}
