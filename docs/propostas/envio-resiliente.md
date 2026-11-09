# Envio que se vira sozinho com as falhas

Bead `jc-a2y.6.11` (P0). Status: **proposta**, com os 9 requisitos do CyberSec incorporados. O contrato vai para 0.6.0: duas rotas novas em `x-proposta`, campos novos opcionais e o motivo `aviso-conta`.

O pedido do Vini: o app tem de ser automático e esperto com as falhas. Hoje, qualquer falha técnica vira `falhou` e espera a pessoa reabrir.

## O que muda

| Situação | Hoje | Proposta |
|---|---|---|
| Falha técnica **antes de interagir com o formulário** (navegador não abre, a página fica em `about:blank`, erro ao ler a página) | `falhou/erro-tecnico`; conta no limite | **volta sozinha para `aprovada`**; **não conta no limite**; nova tentativa em 5 min, 30 min e 2 h (no máximo 3 por aprovação). Na 4ª, `falhou/erro-tecnico` |
| Falha técnica **depois** de interagir | `falhou/erro-tecnico` | igual: `falhou`, e a pessoa decide |
| **2 resultados seguidos iguais da mesma plataforma** (`erro-tecnico`, `site-externo`) | cada candidatura para | a **plataforma** é pausada (só ela). As outras seguem. Só a pessoa retoma |
| **Sessão caída** (login, authwall, checkpoint) **antes de interagir** | `bloqueada/login` | volta para `aprovada` (não conta no limite), e a plataforma fica **`precisa-entrar`** |
| Sessão caída **depois** de interagir | `bloqueada/login` | `bloqueada/login`, e a plataforma fica `precisa-entrar` |
| **Captcha ou aviso de conta** (atividade incomum, conta restrita) | `bloqueada` | `bloqueada`, e a plataforma **para até 00:00 do dia seguinte**. Sem nova tentativa automática |
| **Clique em enviar** sem confirmação | `incerta` (M3) | **igual**: nunca volta sozinha |

## As regras (requisitos do CyberSec)

### 1. "Antes de interagir" é uma marca gravada, não dedução

- `interagiuEm` é gravado numa transação **antes** do primeiro preencher, anexar, desmarcar ou avançar. Conferir um campo que a página trouxe preenchido só lê, e não marca.
- `clicouEnviarEm` é gravado numa transação **antes** do clique final.
- Se a gravação da marca falhar, o passo não acontece (`falhou`), e a candidatura também não volta.
- A volta (`DevolverAprovada`) recusa com qualquer marca gravada. O executor também não pede a volta depois de pedir a marca. São duas travas.
- **Ao abrir o app** (`RecuperarAoAbrir`), uma candidatura que ficou em `enviando` segue a marca gravada:
  - **sem marca** (o app caiu antes de interagir): volta para `aprovada` e conta como volta, com as mesmas regras do item 2 (no máximo 3 e a 4ª vira `falhou`; vencida vira `preparada`);
  - **com qualquer marca:** vira `incerta`, como sempre (M3).
- **Cliques antes da marca (decisão do CyberSec):**
  - abrir a página não conta como interação;
  - no LinkedIn, o botão que abre o Easy Apply também não conta, mas só para os rótulos de uma lista fechada;
  - na Gupy e na InHire, qualquer clique conta, e a marca é gravada antes dele.

  Hoje o app não clica em nada antes da marca: os únicos cliques são avançar e enviar, os dois depois dela. A regra vale para quando o clique do Easy Apply entrar.
- **Testes e mutações:**
  - **Queda antes e depois da gravação da marca:** sem marca volta, com marca vira incerta.
  - **Mutações que reprovam:**
    - tirar a conferência da marca no domínio;
    - não marcar antes de preencher;
    - gravar a marca depois de preencher em vez de antes;
    - não marcar antes do clique.

### 2. A volta não é uma aprovação nova

- **Mesma aprovação:**
  - a versão tem de ser a lida ao entrar em `enviando`;
  - a impressão, recalculada do pacote guardado, tem de ser a aprovada;
  - a aprovação continua a mesma: mesma impressão, mesmo `em`, mesmo `validaAte`. Só o `usadaEm` é apagado.
- **Validade do dia original:**
  - se a falha for depois da meia-noite, a candidatura volta a `preparada`, sem aprovação;
  - uma aprovada com a próxima tentativa marcada para depois da meia-noite vence como hoje (`VencerAprovacoes`).
- **No máximo 3 voltas desde a aprovação em vigor.** Uma aprovação nova (depois de revogar ou reabrir) recomeça a contagem. O histórico em `retentativas` nunca é apagado.
- **`proximaTentativaEm` gravado no banco:**
  - o executor não pega a candidatura antes desse instante, nem com o app reaberto;
  - o app reaberto começa pausado, como hoje.
- **Uma transação, só de `enviando` e sem marca:** a volta, a saída de `tentativas`, a entrada em `retentativas`, a próxima tentativa e o estado da plataforma.

### 3. Limite e teto

- A tentativa devolvida sai de `tentativas`, por isso não conta no limite do dia.
- **Teto firme:** contadas + devolvidas no dia nunca passam de **40 páginas abertas por plataforma** (`LimiteMaximo`). Passou disso, `IniciarEnvio` recusa com 429, como o limite, e a plataforma espera o dia seguinte.
- **`GET /envio` mostra, por plataforma:**
  - `voltasHoje`: as voltas do dia;
  - `paginasHoje`: as páginas do dia;
  - `tetoPaginas`: o teto, 40.
- **Sem laço:**
  - no máximo 3 voltas por aprovação;
  - 2 falhas iguais seguidas pausam a plataforma.

### 4. Estado por plataforma, gravado

Fica na meta do banco (`envio-plataformas`), com falha fechada: um valor ilegível ou um estado desconhecido contam como `pausada`. Num banco novo, as três ficam ativas, porque o envio geral já começa pausado.

| Estado | Quando | Como sai |
|---|---|---|
| `ativa` | padrão | — |
| `pausada` | 2 resultados seguidos iguais (`erro-tecnico` ou `site-externo`), ou `POST .../pausar` (qualquer autor) | só a pessoa: `POST /envio/plataformas/{plataforma}/retomar` (`x-so-tela`; o assistente recebe 403) |
| `precisa-entrar` | sessão caída | veja o item 5, ou retomar na tela |
| `parada-hoje` | captcha ou aviso de conta | sozinho, às 00:00 do dia seguinte (horário local). Retomar dá 409 |

- O executor só pega candidatura de plataforma `ativa`. A conferência que vale é a da transação de `IniciarEnvio`, junto com o pausado geral; a da fila é só leitura.
- Um resultado bom não tira a plataforma da pausa: só a pessoa retoma.
- O `motivo` vem de uma lista fechada: `falhas-seguidas`, `login`, `captcha` ou `aviso-conta`. A pausa pela pessoa ou pelo assistente não tem motivo.

### 5. `precisa-entrar` volta sozinha só depois do login

- Volta sozinha só quando **a pessoa fecha a janela "Entrar nas plataformas"** e o envio geral **não está pausado**. As duas coisas são conferidas na mesma transação.
- As candidaturas que voltaram por login só saem dentro da validade da aprovação delas (vencida vira `preparada`).
- O app fechar o navegador (sair, apagar, fechar o app) não conta como login.
- Com o envio geral pausado, a plataforma fica em `precisa-entrar` até a pessoa retomá-la.

### 6. Captcha e aviso de conta

- A candidatura fica `bloqueada` (motivo `captcha` ou `aviso-conta`). Nada volta sozinho.
- A plataforma fica `parada-hoje` até 00:00 do dia seguinte. Retomar dá 409.
- O motivo novo `aviso-conta` tem a pendência "A plataforma mostrou um aviso sobre a sua conta: veja você antes de tentar de novo".
- O detector procura frases em PT, ES e EN: atividade incomum ou suspeita, conta restrita, suspensa, bloqueada ou limitada. Há páginas fictícias em `testdata/paginas`.

### 7. `incerta` nunca volta sozinha

Isso vale também ao abrir o app. A trava do executor só pede a volta em `falhou/erro-tecnico` e em `bloqueada/login`. A do domínio recusa a volta com `clicouEnviarEm` gravado.

### 8. Só códigos fechados no log e na tela

- `executor: <id> devolvida (erro-tecnico, 1/3) aprovada`
- `executor: <id> falha antes de interagir (<tipo do erro>)`
- `envio: plataforma linkedin pausada (falhas-seguidas)`
- `envio: plataforma linkedin ativa (login)`
- `envio: plataforma linkedin pausada (eu)` e `... retomada (eu)`

Nada da página, do pacote ou da URL.

## Contrato (0.6.0)

- **`GET /envio`:** campo novo `plataformas: [{plataforma, estado, motivo, ate}]`, sempre com as três plataformas.
- **`POST /envio/plataformas/{plataforma}/pausar`:** qualquer autor.
- **`POST /envio/plataformas/{plataforma}/retomar`:** `x-so-tela`. Sai de `pausada` ou `precisa-entrar` para `ativa`; de `parada-hoje` ou `ativa`, dá 409.
- **Candidatura:** campos novos opcionais:
  - `retentativas: [{em, codigo}]` (histórico);
  - `proximaTentativaEm`;
  - `interagiuEm`;
  - `clicouEnviarEm`.
- **`MotivoBloqueio`:** ganha `aviso-conta`.

## Testes

- **Domínio:**
  - a volta: só de `enviando`, sem marca, mesma versão e impressão; a aprovação continua a mesma; sai do limite;
  - 5 min, 30 min e 2 h; a 4ª dá `falhou`;
  - depois da meia-noite, `preparada`;
  - as voltas contam por aprovação;
  - o teto de 40 páginas;
  - as marcas;
  - cada transição da plataforma.
- **Armazém:**
  - falha fechada do estado gravado;
  - `IniciarEnvio` recusado com a plataforma parada, na transação.
- **Executor:**
  - marcas gravadas antes de preencher, anexar e clicar;
  - depois de interagir, nunca volta (`falhou`, `bloqueada`, `incerta`);
  - volta com marca gravada reprova;
  - espera a hora com o app reaberto e depois envia sem aprovar de novo, com limite 1;
  - 2 falhas pausam só a plataforma, e as outras seguem;
  - `precisa-entrar` com o envio geral pausado e liberado;
  - captcha e aviso de conta até amanhã;
  - 3 voltas e depois `falhou`;
  - `incerta` nunca volta.
- **Navegador (Chrome de verdade, só no CI):** fechar a janela de login avisa uma vez; o app fechar não avisa.
- **API:**
  - `retomar` só da tela (403 para o assistente);
  - `pausar` de qualquer autor;
  - `GET /envio` com as plataformas, sem contagem interna.
- **Mutações (17):** cada regra acima tem uma mutação que reprova.
