# Apagar meus dados e sair das plataformas (LGPD)

Bead `jc-a2y.49`. Status: **implementada**. As rotas estão no `api/openapi.yaml` 0.4.10 e são atendidas pela API (`internal/api/dados.go`). Esta proposta bloqueia a `.6.5`, porque o perfil do navegador vai guardar a sessão do LinkedIn. Nada vai para a nuvem.

## Rotas (só a tela, `x-so-tela`)

| Rota | Corpo | O que faz |
|---|---|---|
| `POST /dados/apagar` | `{"confirmacao": "APAGAR MEUS DADOS"}`, a frase exata digitada pela pessoa | Apaga tudo o que o app guarda neste computador e responde `fechar: true`. Logo depois, o app fecha. Ao abrir de novo, é um usuário novo |
| `POST /navegador/sair` | `{}` | Apaga só o perfil do navegador do app (sessão e cookies das plataformas). O app continua aberto, com os dados |

Corpo estrito: campo a mais ou frase diferente dão **400**, e nada é apagado. Vindo do assistente (CLI, MCP), a resposta é **403**. Com candidatura em `enviando`, isto é, o executor numa página, a resposta é **409** `{erro, estado: "enviando"}`, e nada é apagado.

## O que é apagado

Lista fechada (`ItemApagado`), que é o que volta em `apagado` e `faltou`:

| Item | O quê | Onde |
|---|---|---|
| `banco` | vagas, fila de candidaturas, perfil, eventos, limites e configurações (inclusive a de IA) | `<dados>/central.db` e os arquivos `-wal`/`-shm` |
| `curriculos` | PDFs copiados para o app | `<dados>/curriculos/` |
| `backups` | cópias diárias do banco | `<dados>/backups/` |
| `navegador` | perfil próprio do navegador: cookies, sessão e cache das plataformas | `<dados>/navegador/` (`navegador.ApagarPerfil`) |
| `chaves` | chaves de IA (anthropic, openai, google) | cofre do sistema, serviço "Central de Candidaturas" |

**Só esses caminhos, nunca a pasta inteira** (revisão do CyberSec no #54). `CENTRAL_PASTA` pode apontar para uma pasta da pessoa, como Documentos, e outros arquivos que estiverem lá ficam. A raiz `<dados>` só é removida se ficar vazia (`os.Remove`, que não apaga pasta com conteúdo). Uma junção ou link simbólico dentro desses caminhos que aponte para fora da pasta de dados é removido como link: o destino fica intacto. Isso tem teste.

**Não é apagado** (a tela diz isso antes de a pessoa confirmar):
- o login das CLIs de IA (Claude, Codex, Gemini): é da pessoa, fora do app;
- os arquivos originais de onde os currículos foram copiados;
- os dados da janela do app: cache e armazenamento local da tela. No Windows, o WebView2 guarda em `%APPDATA%\<nome do executável>` (padrão do Wails), e no macOS o WebKit guarda em `~/Library`. Não há dado de candidatura nem chave ali, e com o app aberto a pasta fica presa;
- a pasta `dados/` da Central antiga (Node).

O app não grava log em arquivo: o log vai só para a saída de erro do processo e não fica no disco.

`<dados>` é a pasta de dados (`os.UserConfigDir()/Central de Candidaturas`, ou `CENTRAL_PASTA`).

## Ordem

1. Confere a frase e o corpo: 400 sem apagar nada, e 403 para o assistente.
2. Para o executor: cancela o ciclo, **espera ele acabar** e fecha o navegador, esperando o processo do Chrome sair (o cancelamento do chromedp espera o processo). Só depois confere se há candidatura em `enviando`, para não haver corrida entre a conferência e um envio que começasse no meio. Se ainda houver, responde 409 e não apaga nada; o executor fica parado até o app abrir de novo, e a mensagem do 409 diz isso à pessoa. No build padrão, nada disso existe.
3. Apaga as chaves do cofre. Uma chave que já não existe conta como apagada.
4. Apaga `navegador/`, `curriculos/` e `backups/`. Fecha o banco e apaga `central.db`, `-wal`, `-shm` e `-journal`. No Windows, um arquivo aberto não sai: por isso o banco fecha antes. Remove a raiz só se ficar vazia.
5. Responde `{apagado, faltou, completo, fechar}`. Se algo não saiu (arquivo preso, cofre recusou), o item vai para `faltou`, com `completo: false`, e a tela mostra o que restou. Não há caminho nem texto de erro do sistema na resposta. `fechar` é sempre `true` em `/dados/apagar`, mesmo na falha parcial, porque o banco já foi fechado.
6. O app fecha. No `central-dev`, o processo sai.

**`/navegador/sair`:** suspende o executor (nenhum ciclo novo começa, e o atual termina), confere `enviando` (409), fecha o navegador esperando o Chrome sair, apaga só `navegador/` e solta o executor. O perfil é recriado, com as preferências, na próxima vez que o navegador abrir.

## Log (M7)

O log leva só o resultado por item, com o motivo da falha de uma lista fechada: `cofre`, `em-uso` (arquivo aberto por outro programa, no Windows), `permissao` ou `outro`. Por exemplo: `apagar: navegador ok`, `apagar: curriculos faltou (em-uso)`. Nunca leva caminho (que traz o nome do usuário), conteúdo nem chave.

## Testes (`internal/api/dados_test.go`)

- Depois de apagar, nenhum arquivo do app resta na pasta de dados, e o cofre falso (`segredos.Memoria`) fica sem as três chaves.
- Arquivo da pessoa na mesma pasta (`CENTRAL_PASTA` em Documentos) fica, e a raiz fica.
- Uma junção (Windows) ou link simbólico dentro de `curriculos/` apontando para fora: o destino sobrevive.
- Falha parcial: `fechar` continua `true`.
- `sair` remove só `navegador/`: banco, currículos e chaves ficam.
- Frase errada, campo a mais, assistente e `enviando`: nada muda (o teste lista a pasta antes e depois).
- Falha parcial (um arquivo preso, no Windows): `completo: false` e o item em `faltou`.
- O teste roda no Windows (CI) e no macOS (vet). Só dados fictícios.
