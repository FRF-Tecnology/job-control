# Backlog — de ferramenta pessoal a produto

## Visão em uma página

**Um app web (PC e celular) onde dev brasileiro/LATAM gerencia a busca de emprego inteira — achar a vaga certa, aplicar,
acompanhar, se preparar e aprender com cada entrevista — com as candidaturas automatizadas no navegador dele.**

**Pilares, de ponta a ponta**
1. **Encontrar:** busca com seletor de foco internacional / nacional / os dois.
2. **Triar:** aderência, alertas e regras (não inventa experiência, pula vaga que pede CPF).
3. **Aplicar:** extensão no Chrome do PC; o envio é sempre confirmado pela pessoa.
4. **Acompanhar:** emails atualizam o funil sozinhos; follow-ups sugeridos.
5. **Entrevistar:** preparação **antes** (vaga, empresa, perguntas prováveis, histórias do currículo) e revisão **depois**
   (transcript, notas, o que melhorar). Durante, só gravar/transcrever com consentimento — nada de IA soprando resposta ao vivo.
6. **Aprender:** o que funciona por mercado, empresa e etapa.
7. **Conectar:** assistente de IA via MCP, recrutadores, histórico do LinkedIn.

**Onde cada coisa roda**
- **Celular e PC (app web responsivo, instalável como PWA):** funil, triagem ("aplica / pula"), notificações, preparação,
  transcript, revisão e aprendizados.
- **Só no PC:** a automação de candidatura (extensão do Chrome; o Chrome do celular não aceita extensões).
  Fluxo: aprova a fila no celular, o PC aplica quando a extensão estiver aberta.

**Três formas de ter IA (o cliente escolhe)**

| Forma | Quem pensa | Quando roda |
|---|---|---|
| IA do produto (no plano) | O produto, via API | Sempre, inclusive em segundo plano |
| BYOK (chave própria) | O produto, com a chave do cliente | Sempre; plano mais barato |
| Assistente via MCP | Claude, ChatGPT ou Gemini do cliente | Quando ele está conversando com o assistente |

O MCP funciona assim: o assistente do cliente chama o MCP do produto ("aplica nas vagas aprovadas"), o produto repassa
para a extensão no navegador dele, e a extensão preenche e envia. Qualquer assistente com MCP automatiza candidaturas,
sem precisar controlar navegador sozinho.

## Direção de design: estilo Apple + america.gov

**Referências:** Apple (clareza, tipografia, espaço, movimento sutil) e [america.gov](https://www.america.gov)
(National Design Studio: uma única caixa de pergunta como porta de entrada, títulos grandes e diretos, texto simples e
tranquilizador, prévias do produto em vez de explicação, "No app to download").

**Princípios**
1. **Uma porta de entrada:** a home é uma frase e uma caixa — "Olá, Vinicius. O que vamos fazer hoje?" —
   que entende pedidos ("aplica nas vagas USD de hoje", "como foi minha entrevista na Nortal?", "quem me respondeu?").
   Botões e telas continuam existindo; a caixa é o atalho. Combina com o MCP: a mesma linguagem dentro e fora do produto.
2. **Uma coisa por tela:** cada tela responde uma pergunta (Hoje, Vagas, Entrevistas, Aprendizados). Nada de painel lotado.
3. **Tipografia faz o trabalho:** fonte de sistema (SF / Inter), títulos grandes e números em destaque
   ("30 enviadas hoje", "2 retornos"). Hierarquia por tamanho e peso, não por cor e borda.
4. **Paleta contida:** fundo branco ou grafite (modo escuro), texto azul-marinho quase preto, **uma** cor de destaque
   para ação e status. Cores de status só onde importam (retorno, entrevista, oferta).
5. **Espaço e profundidade:** muito respiro, cartões com sombra larga e suave, painéis com desfoque translúcido no estilo Apple.
6. **Movimento com propósito:** transições curtas entre etapas do funil, a vaga "voando" para Enviada ao aplicar,
   contadores que sobem. Respeita "reduzir movimento" do sistema.
7. **Mostrar, não explicar:** o onboarding mostra a prévia do resultado (o funil cheio, uma revisão de entrevista)
   antes de pedir qualquer conexão.
8. **Texto simples e tranquilizador:** frases curtas, sem jargão; deixa claro o que nunca fazemos
   (guardar senha ou CPF, enviar sem você confirmar).
9. **Mobile primeiro na gestão:** a triagem no celular é um gesto (arrasta para aplicar ou pular); tudo cabe com o polegar.
10. **Acessível por padrão:** contraste AA, teclado completo, leitor de tela, textos que crescem.

**Como entra no roadmap:** um design system pequeno (tokens de cor, tipo, espaço, sombra e movimento + 10–15 componentes)
antes de redesenhar a Central. A Central atual vira o primeiro cliente desse design system.

A Central hoje é **uma pessoa, um computador**: Node local, JSON em `dados/`, Claude Code rodando a CLI e o Claude in Chrome
fazendo as candidaturas. Isso funciona para o Vinicius, mas não é produto: um cliente teria que instalar Node, Claude Code,
extensão e conectores, e configurar tudo sozinho.

O plano abaixo trata o **Vinicius como cliente zero**. Cada feature roda primeiro para ele, mas é construída já na forma de produto:
1. **Toda integração é self-serve:** o cliente conecta em 1–2 cliques dentro do produto, sem terminal, sem config, sem chave de API.
2. **O núcleo não sabe onde roda:** as regras de negócio (`nucleo/`) ficam iguais no modo local e na nuvem.
   Só muda o adaptador (armazenamento, email, IA).
3. **A automação roda no navegador do cliente** (extensão), com a sessão dele, e o envio sempre com o clique dele.
   O servidor nunca guarda a senha nem o cookie do LinkedIn de ninguém.
4. **A IA é do produto e não é presa a um provedor:** o núcleo chama um adaptador de IA, que pode usar Claude, GPT ou Gemini
   (ou vários: um modelo barato para triar e um mais forte para perguntas difíceis). O cliente não precisa ter conta em nenhum deles.
   Quem já usa um assistente (Claude, ChatGPT, Gemini) pode, além disso, plugar o produto lá pelo MCP (T8).
5. **IA só onde precisa:** fluxos conhecidos (LinkedIn Easy Apply, Gupy, InHire, Greenhouse, Lever) viram script sem IA.
   A IA entra para triar a vaga, responder pergunta nova e revisar antes de enviar. Toda resposta aprendida vira resposta salva
   do cliente, e a próxima candidatura sai mais barata.

## Caminhos de produto

| | **A) Cliente traz o próprio assistente** | **B) Produto com IA própria** |
|---|---|---|
| O que se vende | Kit: Central hospedada + MCP + skills/plugin | Assinatura: extensão + Central na nuvem |
| O cliente precisa de | Claude, ChatGPT ou Gemini com suporte a MCP | Só o navegador |
| Quem paga a IA | O cliente (assinatura dele) | O produto (API), ou o cliente via BYOK |
| A favor | Já funciona hoje, custo de IA zero, lança em semanas | Mercado grande, assinatura de verdade, qualidade controlada |
| Contra | Público pequeno; skills e prompts são fáceis de copiar, então só dá para cobrar pelo que fica hospedado; suporte na máquina de cada um | Custo de IA por candidatura; construir o agente dentro da extensão |

**Plano:** A como beta pago com 10–20 devs (4–6 semanas) para validar demanda e medir custo de IA por candidatura;
em paralelo, T0 com o adaptador de IA; depois B como produto principal (T6, T7), com o MCP (T8) como canal extra.

**Regras que valem para os dois:**
- Num produto vendido, a IA dos clientes roda via API com chave do produto (ou BYOK). Assinatura pessoal (Claude Max, ChatGPT Plus etc.)
  não pode ser usada para atender clientes.
- A ferramenta usada para construir (Claude Code) não é dependência do produto.
- A lógica importante fica no núcleo e no MCP (padrão aberto), não só em skills, que são mais ligadas a um assistente.
- LinkedIn proíbe automação nos termos de uso: o produto é copiloto (o cliente clica para enviar), com limite diário,
  e dá peso igual aos ATS (Gupy, InHire, Greenhouse, Lever, Ashby).

## Concorrentes de referência (levantado em 2026-11-06)

| | **Leapfrog** ([extensão](https://chromewebstore.google.com/detail/international-jobs-by-lea/cebjkdeabhiafpmbhjlbnpkpclomjgko), [leapfrog.works](https://leapfrog.works)) | **Applika** ([applika.dev](https://applika.dev)) | **Nós** |
|---|---|---|---|
| O que faz | **Encontra:** destaca no LinkedIn vagas remotas de empresas dos EUA, UK, Canadá e Europa que contratam no Brasil/LATAM e pagam em USD/EUR; esconde "remoto" que só contrata local. Tem portal de vagas e alertas. | **Acompanha:** tracker manual de candidaturas com etapas, funil de conversão, tempo por etapa e origem da vaga | Encontra + aplica (copiloto) + acompanha sozinho (email) + aprende (entrevistas) |
| Aplica por você | Não | Não | Sim, com clique do cliente |
| IA | Não | Não | Triagem, perguntas novas, revisão, entrevistas |
| Plataformas | Só LinkedIn | Registro manual de qualquer origem | LinkedIn, Gupy, InHire, Greenhouse, Lever, Ashby |
| Preço | Grátis (20 vagas/dia na extensão); **R$ 119 por trimestre**; **R$ 249 vitalício** | **Grátis e open source**, login com GitHub | A definir |
| Tração | ~10 mil usuários na Chrome Web Store (4,5★, 30 avaliações); diz 26.500+ downloads | "Centenas" de usuários | Cliente zero |

**O que isso ensina**
- **O mercado existe e paga:** o Leapfrog prova que dev brasileiro paga para achar vaga em USD. É exatamente o grupo 1 da nossa prioridade.
- **Âncora de preço:** descoberta sozinha vale ~R$ 40/mês ou R$ 249 uma vez. Para cobrar mais, o produto precisa entregar
  o que vem depois de achar a vaga: candidatar, acompanhar, aprender.
- **Tracker sozinho não se vende:** o Applika é grátis e aberto. O funil da Central é obrigação, não diferencial.
  O diferencial é ele se atualizar sozinho (T1) e virar aprendizado (T3, T5).
- **Tese:** "Leapfrog + candidatura + acompanhamento automático". Acha a vaga internacional certa, aplica com qualidade
  e mostra o que está funcionando, num produto só.

## Público e foco (decidido em 2026-11-06)

**Público:** devs brasileiros, depois LATAM. O produto atende **vagas internacionais e nacionais**, e o cliente escolhe o foco
num seletor (toggle). O marketing de entrada mira quem busca vaga internacional, que é o nicho que o Leapfrog provou que paga.

**Seletor de foco** (`perfil.foco`, muda a qualquer momento na tela e vale para busca, triagem e ordem de aplicação):

| Foco | Busca | Ordem de aplicação | Currículo e respostas |
|---|---|---|---|
| **Internacional** | LinkedIn (Brasil + "Latin America"), Greenhouse, Lever, Ashby, Workable, portais que contratam LATAM | USD LATAM de empresas dos EUA → empresas que contratam LATAM + Europa | Inglês; pretensão em USD/EUR; contractor; fuso; sem sponsorship |
| **Nacional** | LinkedIn Brasil, Gupy, InHire, sites de empresas brasileiras | Mais recentes primeiro | Português; pretensão CLT/PJ em reais |
| **Os dois** (padrão do cliente zero) | As duas | Internacional primeiro; nacional usa o que sobrar do limite do dia | Escolhe o idioma pela vaga |

**O que isso define**
- O núcleo trata cada vaga com `mercado: "internacional" | "nacional"`, detectado na triagem (moeda, local, idioma, contrato).
  O funil e os aprendizados (T5) podem ser vistos por mercado.
- O onboarding pergunta o foco e só pede as respostas padrão do mercado escolhido.
- Os ATS das duas frentes entram no roadmap: Greenhouse, Lever e Ashby para internacional; Gupy e InHire para nacional
  (scripts já existem para LinkedIn, Greenhouse, Ashby, InHire e Gupy).
- **Preço em reais**, com a âncora do Leapfrog (R$ 119/trimestre).
- **Primeiras 10–20 conversas de validação:** metade buscando internacional, metade nacional, para ver qual frente paga mais.

## O que o cliente conecta (e como)

| Integração | Como o cliente conecta | Esforço do produto | Observação |
|---|---|---|---|
| **Conta no produto** | Login com Google (OAuth só de identidade) | P | Escopo mínimo, sem revisão especial do Google |
| **Navegador / candidaturas** | Instala a extensão da Chrome Web Store | G | Faz o papel que hoje é do Claude in Chrome e dos scripts de `automacao/` |
| **Email (retornos)** | **Opção A (MVP):** cria no Gmail um filtro que encaminha emails de ATS para `voce@in.produto.com`. O produto mostra o passo a passo com prints. | P | Não precisa de OAuth do Gmail nem de auditoria |
| | **Opção B (depois):** "Conectar Gmail" via OAuth, só leitura | G | `gmail.readonly` é escopo restrito: exige verificação do Google e auditoria de segurança anual (CASA), paga. Só vale depois de ter tração. |
| **Histórico do LinkedIn** | Pede o export no LinkedIn e arrasta o ZIP para o produto | P | O produto mostra o passo a passo de onde clicar |
| **Currículo** | Upload do PDF; o produto extrai o perfil e o cliente revisa | P | Já é o primeiro passo do onboarding |
| **Transcript de entrevista** | Cola o texto ou envia o `.vtt`/`.txt`. Depois: integração com Zoom, Meet e tl;dv. | P → M | |
| **Claude / ChatGPT / Gemini (opcional)** | "Adicionar ao meu assistente": MCP remoto com login OAuth | M | Para quem quer conversar com as próprias candidaturas |
| **Chave de IA própria (opcional, BYOK)** | Cola a chave da OpenAI, Anthropic ou Google na tela de configurações | P | O cliente paga a IA direto ao provedor; plano mais barato. A chave fica criptografada e nunca aparece de volta na tela. |

**O que o cliente nunca faz:** instalar Node, abrir terminal, configurar MCP à mão. Colar chave de IA só se ele escolher o BYOK.

## Tasks

Tamanhos: **P** (1 sessão), **M** (2–3 sessões), **G** (semanas).

| # | Task | Tamanho | Depende de | Fase |
|---|------|---------|------------|------|
| T0 | **Base de produto:** adaptadores, multiusuário no modelo, API HTTP | M | — | 1 |
| T1 | Emails atualizam as candidaturas sozinhos | M | T0 | 1 |
| T3 | Entrevistas: etapas, transcript e revisão | M | T0 | 1 |
| T4 | Importar histórico do LinkedIn (upload do ZIP) | M | T0 | 2 |
| T6 | Extensão de navegador (o "applyer" do cliente) | G | T0 | 2 |
| T2 | Recrutadores e empresas-alvo | M | T4, T6 | 3 |
| T5 | Aprendizados: passado + atual + entrevistas | M | T1, T3, T4 | 3 |
| T7 | Plataforma na nuvem: login, billing, LGPD, deploy | G | T0, T6, validação | 3 |
| T8 | MCP do applyer (local primeiro, depois remoto com OAuth) | P → M | T0 | 1 / 3 |
| T9 | Avaliação de modelos (Claude × GPT × Gemini) com dados reais das rodadas | M | T0 | 1 |
| T10 | Design system (Apple + america.gov) e nova Central responsiva/PWA com a caixa "O que vamos fazer hoje?" | M → G | T0 | 1 / 2 |

**Em paralelo, desde já:** validação de mercado (ver T7). Com 10 conversas com devs procurando vaga, dá para saber se o foco é
"qualidade em vez de volume" ou outra coisa, antes de investir semanas na extensão e na nuvem.

---

## T0 — Base de produto

**Problema:** hoje o código assume "um perfil, uma pasta `dados/`, uma CLI". Para virar produto sem reescrever tudo,
essa suposição sai do núcleo agora, enquanto o código ainda é pequeno (~2 mil linhas).

**O que muda**
- **Adaptador de armazenamento:** `criarArmazem` vira uma interface com duas implementações. `arquivos` é o atual;
  `postgres` entra na T7. O `servico.mjs` não sabe qual está usando.
- **Dono dos dados:** vagas, eventos, contatos e entrevistas ganham `contaId`. No modo local é sempre `"local"`.
  Nenhuma regra muda.
- **API HTTP versionada** (`/api/v1/...`) cobrindo tudo que a CLI faz hoje. A CLI, a Central web, a extensão e o MCP passam a ser
  só clientes dessa API (no modo local, sem login).
- **Adaptadores de email e IA:** o mesmo esquema de interface:
  - email: `entrada(email)` recebe uma mensagem, venha ela de encaminhamento, de OAuth ou do conector do Claude;
  - IA: o núcleo só chama funções de negócio (`triarVaga`, `responderPergunta`, `revisarAntesDeEnviar`, `classificarEmail`,
    `avaliarEntrevista`, `escreverMensagem`), com entrada e saída em JSON validado. O adaptador escolhe o provedor:
    - `assistente-local`: o assistente do cliente (hoje o Claude Code) faz o papel da IA; modo do cliente zero e do caminho A;
    - `anthropic`, `openai`, `google`: chamada de API com a chave do produto ou do cliente (BYOK);
    - **roteamento por função:** cada função aponta para um provedor e um modelo na configuração
      (ex.: triagem num modelo barato, perguntas difíceis num mais forte), com fallback se um provedor cair;
    - **medição:** cada chamada grava provedor, modelo, tokens, custo estimado e tempo, somados por candidatura.
      É daqui que sai o preço do plano.
- **Regras como código:** as regras do `CLAUDE.md` (CPF, nunca inventar, sem envio sem aprovação) passam a ser validadas no núcleo,
  e não só no prompt. No produto não haverá um `CLAUDE.md` vigiando.

**Pronto quando:** a CLI e a web funcionam só pela API, os 97 testes continuam passando, um teste prova que trocar o adaptador
de armazenamento não muda o comportamento, e outro prova que as regras (CPF, nunca inventar) barram a saída de qualquer provedor de IA.


**Status (2026-11-07, branch `feat/t0-base-produto`): feito, falta só o adaptador de email (entra junto com a T1).**
- Armazém como interface: `nucleo/armazens/arquivos.mjs` (local) e `memoria.mjs` (testes); `criarArmazemPor({ tipo })`.
- `contaId` em vagas e eventos novos, sempre vindo da sessão (nunca do corpo do pedido); local = `"local"`.
- API v1 em `servidor/api.mjs`; o servidor HTTP é só uma casca. Rotas antigas `/api/x` continuam valendo.
  A CLI chama a mesma API no mesmo processo (não precisa da Central aberta); a web usa `/api/v1`.
- Adaptador de IA em `nucleo/ia/`: 6 funções, provedores `assistente-local` / `anthropic` / `openai` / `google`,
  rota por função com reserva, medição de tokens e custo, e a guarda com 10 regras (`REGRAS`) aplicada a todo provedor.
  Configuração de exemplo em `modelo-dados/ia.exemplo.json`. Nenhum provedor foi chamado de verdade ainda (testes usam fetch falso).
- Testes: 97 → 119. `testes/api.test.mjs` prova que trocar o armazém não muda nenhuma resposta;
  `testes/ia.test.mjs` prova que a guarda barra CPF, invenção de experiência e senha nos três provedores.
---

## T10 — Design system e nova Central

**Status (2026-11-07, branch `feat/t10-nova-central`, em cima da T0): primeira entrega feita.**
- Design system em `web/estilo.css`: Inter, tinta #0B1B33, um azul de destaque (#0066CC), superfícies #F5F5F7, raios grandes,
  topo translúcido com desfoque, tema escuro no estilo Apple, movimento só com propósito (respeita "reduzir movimento").
- Casca nova: menu no topo no PC; no celular, 3 abas embaixo (Hoje, Triagem, Vagas) + Mais.
- Hoje: "Olá, <nome>. O que vamos fazer hoje?", caixa única, sugestões pelo foco, seletor Internacional / Nacional / Os dois
  (salvo em `perfil.foco`), números grandes, "Precisa de você" agrupado por pendência, funil e linha do tempo.
- Caixa única: atalhos simples abrem a tela certa; o resto vira **pedido** (`/api/v1/pedidos`) que o assistente faz com `/pedidos`.
- Campo `mercado` na vaga (internacional | nacional), com palpite quando falta (`mercadoDe`).
- Triagem: cartão que se arrasta (direita aplica, esquerda pula), etiqueta de mercado, fila filtrada pelo foco.
- PWA: `manifest.webmanifest`, ícone e service worker (rede primeiro; dados nunca vão para o cache).

**Falta:** instalação do PWA conferida num Chrome real (o navegador embutido do app não registra service worker);
usar a Central pelo celular depende da nuvem (T7), porque hoje ela só atende o próprio computador;
telas Entrevistas e Aprendizados entram com T3 e T5; a caixa responder na hora com IA própria entra quando a T9 escolher o modelo.

---

## T9 — Avaliação de modelos com dados reais

**Problema:** escolher o modelo pela marca é achismo. A diferença entre modelos aparece justamente no que é o coração
do produto: seguir regras à risca, não inventar experiência, perceber resposta pré-preenchida errada.

**O conjunto de testes sai das rodadas do cliente zero** (`dados/evals/`, sem dados pessoais sensíveis):
- **Perguntas de formulário** com a resposta certa (ex.: "anos com React" → 0; "pode começar imediatamente" → Não, 2 semanas;
  "pretensão em USD" → 5000; campo numérico → só dígitos).
- **Pré-preenchimentos errados** que foram corrigidos (Spring Boot 6 → 5, React/Node 3 → 0).
- **Triagem:** vagas aplicadas e puladas com o motivo (Staff, só Python, PcD afirmativa, pede CRM, pede morar no México, salário baixo).
- **Casos proibidos:** formulário pedindo CPF, senha ou criação de conta → a resposta certa é parar.
- **Emails e entrevistas** quando T1 e T3 existirem.

**Como roda:** `cli.mjs avaliar --provedor openai --modelo <id>` passa cada caso pelo adaptador e mede acerto,
violação de regra (peso alto: uma violação reprova), custo e tempo. Resultado em tabela por provedor e por função.

**Pronto quando:** o mesmo conjunto rodou em pelo menos três modelos (um de cada provedor), a configuração padrão de roteamento
foi escolhida pelos números, e o conjunto roda de novo a cada mudança de prompt ou de modelo.

**Status (2026-11-07, branch `feat/t9-avaliacao-modelos`): ferramenta pronta, primeira rodada feita só com Claude.**
- Conjunto em `avaliacao/`: 77 casos (36 triagens tiradas das decisões reais das rodadas, 24 perguntas de formulário,
  6 revisões de formulário, 8 emails, 3 mensagens). Perfil e currículo **fictícios** (o repositório é público); um teste barra dado pessoal.
- `cli.mjs avaliar`: `--provedor anthropic|openai|google --modelo <id>` (chave na variável de ambiente), `--provedor assistente-local`
  (gera os pedidos) + `--respostas` (pontua), `--comparar` (tabela). Resultados em `dados/avaliacoes/`.
- Métricas: acerto depois da guarda; **violações** = regras de segurança que a guarda precisou aplicar (uma reprova o modelo);
  correções de formato; erros (JSON, esquema, provedor); custo e tempo.

Primeira rodada (modo assistente local: Opus, Sonnet e Haiku como agentes, sem chave de API):

| Modelo | Total | Triagem | Perguntas | Revisão | Email | Mensagem | Violações |
|---|---|---|---|---|---|---|---|
| Claude Opus | 97% | 94% | 100% | 100% | 100% | 100% | 0 |
| Claude Sonnet | 97% | 94% | 100% | 100% | 100% | 100% | 0 |
| Claude Haiku | 99% | 97% | 100% | 100% | 100% | 100% | 0 |

O que isso diz (e o que não diz):
- Os três ficaram no teto: o conjunto ainda é fácil demais para separar modelos. Próximo passo: casos mais difíceis
  (descrições longas e ambíguas, pré-preenchimentos sutis, perguntas de formulário com redação estranha).
- Rodou como agente (com tempo para pensar e conferir), não como chamada única de API. Custo e tempo reais só aparecem
  com `--provedor anthropic|openai|google`.
- Os erros que sobraram foram todos de triagem em vaga ambígua ("Híbrido/Remoto", vaga Android): os modelos preferiram "perguntar".
  O rótulo desses dois casos também é discutível.
- Achado na guarda: se o modelo diz "6 anos de React, fonte: respostasPadrao", a guarda aceita (confere só se a fonte existe,
  não se o valor bate com ela). Próximo passo: comparar o valor com a resposta padrão da mesma tecnologia.

**Falta para fechar a T9:** rodar em GPT e Gemini (e Claude via API) com as chaves do dono, escolher o roteamento padrão pelos números
e acrescentar os casos difíceis.

---

---

## T1 — Emails atualizam as candidaturas sozinhos

**Problema:** retornos (confirmação, recusa, convite, teste, oferta) chegam por email e a Central não fica sabendo.

**Como chega o email (adaptador de email da T0)**

| Modo | Para quem | Como |
|---|---|---|
| Encaminhamento | **Cliente do produto (MVP)** | Filtro do Gmail ou Outlook encaminha emails de ATS para um endereço único dele. O produto recebe via webhook de entrada (Postmark, SES ou Cloudflare Email Routing). O onboarding gera o filtro pronto para copiar e testa com um email de exemplo. |
| OAuth Gmail | Cliente, quando houver tração | "Conectar Gmail", só leitura. Exige verificação e CASA (ver tabela de integrações). |
| Conector Gmail do Claude | Cliente zero (Vinicius), hoje | `/emails` no Claude Code, para validar a classificação antes de construir o resto |

As três formas entregam o mesmo objeto para `entrada(email)`, e daí em diante o fluxo é igual.

**Como funciona**
1. O adaptador entrega cada email novo.
2. Um pré-filtro descarta o que não é de candidatura. Ele usa:
   - remetentes de ATS: greenhouse, lever, ashbyhq, gupy, inhire, workable, smartrecruiters, `jobs-noreply@linkedin`;
   - palavras-chave: "application", "candidatura", "unfortunately", "next steps", "entrevista", "assessment", "offer".
3. O Claude classifica cada email:

   | Tipo | O que acontece com a vaga |
   |------|---------------------------|
   | confirmação | só registra o email |
   | recusa | `encerrada` com motivo "Reprovada" |
   | convite de entrevista | `em-processo`, sub-etapa `entrevista-rh` ou `entrevista-tecnica`, pendência "agendar" |
   | teste | `em-processo`, sub-etapa `teste`, `bloqueada` se o teste for pessoal |
   | oferta | `oferta` |
   | outro | não altera a vaga |

4. Para achar a vaga, compara empresa (normalizada com o `dedupe`), domínio do remetente e título.
   Emails sem vaga correspondente vão para uma lista "Emails sem vaga" na tela Hoje, onde a pessoa liga à vaga certa ou descarta.
5. No produto, roda sozinho assim que o email chega. No modo local, roda com `/emails` ou numa tarefa agendada.
6. O cliente recebe a notificação: "Nubank te chamou para entrevista — agende aqui".

**Código**
- `vaga.emails[]`: `{ idMensagem, data, tipo, remetente, assunto, resumo }`. Só um resumo curto, nunca o corpo inteiro.
- `dados/emails.json`: ids já processados e data da última leitura (evita reprocessar).
- CLI: `email-registrar <vagaId> --tipo --data --assunto --resumo --id`, que aplica a transição e grava a linha do tempo.
- Web: os emails aparecem na linha do tempo do detalhe da vaga e a tela Hoje ganha o bloco "Emails sem vaga".
- Testes: classificação → transição, deduplicação por `idMensagem`.

**Regras:** só leitura. Rascunho de resposta pode; enviar exige "pode mandar". A validação de CPF/RG continua valendo para o resumo.

**Pronto quando:** `/emails` processa a caixa, as vagas mudam de etapa sozinhas e rodar duas vezes não duplica nada.

**Status (2026-11-07, branch `feat/t1-emails`): feito no modo local; falta a primeira leitura da caixa real.**
- `nucleo/emails.mjs`: pré-filtro (ATS + palavras), achar a vaga (empresa no domínio, remetente, assunto ou texto; título desempata;
  empate vira "sem vaga"), transições da tabela acima (nunca anda para trás nem mexe em vaga encerrada/pulada).
- Em vez de `vaga.emails[]`, os emails ficam em `dados/emails.json` (um registro por `idMensagem`, com `vagaId` e status
  ligado / sem-vaga / ignorado / descartado) e cada um vira um evento "Email" na linha do tempo da vaga. Só resumo, nunca o corpo.
- API `/api/v1/emails` (registrar um ou um lote, listar, ligar, descartar); CLI `email`, `emails`, `email-ligar`, `email-descartar`.
- Tela Hoje: bloco "Emails sem vaga" (escolhe a vaga e liga, ou descarta); "Em andamento" mostra a pendência (ex.: "Agendar a entrevista").
- `/emails` (`.claude/commands/emails.md`): lê pelo conector do Gmail ou pelo Gmail no navegador, só com o ok da pessoa.
- Testes: cada linha da tabela, achar vaga (inclusive empate), rodar duas vezes, CPF no resumo, CLI de ponta a ponta. 127 → 134.
- Conferido numa Central de demonstração (porta separada, dados de exemplo): recusa → encerrada, convite → entrevista,
  email ambíguo → "sem vaga", ligar pela tela → etapa muda.

- **Primeira rodada real (2026-11-07), pelo Gmail no navegador, só leitura:** 21 emails desde 05/nov (quando a Central começou a
  registrar envios). 3 vagas da CI&T foram para "entrevista" (entrevista com IA no DigAÍ, pendência vinda do próprio email),
  Zup e Nortal foram encerradas como "Reprovada", 14 confirmações e avisos ficaram na linha do tempo. Segunda rodada: 21 repetidos.
  Email não lido não foi aberto (abrir marca como lido): o "Update" da TRM Labs entrou como aviso, sem mudar a etapa.
- Respostas à pergunta em aberto 2: as candidaturas chegam no email do perfil, que é a segunda conta do Gmail no navegador.
- Emails de outubro (BEES, Binance, NinjaOne, entrevistas de outubro) são de candidaturas anteriores à Central:
  entram quando a T4 importar o histórico do LinkedIn. Emails de recrutadores oferecendo vaga (TCS, AMBEV, Paramount, DXC) são leads
  para a T2, não retornos.

**Falta:** no produto, o endereço de encaminhamento (webhook) e a notificação; ler um email sem marcá-lo como lido (API do Gmail ou encaminhamento).

---

## T8 — MCP do applyer

**Problema:** hoje tudo passa por `node ferramentas/cli.mjs ...` via shell. Um servidor MCP deixa qualquer cliente Claude
(Desktop, Code, nuvem, a futura extensão) usar a Central com ferramentas tipadas.

**Ferramentas expostas** (finas, chamando `nucleo/servico.mjs`):
- **Vagas:** `vagas_listar` (filtros: etapa, empresa, aderência, data), `vaga_detalhe`, `vaga_adicionar`, `vaga_mover`,
  `vaga_registrar_envio`, `vaga_duplicada`.
- **Perfil:** `perfil_ler` e `resposta_padrao`, que recebe o texto da pergunta do formulário e devolve a resposta mais parecida.
- **Rotina:** `followups_vencidos`, `rodada_registrar`.
- **Depois de T1 e T3:** `email_registrar`, `entrevista_registrar`, `entrevista_avaliar`.

**Código**
- `mcp/servidor.mjs`: JSON-RPC sobre stdio, implementado à mão. Fica sem dependências, como o resto do projeto
  (a superfície é pequena, uns 200 linhas).
- Reaproveita o `criarArmazem` e a trava de arquivo: Central, CLI e MCP gravam com segurança ao mesmo tempo.
- `README`: como registrar no Claude Desktop (`claude_desktop_config.json`) e no Claude Code (`.mcp.json`).
- Testes: cada ferramenta com um armazém temporário.

**Regras:** as regras do `CLAUDE.md` passam para dentro do servidor. Não existe ferramenta de "enviar mensagem" e CPF/RG continua recusado na validação.

**Duas versões**
- **Local (agora, P):** stdio, para o cliente zero e para quem roda a Central no próprio computador.
- **Remota (produto, M):** MCP via HTTP no domínio do produto, com login OAuth.
  - O cliente clica em "Adicionar ao Claude" (ou cola a URL em Conectores) e faz login com a conta do produto. Nada de config.
  - Cada token só enxerga os dados daquela conta (`contaId` da T0).
  - As mesmas ferramentas, chamando a mesma API `/api/v1`.

**Pronto quando:** o Claude Desktop lista e move vagas pelo MCP local, sem shell. A versão remota fica pronta quando um usuário
de teste conecta pelo fluxo OAuth sem editar nenhum arquivo.

**Status (2026-11-07, branch `feat/t8-mcp`): versão local feita; falta ver no Claude Desktop.**
- `mcp/servidor.mjs`: JSON-RPC 2.0 por stdio, sem dependências (initialize, ping, tools/list, tools/call; versões de protocolo
  2024-11-05 a 2025-11-25). As regras de uso vão no `instructions` do initialize.
- `mcp/ferramentas.mjs`: 18 ferramentas finas em cima da API v1 (as da lista acima + `vaga_editar`, `evento_registrar`,
  `followup_registrar`, `pedidos_listar`, `pedido_concluir`, `emails_registrar`, `emails_sem_vaga`, `resumo`).
  `vaga_registrar_envio` é o único caminho para "enviada". Leitura marcada com `readOnlyHint`. Nenhuma ferramenta envia mensagem.
- Recusa da API (motivo faltando, CPF, vaga inexistente) volta como `isError` para o modelo corrigir, sem derrubar o servidor.
- `.mcp.json` registra o servidor no Claude Code; README explica o Claude Desktop.
- Testes: protocolo, cada ferramenta, recusas, conversa real por stdio (linha quebrada em pedaços). 134 → 140.
- Conferido com os dados reais (só leitura): lista as 3 vagas da CI&T em entrevista e acha "Spring Boot → 5" para pergunta em inglês.

**Falta:** abrir no Claude Desktop (mexe na configuração do app, fica com o dono) e a versão remota com OAuth (junto da T7).

---

## T3 — Entrevistas: etapas, transcript e revisão

**Problema:** depois da candidatura, o processo some. Não fica registro do que foi perguntado, do que funcionou e do que melhorar.

**Como funciona**
1. Cada vaga em processo ganha uma lista de **etapas de entrevista**: RH, técnica, case, cultura, final, com data, entrevistador e duração.
2. Em cada etapa, a pessoa **cola o transcript** ou envia o arquivo (`.txt`, `.vtt`, `.srt` do Meet, Teams ou Zoom).
3. Novo skill **`/revisar-entrevista <vaga> <etapa>`**. Ele lê o transcript, a descrição da vaga, o perfil e as revisões anteriores, e devolve:
   - **Pontos positivos:** respostas fortes, com trecho.
   - **Pontos a melhorar:** estrutura STAR, objetividade, profundidade técnica, perguntas sem resposta clara.
   - **Perguntas feitas**, cada uma com uma resposta melhor sugerida e baseada no currículo real (nunca inventada).
   - **Conversa sobre salário:** como foi conduzida.
   - **Sinais do processo:** o que o entrevistador valoriza e alertas.
   - **Preparação para a próxima etapa:** temas prováveis e perguntas para fazer.
   - **Nota de 1 a 5** por dimensão, para comparar ao longo do tempo.

**Código**
- `vaga.entrevistas[]`: `{ n, tipo, data, entrevistador, duracaoMin, transcript (caminho), avaliacao {...}, resultado }`.
- Os transcripts ficam em `dados/entrevistas/<vagaId>-<n>.md`, locais e fora do git, como todo `dados/`.
- CLI: `entrevista add`, `entrevista transcript <arquivo>` e `entrevista avaliar --json`.
- Servidor: rota de upload do transcript. Web: seção "Entrevistas" no detalhe da vaga, com a avaliação formatada.
- Skill: `.claude/commands/revisar-entrevista.md`, com o roteiro de avaliação fixo para as notas serem comparáveis.

**Regras:** os transcripts têm dados de terceiros e nunca saem da máquina. Gravar a reunião é decisão e responsabilidade da pessoa
(peça consentimento a quem entrevista).

**Pronto quando:** a pessoa cola um transcript na Central, roda `/revisar-entrevista` e vê a avaliação no detalhe da vaga.

---

## T4 — Importar histórico do LinkedIn

**Problema:** anos de candidaturas e conversas com recrutadores estão no LinkedIn e não entram nas métricas.

**Como funciona**
1. A pessoa pede o arquivo em LinkedIn → Configurações → Privacidade de dados → **"Obter uma cópia dos seus dados"**,
   marcando o pacote completo. Chega por email em até 24 h. O pedido e o download ficam com a pessoa.
2. `cli importar-linkedin <pasta-ou-zip>` lê os CSVs:

   | Arquivo do LinkedIn | Vira na Central |
   |---------------------|-----------------|
   | `Jobs/Job Applications.csv` | vagas históricas (`origem: historico`), com data e empresa |
   | `Jobs/Job Applicant Saved Screening Question Responses.csv` | sugestões para `respostasPadrao`, revisadas antes de gravar |
   | `messages.csv` | conversas com recrutadores, ligadas à empresa e à vaga |
   | `Connections.csv` / `Invitations.csv` | contatos (base para T2) |

3. O Claude lê as conversas e marca o desfecho de cada vaga histórica: entrevista, recusa ou sem retorno.

**Código**
- Parser de CSV sem dependências (aspas e quebras de linha dentro de campos), em `nucleo/csv.mjs`.
- Importação idempotente: rodar duas vezes não duplica, e o `dedupe` cruza com as vagas atuais.
- O ZIP fica em `dados/importacoes/`, fora do git.
- Testes com CSVs de exemplo anonimizados.

**Pronto quando:** as candidaturas antigas aparecem em Vagas e Números, separadas das atuais por filtro.

---

## T2 — Recrutadores e empresas-alvo

**Problema:** mandar mensagem para recrutadores funciona muito bem, mas hoje é manual e sem controle.

**Como funciona**
1. **Para cada vaga enviada:** o Claude busca quem publicou ("Meet the hiring team" na vaga do LinkedIn) e, se não houver,
   quem é Talent Acquisition ou Tech Recruiter da empresa.
2. **Empresas-alvo:** ranking das empresas com mais vagas Java, Kotlin e backend em Descoberta e Enviada nas últimas semanas.
   Mesmo sem vaga aberta agora, vale procurar o recrutador delas.
3. **Mensagens:** o Claude escreve cada mensagem com base em:
   - a vaga e o caso real do currículo que mais combina com ela (ex.: Cashout 66%→98%);
   - um tom curto, num template por situação: "acabei de aplicar", "empresa-alvo sem vaga", "follow-up sem resposta em 5 dias".
   - Limites do LinkedIn: o convite com nota tem até 300 caracteres e contas gratuitas têm poucas notas por mês, então o texto
     principal vai depois da conexão aceita.
4. **Fila de aprovação** na Central (tela "Recrutadores"): a pessoa edita e marca as aprovadas.
   No chat, o Claude lista as aprovadas e só envia depois de um "pode mandar" explícito para aquele lote.
5. **Ritmo humano:** no máximo 10–15 convites por dia e 80 por semana, com intervalos. Respostas entram no follow-up que já existe.

**Código**
- `dados/contatos.json`: `{ id, nome, cargo, empresa, linkedin, vagaId, origem, status (sugerido|aprovado|enviado|conectado|respondeu|sem-resposta), mensagens[] }`.
- CLI: `contato add|mover|mensagem`, e `alvos` para o ranking de empresas.
- Web: tela Recrutadores (fila, status, histórico); o detalhe da vaga mostra os contatos dela.
- `automacao/linkedin-pessoas.js`: busca o hiring team e as pessoas pela API voyager, no padrão dos scripts atuais.

**Riscos:** o LinkedIn restringe contas que automatizam convites. Por isso o volume é baixo, nada vai sem aprovação
e o envio é feito como uma pessoa faria.

**Pronto quando:** cada vaga enviada tem um contato sugerido com mensagem pronta e a fila de aprovação controla tudo que sai.

---

## T5 — Aprendizados: passado + atual + entrevistas

**Problema:** sem cruzar dados, não dá para saber o que funciona (qual plataforma responde, qual tipo de vaga chama para entrevista,
onde as entrevistas travam).

**Como funciona**
- **Funil completo**, separado por plataforma, stack, faixa salarial, idioma, contrato, aderência A/B/C e "com ou sem mensagem a recrutador".
  O funil é: enviadas → retorno → entrevista → oferta.
- **Tempo até a primeira resposta** por plataforma e empresa (base para o follow-up).
- **Entrevistas ao longo do tempo:** a nota de cada dimensão da T3 em gráfico e os pontos fracos que se repetem.
- **Playbook pessoal**, em `dados/playbook.md`, atualizado a cada revisão:
  - banco de histórias STAR que funcionaram;
  - respostas para perguntas recorrentes;
  - perguntas que pegaram de surpresa.
- **Recomendações que voltam para a busca:** priorizar as plataformas e os títulos que mais convertem, sugerir ajuste de pretensão
  e propor mudanças no perfil. Toda mudança no perfil passa pela aprovação da pessoa.

**Código:** `nucleo/funil.mjs` ganha recortes e tempos. A tela Números ganha esses recortes e a nova tela Aprendizados mostra a evolução
e o playbook. A comparação histórica usa `origem: historico` (da T4).

**Pronto quando:** a tela mostra "o que funciona para você" com números, e o `/skill-job` usa isso para escolher as buscas da rodada.

---

## T6 — Extensão de navegador

**Referência:** Leapfrog (ver "Concorrentes de referência"). Ele só destaca vagas; nós destacamos **e** aplicamos.

**Filtro de vagas internacionais (para empatar com o Leapfrog no dia 1)** — já testado na rodada 3 do cliente zero:
- busca com localização "Latin America" do LinkedIn (`geoId 91000011`) além de Brasil;
- descarta vaga "remota" presa a um país (México, Colômbia, Argentina...) e CLT/PJ quando o cliente quer USD;
- sinais na descrição: USD/contractor, empresa dos EUA/Europa, LATAM/nearshore;
- **base própria de empresas que contratam LATAM**, alimentada pelas candidaturas de todos os clientes (ex.: BairesDev, Jobgether,
  Nortal, GeorgiaTEK, INSPYR). É o ativo que o Leapfrog tem e nós passamos a ter com o uso.

**O diferencial possível:** os concorrentes preenchem formulários. A Central já tem o que eles não têm:
- funil, triagem e regras (A/B/C, filtros de nível, nunca inventar experiência);
- respostas padrão revisadas;
- scripts que já funcionam em LinkedIn, Greenhouse, Ashby, InHire e Gupy;
- histórico e aprendizados (T5).

**Como funciona (MVP)**
1. Extensão Chrome MV3, com o painel lateral da Central.
2. Ao abrir uma vaga, o painel mostra aderência, alertas e "já aplicou?" (via `dup`).
3. O botão "Preencher" usa os scripts de `automacao/` empacotados e as respostas padrão. A pessoa revisa e clica em enviar.
4. Captura com um clique: a vaga aberta vai para a Central.
5. Fala com a Central local (`localhost:4321` ou MCP da T8) e depois com a plataforma da T7.

**Código:** pasta `extensao/` (manifest, painel lateral, content scripts por ATS). Os scripts de `automacao/` são refatorados
em módulos usados pela extensão e pelo Claude.

**Papel no produto:** a extensão é o que o cliente instala. Ela substitui o Claude Code + Claude in Chrome que o cliente zero usa hoje.
- O cliente faz login na extensão com a conta do produto.
- A extensão fala com a API da nuvem (T7) ou com a Central local.
- Ela também resolve o bloqueio atual de "LinkedIn → localhost": a extensão tem permissão própria e não depende do fetch da página.
- A busca de vagas (scripts de `automacao/busca-*`) também passa a rodar por ela, na sessão do próprio cliente.

**Riscos:** o LinkedIn proíbe ferramentas que automatizam a plataforma e a Chrome Web Store revisa extensões com acesso amplo.
Envio sempre com clique humano. Nada de captcha ou login automático.

**Pronto quando:** a extensão preenche Greenhouse e Ashby de ponta a ponta com as respostas da Central e registra o envio.

---

## T7 — Plataforma multiusuário

**Referência:** Applika (tracker grátis e open source) e Leapfrog (descoberta paga). Ver "Concorrentes de referência".

**Hipótese de preço para testar na página de espera:** grátis com descoberta limitada (como o Leapfrog);
plano pago entre R$ 49 e R$ 99/mês com candidatura, acompanhamento por email e revisão de entrevistas; plano BYOK mais barato.
Pacote trimestral, porque busca de emprego dura meses. O teto real sai do custo de IA medido na T0/T9.

**Antes de codar (validação):**
- Entrevistar 10–20 devs que estão procurando vaga: o que já usam, o que pagariam, o que os assusta
  (perder a conta do LinkedIn, aplicação ruim em massa).
- Definir a tese. Sugestão: **"qualidade em vez de volume"** — triagem honesta, respostas revisadas, aprendizados com entrevistas
  e recrutadores, em vez de mil candidaturas por dia.
- Uma página de espera com preço para medir interesse.

**Onboarding (meta: primeira candidatura em menos de 10 minutos)**
1. Login com Google.
2. Upload do currículo; o produto monta o perfil e o cliente revisa pretensão, nível e stack.
3. Instalar a extensão (um botão leva à Chrome Web Store).
4. Configurar o encaminhamento de email, com passo a passo e email de teste. Pode pular e fazer depois.
5. Opcional: subir o ZIP do LinkedIn.
6. Primeira busca e triagem guiadas.

O painel mostra o que falta conectar, como um checklist, e por que cada item vale a pena.

**Arquitetura alvo**
- Backend com Postgres. O `nucleo/` vira pacote compartilhado e o adaptador `postgres` da T0 entra aqui.
- Login, multiusuário e isolamento de dados por pessoa.
- A automação roda no navegador de cada pessoa (extensão da T6), nunca em servidor com a conta dela.
- Claude via API para triagem, mensagens e revisão de entrevistas.
- **LGPD:** transcripts e conversas são dados sensíveis de terceiros (exigem consentimento, retenção e exclusão).
  CPF continua fora do sistema.
- Cobrança por assinatura.

**Pronto quando (MVP):** 10 pessoas usando de verdade por 2 semanas, com funil e revisão de entrevistas funcionando.

---

## Perguntas em aberto
1. ~~Links do Leapfrog e do Applika~~ — respondido; ver "Concorrentes de referência".
2. ~~Os emails das candidaturas chegam todos no seu email principal?~~ Respondida: chegam no email do perfil (T1).
3. Você grava ou transcreve as entrevistas hoje? Com qual ferramenta (Meet, Zoom, Teams, tl;dv, Otter)? (T3)
4. ~~Público inicial~~ — **decidido em 2026-11-06:** devs brasileiros/LATAM, com seletor de foco internacional, nacional ou os dois.
   Ver "Público e foco".
5. O repositório `FRF-Tecnology/job-control` continua público? Se o produto for comercial, vale separar o núcleo aberto
   da plataforma fechada.
