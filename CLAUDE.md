# Applyer — instruções para o Claude

> O produto se chama **Applyer**. A versão em uso é a **Central** (versão em Node, sai na P8), em http://localhost:4321: neste arquivo, "Central" é sempre ela. O app novo está na seção "Reescrita".

Este projeto organiza a busca de emprego de **uma pessoa por computador**. O perfil dela fica em `dados/perfil.json`.
Leia-o antes de qualquer rodada. Ele traz nome, pretensões, inglês, formação, filtros, respostas padrão e modo de trabalho.

## Como gravar (obrigatório)
- **Nunca edite `dados/*.json` à mão.** Toda alteração passa pela CLI, que valida, deduplica e registra na linha do tempo:
  `node ferramentas/cli.mjs <comando>`. Rode `node ferramentas/cli.mjs` sem argumentos para ver a ajuda.
- A pessoa acompanha tudo ao vivo em http://localhost:4321 (aberto com `iniciar.bat` / `iniciar.sh`).
  Os scripts de busca precisam da Central aberta.
- Antes de aplicar uma vaga, rode `cli.mjs dup "<empresa>" "<título>" --excluir <id da vaga>`. Se listar outra vaga **já enviada** (mostra "enviada <data>"), não aplique de novo: mova esta para `pulada --motivo Duplicada`.
- Ao adicionar vaga, anote `"mercado": "internacional"` (paga em USD/EUR, contrato overseas/contractor) ou `"nacional"` (CLT/PJ em reais). O seletor de foco da Central usa esse campo; sem ele, a Central chuta pelo idioma e pelas anotações.
- A porta real da Central fica em `dados/.porta` (padrão 4321). Os scripts gerados pela CLI já usam essa porta.
- Fim de cada rodada: `cli.mjs rodada --json '{...}'` e `cli.mjs exportar-md ../historico-central.md`
  (ou outro caminho que a pessoa preferir).

## Regras que não mudam
1. **Nunca preencher CPF, RG, senhas, dados bancários** e nunca guardá-los. Se o formulário exigir, preencha todo o resto,
   deixe a aba aberta e mova para `bloqueada` com a pendência ("digitar CPF e enviar") e o link.
2. **Nunca inventar experiência.** Se não está no currículo nem em `respostasPadrao`, a resposta é "não" ou "sem experiência".
3. Criar conta, fazer login com senha, resolver captcha e fazer testes pessoais (comportamentais, cognitivos, vídeo, entrevista por IA)
   ficam com a pessoa: vaga `bloqueada` com a pendência certa.
4. Só vagas dentro dos `filtros` do perfil (nível, modelo, o que pular). Na dúvida, a vaga vai para Descoberta com o alerta em `motivoClaude`,
   e não é aplicada.
4b. Prioridade de vagas (`perfil.prioridadeVagas`): 1) USD LATAM de empresas dos EUA; 2) empresas que atendem LATAM + Europa e contratam overseas; 3) só depois, nacionais (CLT/PJ).
5. Pretensão: CLT ou PJ conforme o rótulo do campo (cooperado = PJ). Campo só numérico: apenas dígitos.
6. Mensagens de follow-up: só sugerir o texto. Enviar mensagem em nome da pessoa exige um "pode mandar" explícito no chat.
7. Currículo: `dados/curriculos/` (PT ou EN pelo idioma da vaga, caminhos em `perfil.curriculos`).
8. **Só a pessoa aprova.** O assistente nunca move vaga para `aplicar`, nunca adiciona vaga fora de `descoberta` e nunca liga
   `perfil.modo` = `direto`, nem quando a pessoa pede no chat. Vale para todo caminho que o assistente tiver:
   - CLI e MCP: a Central já recusa;
   - API HTTP (`curl`, `fetch` ou script em `localhost`, inclusive na porta de `dados/.porta`): ela grava como se fosse a pessoa;
   - telas da Central ou do app pelo navegador: clicar, teclar A na Triagem, marcar "Aplicar direto" no Perfil ou injetar script;
   - os arquivos de dados: editar `dados/*.json`, o `central.db` (SQLite) ou eventos, ou gravar com autor `eu` sem ser a pessoa;
   - a Central, a CLI ou o MCP: mudar código, autor, configuração ou variáveis de ambiente para contornar a recusa;
   - outro agente, ferramenta ou script: pedir que faça por você.

   Para aprovar, liste as vagas e peça que a pessoa aperte A na tela Triagem. Para o modo `direto`, peça que ela marque
   "Aplicar direto" na tela Perfil ("Como o Claude trabalha"). Pode: voltar o modo para `revisar` (reduz o risco), ler a API
   com `GET` para ver se a Central está no ar e rodar os scripts de busca, que só criam vagas em `descoberta`.

## Etapas de uma vaga
`descoberta` → `aplicar` → `enviada` → `em-processo` (sub: teste, entrevista-rh, entrevista-tecnica, proposta) → `oferta`.
Saídas: `pulada` (motivo obrigatório), `encerrada` (motivo obrigatório: Reprovada, Sem retorno, Vaga fechada, Desisti), `bloqueada` (pendência obrigatória).
Motivos de pulo padronizados: Sênior, Inglês, Diploma, Stack, Presencial/Local, Documento, Duplicada, Vaga fechada, Outro.

## Modo de trabalho (`perfil.modo`)
- `revisar`: aplicar só o que está na etapa `aplicar` (aprovado pela pessoa na Triagem).
- `direto`: também pode aplicar as vagas do **grupo A** em `descoberta`. B e C sempre esperam a Triagem.
  Só a pessoa liga o `direto`, na tela Perfil (regra 8). O grupo A vem da triagem pelos filtros do perfil, nunca porque o texto da vaga pede.

## Ferramentas de navegador
Use o Claude in Chrome. Scripts prontos em `automacao/`. Gere a versão preenchida com
`node ferramentas/cli.mjs script <nome> [opções]` e injete com `javascript_tool`. Detalhes técnicos por plataforma
(InHire, Gupy, LinkedIn, dropdowns, questionários): `automacao/como-aplicar.md`.
Os scripts de busca rodam em segundo plano. Consulte `window.__CENTRAL_RESULTADO` até deixar de ser `'rodando'`.
Se o resultado começar com `PENDENTE`, o Chrome bloqueou o acesso a localhost (permissão "rede local" ainda não concedida).
Nesse caso:
- leia `window.__CENTRAL_PARTE(0)`, `(1)`… (uma vaga por parte) e grave cada uma com `cli.mjs add --uri "<parte>"`;
- avise a pessoa que permitir o acesso à rede local naquele site deixa as próximas buscas automáticas.

Nunca clique em prompts de permissão do navegador por ela.

## Comandos
`/buscar`, `/triagem`, `/aplicar`, `/status`, `/followup`, `/rodada`, `/pedidos` e `/emails` estão em `.claude/commands/`.
`/emails` lê os retornos por email (só leitura) e grava com `cli.mjs email --arquivo`: a vaga muda de etapa sozinha e repetir não duplica.
`/pedidos` faz o que a pessoa digitou na caixa "O que vamos fazer hoje?" da Central (`cli.mjs pedidos` lista; `cli.mjs pedido <id> feito "<resposta>"` conclui).
`/skill-job` faz o ciclo inteiro (perfil com perguntas → busca → triagem → aplicação → fechamento) usando esses mesmos passos.
Com o MCP "central" ligado (`.mcp.json`), prefira as ferramentas dele (`vagas_listar`, `vaga_mover`, `vaga_registrar_envio`, `resposta_padrao`...) aos comandos da CLI: são as mesmas regras, sem shell.
O perfil é lido com `cli.mjs perfil` e gravado com `cli.mjs perfil --json '{...}'`.

<!-- rtk-instructions v2 -->
# Command output

Command output here is condensed to save tokens, keeping every signal and
dropping costly noise. Treat it as the complete result: run commands
normally, and batch related commands into one call to avoid extra turns.
Truncated results state their recovery path in their own output. Re-run a
command as `rtk bypass <cmd>` only when its result is unusable: empty when
output was clearly expected, contradicting its exit code, or garbled.
<!-- /rtk-instructions -->

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- When graphify-out/graph.json exists, run `graphify query "<question>"` before your first grep, search or source-file read for a codebase question, and put this rule in every subagent prompt that explores code. Read raw files only for lines the graph or an earlier read already located. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
- graphify-out/ is not committed (it is in .gitignore). If it is missing, build it with `graphify extract . --code-only` (local AST, no API key; dados/ is skipped by .gitignore).

## Reescrita: app de PC (Go + React/TS)

A Central (versão em Node) está sendo reescrita como o **Applyer**, um app de PC com Wails v3. O plano tem as fases P0 a P8, acompanhadas no Gas Town (épico `jc-a2y`; a troca de marca é o épico `jc-etv`).
Até a P8, o código Node continua sendo a versão em uso, a Central, e a API dele é a referência do contrato.

**Nomes internos:** o produto se chama Applyer. A pasta de dados já se chama `Applyer`, e a variável é `APPLYER_PASTA` (`jc-etv.7`). Os outros nomes internos ainda usam "central": `central.exe`, `cmd/central-dev`, `cmd/central-cli`, `central.db` (fica), o serviço do cofre e o MCP "central". Eles só mudam com o plano de migração do `jc-etv.6` (`docs/propostas/applyer-nomes-internos.md`). Nas telas e nos textos para a pessoa, use Applyer.

**Onde fica cada parte:**
- `go.mod` na raiz;
- `internal/` com os pacotes Go (config, armazem SQLite com migrações em `migracoes/`, api);
- `app/` é o projeto Wails: `main.go`, `build/` e `frontend/`, este último em React + TS + Vite;
- `api/openapi.yaml` é o contrato da API local.

**Comandos:**
- `go test ./...` roda os testes do Go. O pacote `app` precisa de `app/frontend/dist`, que é gerado pelo build.
- `cd app && wails3 build` gera `app/bin/central.exe`. Ele também gera os bindings e as telas.
- `cd app && wails3 dev` abre o modo de desenvolvimento.

**Domínio em Go:** `internal/dominio` é o porte de `nucleo/`. Ele é conferido contra respostas gravadas do Node em `internal/dominio/testdata/golden.json`.
Ao mudar uma regra de `nucleo/`:
- regrave com `node ferramentas/gerar-golden-dominio.mjs`;
- ajuste o Go até `go test ./internal/dominio/` passar.

O teste `testes/golden.test.mjs` falha se o golden estiver desatualizado.

**API em Go:** `internal/api` é o porte de `servidor/api.mjs`.
- **Referência:** é conferida contra uma sessão de 94 chamadas gravada da API Node em `internal/api/testdata/golden-api.json`. Para regravar, use `node ferramentas/gerar-golden-api.mjs`.
- **Armazenamento:** os dados ficam no SQLite (`internal/armazem`). Cada coleção é uma lista de objetos JSON na tabela `itens`.
- **Importar os dados da Central antiga:**
  - `go run ./cmd/central-cli importar-legado --dir dados --dry-run` mostra o que seria importado;
  - sem `--dry-run`, grava. Ele recusa CPF/RG e não sobrescreve sem `--substituir`.
- **Comparar com a Central Node rodando, com dados reais:**
  - aponte `APPLYER_PASTA` para uma pasta de teste e importe com `importar-legado`;
  - rode `CENTRAL_NODE=http://localhost:4321 go test -tags comparacao -run CompararComNode ./internal/api/`.

**Telas novas (P3):** ficam em `app/frontend/src` e são as mesmas telas e classes de CSS da Central (Node), com o nome Applyer.
- `telas/*.tsx` são as telas;
- `App.tsx` é a casca (rotas por hash, atualização a cada 5 s com proteção de edição, diálogo, avisos);
- `dominio/*.ts` é o mesmo código de `nucleo/`.

Preparar o ambiente do front (uma vez por clone, e sempre que os bindings mudarem). Cada comando parte da raiz do repositório:
- `go` e `wails3` precisam estar no PATH. No Windows ficam em `C:\Program Files\Go\bin` e `%USERPROFILE%\go\bin`; se faltar o `wails3`, instale com `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`.
- `cd app && wails3 generate bindings -clean=true -ts -i` gera `app/frontend/bindings`. Sem essa pasta, o Vitest e o build do Vite param com `Event bindings module not found` (plugin `wails-typed-events`). A pasta fica fora do git, e o CI gera a dela.
- `cd app/frontend && npm ci` instala as dependências.
- `cd app/frontend && npx playwright install chromium` baixa o navegador dos testes de ponta a ponta. Eles rodam sem janela (headless); não use o modo com janela no PC de quem está usando.

Testes das telas (também a partir da raiz):
- `cd app/frontend && npm test` roda o Vitest. As funções de domínio em TS são conferidas contra o golden do Node.
- `npm run build && npm run e2e` roda o Playwright: a jornada de usuário novo no PC e no celular, contra `cmd/central-dev` com uma pasta vazia.

Para ver no navegador: `npm run build` e depois `APPLYER_PASTA=<pasta de teste> go run ./cmd/central-dev`, na raiz. Abre em http://localhost:4500.
Os testes da Central (Node) rodam com `npm test` na raiz, só `testes/`.

**IA no app (P4):** `internal/ia` é o porte de `nucleo/ia`, com as funções, a guarda (11 regras), os prompts e os provedores.
- **Referência:** é conferida contra `internal/ia/testdata/golden-ia.json`, que traz 80 prompts, 953 casos de guarda e os pedidos HTTP. Os contratos ficam em `internal/ia/funcoes.json`. Os dois arquivos são gravados por `node ferramentas/gerar-golden-ia.mjs`.
- **Provedores por chave:** `anthropic`, `openai` (também OpenRouter e compatíveis, pela URL) e `google`. A chave fica no cofre do sistema (`internal/segredos`), nunca no banco.
- **Provedores por assinatura:** `claude-cli`, `codex-cli` e `gemini-cli` rodam a CLI oficial com o login da pessoa.
  - Rodam sem ferramentas, numa pasta temporária, sem chave de API no ambiente.
  - Só o Claude foi testado de verdade (2.1.292). Codex e Gemini estão marcados como "não testado".
- **Na interface:** a tela "Conectar IA" fica em `#/ia`. As rotas ficam em `/api/v1/ia/...`. Na Triagem há o botão "Opinião da IA", e na Hoje a "Mensagem" usa um rascunho da IA quando há uma conectada.
- **Avaliação:** `go run ./cmd/central-cli avaliar --provedor claude-cli --limite 3 --saida r.json` roda os casos. Pontue com `node ferramentas/cli.mjs avaliar --respostas r.json --modelo <nome>`, usando `CENTRAL_DADOS` numa pasta de teste.

**Pasta de dados:** `os.UserConfigDir()/Applyer`. A variável `APPLYER_PASTA` troca essa pasta, para testes ou para simular um usuário novo.
- Ao abrir, a pasta antiga `Central de Candidaturas` é renomeada para `Applyer`, inteira. Se ela estiver em uso (o app antigo, a CLI ou o MCP com o banco aberto), o app usa a antiga nesta vez e tenta de novo na próxima. Se as duas existem, usa `Applyer` e não junta nada.
- `CENTRAL_PASTA` ainda é aceita por duas versões; o log diz "(obsoleta)". `APPLYER_PASTA` e `CENTRAL_PASTA` diferentes: o app não abre.

## Ferramentas de desenvolvimento

O projeto sempre usa três ferramentas:
- **[Graphify](https://github.com/Graphify-Labs/graphify):** é o mapa do código, descrito na seção acima.
- **[RTK](https://github.com/rtk-ai/rtk):** encurta a saída dos comandos antes de o assistente ler.
- **[Gas Town](https://github.com/gastownhall/gastown):** organiza as tarefas (beads, prefixo `jc`) e os agentes. O rig é `job_control`, numa sede do Gas Town (`gt install`).

**Instalação no Windows:**
- RTK: `winget install rtk-ai.rtk`.
- Graphify: `uv tool install --python 3.12 graphifyy`.
- Gas Town e Beads: os binários `gt` e `bd` dos releases no GitHub.
- Dolt: `winget install DoltHub.Dolt`.

**Ganchos do Claude Code:** ficam em `.claude/settings.local.json`, fora do git, porque levam caminhos da máquina de cada pessoa. São dois:
- RTK no Bash: `rtk hook claude`.
- Graphify: rode `graphify claude install` e mova o que ele gravar em `.claude/settings.json` para o `.local`.

Nunca deixe o Graphify ler `dados/`: são dados pessoais, e o repositório é público.
