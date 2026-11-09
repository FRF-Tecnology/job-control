---
description: Lê os emails de processos seletivos (só leitura) e atualiza as vagas sozinho
---
**Só leitura.** Nunca responda, arquive, apague ou marque emails. Rascunho de resposta só se a pessoa pedir, e enviar só com "pode mandar".

1. Desde quando ler: `node ferramentas/cli.mjs emails --ultimo` (data do email mais recente já registrado; se não houver, use 30 dias).
2. Onde ler, nesta ordem:
   - **Conector do Gmail** (se estiver ligado nesta sessão): busque
     `from:(greenhouse.io OR lever.co OR ashbyhq.com OR gupy.io OR inhire.app OR workable.com OR smartrecruiters.com OR myworkdayjobs.com OR jobs-noreply@linkedin.com OR hackerrank.com OR codility.com) OR subject:(application OR candidatura OR entrevista OR interview OR assessment OR offer OR "processo seletivo")`
     com `after:<data>`.
   - **Gmail no navegador** (Claude in Chrome, conta já logada pela pessoa): só com o "pode ler" dela nesta sessão.
     Faça a mesma busca na caixa de pesquisa e abra cada resultado só para ler. Não clique em nada que mude o email.
   - Sem nenhum dos dois: peça para a pessoa colar os emails ou encaminhar para o endereço do produto (quando existir).
3. Para cada email, monte o item (o id da mensagem é obrigatório: é ele que evita processar duas vezes):
   ```json
   { "email": { "idMensagem": "<id do Gmail>", "de": "<remetente>", "assunto": "<assunto>", "data": "<data>" },
     "classificacao": { "tipo": "confirmacao|rejeicao|entrevista|teste|oferta|outro", "vagaId": "<id ou null>",
                        "resumo": "<uma frase, sem dados pessoais>", "proximoPasso": "<se houver>" } }
   ```
   - `tipo`: confirmacao = recebemos sua candidatura; rejeicao; entrevista = convite ou agendamento; teste; oferta; outro.
   - `vagaId`: só se tiver certeza (confira em `cli.mjs listar --etapa enviada` e `--etapa em-processo`). Na dúvida, `null`:
     a Central (Node) tenta achar pela empresa e, se não der, o email vai para "Emails sem vaga" na tela Hoje.
   - Nunca copie o corpo do email. Nada de CPF, RG, telefone ou endereço no resumo.
4. Grave tudo de uma vez: salve a lista num arquivo temporário e rode `node ferramentas/cli.mjs email --arquivo <arquivo>`.
   Rodar de novo com os mesmos emails não duplica nada ("já registrado").
5. Responda com o que mudou: recusas, convites (com o próximo passo, ex.: "agendar pelo Calendly"), testes, ofertas
   e os emails que ficaram sem vaga.
