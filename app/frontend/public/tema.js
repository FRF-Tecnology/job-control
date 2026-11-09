// Aplica o tema salvo antes de desenhar a tela (evita piscar claro no modo escuro).
// Fica num arquivo, e não inline no index.html, porque a CSP só deixa rodar script do próprio app.
try { var t = localStorage.getItem('tema'); if (t) document.documentElement.dataset.tema = t; } catch (e) {}
