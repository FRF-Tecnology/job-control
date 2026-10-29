---
description: Mostra as vagas que esperam decisão e registra o que a pessoa decidir no chat
---
1. `node ferramentas/cli.mjs listar --etapa descoberta`. Agrupe por aderência (A, B, C) e mostre uma lista curta: empresa, vaga e alerta (`motivoClaude`).
2. Lembre que a forma mais rápida é a tela Triagem (http://localhost:4321/#/triagem): A aplica, P pula.
3. Se a pessoa decidir pelo chat ("aplica as do grupo A", "pula a X por inglês"), grave com `cli.mjs mover <id> aplicar` ou `mover <id> pulada --motivo <motivo>`.
