---
description: Mostra as vagas que esperam decisão e registra o que a pessoa decidir no chat
---
1. `node ferramentas/cli.mjs listar --etapa descoberta`. Agrupe por aderência (A, B, C) e mostre uma lista curta: empresa, vaga e alerta (`motivoClaude`).
2. Lembre que a forma mais rápida é a tela Triagem (http://localhost:4321/#/triagem): A aplica, P pula.
3. Se a pessoa decidir pelo chat:
   - **pular** ("pula a X por inglês"): grave com `cli.mjs mover <id> pulada --motivo <motivo>`;
   - **aplicar** ("aplica as do grupo A"): **não grave**. Só a pessoa aprova uma vaga para aplicar, e a Central recusa essa gravação vinda do assistente. Peça que ela aperte A nessas vagas na tela Triagem (http://localhost:4321/#/triagem) e liste quais são.
