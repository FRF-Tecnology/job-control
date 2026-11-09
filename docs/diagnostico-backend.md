# Diagnóstico do back-end: sync de 2026-11-08

Feito pelo agente de back-end a partir do `reescrita-go` em `c62b89a`. Só documentação: nenhum código, contrato ou migração mudou.

## Ambiente e testes

Passos iguais aos do CI (`.github/workflows/ci.yml`, job `app`), no Windows 11:

| Item | Versão ou resultado |
|---|---|
| Go | 1.27.3 windows/amd64 |
| Node / npm | 24.20.0 / 11.19.0 |
| wails3 | v3.1.0 |
| `wails3 generate bindings -clean=true -ts -i` | ok |
| `npm ci && npm run build` (app/frontend) | ok |
| Vitest (app/frontend) | 125 de 125 |
| `gofmt -l` | sem arquivos |
| `go vet ./...` | limpo |
| `go test ./...` | ok: app, api, armazem, dominio, ia, legado |
| `npm test` (raiz, Node de referência + golden) | 144 de 144 |

Observações:
- Sem os bindings gerados, `npm run build` falha (`Event bindings module not found`), e o pacote `app` não compila sem `app/frontend/dist`. Gere os bindings antes do build, como o CI faz.
- O npm 11 avisa que o `postinstall` do esbuild não está em `allowScripts`. O build passou mesmo assim.
- `internal/segredos` não tem testes.
- Playwright (e2e) não rodou neste diagnóstico.

## Lacuna no contrato: rotas de IA fora do `openapi.yaml`

`api/openapi.yaml` (0.2.0) não lista nenhuma rota de IA. Estas existem em `internal/api/ia.go` e são usadas pela tela Conectar IA:

| Método | Rota |
|---|---|
| GET | `/ia` |
| GET | `/ia/config` |
| PUT | `/ia/config` |
| PUT | `/ia/chaves/{provedor}` |
| DELETE | `/ia/chaves/{provedor}` |
| POST | `/ia/testar` |
| POST | `/ia/triar/{vagaId}` |
| POST | `/ia/mensagem/{vagaId}` |

O contrato não foi editado neste PR. Proposta: uma tarefa própria que documenta as 8 rotas (com 422 e 502) e um teste que compara as rotas do `Handler` com o `openapi.yaml`. Precisa de revisão do CyberSec, porque toca IA e segredos.

## P5 em tarefas pequenas (proposta, nada implementado)

Contrato e segurança vêm primeiro. Rotas, navegador e segredos passam pela revisão do CyberSec, e um achado crítico bloqueia o merge.

| # | Tarefa | Depende de | Aceite |
|---|---|---|---|
| P5.0 | Contrato: rotas de IA no `openapi.yaml` + teste contrato × rotas | — | teste falha se faltar rota; CyberSec revisou |
| P5.1 | Contrato P5: `/navegador/estado`, `/navegador/abrir` (a pessoa faz login), `/fila`, `/fila/{id}/aprovar`, `/fila/{id}/recusar` | P5.0 | rotas no `openapi.yaml` antes do código; front de acordo |
| P5.2 | `internal/navegador`: chromedp com perfil próprio na pasta de dados; abrir, fechar, ler página | — | teste com página salva local; pula sem Chrome; nada de senha guardada |
| P5.3 | Migração nova: fila de candidaturas e contador diário por plataforma | — | migração goose nova; transação, versão sobe; testes no armazém |
| P5.4 | Domínio puro da fila: estados, aprovação obrigatória, limite de 30–40 por dia no LinkedIn | P5.3 | testes de tabela; não altera o estado original |
| P5.5 | Detecção de parada: CPF, RG, senha, captcha, login → vaga `bloqueada` com pendência | P5.2 | páginas salvas fictícias para cada caso |
| P5.6 | Busca: scripts de `automacao/` (LinkedIn, Gupy, InHire) pelo chromedp → `/vagas/importar` (Descoberta) | P5.2 | páginas salvas; não duplica |
| P5.7 | Candidatura Easy Apply e Gupy só após aprovação | P5.4, P5.5, P5.6 | página salva ponta a ponta sem envio real; para antes de CPF |
| P5.8 | Rotas da P5 na API + status ao vivo para a fila | P5.1, P5.4 | testes 400/404/409; mesma origem; CyberSec revisou |
| P5.9 | Candidatura real numa vaga escolhida pelo Vinicius | todas | aceite do bead `jc-a2y.6` |

Fora do escopo deste agente: `cmd/nuvem` (P7) é do Infra. Este agente cuida do cliente local e do contrato da API.
