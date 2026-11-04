---
description: Ciclo completo de candidaturas com a Central. Confere o perfil (perguntando o que falta), busca vagas, faz a triagem, aplica sozinho e registra tudo
argument-hint: "[máximo de envios] [dias de busca] [pode ir direto]"
---
# skill-job: candidaturas automáticas ligadas à Central

Argumentos: $ARGUMENTS. Padrão: até 25 envios, vagas dos últimos 3 dias. Com "pode ir direto", pule a confirmação do perfil, se ele estiver completo.

Siga o `CLAUDE.md` da Central à risca. Toda gravação passa por `node ferramentas/cli.mjs` (rode da pasta `central-candidaturas`). Nunca edite `dados/*.json` à mão.

## 0. Central no ar
1. Veja a porta em `dados/.porta` (padrão 4321) e teste `curl -s localhost:<porta>/api/versao`.
2. Se a Central não responder, suba-a em segundo plano (`node servidor/index.mjs`) ou peça para a pessoa abrir `iniciar.bat`.
3. Diga o link: http://localhost:<porta>. A pessoa acompanha tudo ao vivo por ali.
4. A CLI funciona mesmo com a Central fechada. Só os scripts de busca precisam dela.

## 1. Perfil (perguntas)
Leia o perfil com `cli.mjs perfil`.
- **Campo obrigatório vazio:** pergunte.
- **Perfil completo:** mostre um resumo de 5 linhas e pergunte só "Algo mudou?" (pule com "pode ir direto").

Use AskUserQuestion, com até 4 perguntas por chamada e opções prontas quando fizer sentido. Grave cada bloco na hora com `cli.mjs perfil --json '{...}'`. Esse comando mescla os campos e recusa CPF, RG e senha.

| Bloco | Campos do perfil | Como perguntar |
|---|---|---|
| Identidade | `nome` (completo), `email`, `telefone`, `cidade`, `linkedin`, `github` | texto livre |
| Cargo | `cargo`, `experienciaAnos`, `stackPrincipal`, `stackSecundaria` | stack lida do currículo; peça confirmação |
| Dinheiro | `pretensaoCLT`, `pretensaoPJ` (só números), `salarioAtual`, `contratos` | opções: CLT / PJ / ambos |
| Expectativas | `filtros.niveis`, `filtros.modelo`, `filtros.pularSe`, `palavrasBusca` | níveis Júnior/Pleno/Sênior; Remoto/Híbrido/Presencial; stacks a evitar |
| Formação e idiomas | `formacao` (cursando? conclusão?), `ingles`, `espanhol`, `inicio` | opções de nível |
| Funcionamento | `modo` (`revisar` = aplico só o que você aprovar; `direto` = aplico o grupo A sozinho), `diasFollowUp` | opções |
| Currículos | `curriculos.PT` / `curriculos.EN` (PDFs em `dados/curriculos/`), `curriculosLinkedin.PT` / `.EN` (início do nome do arquivo no LinkedIn) | confira se os arquivos existem |
| Respostas padrão | `respostasPadrao[]`: `{pergunta, resposta}` para tecnologias que surgem nos formulários | grave toda resposta nova dada no meio de uma candidatura |

**Nunca pergunte nem guarde** CPF, RG, senhas ou dados bancários. Esses ficam com a pessoa na hora de enviar.

Antes de aplicar, leia o currículo PDF. É a fonte do que é verdade sobre a experiência.

## 2. Buscar
1. Abra uma aba do Chrome logada em linkedin.com.
2. Gere o script de busca: `cli.mjs script busca-linkedin --dias <N> --palavras '<perfil.palavrasBusca em JSON>'`.
3. Injete com `javascript_tool` e leia `window.__CENTRAL_RESULTADO` até deixar de ser `'rodando'`.
4. Se o resultado começar com `PENDENTE`, o Chrome bloqueou localhost. Leia `window.__CENTRAL_VAGAS` em pedaços curtos (a saída do javascript_tool corta em cerca de 1000 caracteres e bloqueia `? & = @`). Monte um JSON e grave com `cli.mjs add --arquivo <scratchpad>/vagas.json`. Não clique no aviso de permissão: avise a pessoa.
5. Gupy (`script busca-gupy`) e InHire (`script busca-inhire`) seguem o mesmo esquema. Veja `automacao/como-aplicar.md`.
6. Atualize a data da busca: `cli.mjs plataforma --json '{"id":"linkedin","ultimaVarredura":"<hoje>"}'`.

## 3. Triagem
1. Liste as vagas: `cli.mjs listar --etapa descoberta`.
2. Leia a descrição de cada vaga com `LI.desc(id, 0, 900)` e depois `(900, 900)`. Pule o que cair em `filtros.pularSe`:
   `cli.mjs mover <id> pulada --motivo <Sênior|Inglês|Diploma|Stack|Presencial/Local|Documento|Duplicada|Vaga fechada|Outro>`.
3. Se `modo` = `direto`, o grupo A segue para aplicação. Grupos B e C e as dúvidas ficam em Descoberta, com o alerta: `editar <id> --json '{"motivoClaude":"..."}'`.
4. Se `modo` = `revisar`, aplique só a etapa `aplicar`.
5. A mesma empresa repostando a vaga em várias cidades conta uma vez. Marque as cópias como Duplicada.

## 4. Aplicar (uma vaga por vez)
1. **Duplicata:** `cli.mjs dup "<empresa>" "<título>" --excluir <id>`. Se houver uma "enviada <data>", mova para `pulada --motivo Duplicada`.
2. **LinkedIn, Candidatura simplificada:**
   - Uma vez por sessão, gere o helper com `cli.mjs script linkedin-auto` e injete. Ele fica salvo em `localStorage.__LI`.
   - Em cada vaga: navegue para `/jobs/view/<id>/` e rode `Function(localStorage.__LI)(); await LI.open('PT'|'EN')`. O idioma é o da vaga.
   - `REVIEW`: confira o currículo citado e rode `await LI.send(id)`. Só conta como enviada com `applied true`.
   - `STOP`: responda os campos com o perfil e `respostasPadrao` (`LI.tset`, `LI.sel`, `LI.pick`), depois `await LI.auto(lang)` de novo.
   - Pergunta sem resposta no perfil e sem base no currículo: responda "não" ou "0". Se a vaga depender dela, pare e pergunte à pessoa. Depois grave a resposta em `respostasPadrao`.
   - Se a URL redirecionar para "vagas semelhantes" ou o `jobState` não for LISTED, mova para `pulada --motivo "Vaga fechada"`.
3. **Site externo** (link em `applyMethod...OffsiteApply.companyApplyUrl`): preencha com o perfil e anexe o PDF com `find` + `file_upload`.
   - Banner de cookies: recuse os não essenciais.
   - Site que bloqueia screenshot: leia e preencha via JS.
4. **Registrar.** Envio confirmado:
   `cli.mjs mover <id> enviada --rodada <n> --curriculo PT|EN --pretensao <valor usado> --respostas "<respostas não óbvias>"`.
5. **Travou** (login, criar conta, CPF/RG, captcha, teste ou vídeo pessoal): preencha o resto, deixe a aba aberta e rode
   `cli.mjs mover <id> bloqueada --pendencia "<o que falta, em linguagem simples> — <link>"`.

## 5. Fechar a rodada
1. Registre: `cli.mjs rodada --json '{"n":<próximo>,"fontes":[...],"encontradas":N,"enviadas":N,"puladas":N,"bloqueadas":N,"resumo":"<uma frase>"}'`.
2. Exporte o histórico com `cli.mjs exportar-md ../historico-central.md` e faça o backup com `cli.mjs backup`.
3. Responda em poucas linhas:
   - quantas foram enviadas (empresa e vaga);
   - o que precisa da pessoa (bloqueadas, com o motivo);
   - quantas esperam triagem.
4. Follow-ups vencidos (`cli.mjs followup`): só sugira o texto. Envie apenas depois de um "pode mandar".

## Erros que já aconteceram (confira sempre)
| Situação | Faça |
|---|---|
| O LinkedIn preenche sozinho anos de uma tecnologia que a pessoa não tem (ex.: Kotlin = 3) | Antes do `send`, confira cada número contra o perfil e corrija para 0 |
| Campo opcional "salário atual" recebeu a pretensão | Use `salarioAtual`, ou deixe em branco se o campo for opcional |
| Campo de pretensão só numérico | Só dígitos. Rótulo PJ ou cooperado = `pretensaoPJ`; caso contrário, `pretensaoCLT` |
| Formulário de uma página só (sem botão Avançar) | O `auto` já trata como REVIEW. Confira os campos antes de enviar |
| Banner de cookies cobre o botão de enviar | Recuse os cookies e clique de novo |
| A vaga pede "graduação concluída" e a pessoa está cursando | `pulada --motivo Diploma`. Não marque "concluída" |
| Vaga em inglês com inglês avançado obrigatório e perfil Conversacional | `pulada --motivo Inglês` |
| A pessoa respondeu algo novo no chat | Grave na hora com `cli.mjs perfil --json` (`respostasPadrao`) |

Nunca invente experiência, nunca envie mensagens sem um "pode mandar" e nunca clique em prompts de permissão do navegador.
