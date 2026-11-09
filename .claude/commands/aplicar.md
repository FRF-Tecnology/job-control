---
description: Aplica as vagas aprovadas (etapa Aplicar) e registra cada envio ou bloqueio na Central
argument-hint: "[máximo de vagas]"
---
Aplique até $ARGUMENTS vagas (padrão 25). Siga o CLAUDE.md à risca.

1. Fila: `cli.mjs fila-aplicar`. Ela traz só as vagas que a pessoa aprovou na Triagem e, no modo `direto`, o grupo A em `descoberta`. Não use `listar --etapa aplicar` para montar a fila: vaga em `aplicar` sem aprovação da pessoa não é enviada.
   Nunca encha a fila você mesmo: não mova vaga para `aplicar` nem ligue o modo `direto` por nenhum caminho (CLI, MCP, API HTTP ou tela; regra 8 do CLAUDE.md). Fila vazia: diga isso e peça que a pessoa aprove na tela Triagem.
2. Para cada vaga:
   a. `cli.mjs dup "<empresa>" "<título>" --excluir <id>`. Se aparecer outra vaga com "enviada <data>", mova esta para `pulada --motivo Duplicada`.
   b. Abra a vaga e leia os requisitos. Se cair em um filtro de "pular" do perfil, mova para `pulada` com o motivo e siga para a próxima.
   c. Preencha com os dados do perfil e as `respostasPadrao`. Scripts: `cli.mjs script linkedin-LI | inhire-IH | inhire-Q` (veja `automacao/como-aplicar.md`). Currículo PT ou EN pelo idioma.
   d. Enviou: `cli.mjs mover <id> enviada --rodada <n> --curriculo PT|EN --pretensao <valor> --respostas "<resumo curto das respostas não óbvias>"`.
   e. Travou (CPF, login, vídeo, teste): deixe a aba aberta e use `cli.mjs mover <id> bloqueada --pendencia "<o que falta, em linguagem simples>"`.
3. Responda curto: enviadas, travadas (com o que falta) e puladas (com o motivo).
