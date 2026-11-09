# MCP local e CLI no app Go, com a regra M4

Bead `jc-a2y.7.4` (P6). Status: **proposta**. Antes do código, o CyberSec passa os requisitos, e eles viram critério de aceite. Contrato: a lista do que o assistente pode chamar entra no `api/openapi.yaml` como `x-assistente` (abaixo), **sem mudar a `info.version`**.

Hoje o assistente (Claude Code) usa o MCP "central" da Central Node (`mcp/servidor.mjs`, `.mcp.json`), que grava nos dados do Node. O app Go ainda não tem MCP, e a `central-cli` só sabe `importar-legado` e `avaliar`. Esta proposta porta as ferramentas para o app Go seguindo a regra M4 (`docs/propostas/p5-contrato-fila.md`) e a regra 8 do `CLAUDE.md`.

## A regra M4, como fica

- **Um ponto de entrada próprio**, fora do handler da tela: `api.ChamarAssistente(ctx, canal, pedido)`. O `canal` (`mcp` ou `cli`) é fixado pelo **binário**, nunca pelo pedido.
- **Lista de permissão, e negado por padrão.** A lista fica no contrato (`x-assistente`, abaixo). Método e rota fora dela dão **403** antes de qualquer leitura ou gravação. Um teste confere que a lista do código é a do contrato e que nenhuma operação `x-so-tela` está nela.
- **Os cabeçalhos da tela nunca passam.** `X-Pedido` e `X-Impressao`, que confirmam uma proposta de pedido (`jc-a2y.7.1`), só a tela manda. O ponto do assistente monta o pedido sem cabeçalhos.
- **Só a pessoa aprova** (regra 8). O assistente:
  - só lê a fila e os limites;
  - pode **revogar** uma aprovação, porque só reduz o risco e tem volta;
  - pode **pausar** o envio e uma plataforma, porque só reduz o risco; retomar é só da tela.
  - Criar, editar, aprovar (uma a uma ou em lote), cancelar, reabrir e concluir à mão uma candidatura, ler o formulário, mudar o limite, apagar dados, entrar e sair das plataformas, mostrar a janela do envio, o Ao vivo, a IA (configuração, chaves, testar, triar, mensagem) e o perfil (gravar): **403**.
- **As regras do domínio continuam valendo por cima.** O assistente não move para `aplicar` (400), só cria vaga em `descoberta`, nunca liga `perfil.modo = direto` e nunca grava CPF, RG, senha nem banco. A lista só tira rotas; nunca dá ao assistente mais do que o domínio já permite.

## O que o assistente pode chamar (`x-assistente`)

| Ferramenta (MCP) e comando (CLI) | Rota | Por quê |
|---|---|---|
| `resumo` | `GET /resumo` | leitura |
| `vagas_listar` | `GET /vagas` | leitura |
| `vaga_detalhe` | `GET /vagas/{id}` | leitura |
| `vaga_duplicada` | `GET /duplicatas` | leitura |
| `vaga_adicionar` | `POST /vagas/importar` | o domínio só cria em `descoberta` |
| `vaga_mover` | `POST /vagas/{id}/mover` | `aplicar` dá 400 vindo do assistente; os cabeçalhos de pedido não passam |
| `vaga_registrar_envio` | `POST /vagas/{id}/mover` (`etapa: enviada`) | registra o que já foi enviado fora do app, como na Node |
| `vaga_editar` | `PATCH /vagas/{id}` | só os campos editáveis mudam; o domínio ignora o resto, como etapa e datas |
| `evento_registrar` | `POST /vagas/{id}/eventos` | linha do tempo |
| `followups_vencidos` | `GET /followups` | leitura |
| `followup_registrar` | `POST /vagas/{id}/followup` | marcar feito ou adiar; nada é enviado |
| `rodada_registrar` | `PUT /rodadas/{n}` | registro |
| `perfil_ler` e `resposta_padrao` | `GET /perfil` | leitura (a resposta padrão é casada no próprio MCP, como na Node) |
| `pedidos_listar` | `GET /pedidos` | leitura |
| `pedido_concluir` | `POST /pedidos/{id}/concluir` | só a resposta em texto; confirmar proposta é só da tela |
| `candidaturas_listar` (nova) | `GET /candidaturas` | leitura |
| `candidatura_detalhe` (nova) | `GET /candidaturas/{id}` | leitura |
| `limites` (nova) | `GET /candidaturas/limites` | leitura |
| `candidatura_revogar` (nova) | `POST /candidaturas/{id}/revogar` | só reduz o risco e tem volta (M4) |
| `envio_estado` (nova) | `GET /envio` | leitura, só códigos |
| `envio_pausar` (nova) | `POST /envio/pausar` | só reduz o risco |
| `plataforma_pausar` (nova) | `POST /envio/plataformas/{plataforma}/pausar` | só reduz o risco |

**Fora deste recorte:**
- `emails_registrar` e `emails_sem_vaga`: entram com o `.7.3` (Faker), que define as rotas de e-mail no app Go.
- `GET /estado`: ficaria redundante com as ferramentas, e mandaria tudo de uma vez para a IA.
- `PUT`/`PATCH /perfil`: a Node não tinha ferramenta de gravar perfil.

## Processos e dados

- **MCP:** um binário novo, `cmd/central-mcp`, com JSON-RPC 2.0 sobre stdio, sem dependências, como o da Node. O stdout é só do protocolo, e os avisos vão para o stderr.
- **CLI:** a `central-cli` ganha os mesmos comandos, a partir da **mesma tabela** de ferramentas (`internal/assistente`): `central-cli vagas_listar --json '{"etapa":"descoberta"}'`. Uma tabela, dois canais, as mesmas regras.
- **O banco:** os dois abrem o `central.db` da pasta do app (`APPLYER_PASTA`) pelo `internal/armazem`. O WAL, o `busy_timeout` e as transações imediatas deixam o app e eles gravarem juntos. Para não segurar o arquivo (o "Apagar meus dados" e a renomeação da pasta precisam dele livre), o MCP **abre e fecha o banco a cada chamada**.
- **Sem porta TCP.** O app continua sem porta; o MCP e a CLI não falam com ele, só com o banco, pelas mesmas regras da API.
- **`.mcp.json`:** continua apontando para a Node até o PO decidir a troca. Trocar é uma linha, num PR à parte.

## Segurança

- **O texto das vagas é dado não confiável.** As respostas das ferramentas levam texto de vagas e e-mails, que a IA lê. As instruções do servidor dizem isso, e nenhuma ferramenta executa nada do texto.
- **Teto de tamanho** em cada resposta, com corte e aviso, para nenhuma ferramenta despejar o banco inteiro na IA.
- **Sem segredos.** Nada do cofre nem da configuração da IA passa pelo assistente. As rotas `/ia/*` estão fora da lista.
- **Log:** no stderr, só a ferramenta, o status e o tipo do erro. Nunca os argumentos nem o texto de vagas.
- **Entrada:** o MCP confere o `inputSchema` (obrigatórios, tipos, enums), e depois a API valida de novo (`soCampos` e o domínio).

## Decisões para o PO e o CyberSec

1. **O autor gravado.** A M4 diz "Autor `mcp` ou `cli`". Hoje o domínio trata como assistente qualquer autor diferente de `eu`, e a Node e os eventos já gravados usam `claude`.
   - **Recomendo:** o autor dos eventos continua `claude`, e o `canal` (`mcp` ou `cli`) entra no pedido só para a lista de permissão e o log. Assim os dados ficam compatíveis com a Node e com o golden, e a regra 8 continua igual.
   - A alternativa é gravar `mcp` ou `cli` como autor, e então a tela e o histórico mostram "mcp" ou "cli" onde hoje mostram "claude".
2. **`vaga_registrar_envio`** fica, como na Node? Ela registra um envio feito fora do app, por exemplo à mão pela pessoa. Recomendo que fique.
3. **`pedido_concluir`** fica? Ele só grava a resposta em texto. Recomendo que fique.

## Testes previstos

- **Contrato:** a lista `x-assistente` do contrato é a do código, e nenhuma delas é `x-so-tela`. Toda rota do contrato fora da lista dá 403 pelo canal do assistente, inclusive as que não existem. Mesmo com `X-Pedido` e `X-Impressao` no pedido, eles não passam.
- **Regras do domínio pelo MCP:** mover para `aplicar` dá 400; vaga nova fica em `descoberta`; `modo: direto` é recusado; CPF dá 400; aprovar, cancelar, criar e editar candidatura dão 403; revogar funciona.
- **MCP:** `initialize`, `tools/list` e `tools/call`; o stdout tem só JSON-RPC; uma entrada fora do schema é recusada; a resposta grande é cortada; o log não tem argumentos.
- **Processos:** o app e o MCP gravam juntos sem perder gravação, e o MCP não segura o banco entre chamadas (apagar a pasta funciona com o MCP parado).
- **CLI:** os mesmos casos, pelo canal `cli`.
- **Mutações:** uma por regra.

## Ordem dos PRs

1. Esta proposta e o `x-assistente` no contrato.
2. `internal/assistente`: a tabela de ferramentas, o `ChamarAssistente` com a lista e os testes de contrato e de domínio.
3. `cmd/central-mcp` (stdio).
4. Os comandos na `central-cli`.
5. A troca do `.mcp.json`, quando o PO decidir.
