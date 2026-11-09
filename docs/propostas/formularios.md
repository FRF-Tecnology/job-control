# Formulários externos: Google Forms e Typeform

Bead `jc-a2y.7.2` (P6). Status: **proposta**. O contrato vai para 0.10.0: a rota nova entra com `x-proposta`, e as mudanças de schema ficam marcadas como proposta na descrição. O 0.8.0 está no #103/#107 e o 0.9.0 no #94. O CyberSec passou os requisitos antes do contrato, e eles são critério de aceite. A tela é do Jarvis.

Hoje, uma vaga que manda para um formulário vira `outra` e acaba em `bloqueada/site-externo`. Com esta proposta, Google Forms e Typeform viram a plataforma `formulario`. O app lê as perguntas, monta o pacote só com o perfil e as `respostasPadrao`, a pessoa revisa e aprova na Fila, e o envio segue as mesmas regras do LinkedIn, da Gupy e da InHire:
- a impressão aprovada;
- um clique só de enviar;
- o detector de sensíveis em PT, ES e EN;
- nada fora do pacote;
- o limite do dia.

## Hosts: lista exata

| Formulário | Endereço aceito | Guardado como |
|---|---|---|
| Google Forms | `https://docs.google.com/forms/d/e/<id>/viewform` e `https://docs.google.com/forms/d/<id>/viewform` | o mesmo, **sem query nem fragmento** |
| Google (link curto) | `https://forms.gle/<id>` | o curto; o destino é resolvido só no navegador do envio (abaixo) |
| Typeform | `https://<conta>.typeform.com/to/<id>` e `https://form.typeform.com/to/<id>` | o mesmo, sem query nem fragmento |

- O `<id>` e a `<conta>` têm formato fechado: letras, dígitos, `_` e `-`, e a conta segue a regra de host do `MesmaVaga`.
- **Nunca `endsWith`.** Qualquer outro host, inclusive `docs.google.com` fora de `/forms/`, `typeform.com` sem `/to/` ou um embed em outro domínio, continua sendo `outra`.
- **O servidor nunca busca a URL** (nada de SSRF), nem o `forms.gle`. O link curto é aberto só no navegador do envio, e o `EnderecoFinal` confere o destino na lista **antes de qualquer interação**. Destino fora da lista dá `bloqueada/site-externo`. O destino canônico, que é a URL do formulário de verdade, fica guardado na leitura (abaixo), e o envio exige o mesmo destino.

## URL sem pré-preenchimento

O Google aceita `?entry.N=valor`, e o Typeform aceita campos ocultos (`hidden`) na query ou no `#fragmento`. Esses valores iriam junto no envio sem estar no pacote. Por isso:
- **A URL guardada é a canônica, sem query nem fragmento.** O app tira a query e o fragmento ao criar a vaga e a candidatura, e o executor só abre a canônica.
- **Sem esconder da pessoa (decisão do PO):** quando o link trazia respostas pré-preenchidas (`entry.*` ou campos ocultos), a candidatura guarda `preenchimentoIgnorado: true`, e o resumo do pacote na Fila mostra a frase fixa "O link trazia respostas pré-preenchidas; o app ignorou." **Os valores nunca são guardados nem mostrados.** O log fica só com o fato: `formulario: link com pré-preenchimento ignorado`.
- **Vale a regra do #89:** um campo que a página já trouxe preenchido e que não está no pacote bloqueia (`pergunta-nova`). O executor nunca apaga nem troca o valor que veio na página.

## Fluxo

1. A vaga de formulário vai para Aplicar. O pacote preparado sozinho (`.6.12`) fica **sem respostas do formulário**, porque elas dependem das perguntas.
2. **Ler o formulário (rota nova, só a tela):** `POST /candidaturas/{id}/ler-formulario`.
   - O executor abre a URL canônica no navegador do envio, **sem janela**, espera a página montar e lê a estrutura **sem interagir**: sem digitar, clicar em "Próxima", marcar nem enviar.
   - Ele lê a definição que a própria página carregou: todas as perguntas, com texto, tipo, se é obrigatória e as opções, em ordem.
   - **Formulário em várias etapas fica de fora neste recorte (decisão do PO).** É o Google Forms com mais de uma seção. Ele fica `bloqueada/varias-etapas`, com a pendência fixa "Formulário em várias etapas: envie à mão.", e a pessoa envia sozinha. O segundo recorte, que avança as seções lendo a estrutura de cada página com a mesma impressão e as mesmas regras, é o bead `jc-a2y.7.2.1` (P2). O Typeform, que mostra uma pergunta por vez de um formulário só, não conta como várias etapas.
   - Se não der para ler a estrutura inteira sem interagir, a candidatura fica `bloqueada/outro`, e a pessoa envia sozinha.
   - A leitura não conta no limite do dia. Ela conta no teto de 40 páginas abertas por plataforma no dia (`.6.11`).
3. **Pacote:** as respostas vêm só do perfil (contato e pretensão, pelo tipo do campo, como hoje) e das `respostasPadrao`, **com casamento exato da pergunta** (texto normalizado igual). Não há IA neste primeiro recorte e nunca há chute.
   - Pergunta sem par, ou de texto livre sem resposta padrão, fica em branco e entra em `pendentes`.
   - A pessoa responde na Fila (`PUT /candidaturas/{id}/conteudo`, como hoje, com `origem: pessoa`).
   - Com uma pendência obrigatória, `aprovar` dá 409 (`faltam respostas`).
4. **Impressão:** para `formulario`, ela passa a incluir a estrutura, como sha256 do JSON canônico de `{vagaId, plataforma, url, conteudo, estrutura}`. Para as outras plataformas, a fórmula não muda.
5. **Aprovar, na Fila:** igual às outras plataformas, uma a uma ou no lote do `.6.13`. "formulario" conta no limite do dia (30 por padrão) e no teto de 40 com as voltas do `.6.11`.
6. **Envio:** o executor abre a URL canônica, confere o destino na lista e confere o destino guardado. Depois **relê a estrutura e compara o hash**: se mudou, `bloqueada/pergunta-nova`. Em seguida preenche só o pacote e clica uma vez em enviar (no Typeform, com cada "OK" no caminho).

## Bloqueios próprios do formulário

| O que a página mostra | Desfecho |
|---|---|
| Google pede "Fazer login", ou avisa que "o seu e-mail será registrado" (coleta o e-mail da conta) | `bloqueada/login`. **A identidade da conta Google nunca é enviada.** |
| Caixa "enviar uma cópia das respostas para o meu e-mail" | **Nunca é marcada.** O executor confere que ficou desmarcada. |
| Consentimento, LGPD ou termos | `bloqueada/termos` |
| Pergunta sensível (CPF, RG, senha, banco, documento, data de nascimento), pelo detector de hoje em PT, ES e EN | `bloqueada` com o motivo do detector |
| Upload de arquivo dentro do formulário | `bloqueada/outro` (a pessoa envia) |
| Pagamento | `bloqueada/outro` |
| Iframe ou embed de outro domínio | `bloqueada/site-externo` |
| Estrutura diferente da aprovada, ou campo preenchido pela página fora do pacote | `bloqueada/pergunta-nova` |
| Google Forms com mais de uma seção | `bloqueada/varias-etapas` ("Formulário em várias etapas: envie à mão."); o segundo recorte é o `jc-a2y.7.2.1` |

## Typeform: página que monta tudo por script

- O executor espera a página montar antes de ler ou de preencher.
- **Cada "OK" ou "Próxima" é uma interação:** a marca `interagiuEm` é gravada antes, pela regra do `.6.11`. Depois dela, a candidatura nunca volta sozinha para aprovada.

## Confirmação

- O executor detecta a frase de resposta registrada pela lista fixa de hoje, mais "Sua resposta foi registrada", "Your response has been recorded" e "Thanks for completing this typeform", em PT, ES e EN.
- **A URL e o link "Editar sua resposta" da página final nunca são guardados nem vão para o log.** O link tem um token que deixa qualquer um editar a resposta.
- **Nada de captura de tela.** O "Ao vivo" (`.6.9`) mostra a página só em memória, e o quadro é apagado no fim, como já é.

## Contrato (0.10.0)

- **Rota nova** (`x-proposta`, `x-so-tela`): `POST /candidaturas/{id}/ler-formulario`, com o corpo `{}`.
  - 200: a candidatura com `estrutura` e `pendentes`.
  - 404: candidatura não encontrada, ou build sem o envio (o padrão e o `central-dev`).
  - 409: a candidatura não é `formulario` ou não está `preparada`, ou há um envio em andamento ou o login aberto.
  - 403: o assistente.
  - 429: o teto de 40 páginas abertas no dia.
- **Schemas, marcados como proposta:**
  - `PlataformaCandidatura` ganha `formulario`;
  - os limites e `PUT /candidaturas/limites/{plataforma}` ganham `formulario`;
  - `Candidatura` ganha `estrutura` (`{hash, urlFinal, perguntas: [{id, texto, tipo, obrigatoria, opcoes}]}`, texto da página, que a tela mostra como texto) e `pendentes` (os ids das perguntas obrigatórias sem resposta);
  - `ConteudoCandidatura.respostas[]` ganha `perguntaId`, opcional, que só o formulário usa.
- **O que não muda:** a regra do assistente (o MCP e a CLI só leem e revogam) e as outras plataformas.

## Log

Só o tipo e o resultado, como no executor de hoje: `executor: <id> ler-formulario ok (12 perguntas, 3 pendentes)` e `executor: <id> bloqueada (login)`. Nunca vão para o log a pergunta, a resposta, o texto da página, a URL nem o id do formulário.

## Testes previstos

- **Domínio:**
  - a lista exata de hosts, com variantes parecidas: `docs.google.com.exemplo.com`, `forms.gle.exemplo.com`, `typeform.com.exemplo.com`, `typeform.com/x`, homoglifo e punycode;
  - a URL canônica (sem `entry.*`, sem `#hidden`, sem query);
  - a impressão com a estrutura;
  - o casamento exato das respostas e as pendências;
  - `aprovar` com pendência dá 409;
  - o limite e o teto para `formulario`.
- **Executor, com páginas falsas:** estrutura que mudou; campo pré-preenchido fora do pacote; caixa de cópia por e-mail; login; e-mail registrado; termos; upload; pagamento; iframe; várias páginas; `interagiuEm` antes de cada "OK" no Typeform; confirmação; o link de editar nunca guardado nem no log.
- **Navegador, com Chrome de verdade e sem janela (no CI), contra o servidor fictício local:**
  - variantes do Google: várias seções (dá `varias-etapas`), login, e-mail registrado, `entry.*` na URL e cópia por e-mail;
  - variantes do Typeform: página montada por script, `hidden` no fragmento e iframe;
  - o `forms.gle` redirecionando para dentro e para fora da lista.

  **Nada contra o Google ou o Typeform de verdade no CI.**
- **API:** a rota nova com 403 para o assistente, 404 no build padrão e corpo estrito.
- **Mutações:** uma em cada regra acima.

## Ordem dos PRs

1. Este contrato e a proposta.
2. **Domínio e API:** a plataforma, a URL canônica, a impressão com a estrutura, as pendências, o limite e a rota, que dá 404 sem o envio.
3. **Executor e navegador:** a leitura, o envio, os bloqueios e o Typeform. Antes, combino arquivo por arquivo com o Faker, e o trabalho vai depois do #101 (`.6.11`) e do #98, que mexem no executor e na conferência do endereço final.

## Decidido (PO, no #108)

- **Pré-preenchimento na URL:** o app tira a query e o fragmento, mas avisa a pessoa no resumo do pacote, sem os valores (`preenchimentoIgnorado`). O log fica só com o fato.
- **Várias etapas:** o Google Forms com seções fica `bloqueada/varias-etapas` neste recorte, com o motivo claro. O segundo recorte é o `jc-a2y.7.2.1` (P2), com contrato e revisão do CyberSec próprios.
