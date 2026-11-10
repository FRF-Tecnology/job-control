---
description: Busca vagas novas nas plataformas automatizadas e coloca na Descoberta da Central (Node)
argument-hint: "[linkedin|gupy|inhire|todas] [dias]"
---
Busque vagas novas e grave na Central. Argumentos: $ARGUMENTS (padrão: todas, 3 dias).

1. Leia `dados/perfil.json` (filtros, stack, inglês) e confirme que a Central está aberta (`curl -s localhost:$(cat dados/.porta 2>/dev/null || echo 4321)/api/versao`). Se não estiver, peça para a pessoa abrir `iniciar.bat`. O `curl` é só para ler: nunca grave pela API HTTP (regra 8 do CLAUDE.md).
2. Para cada plataforma pedida (status `automatizada` em `dados/plataformas.json`):
   - **LinkedIn**: aba logada em linkedin.com → `node ferramentas/cli.mjs script busca-linkedin --dias N --palavras '<termos do perfil>'` → injete com javascript_tool.
   - **Gupy**: aba em portal.gupy.io → `script busca-gupy --dias N`.
   - **InHire**: aba em qualquer `*.inhire.app` → `script busca-inhire --dias N`.
   - **Ordem de prioridade (`perfil.prioridadeVagas`)**: rode primeiro com `perfil.palavrasBusca` (USD LATAM de empresas dos EUA, depois empresas LATAM + Europa que contratam overseas). Só depois, com o limite do dia que sobrar, rode `perfil.palavrasBuscaNacional` (vagas brasileiras CLT/PJ). Na aplicação, siga a mesma ordem: grupo 1, depois 2, depois nacionais. Para o grupo 1, rode também com `--geo 91000011` (localização "Latin America" no LinkedIn) e descarte vagas restritas a outro país (México, Colômbia, Argentina...).
3. Os scripts já filtram o óbvio e sugerem grupo A, B ou C. Depois, revise com `cli.mjs listar --etapa descoberta`:
   - vaga fora do perfil (sênior disfarçado, presencial, stack errada): `mover <id> pulada --motivo <motivo padrão>`;
   - vaga com ponto de atenção: complete com `editar <id> --json '{"motivoClaude":"..."}'`.
   Para ler a descrição de uma vaga no LinkedIn, use a API de detalhe (`/voyager/api/jobs/jobPostings/<id>`).
4. Atualize `ultimaVarredura` de cada plataforma: `cli.mjs plataforma --json '{"id":"linkedin","ultimaVarredura":"<hoje>"}'`.
5. Se a pessoa pedir uma plataforma `a-testar`, explore manualmente, anote "como aplicar" e "limitações" com `cli.mjs plataforma` e grave as vagas com `cli.mjs add`, sempre em `descoberta`.
6. Responda curto: quantas vagas novas por grupo, e que ela pode decidir na tela **Triagem** (ou em `/triagem`).
