# Diagnóstico do front: sync de 2026-11-08

Escrito pelo agente de front-end para o PO. Sem dados pessoais; só o que o repositório e as ferramentas mostraram.

## O que foi conferido

| Item | Resultado |
|---|---|
| Clone isolado do branch `reescrita-go` | ok, commit `c62b89a` |
| Leitura de `docs/status.md`, `CLAUDE.md`, `api/openapi.yaml` | ok |
| Grafo do código (`graphify extract . --code-only`) | 1259 nós, 3873 arestas, 56 comunidades; `dados/` ausente no clone |
| Node / npm | v24.20.0 / 11.19.0 |
| Go / wails3 | go1.27.3 / v3.1.0 |
| `tsc --noEmit` em `app/frontend` | passou |
| `npm test` (Vitest) | 125 testes, 1 arquivo, passou |
| `npm run build` | passou (`dist/` gerado) |
| `npm run e2e` (Playwright, PC + Pixel 7) | 2 passaram, 2 pulados de propósito (cada jornada roda só no projeto dela) |

## Atenção para quem for rodar na máquina

- O Vitest falha com `Event bindings module not found` se `app/frontend/bindings` não existir.
  A pasta é gerada por `wails3 generate bindings -clean=true -ts -i` dentro de `app/`, como o CI faz.
  Isso não é bug do código: é um passo de preparação que o `CLAUDE.md` ainda não cita na seção das telas.
- `go` e `wails3` podem estar instalados e fora do PATH da sessão. Caminhos usuais no Windows:
  `C:\Program Files\Go\bin` e `%USERPROFILE%\go\bin`.
- O Graphify avisa que `tree_sitter_sql` não está instalado; as 3 migrações `.sql` ficam fora do grafo.

## Propostas de tarefas pequenas de front para a P5 (sem implementar ainda)

Todas dependem de o back-end publicar as rotas em `api/openapi.yaml` primeiro (contrato antes do código).

| # | Tarefa | Aceite | Depende de |
|---|---|---|---|
| F5.1 | Tela "Fila de candidaturas" (`#/fila`): lista do que o navegador quer enviar, com Aprovar / Pular por item | Estados vazio, carregando, erro e sucesso; 375 px sem rolagem lateral; alvo ≥ 44 px; Esc fecha o detalhe; claro e escuro AA | rota `GET /fila` e `POST /fila/{id}/aprovar|pular` no OpenAPI |
| F5.2 | Status ao vivo da candidatura em andamento (passo atual, site, tempo) na tela Hoje | Atualiza só quando `/versao` muda; nunca redesenha com texto em edição; respeita "reduzir movimento" | rota `GET /navegador/estado` |
| F5.3 | Aviso de parada: CPF, senha ou captcha detectados → card "Precisa de você" com link da aba e botão "Marcar como bloqueada" | Nunca mostra nem pede o dado; usa `POST /vagas/{id}/mover` com `etapa: bloqueada` e pendência | F5.2 |
| F5.4 | Contador diário "x de 30 candidaturas no LinkedIn" com bloqueio visual ao chegar no limite | Lê o limite do back; não deixa aprovar acima do limite; texto curto em pt-BR | rota `GET /navegador/limites` |
| F5.5 | Teste Playwright da jornada "aprovar uma candidatura da fila" (PC e celular), com fixture fictícia | Passa em `npm run e2e`; sem dados reais | F5.1 |

Ordem sugerida: F5.1 → F5.2 → F5.3 → F5.4 → F5.5. Cada uma cabe num PR pequeno contra `reescrita-go`.
