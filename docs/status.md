# Status do Applyer

Atualizado em 10/11/2026 pelo PO. Uma página; o PO atualiza no fim do dia. Regras do time: `AGENTS.md`.

## Meta da vez

**Meta 1: o Vini envia 1 candidatura real pelo Applyer, numa vaga que ele escolheu, no Mac dele.**
Depois, meta 2: o Vini usa o app por 5 dias seguidos, com a Lista do dia.

| Fatia | Dono | Estado |
|---|---|---|
| jc-a2y.6.16 pacote do Mac com envio (`Applyer-macos-arm64-envio.zip`) | Kauai | PR #142 em CI; Fallen revisa (nível 2) |
| jc-a2y.6.5 sessão real no Mac: Lista do dia → envio → conferência | Vini, Faker de prontidão | espera o #142 e o horário do Vini |
| jc-a2y.6.4 revisão final de segurança da P5 | Fallen | depois da .6.5 |

**Máquina de teste e de uso real:** o Mac do Vini (Apple Silicon). No Windows dele, nenhuma janela de teste.

## Pronto até aqui
- P0 a P4 fechadas (base, domínio, banco e API, telas, IA).
- P5 (envio) com código pronto: fila, limites, Lista do dia, recuperação sozinha, Ao vivo. Ensaio do QA com dados de teste: 8 de 8.
- P6: pedidos da IA do app, formulários na Fila, central-cli (#140) e ações do assistente na tela (#138).

## Congelado (não serve à meta 1)
P7 (nuvem), P8 (instaladores), rebranding interno (`jc-etv.*`), Linux, Dependabot, Codex/Gemini CLI, e-mails (.7.3, PR #133 fechado), Typeform, landing no código.
Retidos: #128 (Google Form, rascunho até o teste supervisionado) e #141 (`.mcp.json`, em revisão).

## Trilha de design (fora do código)
Bilbo (Claude Fable 5.2): 3 direções a partir do brief do Vini (3 referências que ama, 3 que rejeita, 5 adjetivos).

## Números do dia
| Fatias prontas | PRs integrados | Candidaturas reais |
|---|---|---|
| 0 | 0 | 0 |
