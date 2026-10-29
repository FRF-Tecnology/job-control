---
description: Aplica as vagas aprovadas (etapa Aplicar) e registra cada envio ou bloqueio na Central
argument-hint: "[máximo de vagas]"
---
Aplique até $ARGUMENTS vagas (padrão 25). Siga o CLAUDE.md à risca.

1. Fila: `cli.mjs listar --etapa aplicar`. Se `perfil.modo` = `direto`, inclua também as do grupo A em `descoberta`.
2. Para cada vaga:
   a. `cli.mjs dup "<empresa>" "<título>"`. Se já existe com outro id e já foi enviada, mova esta para `pulada --motivo Duplicada`.
   b. Abra a vaga e leia os requisitos. Se cair em um filtro de "pular" do perfil, mova para `pulada` com o motivo e siga para a próxima.
   c. Preencha com os dados do perfil e as `respostasPadrao`. Scripts: `cli.mjs script linkedin-LI | inhire-IH | inhire-Q` (veja `automacao/como-aplicar.md`). Currículo PT ou EN pelo idioma.
   d. Enviou: `cli.mjs mover <id> enviada --rodada <n> --curriculo PT|EN --pretensao <valor> --respostas "<resumo curto das respostas não óbvias>"`.
   e. Travou (CPF, login, vídeo, teste): deixe a aba aberta e use `cli.mjs mover <id> bloqueada --pendencia "<o que falta, em linguagem simples>"`.
3. Responda curto: enviadas, travadas (com o que falta) e puladas (com o motivo).
