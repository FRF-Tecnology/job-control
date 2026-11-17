# Conta, teste de 1 dia e assinatura — desenho

Data: 11/11/2026 · Status: rascunho para aprovação do Vinicius · Branch: `app/conta-assinatura` (a partir de `reescrita-go`)

## Por que

O Applyer vai ser vendido para pessoas no Brasil e na LATAM. Hoje qualquer um que baixa o app usa tudo, para sempre.
Precisamos de: uma conta por pessoa, um teste grátis curto e o bloqueio de quem não assina. O foco agora é o
funcionamento; o visual das telas novas fica para depois (telas simples, com os componentes que já existem).

## Decisões do Vinicius

1. **A conta vem antes de tudo.** A primeira tela do app é "Criar conta / Entrar". O primeiro acesso que já existe
   (conectar IA → currículo → vagas → condições → revisão, branch `app/onboarding-busca-triagem`) vem depois do login.
2. **Duas formas de criar a conta, dentro do app:** email + senha digitados no app, e conta Google.
3. **Teste grátis de 1 dia**, que começa quando a conta é criada (confirmada).
4. **Um teste por computador.** Uma conta nova num PC que já usou o teste entra direto bloqueada.
5. **Fim do teste sem assinatura = "ver, sem agir".** A pessoa abre o app e vê vagas, candidaturas, perfil e histórico;
   buscar, triar, usar a IA, se candidatar e mexer nos dados ficam travados até assinar.
6. **Sem internet = "ver, sem agir"**, também para assinantes. A busca e a candidatura dependem da internet de qualquer jeito.
7. **Um plano só**: assinatura mensal que libera tudo. Não existem limites diferentes por plano.
8. **Pagamento pelo Stripe.** Ele tem CNPJ e ainda não tem conta no Stripe.
9. **Contas num Postgres nosso, na VPS dele** (junto com o `cmd/nuvem`), não num serviço de login pronto (mudou em 11/11, depois da primeira versão deste
   desenho, que usava Supabase Auth). O servidor `cmd/nuvem` em Go faz tudo: contas, senhas, emails de código, login
   Google, teste, assinatura, Stripe e nota fiscal.
10. **Entra agora.** O congelamento da nuvem (P7) no `docs/status.md` de 10/11 não vale mais ("esquece todo o passado,
    pode fazer tudo daqui pra frente").
11. **Assinatura cortesia** para ele e para quem testa o app: liberado sem pagar, marcado à mão na nuvem.
12. **Bloqueado é só ver:** nem registro manual (etapa, notas, follow-up feito) passa. Só passa a lista "sempre liberado".
13. **Nota fiscal automática desde o primeiro dia:** uma NFS-e emitida a cada pagamento.
14. Na revisão do resumo ele aprovou também: o email precisa ser confirmado antes do teste; o Google abre o navegador
    por alguns segundos e volta; a trava vale no backend do app, não só na tela; o pagamento é trocável.

## Objetivos

- Criar conta e entrar com email + senha ou Google, sem sair do app (exceto os segundos do Google no navegador).
- Confirmar o email, recuperar a senha, sair da conta, excluir a conta.
- Teste de 24 horas por conta e por computador.
- Assinar pelo Stripe, voltar ao app já liberado, cancelar pelo portal do Stripe.
- Bloquear as ações no backend local (API `/api/v1`), valendo para a tela, a CLI e o MCP.
- Funcionar de ponta a ponta em modo de teste (Stripe test mode e um modo falso para os testes automáticos) antes de cobrar de verdade.

## Fora do escopo agora

- Visual caprichado das telas de conta e assinatura.
- Mais de um plano, cupons, plano anual, preço por país.
- Pix e boleto (o Stripe só aceita Pix recorrente com pedido ao suporte e talvez não para conta brasileira; entra depois).
- Celular, sincronizar dados entre computadores, contas de equipe.
- Login com outros provedores (Apple, LinkedIn, GitHub), verificação em duas etapas (2FA), tela de administração
  (cortesia e suporte são feitos direto no banco nesta versão).

## O que não pode acontecer (anti-objetivos)

- **Os dados da pessoa não sobem para a nuvem.** Vagas, currículo, perfil e candidaturas continuam só no PC. A nuvem
  sabe só: email, nome (se o Google mandar), id do computador em hash, estado do teste e da assinatura.
- **Nenhuma senha legível em lugar nenhum.** A nuvem guarda só o resumo Argon2id; o app nunca guarda a senha
  (só o token de sessão, no cofre do sistema); logs nunca levam senha, código, token nem email completo.
- **Não dá para descobrir quem tem conta** pelas respostas de criar conta, entrar ou recuperar senha.
- **Bloqueado nunca perde dados.** O bloqueio só impede ações; nada é apagado nem escondido.
- **Nada de padrão enganoso:** o fim do teste é dito com clareza desde o primeiro dia (quanto falta), cancelar é tão
  fácil quanto assinar (portal do Stripe), sem cobrança surpresa (o teste não pede cartão).
- **Nenhum segredo no repositório** (ele é público): endereço do banco, chave secreta do Stripe, segredo do webhook,
  segredo do cliente Google, credenciais de email, chave do emissor de nota e chave privada de assinatura ficam só no
  servidor, em variáveis de ambiente. O app leva só o endereço da nuvem e a chave pública que confere os tokens.

## Como funciona

### Peças

```
App (Wails, PC da pessoa)                    Nuvem
┌───────────────────────────┐        ┌──────────────────────────────────┐
│ Tela: Conta / Assinatura  │        │ cmd/nuvem (Go)                   │
│        │                  │  HTTPS │  /v1/contas/*   (criar, entrar,  │
│ API local /api/v1/conta/* ├───────►│   código, senha, Google, sair)   │
│ internal/conta            │        │  /v1/sessao/renovar              │
│  - sessão no cofre        │        │  /v1/licenca  /v1/assinar        │
│  - licença assinada       │        │  /v1/portal   /v1/stripe/webhook │
│  - trava das ações        │        └──┬──────────┬──────────┬────────┘
└───────────────────────────┘           │          │          │
                                   ┌────▼───┐ ┌────▼────┐ ┌───▼──────────────┐
                                   │Postgres│ │ Stripe  │ │ Email (SMTP/API) │
                                   └────────┘ └─────────┘ │ Google OAuth     │
                                                          │ Emissor de NFS-e │
                                                          └──────────────────┘
```

- **`cmd/nuvem`** (Go, já previsto no plano P7): servidor HTTP com `net/http`, banco Postgres pelo `pgx`, migrações
  `goose` embutidas (o mesmo padrão do app). Pacotes em `internal/nuvem/...` (contas, sessoes, emails, google,
  licenca, stripe, notas).
- **`internal/conta`** (novo, no app): o cliente da nuvem. A tela fala só com a API local; quem fala com a nuvem é o
  Go do app.

### Banco (Postgres, esquema `applyer`)

- `usuarios(id uuid pk, email citext unique, nome, senha_hash null, google_sub unique null, email_confirmado_em,
  teste_inicio, teste_fim, cortesia bool default false, cortesia_motivo, stripe_cliente, termos_versao,
  termos_aceitos_em, falhas_login, bloqueado_ate, criado_em)`
- `codigos(id, email, tipo: confirmar|recuperar, hash, expira_em, tentativas, usado_em, criado_em)`
- `sessoes(id, usuario_id, familia, hash_renovacao, dispositivo_hash, criada_em, usada_em, revogada_em)`
- `computadores_teste(hash pk, usuario_id null, usado_em)`
- `assinaturas(usuario_id pk, stripe_assinatura, estado, periodo_fim, atualizada_em)` — espelho do Stripe, pelos webhooks
- `eventos_stripe(event_id pk, recebido_em)` — para processar cada webhook uma vez só
- `notas(invoice_id pk, usuario_id, numero, estado, link_pdf, erro, tentativas, criada_em)`

### Senhas, códigos e proteção contra abuso

- Senha: mínimo 10 caracteres, máximo 128; recusada se estiver numa lista embutida das senhas mais comuns (minha
  decisão). Guardada com **Argon2id** (`golang.org/x/crypto/argon2`, parâmetros pelo menos os mínimos do OWASP:
  19 MiB, 2 passadas, 1 fio; o resumo guarda os parâmetros para poder subir depois).
- Códigos de 6 dígitos (confirmar email e recuperar senha): gerados com `crypto/rand`, guardados só como HMAC-SHA256
  com um segredo do servidor, valem **15 min**, no máximo **5 tentativas**; pedir de novo só depois de **60 s**; um
  código novo invalida o anterior.
- Limites por IP e por email (em memória no servidor, minha decisão para uma só instância): criar conta, entrar,
  pedir código. Depois de **10 senhas erradas** seguidas, aquele email espera **15 min** para tentar de novo.
- Respostas iguais para email existente e não existente: "Se o email estiver certo, enviamos um código". Quem tenta
  criar conta com email que já existe recebe um email "você já tem conta; entre ou recupere a senha".

### Criar conta com email e senha

1. Tela "Criar conta": email, senha, aceite dos termos.
2. `POST /api/v1/conta/criar` → nuvem `POST /v1/contas` (cria o usuário sem confirmação e manda o código).
3. A tela pede o código. `POST /api/v1/conta/confirmar {email, codigo}` → `POST /v1/contas/confirmar`, que marca
   `email_confirmado_em` e devolve a sessão.
4. Com a sessão, o app pede a licença. É aqui que o teste começa.

Entrar: `POST /api/v1/conta/entrar` → `POST /v1/contas/entrar` (email + senha). Conta não confirmada recebe novo código
e vai para a tela de código. Esqueci a senha: `POST /v1/contas/recuperar` manda código; `POST /v1/contas/nova-senha
{email, codigo, senha}` troca a senha e **derruba todas as sessões** daquela conta.

### Criar conta ou entrar com Google

O Google não aceita login dentro de janelas embutidas; o padrão para app de PC é o navegador com retorno local (PKCE):

1. A tela chama `POST /api/v1/conta/google`. O Go do app sobe um ouvinte temporário em
   `http://127.0.0.1:<porta livre>/retorno`, gera `state` e o par PKCE e abre o **navegador padrão** em
   `https://accounts.google.com/o/oauth2/v2/auth` com `client_id` (cliente do tipo "Desktop" no Google Cloud),
   `redirect_uri` do ouvinte, `scope=openid email profile`, `code_challenge` e `state`.
2. A pessoa escolhe a conta Google; o Google volta para o ouvinte com `code` e `state` (o app confere o `state`).
3. O ouvinte mostra "Pode voltar para o Applyer" e o app manda `{code, code_verifier, redirect_uri}` para
   `POST {nuvem}/v1/contas/google`. **A nuvem troca o código** (com o segredo do cliente, que fica só nela), confere o
   `id_token` (assinatura pelas chaves públicas do Google, `iss`, `aud`, validade, `email_verified = true`) e:
   - acha o usuário pelo `google_sub`; senão
   - acha pelo email e liga o `google_sub` a essa conta (o Google já garante o email); senão
   - cria a conta já confirmada.
   E devolve a sessão.
4. A tela, que ficou esperando (`GET /api/v1/conta` a cada 2 s, por até 5 min, com botão "Cancelar"), entra.

### Sessão

- A nuvem emite dois tokens:
  - **token de acesso**: assinado com Ed25519 pela nuvem (formato JWT `EdDSA`), vale **15 min**, carrega o id do usuário;
  - **token de renovação**: 32 bytes aleatórios, guardado na nuvem só como SHA-256, **de uso único**: cada renovação
    (`POST /v1/sessao/renovar`) devolve um par novo. Se um token de renovação já usado aparecer de novo, a família
    inteira é revogada (sinal de roubo) e a pessoa precisa entrar de novo. Sessão sem uso por 60 dias expira.
- No app: o token de acesso fica só na memória; o **token de renovação vai para o cofre do sistema** (`go-keyring`,
  igual às chaves de IA). Ao abrir o app, o Go renova; se falhar por falta de internet, a pessoa continua "entrada"
  mas em "ver, sem agir"; se a sessão foi revogada, volta para a tela de entrar.
- Sair: `POST /v1/sessao/sair` revoga a sessão e o app apaga do cofre.
- **Uma conta por instalação.** Trocar de conta = sair e entrar com outra. Os dados locais não são separados por conta
  nesta versão (minha decisão).

### Teste de 1 dia por computador

- **Id do computador:** o app lê o id da máquina (Windows `MachineGuid`; `/etc/machine-id` no Linux;
  `IOPlatformUUID` no Mac; ex.: biblioteca `denisbrodbeck/machineid` com `ProtectedID("applyer")`, que já devolve um
  hash). Só o hash sai do PC.
- Na primeira licença pedida por uma conta, a nuvem grava `teste_inicio = agora`, `teste_fim = agora + 24h`, **se**
  esse hash de computador nunca teve teste (e grava o hash em `computadores_teste`). Se já teve, a conta nasce com o
  teste vencido.
- A conta guarda o teste dela: entrar com a mesma conta em outro PC não ganha outro dia.

### Licença e trava

- `GET {nuvem}/v1/licenca` (com o token de acesso e o hash do computador) devolve um documento **assinado com
  Ed25519** pela nuvem:
  `{usuario, email, estado: "teste"|"assinante"|"bloqueado", teste_fim, periodo_fim, emitida_em, valida_ate}`
  - `assinante` se `cortesia = true` (marcada à mão no banco, sem tela nesta versão), ou se a assinatura está
    `active` ou `trialing`, ou `past_due` dentro de 3 dias de tolerância;
  - `teste` se o email está confirmado e `agora < teste_fim`;
  - senão `bloqueado`.
  - `valida_ate = emitida_em + 30 min`.
- O app guarda a última licença (arquivo na pasta de dados; a assinatura impede editar) e **renova a cada 10 min** e
  ao voltar a internet. A chave pública vem embutida no app.
- **Pode agir** só com licença válida (assinatura confere, `valida_ate` no futuro, usuário = sessão atual) e estado
  `teste` ou `assinante`. Qualquer outra coisa (sem internet há mais de 30 min, teste vencido, assinatura cancelada,
  sem conta) = **ver, sem agir**.
- **Onde a trava fica:** em `API.Chamar` / `rotear` (`internal/api`), não só no `Handler()` HTTP: a CLI
  (`cmd/central-cli`) e o MCP (`cmd/central-mcp`) são processos separados que criam a própria `api.Nova(...)` e chamam
  `Chamar` direto. A trava vale para todo método que não é GET/HEAD/OPTIONS. Passam sempre: `/api/v1/conta/*`,
  `/api/v1/assinatura/*`, preferências da janela/tema e a lista "sempre liberado" abaixo.
  Todo o resto devolve `402 {"erro": "...", "motivo": "teste-acabou" | "sem-assinatura" | "sem-internet" | "sem-conta"}`.
  Ações que começam sozinhas no Go (triagem em segundo plano, preparo automático, envio, retomada de triagens) também
  consultam a trava antes de rodar.
- **CLI e MCP não renovam nada** (minha decisão): só o app renova a sessão e a licença. Eles leem o arquivo de licença
  que o app gravou e conferem a assinatura e o `valida_ate`; licença vencida = ver, sem agir, com a mensagem "abra o
  Applyer para renovar". Motivo: o token de renovação é de uso único; dois processos renovando derrubariam a sessão.
- **Sempre liberado, mesmo bloqueado** (minha decisão, pelo anti-objetivo "bloqueado nunca perde dados" e pela LGPD):
  apagar dados locais, exportar os próprios dados, gerenciar as chaves de IA guardadas, sair da conta, excluir a conta.
- A tela lê `GET /api/v1/conta` → `{entrada, email, estado, teste_fim, motivoBloqueio}` e mostra:
  - no teste: faixa fixa "Seu teste acaba em 5 h 12 min · Assinar";
  - bloqueado: faixa "Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine." + botão; botões de ação
    desabilitados com o motivo; um 402 vindo de qualquer chamada mostra a mesma faixa;
  - sem internet: "Sem conexão: dá para ver, mas não para buscar ou se candidatar."

Sabemos que alguém pode alterar o binário e pular a trava local; com o app local isso não tem como ser impedido 100%.
A assinatura da licença barra o caminho fácil (editar um arquivo); é suficiente nesta fase.

### Assinar e cancelar (Stripe)

- Um produto "Applyer" com um preço mensal recorrente (valor e moeda definidos no painel do Stripe; o código lê o
  `price_id` de variável de ambiente). Começa com **cartão**.
- Assinar: `POST {nuvem}/v1/assinar` cria uma **Stripe Checkout Session** (modo `subscription`, `client_reference_id`
  = usuário, cliente Stripe reaproveitado) e devolve a URL. O app abre no navegador padrão. A página de sucesso diz
  "Pode voltar para o Applyer". O app fica renovando a licença a cada 5 s por até 10 min depois de abrir o Checkout.
- Webhooks em `{nuvem}/v1/stripe/webhook` (assinatura do Stripe conferida; processamento idempotente pelo `event.id`
  em `eventos_stripe`): `checkout.session.completed`, `customer.subscription.created|updated|deleted`,
  `invoice.paid`, `invoice.payment_failed`. Eles atualizam `assinaturas`.
- Cancelar e trocar cartão: `POST {nuvem}/v1/portal` cria uma sessão do **Stripe Customer Portal**; o app abre no navegador.
- A tela "Assinatura" (no menu) mostra estado, próxima cobrança, notas fiscais e os botões Assinar / Gerenciar.

### Emails

- Os emails (código de confirmação, código de recuperação, "você já tem conta", "sua senha foi trocada") saem da
  nuvem por uma interface com duas implementações: **SMTP genérico** (serve para Amazon SES, Brevo, Resend, Mailgun
  etc.) e a falsa dos testes. Minha decisão: SMTP, para não amarrar o código a um fornecedor; qual fornecedor usar é
  do Vinicius.
- Remetente num domínio próprio, com SPF, DKIM e DMARC configurados (senão o código cai no spam).
- Texto simples em português, com o código em destaque e "não pedimos sua senha por email".

### Excluir a conta (LGPD)

Minha decisão: entra no escopo. "Excluir conta" na tela de Conta (com confirmação digitando o email) chama
`DELETE {nuvem}/v1/contas/eu`, que cancela a assinatura no Stripe na hora, revoga as sessões, apaga o usuário, os
códigos e a linha de `assinaturas`. A linha de `computadores_teste` fica (só o hash, com `usuario_id` nulo), para que
excluir e recriar não dê outro teste. As notas fiscais ficam (obrigação fiscal). Os dados locais não são apagados por
isso; a tela oferece "apagar também os dados deste computador" como opção separada.

### Nota fiscal automática

Cada `invoice.paid` do Stripe gera uma NFS-e pelo CNPJ do Vinicius, na nuvem, depois de gravar a assinatura:
- Um emissor por trás de uma interface (`internal/nuvem/notas`), com uma implementação real e uma falsa para os testes.
  Minha sugestão para a real: **NFE.io** (API REST, NFS-e municipal e no padrão nacional). A escolha final é do
  Vinicius com o contador, porque depende do município, do regime tributário e do código de serviço.
- Para a nota, o Checkout do Stripe pede **nome, CPF/CNPJ e endereço de cobrança** (`tax_id_collection` e
  `billing_address_collection` do próprio Stripe). Esses dados ficam no Stripe e no emissor; o app e o nosso banco não
  guardam CPF (regra do produto).
- Emissão idempotente pelo `invoice_id`; falha vira estado `falhou`, com nova tentativa a cada hora por até 24 h e
  alerta no log.
- A tela de Assinatura do app mostra "Notas fiscais" com os links; o emissor também manda por email.
- Cliente fora do Brasil (LATAM): o tratamento da nota de exportação de serviço depende do município; o emissor
  recebe o país do cliente e o contador define a regra. Ponto para o Vinicius confirmar com o contador.

### Termos de uso e privacidade

Minha decisão: o cadastro pede o aceite com links para os dois textos, e a nuvem grava `termos_versao` e
`termos_aceitos_em`. Se a versão mudar, o app pede o aceite de novo no próximo login. O builder escreve um rascunho dos
dois textos em `docs/legal/` para o Vinicius revisar com advogado; até a revisão, a versão é `0-rascunho` e só vale no
build de teste.

### Modos de execução

- **Produção:** o app recebe `APPLYER_NUVEM_URL` e `APPLYER_NUVEM_PUB` (chave pública Ed25519) e o `client_id` do
  Google no build (ldflags). A nuvem lê do ambiente: `DATABASE_URL`, `APPLYER_CHAVE_PRIVADA`, `APPLYER_SEGREDO_CODIGOS`,
  `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_ID`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
  `SMTP_*`, `NOTAS_*`.
- **Teste (Stripe test mode):** mesmo fluxo com chaves de teste e cartão `4242…`, para validar ponta a ponta.
- **Falso (testes automáticos e dev):** `internal/conta` tem uma implementação falsa (contas em memória, código de
  confirmação fixo, relógio controlável, licença assinada por chave de teste) usada pelo `go test`, Vitest e Playwright.
  Ela fica atrás de uma tag de build, fora do executável de produção.
- O `cmd/nuvem` tem testes com Postgres de verdade num contêiner de teste, Google falso (servidor de teste que emite
  `id_token` com chave de teste), email falso e webhooks do Stripe assinados com segredo de teste. Nenhum teste
  automático fala com Stripe, Google ou email de verdade.
- Antes de ligar a cobrança real: uma revisão de segurança da nuvem (skill de revisão de segurança) com foco em login,
  sessão, códigos e webhooks.

## Restrições

- Segue o AGENTS.md: regra nova só no Go; o Node é só leitura; nada de editar `funcoes.json` ou os golden.
- O contrato `api/openapi.yaml` ganha as rotas `/conta/*` e `/assinatura/*` e o 402; a nuvem ganha o seu próprio
  contrato (`api/nuvem.yaml`).
- Repositório público: só dados fictícios nos testes; gitleaks continua no CI.
- **Hospedagem na VPS do Vinicius** (decisão dele): Postgres e `cmd/nuvem` rodam na VPS dele, num
  `docker compose` versionado em `deploy/nuvem/`:
  - `postgres` (imagem oficial, versão fixa), **sem porta aberta para a internet**, só na rede interna do compose;
    volume persistente;
  - `nuvem` (imagem do `cmd/nuvem`), com as variáveis num arquivo `.env` que fica só na VPS (fora do git, permissão 600);
  - `caddy` na frente, com HTTPS automático (Let's Encrypt) no domínio da nuvem (ex.: `api.<dominio>`);
  - **backup**: `pg_dump` diário compactado, guardado 14 dias na VPS e copiado para fora dela (destino a definir com
    ele: outro servidor ou armazenamento de objetos); um teste de restauração faz parte do pronto;
  - firewall só com 22 (SSH), 80 e 443 abertas; atualizações de segurança do sistema ligadas.
  Claude instala e configura por SSH, com chave SSH que o Vinicius autoriza (nunca senha no chat). Os segredos do
  `.env` o Vinicius cola direto na VPS (ou Claude gera os que são nossos, como a chave Ed25519 e o segredo dos códigos,
  dentro da própria VPS, sem passar pelo chat).
- Contas de terceiros são com o Vinicius (precisam do CPF/CNPJ dele): a conta Stripe, o cliente OAuth "Desktop" no
  Google Cloud (com a tela de consentimento publicada), um fornecedor de email com o domínio configurado (SPF, DKIM,
  DMARC), a conta no emissor de nota fiscal (com o contador: município, regime, código de serviço) e o domínio. O plano
  lista cada passo para ele; Claude não cria conta nem digita chave de terceiros.

## Deixado para quem constrói (decisões minhas, marcadas para o Vinicius conferir)

- Código de 6 dígitos por email em vez de link de confirmação.
- Senha mínima de 10 caracteres + lista de senhas comuns; Argon2id com os mínimos do OWASP.
- Tempos: código 15 min / 5 tentativas / reenviar em 60 s; bloqueio de 15 min depois de 10 senhas erradas; token de
  acesso 15 min; sessão parada expira em 60 dias; licença 30 min, renovada a cada 10; 3 dias de tolerância para
  pagamento atrasado.
- Conta Google com o mesmo email de uma conta de senha vira a mesma conta.
- Limites de tentativas em memória (uma instância da nuvem); se um dia houver várias, passa para o Postgres.
- Email por SMTP genérico; fornecedor a escolher.
- Uma conta por instalação; dados locais não separados por conta.
- Na VPS: docker compose com Caddy para HTTPS; backup diário com 14 dias de retenção.
- Textos das telas, da faixa e dos emails: o builder escreve, simples, no tom do app.
- A ordem com a branch `app/onboarding-busca-triagem`: a tela de conta entra na frente do primeiro acesso; se a branch
  dele ainda não estiver na `reescrita-go`, a porta fica no `App.tsx` antes de qualquer tela e o encaixe com o primeiro
  acesso vem quando as duas se juntarem.
