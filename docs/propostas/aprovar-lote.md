# Aprovar a lista do dia em lote

Bead `jc-a2y.6.13` (P1). Status: **proposta**. O contrato vai para 0.7.0, com uma rota nova em `x-proposta`. O CyberSec revisa antes do código. A tela é do Jarvis (`jc-a2y.6.13.1`, PR #100).

Decisão do Vini (09/11): a aprovação é em lote, com 1 clique por dia. O app prepara os pacotes sozinho (`jc-a2y.6.12`), a Fila mostra a "Lista do dia" com o resumo de cada um, e a pessoa pode tirar itens antes de aprovar. Depois do clique, o envio segue sozinho com as regras do `jc-a2y.6.11`.

## A rota

`POST /api/v1/candidaturas/aprovar-lote` (`x-so-tela`)

```json
{ "itens": [ { "id": "cand-…", "impressao": "sha256:v1:…" } ] }
```

Resposta `200`:

```json
{ "aprovadas": ["cand-…"], "recusadas": [ { "id": "cand-…", "motivo": "impressao-mudou" } ] }
```

## Regras (M2 continua valendo, item por item)

- **Só a pessoa.** A rota é `x-so-tela`. O assistente (CLI, MCP, IA) recebe 403, e nada é aprovado: a conferência do autor vem antes de qualquer item.
- **Cada item com a sua impressão.** O item só é aprovado se a impressão enviada for a atual do pacote, a mesma conferência do aprovar de uma. O servidor não confia em lista nenhuma: cada `{id, impressao}` prova o que a tela mostrou. Um pacote editado entre mostrar e aprovar é recusado (`impressao-mudou`), e só ele.
- **As mesmas conferências do aprovar de uma:**
  - a candidatura está em `preparada` (senão, `estado`);
  - a vaga continua em `aplicar` e não foi enviada (senão, `vaga-fora`);
  - o id existe (senão, `nao-encontrada`).
- **Limite do dia, contando o que já vai sair.** Por plataforma, as já enviadas hoje mais as aprovadas que ainda não saíram (válidas hoje, contando as aprovadas no próprio lote) não passam do limite. Os itens que passam, na ordem do pedido, são recusados (`limite`). A tela já faz essa conta; o servidor faz a mesma.
  - **Proposta:** o aprovar de uma também passa a contar as aprovadas que ainda não saíram. Hoje ele só conta as que já entraram em `enviando`; o executor segura o excesso na hora de enviar, mas a pessoa veria "aprovada" a mais.
- **Uma transação só** para o lote inteiro. Cada item aprovado sobe a versão e grava o evento, como hoje. A ordem do pedido decide quem entra no limite.
- **Validade:** até o fim do dia local, como no aprovar de uma. Uma aprovação vencida volta a `preparada` (`VencerAprovacoes`).
- **Corpo estrito:**
  - de 1 a 120 itens (3 plataformas × 40, o maior limite), só `id` e `impressao`, sem outros campos;
  - impressão no formato `sha256:v1:` com 64 hex;
  - um id repetido dá 400, e nada é aprovado.
- **Motivos de recusa:** lista fechada (`nao-encontrada`, `estado`, `impressao-mudou`, `vaga-fora`, `limite`), sem texto da candidatura nem da vaga.
- **Log, só números e o autor:** `fila: lote aprovado (eu): 12 aprovadas, 1 recusada(s)`.

## Testes previstos

- **Domínio:**
  - o lote aprova só os itens com impressão atual;
  - um item editado é recusado sem afetar os outros;
  - o limite conta as já enviadas, as aprovadas que não saíram e as do lote, na ordem;
  - id repetido;
  - vaga fora de `aplicar`;
  - não encontrada;
  - estado diferente de `preparada`.
- **API:**
  - 403 para o assistente, e nada muda no banco;
  - corpo estrito (campo a mais, 0 itens, 121 itens, impressão fora do formato);
  - a resposta só com ids e motivos;
  - o log só com números.
- **Mutações:**
  - não conferir a impressão;
  - não contar as aprovadas no limite;
  - aceitar autor que não é a pessoa;
  - aceitar id repetido.
- **Tela (Jarvis):** Vitest e e2e da "Lista do dia" contra a rota de verdade.
