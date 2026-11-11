# VPS e contas externas — plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Passos marcados **[Vini]** só ele faz; pare e peça, com a lista curta do que falta.

**Goal:** A nuvem (`cmd/nuvem` + Postgres) rodando na VPS do Vinicius com HTTPS e backup, ligada ao Google, ao email, ao Stripe (modo de teste primeiro) e ao emissor de NFS-e, e o app de teste falando com ela de ponta a ponta.

**Architecture:** `docker compose` versionado em `deploy/nuvem/` (postgres + nuvem + caddy + backup), segredos num `.env` que só existe na VPS. Claude opera por SSH com chave autorizada pelo Vinicius e pelos painéis web no Chrome dele, onde ele já está logado.

**Tech Stack:** Docker + Compose, `postgres:17`, `caddy:2`, imagem do `cmd/nuvem` (plano da nuvem, Task 12), `pg_dump`, `ufw`.

**Spec:** `docs/superpowers/specs/2026-11-11-conta-e-assinatura-design.md` · **Depende de:** plano da nuvem completo (imagem e `api/nuvem.yaml`).

## Global Constraints

- Nenhum segredo passa pelo chat nem entra no git. Segredos de terceiros: o Vinicius cola direto no `.env` da VPS. Segredos nossos (`APPLYER_CHAVE_PRIVADA`, `APPLYER_SEGREDO_CODIGOS`, senha do Postgres): Claude gera **dentro da VPS** (`openssl`/binário da nuvem) e grava no `.env` sem imprimir.
- Claude não cria conta em serviço de terceiros, não digita senha, chave, CNPJ/CPF ou dado bancário, não aceita termos nem paga nada. Isso é **[Vini]**.
- Postgres nunca exposto na internet. Firewall: só 22, 80, 443.
- Stripe começa em **modo de teste**; virar para produção é decisão explícita do Vinicius.
- Antes de qualquer comando que muda a VPS, Claude confere o que já roda nela (outros serviços dele não podem cair).

## Review Focus

- A VPS já hospeda outras coisas nas portas 80/443 → o Caddy não pode tomar as portas de um serviço existente; descobrir antes e, se houver proxy, encaixar a nuvem nele → Task 1, passo de inventário.
- Backup que nunca foi restaurado não é backup → Task 3 restaura num banco descartável e compara contagens.
- Reiniciar a VPS → tudo volta sozinho (`restart: unless-stopped`) e o HTTPS renova sozinho → Task 3, passo de reinício.
- Webhook do Stripe apontando para o endereço errado ou com segredo trocado → nenhum pagamento libera ninguém; o teste de ponta a ponta tem de ver a licença virar `assinante` → Task 6.
- Email de código caindo no spam → teste real com Gmail e Outlook antes de liberar → Task 5.

---

### Task 1: Acesso e inventário da VPS

- [ ] **Step 1 [Vini]:** Informar no chat o endereço (IP ou nome) da VPS, o usuário SSH e o sistema (ex.: Ubuntu 24.04). Autorizar a chave pública que Claude gerar no PC (`ssh-keygen -t ed25519 -f ~/.ssh/applyer_vps -C applyer-claude`): Claude mostra só a **chave pública**; o Vinicius cola em `~/.ssh/authorized_keys` da VPS.
- [ ] **Step 2:** `ssh -i ~/.ssh/applyer_vps <usuario>@<vps> 'uname -a; docker --version; ss -tlnp; ufw status'` → anotar o que roda, que portas estão ocupadas e se já há Docker/proxy.
- [ ] **Step 3:** Se 80/443 já estão com outro proxy (nginx, Traefik, Caddy), o plano troca o serviço `caddy` por uma entrada nesse proxy; registrar a decisão em `deploy/nuvem/README.md`.
- [ ] **Step 4:** Instalar Docker Engine + Compose plugin (repositório oficial) se faltar; `ufw allow 22,80,443/tcp && ufw enable` (só se o ufw não estiver gerenciado de outro jeito); atualizações automáticas de segurança (`unattended-upgrades`).

### Task 2: Domínio

- [ ] **Step 1 [Vini]:** Dizer qual domínio usar e onde ele é gerenciado (Registro.br, Cloudflare…), e deixar o painel aberto no Chrome.
- [ ] **Step 2:** Pelo Chrome, criar o registro `A` (e `AAAA` se houver IPv6) de `api.<dominio>` para a VPS. Conferir com `nslookup api.<dominio>`.

### Task 3: Compose, HTTPS e backup

**Files:**
- Create: `deploy/nuvem/compose.yml`, `deploy/nuvem/Caddyfile`, `deploy/nuvem/backup.sh`, `deploy/nuvem/.env.exemplo` (só nomes das variáveis, sem valores), `deploy/nuvem/README.md` (como subir, atualizar, restaurar)

**Interfaces:**
- `compose.yml`: serviços `postgres` (volume `pg`, rede interna, healthcheck `pg_isready`), `nuvem` (imagem `ghcr.io/frf-tecnology/applyer-nuvem:<tag>`, `env_file: .env`, depende do postgres saudável), `caddy` (portas 80/443, volume de certificados), `backup` (cron diário 03:00: `pg_dump -Fc` → `/backups/applyer-AAAA-MM-DD.dump`, apaga com mais de 14 dias). Todos `restart: unless-stopped`.
- `Caddyfile`: `api.<dominio> { reverse_proxy nuvem:8080 }`.

- [ ] **Step 1:** Commitar os arquivos; publicar a imagem da nuvem no GHCR pelo CI (job do plano da nuvem).
- [ ] **Step 2:** Na VPS: `/opt/applyer/` com os arquivos; gerar dentro da VPS a senha do Postgres, `APPLYER_SEGREDO_CODIGOS` (`openssl rand -base64 32`) e o par Ed25519 (`docker run --rm -v /opt/applyer:/saida <imagem> /nuvem chaves -pub /saida/licenca.pub >> .env`, subcomando do plano da nuvem, Task 12); `chmod 600 .env`.
- [ ] **Step 3:** `docker compose up -d`; `curl https://api.<dominio>/v1/saude` → `{"ok":true}` com certificado válido.
- [ ] **Step 4:** Copiar `licenca.pub` (é pública) para o build do app (`APPLYER_NUVEM_PUB`).
- [ ] **Step 5:** Backup: rodar o job à mão, restaurar o dump num contêiner Postgres descartável, comparar a contagem de linhas de `usuarios`. **[Vini]** escolher o destino da cópia fora da VPS (outro servidor ou armazenamento de objetos); Claude configura o envio.
- [ ] **Step 6:** `sudo reboot` (com aviso ao Vinicius antes); depois conferir que `saude` responde de novo sem intervenção.

### Task 4: Login com Google

- [ ] **Step 1 [Vini]:** Abrir o Google Cloud Console no Chrome com a conta dele e criar (ou escolher) um projeto "Applyer". Aceitar termos se aparecer.
- [ ] **Step 2:** Pelo Chrome: tela de consentimento OAuth (tipo Externo, nome "Applyer", email de suporte dele, escopos `openid email profile`, links de termos e privacidade), e credencial **OAuth client ID** tipo **Desktop app**.
- [ ] **Step 3 [Vini]:** Publicar a tela de consentimento (sai do modo teste) e copiar o Client ID e o Client secret direto para o `.env` da VPS (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`). O Client ID também vai para o build do app (`GoogleClientID`), por ser público.

### Task 5: Email

- [ ] **Step 1 [Vini]:** Criar a conta no fornecedor de email escolhido (sugestão: Amazon SES, Brevo ou Resend) e deixar o painel aberto.
- [ ] **Step 2:** Pelo Chrome: verificar o domínio no fornecedor e criar no painel do domínio os registros SPF, DKIM e DMARC (`v=DMARC1; p=quarantine; rua=mailto:<email dele>`); remetente `nao-responda@<dominio>`.
- [ ] **Step 3 [Vini]:** Gerar as credenciais SMTP e colar no `.env` (`SMTP_HOST`, `SMTP_PORTA`, `SMTP_USUARIO`, `SMTP_SENHA`, `SMTP_REMETENTE`).
- [ ] **Step 4:** `docker compose up -d nuvem`; criar uma conta de teste com um Gmail e um Outlook do Vinicius → o código chega na caixa de entrada (não no spam). Ajustar até chegar.

### Task 6: Stripe (modo de teste)

- [ ] **Step 1 [Vini]:** Criar a conta Stripe com o CNPJ, dados bancários e documentos; deixar o painel aberto em **modo de teste**.
- [ ] **Step 2:** Pelo Chrome (modo de teste): produto "Applyer", preço mensal recorrente (**[Vini]** diz o valor e a moeda); ligar coleta de CPF/CNPJ (Tax ID) no Checkout; configurar o Customer Portal (cancelar, trocar cartão, ver faturas); criar o endpoint de webhook `https://api.<dominio>/v1/stripe/webhook` com os eventos `checkout.session.completed`, `customer.subscription.created`, `customer.subscription.updated`, `customer.subscription.deleted`, `invoice.paid`, `invoice.payment_failed`.
- [ ] **Step 3 [Vini]:** Colar no `.env`: `STRIPE_SECRET_KEY` (sk_test_…), `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRICE_ID`.
- [ ] **Step 4:** Ponta a ponta com o app de teste (build com o endereço da nuvem): criar conta → teste → assinar com o cartão `4242 4242 4242 4242` → licença vira `assinante` em menos de 1 min → cancelar no portal → no fim do período, `bloqueado`. Registrar capturas.

### Task 7: Nota fiscal

- [ ] **Step 1 [Vini]:** Com o contador: confirmar município, regime, código de serviço e o tratamento para clientes fora do Brasil; criar a conta no emissor (sugestão: NFE.io), cadastrar a empresa e o certificado digital.
- [ ] **Step 2 [Vini]:** Colar no `.env`: `NOTAS_PROVEDOR=nfeio`, `NOTAS_CHAVE`, `NOTAS_EMPRESA`, `NOTAS_SERVICO`.
- [ ] **Step 3:** Pagamento de teste no Stripe → nota emitida no ambiente de homologação do emissor; link aparece na tela Assinatura do app.

### Task 8: Virar para produção

- [ ] **Step 1 [Vini]:** Decidir a data; trocar no `.env` as chaves do Stripe de teste pelas de produção e criar o webhook de produção (Claude refaz o Step 2 da Task 6 no modo produção, sem tocar em chaves); emissor em produção.
- [ ] **Step 2:** Termos e privacidade revisados pelo advogado → `APPLYER_TERMOS_VERSAO=1`; build do app de produção com as três variáveis; marcar cortesia (`update applyer.usuarios set cortesia = true, cortesia_motivo = 'testador' where email = …`) para o Vinicius e os testadores que ele listar.
- [ ] **Step 3:** Um pagamento real pequeno do próprio Vinicius, com nota emitida, e o estorno pelo painel (**[Vini]**).
