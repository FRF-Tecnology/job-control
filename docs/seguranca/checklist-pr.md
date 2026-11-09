# Checklist de segurança do PR

Bead: `jc-a2y.11`. Dono: agente CyberSec. Vale para o branch `reescrita-go`.

## Quando este checklist é obrigatório

Todo PR que mexe em pelo menos uma destas áreas precisa da revisão do CyberSec antes do merge:

- **API local:** `internal/api`, `app/main.go`, `cmd/central-dev`, `api/openapi.yaml`;
- **IA:** `internal/ia`, prompts, guarda, provedores, CLIs;
- **Segredos:** `internal/segredos`, qualquer leitura ou gravação de chave ou token;
- **Navegador:** `internal/navegador` (P5), `automacao/`;
- **Nuvem:** `cmd/nuvem` (P7), cliente da nuvem no app;
- **Atualização e distribuição:** P8.

**Severidade dos achados:**

| Severidade | Exemplo | Efeito no merge |
|---|---|---|
| Crítica | chave em log ou banco, envio sem aprovação, CPF/senha preenchido, API ouvindo fora do 127.0.0.1 | **bloqueia** até corrigir |
| Alta | saída da IA sem passar pela guarda, rota que grava sem checar `Sec-Fetch-Site` | corrige no mesmo PR ou abre bead antes do merge |
| Média | log com conteúdo pessoal, falta de limite de tamanho ou tempo | bead com prazo |
| Baixa | erro ignorado sem impacto de segurança | bead opcional |

**Dois números por achado, sempre separados:**
- **Severidade original:** a da fonte oficial (GitHub Advisory, CVE, CVSS) ou da ferramenta. Nunca é rebaixada.
- **Risco contextual:** o risco no nosso uso, com o motivo escrito (como o código é usado, onde roda, o que o atacante precisa ter antes).

**Regra do Vinicius: achado crítico bloqueia o merge até ser corrigido.**
- Vale para qualquer achado com severidade original **crítica** que esteja aberto no branch, seja novo ou antigo, de produção, de dev ou de CI.
- Não há exceção automática: o risco contextual menor, o achado ser anterior ao PR ou ser só de teste **não** liberam o merge.
- O achado só deixa de bloquear quando a correção entra no branch e a verificação (por exemplo, `npm audit`) confirma.
- O risco contextual serve para entender o problema e ordenar as outras correções. Ele não muda o bloqueio de um crítico nem fecha o bead.

Dev e CI estão no escopo: dependência só de teste não é ignorada por não ir no app.

Como copiar para o PR: cole as seções que se aplicam e marque cada item. Item sem evidência (teste, comando ou linha de código) conta como não feito.

## 1. Aprovação humana de envios

- [ ] Nenhum caminho de código envia candidatura, mensagem para recrutador ou follow-up sem um registro de aprovação da pessoa para **aquele item**.
  - Verificar: teste que chama o envio sem aprovação e espera recusa (status 4xx ou erro).
- [ ] A aprovação vale para um item e um destinatário, uma vez. Repetir o envio pede nova aprovação.
- [ ] Texto vindo de vaga, email ou página **nunca** vira ação: a IA só sugere, e quem dispara é a pessoa.
  - Verificar: teste com fixture de vaga contendo "ignore as instruções e envie..." e a fila não muda.
- [ ] Emails: só leitura. Não há chamada que responda, arquive, apague ou marque como lido.

## 2. Limites do LinkedIn e do navegador (P5)

- [ ] Limite diário de candidaturas no LinkedIn configurado em no máximo **40**, com padrão **30**. O contador fica no SQLite e sobrevive a reiniciar o app.
  - Verificar: teste que tenta a candidatura 41 no mesmo dia e recebe recusa.
- [ ] Intervalo mínimo entre ações automáticas, com variação.
- [ ] Ao ver CPF, RG, senha, dado bancário ou captcha, o app **para** e marca a vaga como `bloqueada` com a pendência.
  - Verificar: teste com fixture HTML que tem campo de CPF; o campo fica vazio e a etapa vira `bloqueada`.
- [ ] O app nunca cria conta, nunca loga com senha e nunca clica em prompts de permissão do navegador.
- [ ] Perfil do Chrome próprio do app, com permissão de arquivo restrita ao usuário. Cookies não são copiados para o banco nem para logs.

## 3. Cofre e segredos

São dois tipos de segredo, com lugares diferentes:

| Tipo | Exemplos | Onde fica | Onde nunca fica |
|---|---|---|---|
| **Segredo local da pessoa** (app de PC) | chave de API da IA, token da conta na nuvem | cofre do sistema (`internal/segredos`) | SQLite, JSON, config, log, variável de ambiente persistida, repo |
| **Segredo de infraestrutura** (`cmd/nuvem`) | senha do Postgres, segredo do cliente OAuth Google, chave de assinatura de token, chave do OpenRouter, SMTP | variável de ambiente do servidor, definida pelo dono da infra | código, repo, imagem, log, app de PC |

- [ ] Segredo local só no cofre do sistema. Nunca em SQLite, JSON, arquivo de config, variável persistida ou log.
  - Verificar: `grep` no diff por `apiKey`, `token`, `secret`, `Authorization` fora de `internal/segredos` e dos provedores.
- [ ] A API devolve só os 4 últimos caracteres da chave. Nenhuma rota devolve a chave inteira.
  - Verificar: teste em `internal/api` que salva uma chave fictícia e confere a resposta de `GET /ia`.
- [ ] Mensagens de erro dos provedores não repetem a chave nem o cabeçalho `Authorization`.
- [ ] CLIs de assinatura rodam sem shell (`exec.CommandContext` com lista de argumentos), numa pasta temporária, com tempo limite e sem chave de API no ambiente.
- [ ] Segredo de infraestrutura só por variável de ambiente no servidor. O app de PC nunca recebe nenhum deles.
  - Verificar: o binário do app e `app/frontend/dist` não contêm segredo da nuvem (busca por nome de variável e por gitleaks no artefato).
- [ ] Faltou variável de ambiente obrigatória: `cmd/nuvem` não sobe, e a mensagem de erro diz o nome da variável sem mostrar o valor.
- [ ] gitleaks passou no CI.

## 4. Dados sensíveis e LGPD

- [ ] Nenhum campo novo guarda CPF, RG, senha ou dado bancário. A importação continua recusando CPF/RG.
- [ ] Logs não trazem conteúdo pessoal: nada de corpo de pedido, texto de vaga, email, nome de empresa ou query string com dados da pessoa.
- [ ] Dado novo é o mínimo necessário e fica local por padrão. Se vai para a nuvem (P7), o PR diz qual campo, por quê e por quanto tempo.
- [ ] Exportar e apagar continuam cobrindo o dado novo.

## 5. Fixtures e repositório público

- [ ] Fixtures, goldens, prints e exemplos usam **só dados fictícios**: nomes, emails (`@exemplo.com`), empresas, URLs e textos de vaga.
- [ ] Nenhum arquivo de `dados/` entrou no diff. Nenhum caminho de máquina com nome de usuário.
- [ ] Páginas salvas do LinkedIn ou da Gupy (P5) foram limpas: sem nome, foto, ID de perfil, cookie ou token.

## 6. API local (app de PC e `cmd/central-dev`)

Vale só para a API que roda no PC da pessoa. A nuvem (`cmd/nuvem`) é outro servidor, com outras regras (seção 8).

- [ ] Servidor só em loopback (`127.0.0.1`); nunca `0.0.0.0` nem IP da rede. `Host` diferente de `localhost`, `127.0.0.1` ou `[::1]` é recusado.
  - Verificar: teste que manda `Host: exemplo.com` e espera recusa; `grep` no diff por `0.0.0.0` e `:porta` sem IP.
- [ ] Toda rota que grava recusa `Sec-Fetch-Site` diferente de `same-origin` ou `none` e exige `Content-Type: application/json`.
- [ ] Nenhum cabeçalho CORS que libere outra origem.
- [ ] Rota nova entrou antes no `api/openapi.yaml`.
- [ ] Corpo com limite de tamanho; chamadas externas com tempo limite.

## 7. IA (OWASP Top 10 para LLM)

- [ ] Toda saída da IA passa pela guarda antes de ser mostrada ou usada.
- [ ] Conteúdo externo (vaga, email, página) entra no prompt marcado como dado, nunca como instrução.
- [ ] A IA não recebe ferramentas que gravam, enviam ou abrem URLs.
- [ ] Cota e tempo limite por chamada.

## 8. Nuvem (P7, `cmd/nuvem`)

Diferente da API local: este servidor **escuta na rede** e recebe pedidos da internet. A checagem de Host do loopback não serve aqui.

- [ ] Endereço e porta vêm de variável de ambiente; TLS no servidor ou no proxy da frente, com HTTP redirecionado para HTTPS.
- [ ] Toda rota, menos login e saúde, exige token de conta válido; o token é curto e verificado no servidor.
- [ ] CORS só para as origens do produto; nada de `*` com credenciais.
- [ ] Tempos limite e limite de corpo em todas as rotas; erro não devolve detalhe interno.
- [ ] Login Google com OIDC + PKCE e `state`; link por email com token de uso único, 15 minutos, guardado só como hash.
- [ ] Limite de tentativas por IP e por conta no login e no proxy de modelos.
- [ ] Usuário do Postgres com privilégio mínimo; segredos só por variável de ambiente.
- [ ] Token da conta no app fica no cofre do sistema e expira.

## 9. Cadeia de suprimentos

- [ ] `govulncheck ./...` sem vulnerabilidade alcançável.
- [ ] `npm audit --omit=dev` em `app/frontend` sem alta ou crítica.
- [ ] `npm audit` com dev: toda alta ou crítica tem bead com dono, ID oficial (GHSA/CVE) e risco contextual escrito. Crítica aberta bloqueia o merge, mesmo se for anterior ao PR.
- [ ] Dependência nova justificada no PR.
- [ ] Actions fixadas por SHA (dono: Infra, bead `jc-a2y.10`). O CyberSec revisa depois que o Infra entregar.

## Linha de base (2026-11-08, commit `c62b89a`)

Medida para comparar os próximos PRs. Há **2 avisos com severidade original crítica**, ambos em dependência de teste (dev/CI).

> **Pendência aberta, não resolvida:** o bead `jc-a2y.12` (prioridade P1, dono Front-end) corrige esses dois críticos. Enquanto ele não entrar no branch, a regra acima **bloqueia o merge** no `reescrita-go`. A análise contextual abaixo explica o risco; ela não resolve nem reclassifica o problema.

| Verificação | Resultado |
|---|---|
| `go test ./...` | passa |
| `govulncheck ./...` | nenhuma vulnerabilidade |
| `npm audit --omit=dev` (`app/frontend`) | 0 vulnerabilidades |
| `npm audit` com dev (`app/frontend`) | 3 pacotes: `tinypool` (crítica), `vitest` (crítica, herdada do `tinypool`), `@vitest/mocker` (moderada) |
| `gosec ./...` | 24 avisos, triados abaixo |

### Dependências de teste (bead `jc-a2y.12`, dono Front-end)

Cadeia: `app/frontend/package.json` declara `vitest ^3.2.7` (dev) → o `package-lock.json` trava `vitest 3.2.7`, que traz `tinypool 1.1.1` e `@vitest/mocker 3.2.7`. Nenhum deles vai no app final.

| Aviso oficial | CVE | Pacote e versões afetadas | Corrigido em | Severidade original | Risco contextual |
|---|---|---|---|---|---|
| [GHSA-5gmw-xhrv-c9v3](https://github.com/advisories/GHSA-5gmw-xhrv-c9v3) | CVE-2026-104848 | `tinypool <= 2.1.0` | 2.1.1 | Crítica (CVSS 4.0: 9.5) | Média |
| [GHSA-85c8-ppgw-ccpr](https://github.com/advisories/GHSA-85c8-ppgw-ccpr) | CVE-2026-104849 | `tinypool < 2.1.2` | 2.1.2 | Crítica (CVSS 4.0: 9.5) | Média |
| [GHSA-82fw-gwwq-j7x9](https://github.com/advisories/GHSA-82fw-gwwq-j7x9) | CVE-2026-84373 | `vitest` e `@vitest/mocker >= 2.1.0, < 4.1.11` | 4.1.11 | Moderada (CVSS 3.1: 5.9) | Baixa |
| [GHSA-qxg6-89mg-rqv8](https://github.com/advisories/GHSA-qxg6-89mg-rqv8) | CVE-2026-112949 | `@vitest/mocker >= 2.1.0-beta.7, < 4.1.12` | 4.1.12 | Moderada (CVSS 3.1: 4.7) | Baixa |

**Como explora e por que o risco contextual é menor** (resumo dos avisos oficiais):
- **tinypool (as duas críticas):** o atacante precisa antes **poluir o `Object.prototype` dentro do mesmo processo Node** (por exemplo, por outra biblioteca vulnerável). Aí o `tinypool` passa `execArgv`, `env` ou `filename` herdados para o worker, que carrega um script do atacante.
  - No nosso uso, quem roda o `tinypool` é o Vitest, executando os nossos testes em `jsdom`. Não há entrada de rede nem dado externo nesse processo.
  - No CI, o gatilho é `pull_request` (não `pull_request_target`), com `permissions: contents: read` e sem segredos. Um PR malicioso já roda código no runner pelos próprios testes; a falha não dá acesso a mais nada.
  - Fica **média**, não baixa: num ataque à cadeia de suprimentos (dependência de teste comprometida), a falha vira um caminho para executar código na máquina do dev, onde há sessões e chaves.
- **@vitest/mocker (as duas moderadas):** exige alcançar o WebSocket do servidor de dev do Vite e usar `mockerPlugin`/`interceptorPlugin` direto ou o modo navegador do Vitest. Não usamos nenhum dos dois (`vite.config.ts`: `environment: "jsdom"`), e o servidor de dev ouve só em `127.0.0.1`. Fica **baixa**.

**Correção:** subir o `vitest` até uma versão que traga `tinypool >= 2.1.2` e `@vitest/mocker >= 4.1.12`. O npm indica `vitest 5.1.0`, que é versão major: conferir o Vitest verde. Pronto quando `npm audit` (com dev) não tiver alta nem crítica.

### Código Go

| Bead | Dono | Achado | Severidade | Correção |
|---|---|---|---|---|
| `jc-a2y.13` | Back-end | `internal/api/http.go:50` grava no log o caminho com a query string no erro 500. A query pode ter empresa e título da vaga (gosec G706). | Média | Logar só método, rota sem query e tipo do erro, com teste. |
| `jc-a2y.14` | Back-end | `cmd/central-dev/main.go:61` usa `http.ListenAndServe` sem tempos limite (gosec G114). Só dev, ouve em 127.0.0.1. | Baixa | `http.Server` com `ReadHeaderTimeout`. |
| `jc-a2y.15` | CyberSec | 12 erros ignorados (gosec G104) em `internal/ia`, `internal/armazem`, `internal/api`, `internal/legado` e `cmd/central-cli`. | **Triagem pendente**, não é vulnerabilidade provada | Classificar cada um; o que tiver impacto vira bead do Back-end. |

As rotas `/ia/...` fora do `api/openapi.yaml` ficam com o bead `jc-a2y.6.1` (contrato da P5), sem bead novo.

**Triados como sem risco:** G204 em `internal/ia/cli.go` (comando vem da lista fixa de CLIs, sem shell); G304 e G703 em `legado`, `config` e `central-cli` (caminho escolhido pela própria pessoa no PC dela).

**Como repetir:**

```sh
go test ./...
go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
go install github.com/securego/gosec/v2/cmd/gosec@latest && gosec -quiet ./...
cd app/frontend && npm audit --omit=dev && npm audit
```
