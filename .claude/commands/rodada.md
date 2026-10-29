---
description: Rodada completa — status, busca, aplicação, follow-ups e histórico
argument-hint: "[máximo de envios]"
---
Faça uma rodada completa, nesta ordem:
1. Confira o que estava `bloqueada`. Se a pessoa resolveu, ela marca "Enviei" na tela; senão, siga.
2. `/status` (resumido).
3. `/buscar todas 3`.
4. `/aplicar $ARGUMENTS`.
5. `/followup` (só listar).
6. Feche a rodada:
   - `cli.mjs rodada --json '{"n":<próximo número>,"fontes":[...],"encontradas":N,"enviadas":N,"puladas":N,"bloqueadas":N,"resumo":"<uma frase>"}'`
   - `cli.mjs exportar-md ../historico-central.md`
   - `cli.mjs backup`
7. Responda em poucas linhas: enviadas, o que precisa da pessoa e quantas esperam triagem.
