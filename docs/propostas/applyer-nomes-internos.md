# Applyer: plano de migração dos nomes internos

Bead `jc-etv.6` (épico `jc-etv`). Status: **proposta, nada executado**. O Vinicius decide antes da P8, a primeira distribuição.

O nome visível (janela, telas, metadados do executável) muda no `jc-a2y.61` e no `jc-etv.2`. Este plano cobre só os nomes internos. Mudar um nome interno sem migração faz o app abrir vazio, perder as chaves de IA ou quebrar quem já usa.

## Resumo e recomendação

Hoje só o Vinicius usa o app novo, e ele ainda não foi distribuído. As trocas de identidade (bundle id, MSIX, nome do executável) **custam quase nada antes da primeira distribuição e muito depois**. A pasta de dados e o cofre precisam de código de migração com testes, porque já guardam dados reais.

| # | Nome interno | Hoje | Proposta | Quando |
|---|---|---|---|---|
| 1 | Pasta de dados | `Central de Candidaturas` (em `os.UserConfigDir()`) | `Applyer`, migrada ao abrir | Antes da P8 |
| 2 | Variável da pasta | `CENTRAL_PASTA` | `APPLYER_PASTA`, aceitando a antiga | Junto com o 1 |
| 3 | Cofre das chaves de IA | serviço `Central de Candidaturas` | serviço `Applyer`, migrada na primeira leitura | Junto com o 1 |
| 4 | Identidade do app | `com.frftecnology.central` (bundle id, MSIX) | `com.frftecnology.applyer` | Antes da P8, nunca depois |
| 5 | Executáveis | `central.exe`, `cmd/central-cli`, `cmd/central-dev` | `applyer.exe`, `cmd/applyer-cli`, `cmd/applyer-dev` | Antes da P8 |
| 6 | Banco | `central.db` | **não muda** (ver abaixo) | — |
| 7 | Módulo Go e repositório | `github.com/FRF-Tecnology/job-control` | `github.com/FRF-Tecnology/applyer` | Por último, num PR só |
| 8 | Variáveis de desenvolvimento | `CENTRAL_TELAS`, `CENTRAL_NODE`, `CENTRAL_CLI_FALSA*`, `CENTRAL_HOJE` | `APPLYER_*`, sem compatibilidade | Junto com o 5 |
| 9 | Central Node (v1) | `CENTRAL_DADOS`, `dados/`, MCP `central`, `ferramentas/cli.mjs` | **não muda**: sai na P8 | — |

**Não mudam:** o prefixo dos beads (`jc`), o rig `job_control` do Gas Town, as rotas `/api/v1/...`, os nomes dos pacotes Go (`internal/...`) e os textos dos logs.

## 1. Pasta de dados: `Central de Candidaturas` → `Applyer`

Onde fica hoje: `%APPDATA%\Central de Candidaturas` (Windows), `~/Library/Application Support/Central de Candidaturas` (macOS) e `~/.config/Central de Candidaturas` (Linux). Guarda o `central.db`, os `curriculos/`, os `backups/` e o perfil do navegador (`navegador/`, com a sessão das plataformas).

**Migração, em `config.PastaDados`, ao abrir:**
1. Se `Applyer` existe, usa `Applyer`, e acabou.
2. Se só `Central de Candidaturas` existe, renomeia a pasta inteira (`os.Rename`, na mesma pasta-mãe, então no mesmo disco e de uma vez).
   - **Não copia e apaga.** Se o rename falhar (outro processo com o banco aberto: o app antigo, a CLI, o MCP), o app usa a pasta antiga nesta vez, avisa na tela e tenta de novo na próxima abertura. Nunca fica meio migrado.
   - O rename mantém o dono e a DACL protegida do `jc-a2y.58`. Depois, o `ProtegerPasta` confere de novo (dono e DACL).
   - O perfil do navegador vai junto. A chave dos cookies fica no `Local State`, cifrada para o usuário (DPAPI), e não depende do caminho do perfil. **Isso ainda não foi conferido**, nem com a cifra ligada ao app do Chrome 127 ou mais novo. O teste da migração confere: perfil movido, sessão fictícia ainda válida. Se falhar, o pior caso é a pessoa entrar de novo nas plataformas ("Entrar nas plataformas"); nenhum dado se perde.
3. Se as duas existem, por exemplo depois de um teste, usa `Applyer` e **não junta nada sozinho**. A tela avisa que a antiga continua lá, para a pessoa decidir.
4. Um evento na linha do tempo registra "pasta migrada", sem caminho.

**Volta atrás:** renomear `Applyer` para `Central de Candidaturas` com o app fechado. A versão antiga do app abre normalmente, porque o formato do banco não muda.

**Testes:** pasta temporária com as quatro situações (nova, antiga, as duas, nenhuma); rename recusado com o banco aberto por outro processo; no Windows, DACL e dono iguais depois do rename; `TestCentralDevNuncaTrazOExecutor` e o e2e com a pasta vazia continuam passando.

## 2. `CENTRAL_PASTA` → `APPLYER_PASTA`

- `APPLYER_PASTA` passa a ser a variável.
- `CENTRAL_PASTA` continua aceita por pelo menos duas versões. O log de partida diz `pasta: CENTRAL_PASTA (obsoleta)`, sem o caminho (`jc-a2y.57`).
- **As duas definidas e diferentes:** o app não abre, com a mensagem "defina só APPLYER_PASTA". Falha fechada, porque abrir a pasta errada seria pior.
- As duas iguais: usa e segue.
- Documentos, roteiros e testes passam a usar `APPLYER_PASTA`. O `docs/roteiro-6-5.md` só muda depois da `.6.5`.

## 3. Cofre das chaves de IA: serviço `Central de Candidaturas` → `Applyer`

O `internal/segredos` grava cada chave no cofre do sistema (Gerenciador de Credenciais, Keychain, Secret Service) com o serviço `Central de Candidaturas` e o provedor como conta.

**Migração, na primeira leitura de cada provedor:**
1. Lê do serviço `Applyer`. Se achou, usa.
2. Se não achou, lê do serviço antigo. Se achou, grava no `Applyer`, lê de volta para conferir e só então apaga a antiga. Falhou em qualquer passo: usa a antiga e tenta na próxima vez.
3. A chave nunca vai para log, banco, resposta nem mensagem de erro (só os 4 últimos caracteres, como hoje).
4. "Apagar meus dados" apaga dos **dois** serviços enquanto a compatibilidade existir.

**Testes:** com o `segredos.Memoria` (fictício), nas situações só antiga, só nova, as duas e falha na gravação. O teste confere que a chave não aparece no log. **Revisão do CyberSec obrigatória.**

> Hoje o `internal/segredos` não tem teste próprio. Proposto ao PO como bead pequeno. É bom que ele entre antes desta migração.

## 4. Identidade do app: `com.frftecnology.central` → `com.frftecnology.applyer`

- **macOS:** o bundle id é a identidade do `.app`. Trocar perde o armazenamento do WebKit (hoje, só a preferência de tema) e as permissões que o sistema deu ao id antigo. O cofre não muda, porque o go-keyring usa o nome do serviço, não o bundle id.
- **Windows (MSIX):** o `Identity Name` é o app. Com outro nome, o Windows trata como outro app, sem atualização em cima do antigo.
- **Recomendação:** trocar **antes da primeira distribuição** e nunca mais. Depois que alguém instalar, fica `com.frftecnology.central` para sempre, e só o nome visível muda.

## 5. Executáveis: `central.exe`, `central-cli`, `central-dev`

- `APP_NAME: "central"` vira `"applyer"` em `app/Taskfile.yml`. Os `cmd/central-cli` e `cmd/central-dev` viram `cmd/applyer-cli` e `cmd/applyer-dev`.
- **WebView2:** guarda o cache por nome do executável. Trocar perde só o cache e a preferência de tema. Os dados do app ficam no SQLite, não ali.
- **O que muda junto (21 arquivos hoje):** `ci.yml`, `vite.config.ts`, `playwright.config.ts` (`go run ./cmd/central-dev`), `CLAUDE.md`, `docs/status.md`, os manifestos de build, o MSIX e o `docs/roteiro-6-5.md` (o `go version -m app\bin\central.exe`).
- Sem compatibilidade: são ferramentas de desenvolvimento, e ninguém tem atalho para o `central.exe` além do Vinicius.

## 6. `central.db`: não muda

O nome do arquivo não aparece para a pessoa, e mudar só daria risco. "Apagar meus dados", os backups e o importador conhecem o nome. Fica `central.db` dentro da pasta `Applyer`.

## 7. Módulo Go e repositório

- O GitHub redireciona o repositório renomeado (git, web e `go get`). Mesmo assim, é bom trocar o remoto de cada clone (`git remote set-url`).
- O módulo vira `github.com/FRF-Tecnology/applyer`: o `go.mod` e os imports de 49 arquivos `.go`, uma troca mecânica num PR só, com `gofmt` e `go test ./...`.
- Por último, para não conflitar com PRs abertos. Combinar uma janela sem PR aberto. O Gas Town (`jc`, `job_control`) não muda.
- **Kauai:** `ci.yml` e badges. **Todos:** os clones dos agentes (`git remote set-url`).

## 8 e 9. Variáveis de desenvolvimento e Central Node

- `CENTRAL_TELAS`, `CENTRAL_NODE`, `CENTRAL_CLI_FALSA`, `CENTRAL_CLI_FALSA_API` e `CENTRAL_HOJE` viram `APPLYER_*` junto com o item 5. São só dos testes e do desenvolvimento.
- **A Central Node (v1) não muda:** `CENTRAL_DADOS`, `dados/`, o MCP `central`, `ferramentas/cli.mjs` e os scripts de `automacao/` (`window.__CENTRAL_*`). Ela é a referência até a P8 e sai depois. Mexer agora só criaria diferença com o golden.

## Ordem sugerida

1. Teste próprio do `internal/segredos` (bead novo, pequeno).
2. PR A: itens 1, 2 e 3 (pasta, variável e cofre), com migração, testes e revisão do CyberSec.
3. PR B: itens 4, 5 e 8 (identidade, executáveis e variáveis de dev), antes da primeira distribuição.
4. Depois da `.6.5` e com a fila de PRs vazia, PR C: item 7 (módulo e repositório).
5. Na P8: a Central Node sai, e o item 9 deixa de existir.

## Para o Vinicius decidir

| Pergunta | Recomendação |
|---|---|
| Migrar a pasta de dados para `Applyer`? | Sim, com o rename ao abrir (item 1) |
| Trocar o bundle id e o MSIX? | Sim, mas só se for antes da primeira distribuição (item 4) |
| Renomear o repositório para `applyer`? | Sim, por último (item 7) |
| Manter `central.db` dentro da pasta nova? | Sim (item 6) |
| Por quanto tempo aceitar `CENTRAL_PASTA`? | Duas versões, com aviso no log |
