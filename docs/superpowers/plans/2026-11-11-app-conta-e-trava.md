# Conta, licença e trava no app — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** O app pede conta antes de tudo (email+senha ou Google), guarda a sessão no cofre, mantém uma licença assinada fresca e trava toda ação no backend local quando o teste acabou, a assinatura não existe ou não há internet.

**Architecture:** Um pacote novo `internal/conta` é o único que fala com a nuvem (contrato `api/nuvem.yaml`). A API local ganha `/conta/*` e `/assinatura/*` e uma trava em `API.Chamar` que vale para tela, CLI e MCP. A CLI e o MCP só leem o arquivo de licença que o app grava. A tela ganha uma porta de entrada no `App.tsx`, uma faixa de estado e as telas Conta e Assinatura, sem capricho visual.

**Tech Stack:** Go 1.26 (pacotes existentes + `github.com/denisbrodbeck/machineid`), `internal/licenca` (do plano da nuvem, Task 1), `segredos.Cofre`, React + TS + Vitest + Playwright.

**Spec:** `docs/superpowers/specs/2026-11-11-conta-e-assinatura-design.md` · **Depende de:** `docs/superpowers/plans/2026-11-11-nuvem-contas.md` Task 1 (`internal/licenca`) e Task 12 (`api/nuvem.yaml`). Pode começar logo depois da Task 1 de lá, usando a nuvem falsa daqui.

## Global Constraints

- Seguir o AGENTS.md: regra nova só no Go; não editar `funcoes.json` nem os golden; nenhum teste usa o cofre real (usar `segredos.Memoria`); testes de navegador sem janela.
- O app nunca guarda senha. O token de renovação vai para o cofre na chave `conta/renovacao`; o de acesso fica só em memória.
- Só o processo do app renova sessão e licença. CLI e MCP nunca chamam a nuvem.
- Licença: renovar a cada 10 min, ao abrir e ao voltar a internet; vale 30 min (`valida_ate`); arquivo `licenca.json` na pasta de dados.
- 402 com corpo `{"erro": <frase>, "motivo": "teste-acabou" | "sem-assinatura" | "sem-internet" | "sem-conta"}`.
- Sempre liberado, mesmo bloqueado: GET/HEAD/OPTIONS, `/conta/*`, `/assinatura/*`, `/dados/apagar`, `/app/janela`, `/app/janela/padrao`, `/ia/chaves/*`.
- Endereço da nuvem, chave pública e `client_id` do Google entram no build por `-ldflags -X` (`internal/conta.EnderecoNuvem`, `ChavePublica`, `GoogleClientID`). Sem eles, o build de produção recusa abrir (mensagem "build sem nuvem configurada").
- A nuvem falsa fica atrás da tag de build `contafalsa`; o executável de produção não a traz. `central-dev` e os testes usam a tag.
- Textos curtos em pt-BR no tom do app. Frases fixas desta lista (exatas):
  - faixa do teste: `Seu teste acaba em {h} h {m} min · Assinar`
  - bloqueado: `Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.`
  - sem assinatura (já foi assinante): `Sua assinatura não está ativa. Você ainda vê tudo; para buscar e se candidatar, assine.`
  - sem internet: `Sem conexão: dá para ver, mas não para buscar ou se candidatar.`
  - CLI/MCP com licença vencida: `Abra o Applyer para renovar o acesso.`

## Review Focus

- O relógio do PC adiantado ou atrasado horas → a licença não pode virar "válida para sempre" nem "sempre vencida": a conferência usa `valida_ate` contra o relógio local, mas a nuvem manda também `emitida_em`; licença com `emitida_em` mais de 5 min no futuro do relógio local é aceita só se a nuvem acabou de emiti-la nesta sessão (diferença guardada) → Task 2 `TestRelogioDesajustado`.
- App aberto quando a internet cai no meio do dia → passados 30 min da última licença, ações dão 402 `sem-internet`; ao voltar, libera sem reiniciar → Task 2 `TestQuedaEVoltaDaInternet`.
- Fechar o navegador no meio do login Google (ou nunca voltar) → a tela não fica presa: "Cancelar" e o tempo de 5 min liberam a porta e o ouvinte local fecha → Task 3 `TestGoogleCanceladoFechaOuvinte`.
- Ação automática em segundo plano (envio, preparo, retomada de triagens) quando o teste acaba no meio dela → o próximo ciclo não começa → Task 5 `TestEnvioNaoComecaBloqueado`.
- CLI rodando com o app fechado há horas → ações recusadas com a frase fixa, leituras funcionam → Task 6 `TestCLISoLeLicenca`.

---

### Task 1: Cliente da nuvem e nuvem falsa

**Files:**
- Create: `internal/conta/nuvem.go` (interface + tipos), `internal/conta/nuvem_http.go`, `internal/conta/nuvem_falsa.go` (`//go:build contafalsa`), `internal/conta/dispositivo.go`
- Test: `internal/conta/nuvem_http_test.go`, `internal/conta/nuvem_falsa_test.go` (`//go:build contafalsa`)

**Interfaces:**
- Consumes: `api/nuvem.yaml`; `licenca.Documento`.
- Produces:
  - `type Par struct { Acesso, Renovacao string; AcessoAte time.Time }`
  - `type ErroNuvem struct { Status int; Msg, Motivo string }` (implementa `error`; `Msg` é a frase da nuvem, mostrada como está)
  - `var ErrSemInternet = errors.New("sem internet")` — qualquer falha de rede/timeout vira este erro
  - `type Nuvem interface {`
    `Criar(ctx, email, senha, termos string) error;`
    `Confirmar(ctx, email, codigo, senha, dispositivo, termos string) (Par, error);`
    `Entrar(ctx, email, senha, dispositivo string) (Par, error);`
    `Recuperar(ctx, email string) error;`
    `NovaSenha(ctx, email, codigo, senha, dispositivo string) (Par, error);`
    `Google(ctx, code, verifier, redirectURI, dispositivo, termos string) (Par, error);`
    `Renovar(ctx, renovacao string) (Par, error);`
    `Sair(ctx, renovacao string) error;`
    `Licenca(ctx, acesso, dispositivo string) (licenca.Documento, error);`
    `Assinar(ctx, acesso string) (url string, err error);`
    `Portal(ctx, acesso string) (url string, err error);`
    `Notas(ctx, acesso string) ([]Nota, error);`
    `Excluir(ctx, acesso, email string) error }`
  - `func NovaHTTP(endereco string, cliente *http.Client) Nuvem` — timeout 15 s, só `https://` (exceto `http://127.0.0.1` nos testes)
  - `func NovaFalsa(priv ed25519.PrivateKey, agora func() time.Time) *Falsa` (tag `contafalsa`) — contas em memória, código sempre `123456`, mesma regra de teste/licença do desenho, `Falsa.Avancar(d time.Duration)`, `Falsa.Cortesia(email)`, `Falsa.Assinar(email)`, `Falsa.SemInternet(bool)`
  - `func Dispositivo() (string, error)` — `machineid.ProtectedID("applyer")` (hex 64)

- [ ] **Step 1:** Testes: `TestHTTPEntrar` (httptest devolve Par; confere método, caminho e corpo JSON), `TestHTTPErroDaNuvem` (401 `{"erro":"Email ou senha incorretos"}` → `*ErroNuvem` com a frase), `TestHTTPSemInternet` (servidor fechado → `ErrSemInternet`), `TestHTTPRecusaHTTPForaDoLoopback`, `TestFalsaFluxoCompleto` (criar → confirmar 123456 → licença teste → `Avancar(24h+1s)` → bloqueado), `TestDispositivoHex64`.
- [ ] **Step 2:** `go test ./internal/conta/` e `go test -tags contafalsa ./internal/conta/` → FAIL.
- [ ] **Step 3:** Implementar.
- [ ] **Step 4:** PASS nas duas formas.
- [ ] **Step 5:** Commit.

### Task 2: Sessão, licença em disco e "pode agir"

**Files:**
- Create: `internal/conta/conta.go`, `internal/conta/licenca_arquivo.go`
- Test: `internal/conta/conta_test.go` (`//go:build contafalsa`), `internal/conta/licenca_arquivo_test.go`

**Interfaces:**
- Consumes: Task 1; `segredos.Cofre`; `licenca.Conferir`.
- Produces:
  - `type Situacao struct { Entrada bool; Email string; Estado licenca.Estado; TesteFim *time.Time; PeriodoFim *time.Time; Motivo string }` (JSON `entrada, email, estado, teste_fim, periodo_fim, motivo`; `Motivo` ∈ `""`, `teste-acabou`, `sem-assinatura`, `sem-internet`, `sem-conta`)
  - `type Conta struct{...}`; `func Nova(n Nuvem, cofre segredos.Cofre, pasta string, pub ed25519.PublicKey, agora func() time.Time) *Conta`
  - `func (c *Conta) Iniciar(ctx context.Context)` — renova a sessão pelo cofre e pede licença; depois um laço a cada 10 min (para com `ctx`)
  - `func (c *Conta) Situacao() Situacao`
  - `func (c *Conta) PodeAgir() (bool, string)` — `(true, "")` só com licença válida de estado teste/assinante; senão `(false, motivo)`. Motivo: sem sessão → `sem-conta`; licença vencida por falta de rede → `sem-internet`; bloqueado com `PeriodoFim != nil` → `sem-assinatura`; bloqueado sem período → `teste-acabou`
  - `func (c *Conta) Entrou(ctx, p Par) error` — grava a renovação no cofre, guarda o acesso, pede licença
  - `func (c *Conta) Sair(ctx) error` — chama `Nuvem.Sair`, apaga cofre e `licenca.json`
  - `func (c *Conta) Acesso(ctx) (string, error)` — renova se faltar menos de 1 min para `AcessoAte`
  - `func (c *Conta) AtualizarLicenca(ctx) error`
  - `func LerLicencaDoDisco(pasta string, pub ed25519.PublicKey, agora time.Time) (licenca.Licenca, error)` (usada pela CLI e MCP)
  - `func GravarLicenca(pasta string, d licenca.Documento) error` — escrita atômica (temp + rename), permissão 0600

- [ ] **Step 1:** Testes (Falsa + `segredos.Memoria` + relógio controlável): `TestSemContaNaoAge` (`sem-conta`), `TestEntrouPodeAgir`, `TestRenovacaoNoCofreNaoNoDisco` (varre a pasta: o token não aparece em arquivo nenhum), `TestTesteAcabaBloqueia` (`teste-acabou`), `TestAssinaturaCanceladaMotivo` (`sem-assinatura`), `TestQuedaEVoltaDaInternet` (Falsa sem internet: até 30 min ainda age; 30m1s → `sem-internet`; internet volta + `AtualizarLicenca` → age), `TestRelogioDesajustado` (relógio local 3 h atrasado: licença recém-emitida é aceita e vence 30 min depois pelo relógio local, medido pela diferença guardada), `TestLicencaEditadaNoDiscoRecusada`, `TestSairApagaTudo`, `TestIniciarComSessaoRevogadaVoltaParaEntrar`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 3: Login com Google pelo navegador

**Files:**
- Create: `internal/conta/google.go`
- Test: `internal/conta/google_test.go`

**Interfaces:**
- Consumes: Task 2 (`Conta.Entrou`), `Nuvem.Google`.
- Produces:
  - `func (c *Conta) IniciarGoogle(ctx context.Context, abrir func(url string) error, termos string) error` — ouvinte em `127.0.0.1:0`, caminho `/retorno`; `state` 32 bytes; PKCE S256; URL `https://accounts.google.com/o/oauth2/v2/auth?client_id=…&redirect_uri=http://127.0.0.1:<porta>/retorno&response_type=code&scope=openid%20email%20profile&code_challenge=…&code_challenge_method=S256&state=…&prompt=select_account`; volta logo (o resto corre em segundo plano); só um login Google por vez (`ErrGoogleEmAndamento`)
  - `func (c *Conta) CancelarGoogle()`
  - `func (c *Conta) EstadoGoogle() string` — `""` | `aguardando` | `erro:<frase>`
  - O ouvinte: confere `state` (diferente → página de erro, não chama a nuvem), troca na nuvem, responde HTML "Pode voltar para o Applyer" e fecha; prazo de 5 min fecha sozinho

- [ ] **Step 1:** Testes (abrir falso captura a URL; o teste faz o GET no ouvinte): `TestGoogleURLTemPKCEEState`, `TestGoogleRetornoEntra`, `TestGoogleStateErradoNaoEntra`, `TestGoogleCanceladoFechaOuvinte` (após `CancelarGoogle`, conectar na porta falha), `TestGooglePrazoCincoMinutos`, `TestGoogleUmDeCadaVez`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 4: Rotas `/conta/*` e `/assinatura/*` na API local

**Files:**
- Create: `internal/api/conta.go`
- Modify: `internal/api/api.go` (campo `conta`, `Opcoes.Conta`, despacho em `rotear` antes de `/candidaturas`), `api/openapi.yaml` (rotas novas, esquemas `SituacaoConta`, `Erro402`; `info.version` sobe uma vez)
- Test: `internal/api/conta_test.go` (`//go:build contafalsa`)

**Interfaces:**
- Consumes: Tasks 1–3. Para abrir o navegador: `Opcoes.AbrirNavegador func(url string) error` (app: `application.Get().Browser.OpenURL`).
- Produces (só a tela: `autor == "eu"`; do assistente → 403):
  - `GET /conta` → `200 Situacao` (+ `google` = `EstadoGoogle()`)
  - `POST /conta/criar {email, senha, termos}` → 202 `{mensagem}`; `POST /conta/confirmar {email, codigo, senha}` (a tela guarda a senha só em memória entre criar/entrar e confirmar; ruling R10 da nuvem) → 200 `Situacao`; `POST /conta/entrar {email, senha}` → 200 `Situacao`; `POST /conta/recuperar {email}` → 202; `POST /conta/nova-senha {email, codigo, senha}` → 200; `POST /conta/google {termos}` → 202; `POST /conta/google/cancelar` → 204; `POST /conta/sair` → 204; `POST /conta/excluir {email, apagarDados: bool}` → 204 (com `apagarDados` chama o mesmo caminho de `/dados/apagar` depois)
  - `GET /assinatura` → `200 {situacao, notas: [...]}`; `POST /assinatura/assinar` → abre o Checkout no navegador e liga a renovação de licença a cada 5 s por até 10 min → 202; `POST /assinatura/gerenciar` → abre o portal → 202
  - Termos que mudaram: a nuvem responde 409 `motivo: "termos"`; a rota repassa e a tela mostra os links e "Aceito os novos termos", reenviando o mesmo pedido com `termos` = versão atual (`entrar`, `confirmar` e `google` aceitam o campo `termos`).
  - Erros da nuvem: `*ErroNuvem` → mesmo status e `{erro: Msg, motivo}`; `ErrSemInternet` → `503 {"erro":"Sem conexão com a internet","motivo":"sem-internet"}`
  - `termos` = `conta.TermosVersao` (constante, `0-rascunho` até a revisão)

- [ ] **Step 1:** Testes: `TestContaFluxoPelaAPI` (criar → confirmar → GET /conta entrada=true estado=teste), `TestContaErroDaNuvemRepassado`, `TestContaSemInternet503`, `TestContaSoDaTela` (canal mcp → 403), `TestAssinarAbreNavegador` (AbrirNavegador falso recebe a URL da Falsa), `TestExcluirComApagarDados`; e o `TestRotasDoCodigoEstaoNoContrato` existente continua passando.
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** `go test -tags contafalsa ./internal/api/ ./internal/conta/` PASS e `go test ./...` PASS. **Step 5:** Commit.

### Task 5: Trava das ações

**Files:**
- Create: `internal/api/trava.go`
- Modify: `internal/api/api.go` (`Chamar`: a trava roda depois do desvio do assistente e de `ehPedidoDeApagar`, antes de `rotear`; `Opcoes.PodeAgir func() (bool, string)`), `internal/envio/envio_dev.go` e `envio_padrao.go` (`Ligar` recebe `podeAgir func() (bool, string)`; o executor não começa ciclo com `false`), `app/frontend/src/app/preparoAutomatico.ts` (não tenta preparar após 402)
- Test: `internal/api/trava_test.go`, `internal/envio/trava_test.go`

**Interfaces:**
- Produces:
  - `func liberadoSemLicenca(metodo, caminho string) bool` — regra da lista global "Sempre liberado"
  - `func respostaTrava(motivo string) Resposta` — 402 com a frase fixa de cada motivo
  - `Opcoes.PodeAgir == nil` → tudo liberado (testes antigos e `central-dev` sem conta). Os três binários de produção sempre passam a função (Task 6 prova).
  - Trabalho em segundo plano que ainda não está na `reescrita-go` (a triagem assíncrona da busca da branch `app/busca-candidatura-aovivo`: `triarBusca` e `RetomarTriagens`): quando juntar, cada uma consulta `a.podeAgir` antes de começar e para entre vagas se passar a `false`.

- [ ] **Step 1:** Testes: tabela `TestTravaPorRota` (bloqueado: `POST /vagas`, `POST /vagas/x/mover`, `PUT /perfil`, `POST /candidaturas/x/aprovar`, `PUT /ia/config` → 402; `GET /estado`, `POST /dados/apagar`, `PUT /ia/chaves/anthropic`, `POST /conta/sair`, `POST /app/janela` → passam), `TestTravaMotivos` (os 4 motivos → as frases fixas), `TestTravaValeParaAssistente` (canal cli, `ChamarAssistente` de uma escrita → 402), `TestEnvioNaoComecaBloqueado`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 6: Ligar no app, na CLI, no MCP e no central-dev

**Files:**
- Modify: `app/main.go` (cria `conta.Nova(conta.NovaHTTP(conta.EnderecoNuvem, …), cofre, pasta, pub, time.Now)`, `Iniciar` em goroutine, passa `Conta`, `PodeAgir`, `AbrirNavegador` em `api.Opcoes` e `podeAgir` em `envio.Ligar`), `cmd/central-cli/main.go` e `cmd/central-mcp/main.go` (`PodeAgir` lendo `conta.LerLicencaDoDisco` a cada pedido; motivo vencida → frase `Abra o Applyer para renovar o acesso.`), `cmd/central-dev/main.go` (com `-tags contafalsa` usa a Falsa; sem a tag, sem conta), `app/Taskfile.yml` / `app/build` (ldflags `-X` das três variáveis a partir de variáveis de ambiente do build; o CI usa valores de teste)
- Test: `cmd/central-cli/trava_test.go`, `cmd/central-mcp/trava_test.go`, `app/main_test.go` (novo caso)

- [ ] **Step 1:** Testes: `TestCLISoLeLicenca` (pasta com licença válida → escreve; vencida → 402 com a frase; sem arquivo → `sem-conta`; nenhuma chamada de rede: `http.DefaultTransport` trocado por um que falha o teste), o mesmo para o MCP, `TestAppPassaPodeAgir` (o `api.Opcoes` montado no app tem `PodeAgir` e `Conta` não nulos), `TestBuildSemNuvemRecusa` (variáveis de build vazias → erro "build sem nuvem configurada").
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** `go test ./...` e `go test -tags contafalsa ./...` PASS; `go vet` nas duas formas. **Step 5:** Commit.

### Task 7: Telas — porta de entrada, faixa e assinatura

**Files:**
- Create: `app/frontend/src/telas/Conta.tsx` (criar, entrar, código, esqueci a senha, aguardando Google), `app/frontend/src/telas/Assinatura.tsx`, `app/frontend/src/telas/FaixaConta.tsx`, `app/frontend/src/app/conta.ts` (tipos `Situacao`, `textoFaixa(s, agora)`), `docs/legal/termos.md`, `docs/legal/privacidade.md` (rascunhos, versão `0-rascunho`)
- Modify: `app/frontend/src/App.tsx` (lê `GET /api/v1/conta` ao abrir e a cada 60 s; `entrada=false` → só a tela Conta; depois o app normal; rota `assinatura` no menu; conteúdo de cada tela dentro de `<fieldset disabled={!podeAgir} className="trava">` exceto `perfil`, `ia`, `assinatura`), `app/frontend/src/app/contexto.ts` (`ErroApi` com status 402 dispara `window.dispatchEvent(new CustomEvent('applyer:trava', {detail: motivo}))`; `App` relê `/conta`), `app/frontend/src/telas/Perfil.tsx` (seção Conta: email, Sair, Excluir conta com confirmação digitando o email e caixa "apagar também os dados deste computador")
- Test: `app/frontend/src/telas/conta.test.tsx`, `app/frontend/src/telas/assinatura.test.tsx`, `app/frontend/src/telas/faixa-conta.test.tsx`, `app/frontend/e2e/conta.spec.ts`

**Interfaces:**
- Consumes: rotas da Task 4; frases fixas da lista global.
- Produces: `textoFaixa(s: Situacao, agora: Date): string | null`; `podeAgir` no contexto do app (`useApp().podeAgir`).
- A ordem com o primeiro acesso da branch `app/onboarding-busca-triagem`: a porta da conta vem antes do teste `onboardingConcluido`; se essa branch ainda não estiver na `reescrita-go`, só a porta entra e o encaixe fica para a junção.

- [ ] **Step 1:** Testes Vitest: `textoFaixa` (5 h 12 min → `Seu teste acaba em 5 h 12 min · Assinar`; bloqueado/sem-assinatura/sem-internet → as frases exatas; assinante → null), `TestCriarContaPedeCodigo` (envia, mostra campo de código, confirma), `TestErroDaNuvemAparece`, `TestGoogleAguardandoECancelar`, `TestTravaDesabilitaAcoes` (podeAgir=false → botões da Triagem desabilitados; Perfil não), `TestEvento402MostraFaixa`, `TestExcluirContaExigeEmail`. Playwright `conta.spec.ts` (central-dev com `contafalsa`, sem janela): abrir → criar conta → código 123456 → primeiro acesso/Hoje → faixa do teste → (rota de teste da Falsa avança 24 h) → faixa de bloqueio e 402 ao buscar → assinar (Falsa) → liberado.
- [ ] **Step 2:** `npm test` e `npm run e2e` → FAIL.
- [ ] **Step 3:** Implementar (sem capricho visual; componentes e classes existentes).
- [ ] **Step 4:** `npx tsc --noEmit -p .`, `npm test`, `npm run build`, `npm run e2e` PASS; `go test ./...` PASS.
- [ ] **Step 5:** Commit.

### Task 8: Verificação final no app compilado

- [ ] **Step 1:** `wails3 build EXTRA_TAGS=contafalsa` (build de QA) e rodar a jornada da Task 7 no executável, sem janela visível no PC do Vini (máquina de teste), com captura para o PR.
- [ ] **Step 2:** Rodar o skill `security-review` sobre `internal/conta`, `internal/api/conta.go`, `internal/api/trava.go`; corrigir o confirmado com teste.
- [ ] **Step 3:** Todas as suítes do AGENTS.md (Go nas duas tags, raiz `npm test`, Vitest, Playwright) PASS. Commit.
