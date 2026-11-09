# Envio que se vira sozinho com as falhas

Bead `jc-a2y.6.11` (P0). Status: **proposta**. O contrato vai para 0.6.0: uma rota nova em `x-proposta` e campos novos opcionais. O CyberSec revisa antes do código.

O pedido do Vini: o app tem de ser automático e esperto com as falhas. Hoje, qualquer falha técnica vira `falhou` e espera a pessoa reabrir.

## O que muda

| Situação | Hoje | Proposta |
|---|---|---|
| Falha técnica **antes de interagir com o formulário** (navegador não abre, a página não navega e fica em `about:blank`, erro ao ler a página) | `falhou/erro-tecnico`; conta no limite | **volta sozinha para `aprovada`**; **não conta no limite**; nova tentativa em 5 min, 30 min e 2 h (no máximo 3). Depois da 3ª, `falhou/erro-tecnico` como hoje |
| Falha técnica **depois** de interagir (campo preenchido, arquivo anexado, avançou etapa) | `falhou/erro-tecnico` | igual a hoje: `falhou`, e a pessoa decide |
| **2 resultados seguidos iguais da mesma plataforma** (`erro-tecnico`, `site-externo`) | cada candidatura para | a **plataforma** é pausada (só ela), com aviso no painel; as outras seguem. A pessoa retoma a plataforma na tela |
| **Login expirado** (authwall, login ou checkpoint antes de interagir) | `bloqueada/login` | a candidatura volta para `aprovada` (não conta no limite), e a plataforma fica **"precisa entrar"**. Quando a pessoa fecha a janela de "Entrar nas plataformas", ela **retoma sozinha** |
| **Captcha ou aviso de conta** (atividade incomum, restrição) | `bloqueada` | `bloqueada` como hoje, e a plataforma **para até amanhã** (regra do roteiro: não tentar de novo no mesmo dia) |
| Depois do **clique em enviar** | `incerta` (M3) | **igual**: nunca reenvia (M2 e M3 valem) |

## Por que a volta para `aprovada` é segura (o ponto do CyberSec)

- **Só antes de interagir.** O executor marca "interagiu" na primeira ação no formulário: preencher, anexar, desmarcar, avançar ou enviar. Abrir a página e clicar em "Candidatura simplificada" (que só abre o formulário, sem dado) não contam. Com "interagiu", nunca volta: o resultado é o de hoje (`falhou`, `bloqueada` ou `incerta`).
- **Uso único da aprovação.** Hoje a aprovação é "usada" ao entrar em `enviando`. Na volta, `usadaEm` é apagado, porque nada do pacote saiu do app nem foi digitado em lugar nenhum. A mesma aprovação vale para a nova tentativa, com a mesma impressão; qualquer mudança no pacote exige aprovar de novo, como hoje.
- **Validade no dia.** A aprovação continua valendo só até 23:59:59 do dia em que foi dada. Uma nova tentativa marcada para depois disso não acontece: a aprovação vence e a candidatura volta a `preparada` (`VencerAprovacoes`, como hoje).
- **Limite.** A tentativa devolvida sai de `tentativas` (não conta) e fica em `retentativas`, com o instante e o código, para auditoria. O limite do dia continua contando só as entradas em `enviando` que interagiram ou terminaram.
- **No máximo 3 voltas** por aprovação. Na 4ª falha antes de interagir: `falhou/erro-tecnico`.
- **Tudo numa transação:** a volta, a remoção da tentativa, a próxima tentativa e o estado da plataforma, com a versão subindo, como as outras gravações.

## Estado por plataforma

Fica na meta do banco (`envio-plataformas`), com falha fechada: estado ilegível = plataforma pausada.

| Estado | Quando | Como sai |
|---|---|---|
| `ativa` | padrão | — |
| `pausada` | 2 resultados seguidos iguais (`erro-tecnico` ou `site-externo`) | a pessoa retoma na tela (`POST /envio/plataformas/{plataforma}/retomar`, `x-so-tela`) |
| `precisa-entrar` | login expirado antes de interagir | sozinho, quando a janela de "Entrar nas plataformas" fecha; ou retomar na tela |
| `parada-hoje` | captcha ou aviso de conta | sozinho, às 00:00 do dia seguinte (horário local). Retomar na tela dá 409 |

O executor não pega candidatura de plataforma fora de `ativa`. O "pausar" geral continua acima de tudo.

## Contrato (0.6.0)

- `GET /envio`: campo novo opcional `plataformas: [{plataforma, estado, motivo, ate}]`. O `motivo` vem de uma lista fechada (`falhas-seguidas`, `login`, `captcha`, `aviso-conta`), sem texto da página.
- `POST /envio/plataformas/{plataforma}/retomar` (`x-so-tela`, `x-proposta`): de `pausada` ou `precisa-entrar` para `ativa`; de `parada-hoje` dá 409.
- Candidatura: campos novos opcionais `retentativas: [{em, codigo}]` e `proximaTentativa`.
- Motivo novo de bloqueio `aviso-conta` ("A plataforma mostrou um aviso sobre a sua conta: veja você antes de tentar de novo"), com frases do detector em PT, ES e EN (atividade incomum, conta restrita, verificação de segurança).
- Log, só com código: `executor: devolvida (erro-tecnico, 1/3)`, `envio: plataforma linkedin pausada (falhas-seguidas)`, `envio: plataforma linkedin retomada (eu)`.

## Testes previstos

- **Domínio:**
  - a volta só sai de `enviando`, apaga `usadaEm`, tira a tentativa do limite e soma em `retentativas`;
  - a 4ª dá `falhou`;
  - a aprovação vencida antes da próxima tentativa vira `preparada`;
  - a versão sobe.
- **Executor:**
  - antes de interagir, devolve; depois de preencher, `falhou` como hoje;
  - depois do clique em enviar, `incerta`, nunca devolve;
  - respeita a `proximaTentativa` e o estado da plataforma;
  - 2 iguais pausam só a plataforma;
  - login antes de interagir dá `precisa-entrar` e volta sozinho ao fechar o login;
  - captcha dá `parada-hoje`, e retomar dá 409.
- **API:** `retomar` só da tela; `GET /envio` com as plataformas; nada de texto da página.
- **Chrome real sem janela:** a página que não navega (servidor fictício que não responde) devolve e, na próxima tentativa, envia.
- **Mutações:** em cada regra (interagiu, `usadaEm`, limite, 3 voltas, 2 iguais, `parada-hoje`).
