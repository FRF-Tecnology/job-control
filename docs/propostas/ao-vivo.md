# "Ao vivo": ver o envio dentro do app

Bead `jc-a2y.6.9` (P1). Status: **no código** (contrato 0.8.0, aprovado pelo CyberSec no #103). Os 8 requisitos dele (msg 1dda89e8) são critério de aceite. A tela é do Jarvis.

**Onde está no código:**
- `internal/aovivo`: o último quadro e a etapa, só em memória. Desliga sozinho sem leitura por 10 s, e a etapa final some em 30 s.
- `internal/api/aovivo.go`: as rotas. Ler e ligar exigem autor `eu` **e** o pedido marcado pelo Handler como da janela (`Sec-Fetch-Site: same-origin` e Host `wails.localhost` ou `localhost`, sem porta). Desligar: qualquer autor. Pausar, abrir o login, sair das plataformas e apagar os dados apagam o quadro.
- `internal/navegador/aovivo.go`: o screencast, só nas abas do `NovaPagina` (a janela de login não passa por ali).
- `internal/executor`: `Opcoes.Etapa` com os passos (abrindo, preenchendo, revisando, enviando e o desfecho).
- `internal/envio/envio_dev.go` e `app/main.go`: ligam tudo só no build `envio_dev`. No padrão e no `central-dev`, a rota dá 404.

O pedido do Vini: deixar evidente, dentro do app, que a candidatura está sendo aplicada. Em vez de uma janela separada, a Fila mostra um painel "Ao vivo" com a imagem do navegador do envio e a etapa. O painel é só para ver: não clica, não digita e não controla nada.

## Como funciona

1. A pessoa abre o painel. A tela chama `POST /envio/ao-vivo {ligado: true}`, e o log registra "ao vivo: ligado (eu)".
2. Enquanto houver candidatura em `enviando`, o navegador do envio, sem janela, manda quadros pelo DevTools (CDP `Page.startScreencast`, só pelo pipe que já existe, sem porta). Os quadros têm:
   - formato JPEG, qualidade 40;
   - no máximo 1280×800;
   - no máximo 1 quadro por segundo (`everyNthFrame` e descarte no app).
   O app guarda **só o último quadro, em memória**.
3. A tela lê `GET /envio/ao-vivo` a cada segundo e mostra `quadro.imagem` (`data:image/jpeg;base64,…`) num `<img src>`, sem `innerHTML`, e a `etapa`.
4. O painel fecha de três jeitos:
   - a tela chama `POST /envio/ao-vivo {ligado: false}`;
   - a tela some sem avisar: sem leitura por 10 s, o app desliga sozinho;
   - o app fecha.
   Ao fechar, o app chama `stopScreencast` e apaga o quadro, e o log registra "ao vivo: desligado (eu)".

## Os 8 requisitos do CyberSec

1. **Só pelo Handler da janela do Wails.** Nada de porta nova nem WebSocket; o DevTools continua só pelo pipe. A rota não existe no build padrão nem no `central-dev` (404): só o app com o envio passa o gancho (`Opcoes.AoVivo`), como já é com `Opcoes.Janela`.
2. **`x-so-tela`:**
   - **Para ler:** o Handler exige `Sec-Fetch-Site: same-origin` e o `Host` da janela do app (`wails.localhost` no Windows, `wails://localhost` no Mac). Sem os dois, 403.
   - **Para ligar:** só a pessoa. Qualquer autor desliga.
   - **Onde o quadro não entra:** o assistente, a CLI, o MCP, as funções de IA, o backup e o log nunca recebem o quadro, porque ele não fica no estado nem no banco.
3. **Só a aba do envio do executor.** A janela de login nunca entra. Com o login aberto, a etapa é `login-aberto` e não há quadro. Com o envio pausado, `pausado` e sem quadro.
4. **Só o último quadro, em memória.** Ele é apagado:
   - ao terminar a candidatura, depois de 30 s com a etapa final;
   - ao pausar;
   - ao sair das plataformas;
   - ao apagar os dados;
   - ao desligar o ao vivo;
   - ao fechar o app.

   Ele nunca vai para disco, temp, log, banco ou backup. A resposta leva `Cache-Control: no-store`, como todas as respostas da API.
5. **Screencast só com o painel aberto e com envio em andamento.** Qualidade e frequência ficam baixas, e o tamanho tem limite. Não há rota de clique nem de tecla: só visualização.
6. **Na tela:**
   - um `<img>` com `data:` (a CSP passa a ter `img-src 'self' data:`, e só isso muda);
   - nada de `innerHTML`;
   - a etapa vem de uma lista fechada: `desligado`, `sem-envio`, `abrindo`, `preenchendo`, `revisando`, `enviando`, `confirmada`, `bloqueada`, `falhou`, `incerta`, `login-aberto`, `pausado`.
7. **Log:** só "ao vivo: ligado (eu)" e "ao vivo: desligado (eu)" (ou o autor de quem desligou). Nada do quadro, da etapa nem da página.
8. **Testes:**
   - **API:**
     - o assistente recebe 403 ao ler e ao ligar;
     - leitura sem `Sec-Fetch-Site: same-origin` ou com outro `Host` recebe 403;
     - 404 no build padrão e no `central-dev`;
     - `no-store`;
     - o log só com ligado e desligado.
   - **Executor e navegador:**
     - nenhum quadro depois de terminar, pausar, sair, apagar ou desligar;
     - login aberto não dá quadro;
     - sem leitura por 10 s, desliga.
   - **Disco:** nenhum arquivo novo na pasta de dados nem no temp durante um envio com o ao vivo ligado.
   - **Chrome de verdade, sem janela (só no CI):** o envio fictício manda quadros e, depois de desligar, para de mandar.

## O que muda para o Jarvis

- O painel "Ao vivo" na Fila: liga ao abrir, lê a cada segundo e desliga ao fechar.
- A frase de cada etapa. Sem envio: "Nada sendo enviado agora".
- Na CSP de `index.html`: `img-src 'self' data:`.
