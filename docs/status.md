# Status do projeto: sync para o PO

Atualizado em 2026-11-08. Escrito para o PO continuar o trabalho a partir daqui.

## Resumo

O **Applyer** (antes Central de Candidaturas) está virando um **app de PC vendável**: Go no back, React + TS no front, Wails v3 por cima.
A reescrita fica no branch `reescrita-go`, separado do `main`.

**Fases feitas:**
- **P0:** base;
- **P1:** domínio;
- **P2:** banco e API;
- **P3:** telas;
- **P4:** IA.

O CI está verde no último commit (`8a7ce37`), com 7 commits à frente do `main`. Esse branch só vira PR para o `main` na fase P8.

**Próximas fases:**
- **P5:** navegador e candidatura automática;
- **P7:** nuvem pequena;
- **P6:** emails pelo navegador, MCP e CLI;
- **P8:** instaladores, remoção do Node e merge.

P5 e P7 podem começar agora e não dependem uma da outra.

## Para onde vamos (decisões do Vinicius, não reabrir)

- **Produto:** app de PC para Windows, Mac e Linux. Público: devs BR/LATAM procurando vaga.
  - Prioridade das vagas: primeiro internacionais em USD (empresas dos EUA contratando na LATAM), depois LATAM + Europa, depois nacionais.
  - Celular e sincronização ficam para depois, como opcional.
- **Por que app e não site:** duas coisas só funcionam no PC da pessoa:
  - usar a assinatura de IA dela (CLIs oficiais);
  - candidatar no navegador logado dela.
  - Por isso, nada de versão web.
- **Stack:**
  - Go (módulo `github.com/FRF-Tecnology/job-control`) e Wails v3 (projeto em `app/`);
  - React 18 + TS + Vite (`app/frontend/`);
  - SQLite local (`modernc.org/sqlite`, sem cgo, migrações goose). **Não é Postgres:** o Vinicius cogitou e voltou atrás. Postgres só na nuvem (P7).
- **Dados:** ficam no PC da pessoa (`UserConfigDir/Central de Candidaturas`, ou `CENTRAL_PASTA`). Os nomes internos ainda usam "central" e só mudam com o plano do `jc-etv.6`.
- **IA da pessoa, igual ao OpenBot**, por três caminhos:
  1. **Assinatura:** Claude Code, Codex ou Gemini CLI instalados e logados no PC;
  2. **Chave própria:** OpenAI, Anthropic, Google ou endereço compatível. A chave fica no cofre do sistema e a tela só mostra os 4 últimos caracteres;
  3. **Modelos gratuitos:** OpenRouter pela nuvem do produto, com cota. Depende da P7.
- **Login:** Google + link por email, só na nuvem (P7). A **infraestrutura é do Vinicius**: hospedagem, domínio, Postgres e certificados. O código só lê variáveis de ambiente.
- **Design:** Apple + america.gov.
  - uma caixa "O que vamos fazer hoje?" como porta de entrada;
  - tipografia grande e muito espaço;
  - uma cor de destaque;
  - modo escuro;
  - 375 px no celular.
- **Ferramentas obrigatórias no código:**
  - Gas Town: beads `jc-*`, rig `job_control` em `~/gt`;
  - Graphify: `graphify query` antes de ler código e `graphify update .` depois de mudar;
  - RTK.

Atenção: o `docs/backlog.md` ainda descreve a visão antiga (site + PWA + extensão do Chrome).
**A direção que vale é a deste documento e do épico `jc-a2y`.** As ideias de produto do backlog continuam valendo:
- **T3 Entrevistas:** espera uma skill de entrevistas que o Vinicius vai mandar;
- **T4:** histórico do LinkedIn;
- **T2:** recrutadores;
- **T5:** aprendizados.

No novo plano, elas entram depois da P8 ou quando o Vinicius pedir.

## Onde estamos (épico `jc-a2y` no Gas Town)

| Bead | Fase | Estado | O que entregou |
|---|---|---|---|
| jc-a2y.1 | P0 Base | ✓ | módulo Go, esqueleto Wails, SQLite + goose, `api/openapi.yaml`, CI (Node, app no Windows, gitleaks) |
| jc-a2y.2 | P1 Domínio | ✓ | `internal/dominio`: porte 1:1 do `nucleo/` Node; 307 casos golden iguais |
| jc-a2y.3 | P2 Armazém + API | ✓ | `internal/armazem` (SQLite, backup diário), `internal/api` (`/api/v1` completa, 94 chamadas golden iguais), `central-cli importar-legado` |
| jc-a2y.4 | P3 Telas | ✓ | 9 telas em React (Hoje, Triagem, Quadro, Vagas, Números, Plataformas, Rodadas, Perfil, Detalhe), Vitest + Playwright (PC e Pixel 7) |
| jc-a2y.5 | P4 IA | ✓ | `internal/ia` (prompts byte a byte iguais ao Node, guarda com 10 regras, roteador com reserva), `internal/segredos` (cofre), tela Conectar IA, "Opinião da IA" na triagem, rascunho de follow-up |
| jc-a2y.6 | P5 Navegador | ○ pronta | — |
| jc-a2y.7 | P6 Emails, MCP, CLI | ○ espera P5 | — |
| jc-a2y.8 | P7 Nuvem | ○ pronta | — |
| jc-a2y.9 | P8 Distribuição | ○ espera tudo | — |

**Como a paridade é garantida:** o Node (`nucleo/`, `servidor/`) é a referência até a P8.
- Os scripts `ferramentas/gerar-golden-*.mjs` gravam o que o Node responde.
- O Go precisa bater igual.
- O `testes/golden.test.mjs` falha se as fixtures ficarem velhas.
- Os dados reais do Vinicius, importados numa cópia, deram respostas idênticas nas 24 rotas.

**Qualidade da IA:** avaliação real pela assinatura do Claude (Haiku, esforço baixo), 15 casos:
- 93% de acerto;
- 0 violações graves.

### Problemas conhecidos (abertos)

1. **A triagem pela CLI leva 17–25 s por vaga.** Ideia: mandar várias vagas por chamada.
2. **Codex e Gemini CLI não foram testados de verdade.** Não estão instalados; a tela os marca como "não testado".
3. **O rascunho de mensagem para recrutador saiu longo** (955 caracteres) e citou uma tecnologia fora do perfil. Ajustar o prompt ou a guarda.
4. **Modelos gratuitos** só funcionam depois da P7.
5. **O PWA ficou de fora** da versão nova (decisão consciente: o produto é app de PC).
6. **Antes do lançamento:** revisar os termos de uso da Anthropic, da OpenAI e do Google sobre usar a assinatura por um app de terceiros. Cada CLI já tem chave liga/desliga.

## Próximos passos sugeridos

### P5: Navegador (`internal/navegador`)

Pronta quando houver uma candidatura de teste de ponta a ponta numa vaga real escolhida pelo Vinicius.

- **Chrome:** chromedp com um perfil próprio do app. A pessoa faz login no LinkedIn e na Gupy uma vez, ela mesma.
- **Busca:** reaproveitar os scripts de `automacao/` (LinkedIn, Gupy, InHire) e o Easy Apply.
- **Testes:** com páginas salvas (fixtures), porque os sites mudam.
- **Regras:**
  - no máximo 30–40 candidaturas por dia no LinkedIn;
  - a pessoa aprova cada envio;
  - CPF, senha e captcha: o app para e avisa.
- **Divisão:**
  - back: navegador, fila, limites e rotas na API;
  - front: fila de aprovação e status ao vivo.

### P7: Nuvem pequena (`cmd/nuvem`)

Pronta quando for possível entrar e usar um modelo grátis com cota.

- **Login:** Google (OIDC + PKCE) e link por email (token de uso único, 15 min).
- **Também:** licença e proxy do OpenRouter com cota por conta.
- **Infra:** Postgres por variável de ambiente; a infra é do Vinicius.
- **No app:** o token da conta fica no cofre do sistema.
- **Divisão:**
  - back: `cmd/nuvem` + cliente no app;
  - front: tela de entrar e o caminho "modelos gratuitos" na Conectar IA (hoje está "em breve").

### P6: Emails, MCP e CLI (depois da P5)

- **Emails:** pelo Gmail logado no navegador do app, só leitura.
- **MCP:** servidor local com as 18 ferramentas, chamando a API Go.
- **CLI:** `.claude/commands/*.md` passam a chamar o executável Go.

### P8: Distribuição (no fim)

- instaladores (goreleaser);
- assinatura de código (certificado do Vinicius);
- atualização automática;
- remover o Node;
- PR do `reescrita-go` para o `main`.

**Decisões que o PO deve levar ao Vinicius:** a ordem P5 × P7 (ele ainda não escolheu) e a vaga real para o teste da P5.

## Mapa do código (branch `reescrita-go`)

```
app/                 Wails: main.go (rota /api/ → API Go; resto → frontend/dist), frontend/ (React + TS)
cmd/central-cli/     importar-legado, avaliar
cmd/central-dev/     servidor de dev em 127.0.0.1:4500 (usado pelo Playwright)
internal/config      pasta de dados
internal/armazem     SQLite, migrações, backup, uso de IA
internal/dominio     regras de negócio (porte do nucleo/)
internal/api         /api/v1 independente de transporte + Handler HTTP (checa Host/Sec-Fetch-Site)
internal/ia          funções de IA, guarda, provedores (anthropic, openai, google, claude-cli, codex-cli, gemini-cli)
internal/segredos    cofre do sistema (go-keyring)
internal/legado      importação dos dados/*.json antigos (recusa CPF/RG)
api/openapi.yaml     contrato (v0.2.0) — rota nova entra aqui primeiro
nucleo/, servidor/   Node: referência até a P8
```

Os comandos de build, teste e dev estão no `CLAUDE.md`, nas seções sobre a reescrita, as telas e a IA.

**Fluxo de trabalho:**
- **Bead antes de começar.** Confira com `bd list` e `bd ready`, rodando em `~/gt/job_control`.
- **Contrato primeiro.** Mudança de rota passa antes pelo `openapi.yaml`.
- **Golden em dia.** Se mudar algo do Node de referência, gere a golden de novo.
- **Pronto quando:**
  - `go test ./...`, `npm test`, Vitest e Playwright passam;
  - claro e escuro estão conferidos, e o layout cabe em 375 px;
  - o texto está em pt-BR.
- **Antes do PR:** rode `graphify update .`; o gitleaks roda no CI.
- **PR e merge:** o PO tem autonomia para aprovar e fazer merge, desde que o CI esteja verde e o critério de aceite tenha sido conferido.

## Regras do produto (sempre)

- Nunca guardar ou preencher CPF, RG, senha ou dados bancários.
- Nunca criar conta, logar com senha ou resolver captcha pela pessoa.
- Chave de API só no cofre do sistema, nunca em banco, arquivo ou log.
- Nada enviado em nome da pessoa sem aprovação: candidatura, mensagem para recrutador, follow-up.
- Nunca inventar experiência no currículo nem nas respostas.
- Emails: só leitura. Nunca responder, arquivar, apagar ou marcar como lido.
- LinkedIn: no máximo 30–40 candidaturas por dia, com aprovação humana.
- O repo é **público**: só dados fictícios nos testes, `dados/` fora do git, e nenhum email ou dado pessoal no diff.
