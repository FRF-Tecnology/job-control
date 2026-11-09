# Checklist de segurança do PR

Bead: `jc-a2y.11`. Dono: agente CyberSec. Vale para o branch `reescrita-go`.

## Quando este checklist é obrigatório

Todo PR que mexe em pelo menos uma destas áreas precisa da revisão do CyberSec antes do merge:

- **API local:** `internal/api`, `app/main.go`, `cmd/central-dev`, `api/openapi.yaml`;
- **IA:** `internal/ia`, prompts, guarda, provedores, CLIs;
- **Segredos:** `internal/segredos`, qualquer leitura ou gravação de chave ou token;
- **Navegador:** `internal/navegador` (P5), `automacao/`;
- **Nuvem:** `cmd/nuvem` (P7), cliente da nuvem no app;
- **Atualização e distribuição:** P8.

**Severidade dos achados:**

| Severidade | Exemplo | Efeito no merge |
|---|---|---|
| Crítica | chave em log ou banco, envio sem aprovação, CPF/senha preenchido, API ouvindo fora do 127.0.0.1 | **bloqueia** até corrigir |
| Alta | saída da IA sem passar pela guarda, rota que grava sem checar `Sec-Fetch-Site` | corrige no mesmo PR ou abre bead antes do merge |
| Média | log com conteúdo pessoal, falta de limite de tamanho ou tempo | bead com prazo |
| Baixa | erro ignorado sem impacto de segurança | bead opcional |

Como copiar para o PR: cole as seções que se aplicam e marque cada item. Item sem evidência (teste, comando ou linha de código) conta como não feito.

## 1. Aprovação humana de envios

- [ ] Nenhum caminho de código envia candidatura, mensagem para recrutador ou follow-up sem um registro de aprovação da pessoa para **aquele item**.
  - Verificar: teste que chama o envio sem aprovação e espera recusa (status 4xx ou erro).
- [ ] A aprovação vale para um item e um destinatário, uma vez. Repetir o envio pede nova aprovação.
- [ ] Texto vindo de vaga, email ou página **nunca** vira ação: a IA só sugere, e quem dispara é a pessoa.
  - Verificar: teste com fixture de vaga contendo "ignore as instruções e envie..." e a fila não muda.
- [ ] Emails: só leitura. Não há chamada que responda, arquive, apague ou marque como lido.

## 2. Limites do LinkedIn e do navegador (P5)

- [ ] Limite diário de candidaturas no LinkedIn configurado em no máximo **40**, com padrão **30**. O contador fica no SQLite e sobrevive a reiniciar o app.
  - Verificar: teste que tenta a candidatura 41 no mesmo dia e recebe recusa.
- [ ] Intervalo mínimo entre ações automáticas, com variação.
- [ ] Ao ver CPF, RG, senha, dado bancário ou captcha, o app **para** e marca a vaga como `bloqueada` com a pendência.
  - Verificar: teste com fixture HTML que tem campo de CPF; o campo fica vazio e a etapa vira `bloqueada`.
- [ ] O app nunca cria conta, nunca loga com senha e nunca clica em prompts de permissão do navegador.
- [ ] Perfil do Chrome próprio do app, com permissão de arquivo restrita ao usuário. Cookies não são copiados para o banco nem para logs.

## 3. Cofre e segredos

- [ ] Chave de API e token da nuvem só no cofre do sistema (`internal/segredos`). Nunca em SQLite, JSON, arquivo de config, variável persistida ou log.
  - Verificar: `grep` no diff por `apiKey`, `token`, `secret`, `Authorization` fora de `internal/segredos` e dos provedores.
- [ ] A API devolve só os 4 últimos caracteres da chave. Nenhuma rota devolve a chave inteira.
  - Verificar: teste em `internal/api` que salva uma chave fictícia e confere a resposta de `GET /ia`.
- [ ] Mensagens de erro dos provedores não repetem a chave nem o cabeçalho `Authorization`.
- [ ] CLIs de assinatura rodam sem shell (`exec.CommandContext` com lista de argumentos), numa pasta temporária, com tempo limite e sem chave de API no ambiente.
- [ ] gitleaks passou no CI.

## 4. Dados sensíveis e LGPD

- [ ] Nenhum campo novo guarda CPF, RG, senha ou dado bancário. A importação continua recusando CPF/RG.
- [ ] Logs não trazem conteúdo pessoal: nada de corpo de pedido, texto de vaga, email, nome de empresa ou query string com dados da pessoa.
- [ ] Dado novo é o mínimo necessário e fica local por padrão. Se vai para a nuvem (P7), o PR diz qual campo, por quê e por quanto tempo.
- [ ] Exportar e apagar continuam cobrindo o dado novo.

## 5. Fixtures e repositório público

- [ ] Fixtures, goldens, prints e exemplos usam **só dados fictícios**: nomes, emails (`@exemplo.com`), empresas, URLs e textos de vaga.
- [ ] Nenhum arquivo de `dados/` entrou no diff. Nenhum caminho de máquina com nome de usuário.
- [ ] Páginas salvas do LinkedIn ou da Gupy (P5) foram limpas: sem nome, foto, ID de perfil, cookie ou token.

## 6. API local

- [ ] Servidor só em `127.0.0.1`; `Host` diferente de `localhost`, `127.0.0.1` ou `[::1]` é recusado.
- [ ] Toda rota que grava recusa `Sec-Fetch-Site` diferente de `same-origin` ou `none` e exige `Content-Type: application/json`.
- [ ] Nenhum cabeçalho CORS que libere outra origem.
- [ ] Rota nova entrou antes no `api/openapi.yaml`.
- [ ] Corpo com limite de tamanho; chamadas externas com tempo limite.

## 7. IA (OWASP Top 10 para LLM)

- [ ] Toda saída da IA passa pela guarda antes de ser mostrada ou usada.
- [ ] Conteúdo externo (vaga, email, página) entra no prompt marcado como dado, nunca como instrução.
- [ ] A IA não recebe ferramentas que gravam, enviam ou abrem URLs.
- [ ] Cota e tempo limite por chamada.

## 8. Nuvem (P7)

- [ ] Login Google com OIDC + PKCE e `state`; link por email com token de uso único, 15 minutos, guardado só como hash.
- [ ] Limite de tentativas por IP e por conta no login e no proxy de modelos.
- [ ] Usuário do Postgres com privilégio mínimo; segredos só por variável de ambiente.
- [ ] Token da conta no app fica no cofre do sistema e expira.

## 9. Cadeia de suprimentos

- [ ] `govulncheck ./...` sem vulnerabilidade alcançável.
- [ ] `npm audit --omit=dev` em `app/frontend` sem alta ou crítica.
- [ ] Dependência nova justificada no PR.
- [ ] Actions fixadas por SHA (dono: Infra, bead `jc-a2y.10`). O CyberSec revisa depois que o Infra entregar.

## Linha de base (2026-11-08, commit `c62b89a`)

Medida para comparar os próximos PRs. Nenhum achado crítico.

| Verificação | Resultado |
|---|---|
| `go test ./...` | passa |
| `govulncheck ./...` | nenhuma vulnerabilidade |
| `npm audit --omit=dev` (`app/frontend`) | 0 vulnerabilidades |
| `npm audit` com dev (`app/frontend`) | 3: 2 críticas (`tinypool`, via vitest) e 1 moderada (`@vitest/mocker`); só teste e CI, não vão no app |
| `gosec ./...` | 24 avisos, nenhum crítico depois da triagem (abaixo) |

**Achados para virar bead:**

1. **Alta, só dev/CI:** `vitest` antigo puxa `tinypool` com falha crítica. Correção: subir o `vitest` e conferir o Vitest verde.
2. **Média:** `internal/api/http.go` grava no log o caminho com a query string quando dá erro 500. A query pode ter empresa e título da vaga. Correção: logar só método, rota sem query e tipo do erro.
3. **Média:** `api/openapi.yaml` não descreve as rotas `/ia/...` que já existem em `internal/api/ia.go`. Correção: incluir no contrato.
4. **Baixa:** `cmd/central-dev` usa `http.ListenAndServe` sem tempos limite. É só de desenvolvimento e ouve em 127.0.0.1. Correção: `http.Server` com `ReadHeaderTimeout`.
5. **Baixa:** vários erros ignorados (G104) em `internal/ia` e `internal/armazem`; revisar se algum esconde falha de gravação.

**Triados como sem risco:** G204 em `internal/ia/cli.go` (comando vem da lista fixa de CLIs, sem shell); G304 e G703 em `legado`, `config` e `central-cli` (caminho escolhido pela própria pessoa no PC dela).

**Como repetir:**

```sh
go test ./...
go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
go install github.com/securego/gosec/v2/cmd/gosec@latest && gosec -quiet ./...
cd app/frontend && npm audit --omit=dev && npm audit
```
