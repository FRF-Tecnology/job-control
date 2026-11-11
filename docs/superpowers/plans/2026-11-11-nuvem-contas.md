# Nuvem de contas e assinatura (cmd/nuvem) — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Um servidor Go (`cmd/nuvem`) com Postgres que cria e autentica contas (email+senha e Google), controla o teste de 1 dia por computador, emite a licença assinada, fala com o Stripe e emite NFS-e.

**Architecture:** `net/http` puro com rotas `/v1/...`, Postgres via `pgx/v5` e migrações `goose` embutidas (mesmo padrão de `internal/armazem`). Cada responsabilidade num pacote em `internal/nuvem/` com interface + implementação falsa para os testes; `internal/licenca` é compartilhado com o app (sem dependência de servidor). Todo relógio e todo gerador aleatório são injetáveis.

**Tech Stack:** Go 1.26, `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3` (já no go.mod), `golang.org/x/crypto/argon2`, `github.com/golang-jwt/jwt/v5` (EdDSA), `github.com/coreos/go-oidc/v3` + `golang.org/x/oauth2`, `github.com/stripe/stripe-go` (última v8x estável na hora da execução, versão fixa), `crypto/ed25519`.

**Spec:** `docs/superpowers/specs/2026-11-11-conta-e-assinatura-design.md`

## Global Constraints

- Nível 2 de revisão (AGENTS.md): `cmd/nuvem` passa por revisão no commit exato.
- Repositório público: nenhum segredo, só dados fictícios (`ana@exemplo.com`), gitleaks no CI.
- Logs nunca levam senha, código, token, segredo, nem email completo (só `a***@exemplo.com`).
- Nunca guardar CPF/RG/dado bancário no nosso banco.
- Senha: mínimo 10, máximo 128 caracteres; recusar as comuns; Argon2id com pelo menos 19 MiB, 2 passadas, 1 fio.
- Código: 6 dígitos, `crypto/rand`, guardado como HMAC-SHA256 com `APPLYER_SEGREDO_CODIGOS`, vale 15 min, 5 tentativas, reenviar só após 60 s, código novo invalida o anterior.
- Bloqueio de login: 10 senhas erradas seguidas → 15 min de espera para aquele email.
- Token de acesso: JWT EdDSA, 15 min. Token de renovação: 32 bytes aleatórios, guardado como SHA-256, uso único, reutilização revoga a família, sessão parada expira em 60 dias.
- Licença: Ed25519, `valida_ate = emitida_em + 30 min`; `past_due` vale como assinante por 3 dias.
- Teste: 24 h a partir da primeira licença com email confirmado; um por hash de computador.
- Respostas de criar/recuperar iguais para email existente ou não: `"Se o email estiver certo, enviamos um código"`.
- Variáveis de ambiente: `DATABASE_URL`, `APPLYER_CHAVE_PRIVADA` (Ed25519 em base64), `APPLYER_SEGREDO_CODIGOS`, `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_ID`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `SMTP_HOST`, `SMTP_PORTA`, `SMTP_USUARIO`, `SMTP_SENHA`, `SMTP_REMETENTE`, `NOTAS_PROVEDOR` (`nfeio`|`falso`), `NOTAS_CHAVE`, `NOTAS_EMPRESA`, `NOTAS_SERVICO`, `APPLYER_TERMOS_VERSAO`, `APPLYER_ENDERECO` (porta, padrão `:8080`), `APPLYER_ENDERECO_PUBLICO` (ex.: `https://api.<dominio>`).
- Testes com Postgres usam `APPLYER_PG_TESTE` (DSN); sem ela o teste dá `t.Skip`. O CI ganha um job Linux com serviço `postgres:17` que define a variável.
- Nenhum teste fala com Stripe, Google, SMTP ou emissor de verdade.

## Review Focus

- Email com maiúsculas/espaços (`" Ana@Exemplo.com "`) tem de ser a mesma conta que `ana@exemplo.com` → Task 4 `TestEmailNormalizado`.
- Dois pedidos de renovação com o mesmo token ao mesmo tempo (app aberto em duas janelas) → só um vence, e o outro revoga a família; o teste prova que não há dois pares válidos → Task 5 `TestRenovarConcorrente`.
- Webhook do Stripe repetido ou fora de ordem (`subscription.updated` chegando antes do `checkout.session.completed`) → estado final correto e idempotente → Task 9 `TestWebhookForaDeOrdem`.
- Pessoa entra com Google usando o email de uma conta de senha ainda NÃO confirmada → a conta é ligada e passa a confirmada, sem criar duas → Task 7 `TestGoogleLigaContaNaoConfirmada`.
- Excluir a conta e criar de novo no mesmo PC → teste já vencido → Task 11 `TestExcluirERecriarSemNovoTeste`.

---

### Task 1: Licença compartilhada (`internal/licenca`)

**Files:**
- Create: `internal/licenca/licenca.go`
- Test: `internal/licenca/licenca_test.go`

**Interfaces:**
- Produces:
  - `type Estado string` com `Teste = "teste"`, `Assinante = "assinante"`, `Bloqueado = "bloqueado"`
  - `type Licenca struct { Usuario, Email string; Estado Estado; TesteFim, PeriodoFim *time.Time; EmitidaEm, ValidaAte time.Time }` (JSON: `usuario, email, estado, teste_fim, periodo_fim, emitida_em, valida_ate`)
  - `type Documento struct { Dados string `json:"dados"`; Assinatura string `json:"assinatura"` }` — `Dados` = base64 do JSON da `Licenca`, `Assinatura` = base64 da assinatura Ed25519 dos bytes decodificados de `Dados`
  - `func Assinar(priv ed25519.PrivateKey, l Licenca) (Documento, error)`
  - `func Conferir(pub ed25519.PublicKey, d Documento, agora time.Time) (Licenca, error)` — erro `ErrAssinatura` ou `ErrVencida` (agora > ValidaAte)
  - `func (l Licenca) PodeAgir() bool` — Estado ∈ {Teste, Assinante}

- [ ] **Step 1: Testes** `TestAssinarEConferir` (ida e volta igual), `TestConferirRecusaDadosAlterados` (troca um byte de `Dados` → `ErrAssinatura`), `TestConferirRecusaOutraChave`, `TestConferirVencida` (agora = ValidaAte+1s → `ErrVencida`), `TestPodeAgir` (tabela dos 3 estados).
- [ ] **Step 2:** `go test ./internal/licenca/` → FAIL (pacote não existe).
- [ ] **Step 3:** Implementar os tipos e funções acima. Só biblioteca padrão.
- [ ] **Step 4:** `go test ./internal/licenca/` → PASS.
- [ ] **Step 5:** Commit `Licença assinada compartilhada entre a nuvem e o app`.

### Task 2: Esqueleto da nuvem, banco e migração

**Files:**
- Create: `cmd/nuvem/main.go`, `internal/nuvem/config.go`, `internal/nuvem/servidor.go`, `internal/nuvem/banco/banco.go`, `internal/nuvem/banco/migracoes/00001_contas.sql`, `internal/nuvem/pgteste/pgteste.go` (ajuda de teste importada pelos testes dos outros pacotes)
- Modify: `.github/workflows/ci.yml` (job `nuvem` em ubuntu com `services: postgres:17`, `APPLYER_PG_TESTE`)
- Test: `internal/nuvem/banco/banco_test.go`, `internal/nuvem/servidor_test.go`

**Interfaces:**
- Produces:
  - `type Config struct{...}` com um campo por variável da lista global; `func LerConfig(env func(string) string) (Config, error)` — erro nomeando a variável que falta (nunca o valor).
  - `func banco.Abrir(ctx context.Context, dsn string) (*pgxpool.Pool, error)` — abre e roda as migrações goose (embed `migracoes/*.sql`, esquema `applyer`).
  - `func pgteste.Banco(t *testing.T) *pgxpool.Pool` — `t.Skip` sem `APPLYER_PG_TESTE`; cria um esquema temporário por teste e apaga no `t.Cleanup`.
  - `type Servidor struct{...}`; `func Novo(cfg Config, dep Dependencias) *Servidor`; `func (s *Servidor) Handler() http.Handler`.
  - `type Dependencias struct { Banco *pgxpool.Pool; Agora func() time.Time; Aleatorio io.Reader; Emails emails.Enviador; Google google.Trocador; Stripe pagamento.Stripe; Notas notas.Emissor }` (os tipos chegam nas tasks seguintes; nesta task só `Banco`, `Agora`, `Aleatorio`).
  - `GET /v1/saude` → `200 {"ok": true}`.
- Migração `00001_contas.sql`: as tabelas do desenho (`usuarios`, `codigos`, `sessoes`, `computadores_teste`, `assinaturas`, `eventos_stripe`, `notas`) com `email citext unique` (`create extension if not exists citext`), `google_sub text unique`, índices em `sessoes(familia)`, `sessoes(hash_renovacao) unique`, `codigos(email, tipo)`.

- [ ] **Step 1:** Testes `TestLerConfigFaltando` (sem `DATABASE_URL` → erro contém "DATABASE_URL"), `TestMigracoesSobem` (pgteste; consulta `information_schema.tables` e acha as 7 tabelas), `TestSaude` (httptest → 200).
- [ ] **Step 2:** Rodar → FAIL.
- [ ] **Step 3:** Implementar; `cmd/nuvem/main.go` lê config, abre banco, sobe `http.Server` com `ReadHeaderTimeout: 10s`, corpo limitado a 64 KiB por rota (exceto webhook: 1 MiB), desliga com sinal.
- [ ] **Step 4:** `go test ./internal/nuvem/... ./internal/licenca/` → PASS (com `APPLYER_PG_TESTE` definido no CI).
- [ ] **Step 5:** Commit.

### Task 3: Senhas e limites de tentativa

**Files:**
- Create: `internal/nuvem/senhas/senhas.go`, `internal/nuvem/senhas/comuns.txt` (embed; as 10 000 senhas mais comuns de uma lista pública de domínio público, ex. SecLists `10k-most-common.txt`), `internal/nuvem/limites/limites.go`
- Test: `internal/nuvem/senhas/senhas_test.go`, `internal/nuvem/limites/limites_test.go`

**Interfaces:**
- Produces:
  - `func senhas.Validar(s string) error` — `ErrCurta` (<10 runas), `ErrLonga` (>128), `ErrComum`
  - `func senhas.Resumir(s string) (string, error)` — formato PHC `$argon2id$v=19$m=19456,t=2,p=1$<sal>$<hash>`, sal de 16 bytes
  - `func senhas.Conferir(resumo, s string) (bool, error)` — comparação em tempo constante; lê parâmetros do resumo
  - `type limites.Janela struct`; `func limites.Nova(max int, janela time.Duration, agora func() time.Time) *Janela`; `func (j *Janela) Permitir(chave string) bool` — em memória, com limpeza periódica

- [ ] **Step 1:** Testes: `TestValidar` (tabela: 9 chars → ErrCurta; 129 → ErrLonga; `"password123"` → ErrComum; `"cavalo bateria grampo"` → nil), `TestResumirEConferir`, `TestConferirSenhaErrada`, `TestResumoTemParametros` (prefixo `$argon2id$v=19$m=19456,t=2,p=1$`), `TestJanela` (max 3 em 1 min: 4ª nega; após 61 s volta a permitir).
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 4: Códigos por email e envio de email

**Files:**
- Create: `internal/nuvem/codigos/codigos.go`, `internal/nuvem/emails/emails.go`, `internal/nuvem/emails/smtp.go`, `internal/nuvem/emails/falso.go`, `internal/nuvem/emails/modelos.go`, `internal/nuvem/contas/email.go`
- Test: `internal/nuvem/codigos/codigos_test.go`, `internal/nuvem/emails/emails_test.go`, `internal/nuvem/contas/email_test.go`

**Interfaces:**
- Produces:
  - `func contas.NormalizarEmail(s string) (string, error)` — `strings.TrimSpace` + minúsculas; recusa sem `@` ou com espaço interno; máx. 254
  - `type codigos.Tipo string` (`Confirmar = "confirmar"`, `Recuperar = "recuperar"`)
  - `type codigos.Repo struct{...}`; `func codigos.Novo(db *pgxpool.Pool, segredo []byte, agora func() time.Time, aleatorio io.Reader) *Repo`
  - `func (r *Repo) Gerar(ctx, email string, t Tipo) (codigo string, err error)` — `ErrEspere` se o último do mesmo tipo tem menos de 60 s; invalida os anteriores
  - `func (r *Repo) Conferir(ctx, email string, t Tipo, codigo string) error` — `ErrCodigoInvalido` (errado, vencido, usado, sem código) ou `ErrTentativas` (6ª tentativa); marca `usado_em` no acerto
  - `type emails.Mensagem struct { Para, Assunto, Texto string }`; `type emails.Enviador interface { Enviar(ctx context.Context, m Mensagem) error }`
  - `emails.SMTP{Host, Porta, Usuario, Senha, Remetente string}` (STARTTLS obrigatório; recusa conexão sem TLS)
  - `emails.Falso` com `Enviadas []Mensagem` (seguro para uso concorrente)
  - `func emails.Codigo(para, codigo string, t codigos.Tipo) Mensagem`, `func emails.JaTemConta(para string) Mensagem`, `func emails.SenhaTrocada(para string) Mensagem` — texto simples em pt-BR; o código em linha própria; frase "Nós nunca pedimos sua senha por email."

- [ ] **Step 1:** Testes: `TestEmailNormalizado` (`" Ana@Exemplo.com "` → `ana@exemplo.com`), `TestGerarSeisDigitos` (regex `^\d{6}$`), `TestCodigoGuardadoSoComoHMAC` (o texto do código não aparece na linha do banco), `TestConferirCerto`, `TestConferirVencido` (relógio +15m1s), `TestQuintaTentativaAindaValeSextaNao`, `TestEspereSessentaSegundos`, `TestCodigoNovoInvalidaAnterior`, `TestModeloCodigo` (assunto e texto contêm o código e a frase de segurança), `TestSMTPExigeTLS` (servidor SMTP de teste sem STARTTLS → erro).
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 5: Sessões e tokens

**Files:**
- Create: `internal/nuvem/sessoes/sessoes.go`
- Test: `internal/nuvem/sessoes/sessoes_test.go`

**Interfaces:**
- Produces:
  - `type Par struct { Acesso string `json:"acesso"`; Renovacao string `json:"renovacao"`; AcessoAte time.Time `json:"acesso_ate"` }`
  - `func Novo(db *pgxpool.Pool, priv ed25519.PrivateKey, agora func() time.Time, aleatorio io.Reader) *Repo`
  - `func (r *Repo) Abrir(ctx, usuarioID, dispositivoHash string) (Par, error)` — família nova
  - `func (r *Repo) Renovar(ctx, renovacao string) (Par, error)` — numa transação com `select ... for update`; token já usado → revoga a família e devolve `ErrReuso`; revogado/expirado (60 dias sem uso) → `ErrSessao`
  - `func (r *Repo) Sair(ctx, renovacao string) error`; `func (r *Repo) RevogarTodas(ctx, usuarioID string) error`
  - `func (r *Repo) ConferirAcesso(token string) (usuarioID string, err error)` — JWT `alg=EdDSA`, `sub`, `exp` = 15 min, `iss = "applyer-nuvem"`; recusa qualquer outro `alg`
  - `func Middleware(r *Repo, next func(w http.ResponseWriter, req *http.Request, usuarioID string)) http.Handler` — lê `Authorization: Bearer`, 401 `{"erro":"sessão inválida"}`

- [ ] **Step 1:** Testes: `TestAbrirERenovar` (renovação devolve par novo; o antigo não vale mais), `TestReusoRevogaFamilia` (usa o antigo de novo → `ErrReuso`; o par novo também passa a falhar), `TestRenovarConcorrente` (20 goroutines com o mesmo token → exatamente 1 sucesso), `TestSessaoParadaExpira` (relógio +60 dias +1 s → `ErrSessao`), `TestAcessoExpira` (+15m1s → erro), `TestAcessoRecusaAlgNone`, `TestRevogarTodas`, `TestSoHashNoBanco` (o token não aparece na tabela).
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 6: Rotas de conta por email e senha

**Files:**
- Create: `internal/nuvem/contas/contas.go` (repo de `usuarios`), `internal/nuvem/rotas_contas.go`
- Modify: `internal/nuvem/servidor.go`
- Test: `internal/nuvem/rotas_contas_test.go`

**Interfaces:**
- Consumes: Tasks 3, 4, 5.
- Produces (todas JSON; erro sempre `{"erro": "<frase>"}`):
  - `POST /v1/contas` `{email, senha, termos_versao}` → `202 {"mensagem": "Se o email estiver certo, enviamos um código"}`. Email novo: cria usuário não confirmado + código `confirmar`. Email existente confirmado: manda `JaTemConta`, mesma resposta. Existente não confirmado: troca a senha pendente e manda código novo. `termos_versao` diferente de `APPLYER_TERMOS_VERSAO` → 400.
  - `POST /v1/contas/confirmar` `{email, codigo, dispositivo}` → `200 Par` e marca `email_confirmado_em`, `termos_aceitos_em`.
  - `POST /v1/contas/entrar` `{email, senha, dispositivo}` → `200 Par`; errado → `401 {"erro":"Email ou senha incorretos"}`; não confirmado → `403 {"erro":"Confirme seu email","motivo":"confirmar"}` + código novo; bloqueado → `429 {"erro":"Muitas tentativas. Tente de novo em 15 minutos"}`.
  - `POST /v1/contas/recuperar` `{email}` → `202` mesma frase; manda código `recuperar` só se a conta existe.
  - `POST /v1/contas/nova-senha` `{email, codigo, senha, dispositivo}` → `200 Par`; `RevogarTodas` antes de abrir a nova; manda `SenhaTrocada`.
  - `POST /v1/sessao/renovar` `{renovacao}` → `200 Par`; `POST /v1/sessao/sair` `{renovacao}` → `204`.
  - Termos: `entrar`, `confirmar` e `google` respondem `409 {"erro":"Os termos mudaram","motivo":"termos","versao":"<atual>"}` quando `termos_versao` do usuário é diferente de `APPLYER_TERMOS_VERSAO` e o corpo não traz a versão atual; com `termos_versao` atual no corpo, gravam o aceite e seguem.
  - Limites (Task 3): por IP 20/min em todas essas rotas; por email 5/15 min em `contas`, `recuperar`; `ErrEspere` → `429`.
  - `func contas.Repo.PorEmail / PorID / PorGoogle / Criar / Confirmar / TrocarSenha / RegistrarFalha / ZerarFalhas` (usadas pelas tasks 7, 8, 11).

- [ ] **Step 1:** Testes (httptest + pgteste + emails.Falso + relógio fixo): `TestCriarConfirmarEntrar`, `TestCriarComEmailExistenteNaoRevela` (corpo e status iguais aos de email novo; o Falso recebeu `JaTemConta`), `TestEntrarSenhaErrada`, `TestDezErrosBloqueiamQuinzeMinutos` (11ª → 429; +15m → volta), `TestEntrarSemConfirmarPedeCodigo`, `TestRecuperarEmailInexistenteMesmaResposta`, `TestNovaSenhaDerrubaSessoes` (renovação antiga → 401), `TestTermosVersaoErrada`, `TestTermosMudaramPedeAceite` (usuário com versão antiga: entrar sem versão → 409 motivo termos; com a versão atual → 200 e grava o aceite), `TestLimitePorIP`, `TestLogsSemSegredos` (captura o log: não contém senha, código, token nem o email completo).
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 7: Login com Google

**Files:**
- Create: `internal/nuvem/google/google.go`, `internal/nuvem/google/falso.go`, `internal/nuvem/rotas_google.go`
- Test: `internal/nuvem/google/google_test.go`, `internal/nuvem/rotas_google_test.go`

**Interfaces:**
- Produces:
  - `type Identidade struct { Sub, Email, Nome string }`
  - `type Trocador interface { Trocar(ctx context.Context, code, verifier, redirectURI string) (Identidade, error) }`
  - `google.Real{ClientID, ClientSecret string}` — `oauth2.Config.Exchange` com `oauth2.VerifierOption(verifier)`, confere o `id_token` com `go-oidc` (`issuer https://accounts.google.com`, `ClientID`), exige `email_verified = true`; `redirectURI` aceito só se casar `^http://127\.0\.0\.1:\d{1,5}/retorno$`
  - `google.Falso{Identidades map[string]Identidade}` (chave = code)
  - `POST /v1/contas/google` `{code, verifier, redirect_uri, dispositivo, termos_versao}` → `200 Par`. Ordem: por `google_sub`; senão por email (liga o sub e marca confirmada); senão cria confirmada.

- [ ] **Step 1:** Testes: `TestRedirectSoLoopback` (`http://evil.com/retorno` → erro), `TestRealConfereIDToken` (servidor OIDC de teste com chave própria emitindo id_token; aud errado → erro; `email_verified=false` → erro), `TestGoogleCriaConta`, `TestGoogleLigaPorEmail`, `TestGoogleLigaContaNaoConfirmada`, `TestGoogleMesmoSubEntraNaMesmaConta`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 8: Teste de 1 dia e licença

**Files:**
- Create: `internal/nuvem/rotas_licenca.go`, `internal/nuvem/teste.go`
- Test: `internal/nuvem/rotas_licenca_test.go`

**Interfaces:**
- Consumes: Task 1 (`licenca.Assinar`), Task 5 (`Middleware`).
- Produces:
  - `func estadoDa(u contas.Usuario, a *Assinatura, agora time.Time) licenca.Estado` — regra do desenho, nesta ordem: `cortesia` → Assinante; assinatura `active`|`trialing` → Assinante; `past_due` e `agora < periodo_fim + 72h` → Assinante; email confirmado e `agora < teste_fim` → Teste; senão Bloqueado
  - `GET /v1/licenca?dispositivo=<hash>` (autenticado) → `200 licenca.Documento`. Primeira chamada de conta confirmada sem `teste_inicio`: numa transação, se `dispositivo` não está em `computadores_teste` grava `teste_inicio=agora`, `teste_fim=agora+24h` e insere o hash; se está, grava `teste_inicio=teste_fim=agora` (teste já vencido). `dispositivo` precisa casar `^[0-9a-f]{64}$`.

- [ ] **Step 1:** Testes: `TestPrimeiraLicencaComecaTeste` (estado teste, teste_fim = agora+24h, valida_ate = agora+30m), `TestTesteAcaba` (+24h1s → bloqueado), `TestSegundaContaMesmoPCJaBloqueada`, `TestMesmaContaOutroPCNaoGanhaOutroDia`, `TestCortesia`, `TestAssinaturaAtiva`, `TestPastDueTresDias` (+72h ainda assinante; +72h1s bloqueado), `TestLicencaConfereComChavePublica`, `TestDispositivoInvalido` (400).
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 9: Stripe — assinar, portal e webhooks

**Files:**
- Create: `internal/nuvem/pagamento/stripe.go`, `internal/nuvem/pagamento/falso.go`, `internal/nuvem/rotas_pagamento.go`
- Test: `internal/nuvem/rotas_pagamento_test.go`

**Interfaces:**
- Produces:
  - `type Stripe interface { CriarCliente(ctx, email, usuarioID string) (string, error); Checkout(ctx, cliente, usuarioID string) (url string, err error); Portal(ctx, cliente string) (url string, err error); Cancelar(ctx, assinatura string) error; Evento(corpo []byte, assinaturaCabecalho string) (stripe.Event, error) }`
  - Real: Checkout `mode=subscription`, `line_items=[{price: STRIPE_PRICE_ID, quantity: 1}]`, `client_reference_id=usuarioID`, `customer=cliente`, `tax_id_collection.enabled=true`, `billing_address_collection=required`, `customer_update.name=auto`, `customer_update.address=auto`, `success_url` e `cancel_url` = `APPLYER_ENDERECO_PUBLICO + "/v1/pagamento/voltar"` (página HTML estática "Pode voltar para o Applyer"); `Evento` = `webhook.ConstructEvent` com tolerância padrão.
  - `POST /v1/assinar` (autenticado) → `200 {"url": ...}`; cria o cliente Stripe na primeira vez e grava `usuarios.stripe_cliente`.
  - `POST /v1/portal` (autenticado) → `200 {"url": ...}`; sem cliente → 409.
  - `POST /v1/stripe/webhook` → `200` sempre que a assinatura do cabeçalho confere (inclusive evento já visto); assinatura inválida → 400. Idempotência por `insert into eventos_stripe ... on conflict do nothing`. Para `customer.subscription.*` e `checkout.session.completed`, **busca a assinatura atual no Stripe** e grava esse estado (assim a ordem dos eventos não importa). `invoice.paid` chama `notas.Agendar` (Task 10).
  - `GET /v1/pagamento/voltar` → HTML simples.

- [ ] **Step 1:** Testes com `pagamento.Falso`: `TestAssinarCriaClienteUmaVez`, `TestPortalSemCliente409`, `TestWebhookAssinaturaInvalida`, `TestWebhookRepetidoIdempotente`, `TestWebhookForaDeOrdem` (updated antes do completed → estado final `active`), `TestCanceladaBloqueia` (deleted → licença bloqueado após teste vencido), `TestInvoicePaidAgendaNota`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 10: Nota fiscal automática

**Files:**
- Create: `internal/nuvem/notas/notas.go`, `internal/nuvem/notas/nfeio.go`, `internal/nuvem/notas/falso.go`, `internal/nuvem/notas/fila.go`, `internal/nuvem/rotas_notas.go`
- Test: `internal/nuvem/notas/notas_test.go`

**Interfaces:**
- Produces:
  - `type Pedido struct { InvoiceID, UsuarioID, Nome, Documento, Email, Pais string; ValorCentavos int64; Moeda string; Endereco map[string]string }` (Documento = tax id do Stripe; **não é gravado no nosso banco**, só repassado ao emissor)
  - `type Emissor interface { Emitir(ctx context.Context, p Pedido) (numero, linkPDF string, err error) }`
  - `notas.NFEio{Chave, Empresa string}` — `POST https://api.nfe.io/v1/companies/{Empresa}/serviceinvoices` com tomador, valor e descrição "Assinatura mensal Applyer"; código de serviço lido de `NOTAS_SERVICO`
  - `func (f *Fila) Agendar(ctx, invoiceID, usuarioID string) error` — insere `notas` com estado `pendente` (idempotente)
  - `func (f *Fila) Rodar(ctx)` — a cada 1 h (e logo após `Agendar`): para cada `pendente`/`falhou` com `tentativas < 24`, busca a fatura no Stripe (nome, tax id, endereço, valor), chama `Emitir`, grava `emitida` + número + link, ou `falhou` + erro (sem dados pessoais no erro)
  - `GET /v1/notas` (autenticado) → `200 [{numero, link_pdf, estado, criada_em}]`

- [ ] **Step 1:** Testes: `TestAgendarIdempotente`, `TestEmitirGravaNumero`, `TestFalhaTentaDeNovo` (Falso falha 2× → 3ª emite; tentativas = 3), `TestDesisteDepoisDe24`, `TestDocumentoNaoVaiProBanco` (nenhuma coluna contém o CPF fictício `00000000191`), `TestNFEioMontaPedido` (httptest no lugar da API: confere caminho, cabeçalho `Authorization` e corpo).
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 11: Excluir conta

**Files:**
- Create: `internal/nuvem/rotas_excluir.go`
- Test: `internal/nuvem/rotas_excluir_test.go`

**Interfaces:**
- Produces: `DELETE /v1/contas/eu` (autenticado) `{email}` (precisa casar com a conta) → `204`. Em transação: `Stripe.Cancelar` se houver assinatura ativa (antes de apagar; falha do Stripe → 502 e nada é apagado), `RevogarTodas`, apaga `codigos`, `sessoes`, `assinaturas`, `usuarios`; `computadores_teste.usuario_id = null`; `notas.usuario_id` fica (obrigação fiscal).

- [ ] **Step 1:** Testes: `TestExcluirApagaTudo`, `TestExcluirEmailErrado400`, `TestExcluirCancelaStripe`, `TestExcluirFalhaStripeNaoApaga`, `TestExcluirERecriarSemNovoTeste`.
- [ ] **Step 2:** FAIL. **Step 3:** Implementar. **Step 4:** PASS. **Step 5:** Commit.

### Task 12: Contrato da nuvem, imagem e revisão de segurança

**Files:**
- Create: `api/nuvem.yaml` (OpenAPI 3.1 de todas as rotas `/v1/...` acima), `internal/nuvem/contrato_test.go`, `deploy/nuvem/Dockerfile`
- Modify: `.github/workflows/ci.yml` (build da imagem no job `nuvem`)

**Interfaces:**
- Produces: `api/nuvem.yaml` — consumido pelo plano do app (`internal/conta`). Imagem: multi-stage, `golang:1.26` → `gcr.io/distroless/static-debian12:nonroot`, `CGO_ENABLED=0`, binário `/nuvem`, porta 8080. Publicada no GHCR (`ghcr.io/frf-tecnology/applyer-nuvem:<sha>`) pelo CI a cada push na `reescrita-go`.
- Subcomando `nuvem chaves`: gera um par Ed25519, imprime `APPLYER_CHAVE_PRIVADA=<base64>` no stdout e grava a pública (base64) no caminho de `-pub` (padrão `licenca.pub`). Teste `TestSubcomandoChaves` (a privada impressa assina e a pública gravada confere, via `internal/licenca`).

- [ ] **Step 1:** Teste `TestRotasDoServidorEstaoNoContrato` (percorre o `ServeMux` registrado e confere cada método+caminho em `api/nuvem.yaml`, no mesmo estilo de `internal/api/contrato_test.go`).
- [ ] **Step 2:** FAIL. **Step 3:** Escrever o contrato e o Dockerfile. **Step 4:** PASS; `docker build -f deploy/nuvem/Dockerfile .` no CI.
- [ ] **Step 5:** Rodar o skill `security-review` sobre `cmd/nuvem`, `internal/nuvem`, `internal/licenca`; corrigir o que for confirmado, com teste.
- [ ] **Step 6:** Commit.
