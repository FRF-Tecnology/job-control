---
description: Confere retornos (vagas encerradas, testes, entrevistas) e atualiza a linha do tempo
---
1. LinkedIn: em "Minhas vagas → Candidaturas", veja quais foram visualizadas ou encerradas. Para cada mudança:
   `cli.mjs evento <id> retorno "<o que aconteceu>"` e, se encerrou, `mover <id> encerrada --motivo "Vaga fechada"`.
2. Gupy (logado): etapa de cada candidatura.
   - Teste pendente: `mover <id> em-processo --sub teste --texto "<qual teste>"` (o teste em si é da pessoa).
   - Reprovada: `mover <id> encerrada --motivo Reprovada`.
3. E-mails de retorno: só se a pessoa pedir e o Gmail estiver conectado.
4. Sem retorno há mais de 30 dias: sugira encerrar como "Sem retorno". Não encerre sozinho.
5. Responda com o que mudou.
