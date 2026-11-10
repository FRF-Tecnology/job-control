# Applyer — regras do time

Meta da vez: veja o topo do docs/status.md. O que não serve à meta está congelado.

## Como trabalhar
- 1 bead = 1 PR = contrato + código + testes. Nada de PR só de contrato ou só de versão. O `info.version` sobe uma vez por fase (PO).
- Branch a partir de reescrita-go, nunca empilhado.
- Abriu o PR: ligue o auto-merge (`gh pr merge --auto --squash`) e encerre o turno. Não espere nem consulte o CI em laço.
- Sessão nova por fatia. O estado mora no bead e no docs/status.md.
- Mensagem com até 80 palavras: tarefa, pronto (link do PR e uma linha) ou bloqueio. A evidência vai no PR.
- Sem confirmação de leitura e sem aviso de "estou parado".
- Nunca leia o api/openapi.yaml inteiro: busque a rota com grep ou `graphify query`.
- Regra nova nasce só no Go. O Node (nucleo/, servidor/) é só leitura; os golden atuais são retrato fixo.
- Beads: `bd` em `~/gt/job_control`, com `--actor "<seu nome>"`, sem `--force`.

## Revisão por risco
- Nível 0 (documentação, testes, texto de tela, CSS): só o CI.
- Nível 1 (código comum): CI e a lista de conferência do autor no PR.
- Nível 2 (internal/segredos, navegador, executor, envio, guarda da IA, mcp, assistente, cmd/nuvem, .github/workflows): Fallen, no commit exato.

## Pronto
- O QA viu funcionar no app compilado, com captura ou vídeo curto no PR.
- Se a jornada real quebra, a linha para: nada novo entra até consertar.

## PC do Vini
- Nenhuma janela, navegador ou app visível no PC de uso dele. Teste com janela só na máquina de teste.
- Teste de navegador sempre sem janela. Nenhum teste usa o cofre real.

## Regras do produto
- Nunca CPF, RG, senha ou dado bancário. Chave de API só no cofre.
- Nada enviado em nome da pessoa sem aprovação dela. A IA nunca aprova, envia, apaga, nem mexe em limites ou chaves.
- Nunca inventar experiência. LinkedIn com limite diário e aprovação humana.
- O repo é público: só dados fictícios.

## Mapa do código
- `internal/`: dominio, armazem (SQLite + goose em `migracoes/`), api (`/api/v1`), ia (guarda com 11 regras), segredos, navegador, executor, envio, aovivo, assistente, mcp, curriculos, legado, config.
- `cmd/`: central-cli, central-dev, central-mcp. `app/`: projeto Wails (`main.go`, `build/`, `frontend/` em React + TS + Vite).
- `api/openapi.yaml`: contrato da API local.
- Pasta de dados: `os.UserConfigDir()/Applyer`; `APPLYER_PASTA` troca a pasta (testes).

## Comandos (a partir da raiz)
- `go` e `wails3` no PATH (`go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`).
- Bindings: `cd app && wails3 generate bindings -clean=true -ts -i`. Front: `cd app/frontend && npm ci`.
- Testes Go: `go test ./...` (o pacote `app` precisa de `app/frontend/dist`).
- Telas: `cd app/frontend && npm test`; `npm run build && npm run e2e` (headless).
- App: `cd app && wails3 build` gera `app/bin/central.exe`.
- Ver no navegador: `npm run build` e `APPLYER_PASTA=<pasta de teste> go run ./cmd/central-dev` (http://localhost:4500).
