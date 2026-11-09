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

**Não é apagado:**
- o login das CLIs de IA (Claude, Codex, Gemini): é da pessoa, fora do app;
- os arquivos originais de onde os currículos foram copiados;
- a pasta `dados/` da Central antiga (Node).

`<dados>` é a pasta de dados (`os.UserConfigDir()/Central de Candidaturas`, ou `CENTRAL_PASTA`). No fim, a própria pasta é removida se ficar vazia.

## Ordem

1. Confere a frase e o corpo. Se houver candidatura em `enviando`, responde 409.
2. Para o executor e fecha o navegador do app. No build padrão, ambos já estão desligados.
3. Apaga as chaves do cofre. Uma chave que já não existe conta como apagada.
4. Fecha o banco e apaga a pasta de dados inteira. No Windows, um arquivo aberto não sai: por isso o banco fecha antes, e o app fecha logo depois de responder.
5. Responde `{apagado, faltou, completo, fechar}`. Se algo não saiu (arquivo preso, cofre recusou), o item vai para `faltou`, com `completo: false`, e a tela mostra o que restou. Não há caminho nem texto de erro do sistema na resposta.
6. O app fecha. No `central-dev`, o processo sai.

`/navegador/sair` faz só os passos 1, 2 e o do perfil.

## Log (M7)

O log leva só o resultado por item (por exemplo `apagar: navegador ok`, `apagar: chaves faltou (tipo do erro)`). Nunca leva caminho (que traz o nome do usuário), conteúdo nem chave.

## Testes (`internal/api/dados_test.go`)

- Depois de apagar, a pasta de dados não tem nenhum arquivo, e o cofre falso (`segredos.Memoria`) fica sem as três chaves.
- `sair` remove só `navegador/`: banco, currículos e chaves ficam.
- Frase errada, campo a mais, assistente e `enviando`: nada muda (o teste lista a pasta antes e depois).
- Falha parcial (um arquivo preso, no Windows): `completo: false` e o item em `faltou`.
- O teste roda no Windows (CI) e no macOS (vet). Só dados fictícios.
