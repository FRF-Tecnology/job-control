# Preencher no site da empresa

Status: **proposta com código** (contrato 0.21.0), para revisão do CyberSec no PR. Pedido da Maritza (PO).

## O problema

Muitas vagas do LinkedIn não têm Candidatura simplificada. O "Candidatar-se" leva ao site da empresa: Greenhouse, Lever, Workday, ou um site próprio. Hoje o envio para nessas vagas de propósito (M5: a página tem de ficar na plataforma e na vaga aprovada, senão `bloqueada/site-externo`), e a pessoa preenche tudo à mão.

Uma lista de sistemas conhecidos (Greenhouse, Lever…) não cobre os sites próprios, que são muitos.

## A proposta

O envio automático **não muda**: a M5 continua valendo para ele.

Entra um **caminho separado**, iniciado pela pessoa, que só **preenche**. Quem revisa e envia é a pessoa.

1. A vaga já está reservada com "Aplicar na vaga" (`iniciar-manual`, como hoje).
2. "Abrir e preencher no app" (`POST /vagas/{id}/abrir-para-preencher`):
   - o app abre a vaga numa **janela visível** do navegador dele, com a sessão da pessoa, e tenta clicar em "Candidatar-se";
   - o site da empresa abre na mesma aba (o app troca o `window.open` da página por uma navegação na própria aba). Se não abrir, a pessoa clica.
3. "Preencher esta página" (`POST /vagas/{id}/preencher` sem corpo): o app devolve **só o domínio** da aba do site, e a tela pergunta "Preencher em careers.acme.com?".
4. "Preencher" (`POST /vagas/{id}/preencher {dominio}`): o app confere de novo que a aba está nesse domínio (na lista de abas e na própria aba) e preenche.
5. A pessoa revisa, completa, avança e envia no site. Depois, "Já enviei" (`confirmar-envio-manual`, como hoje).

## Regras

**Quem e quando:**
- Só a pessoa, só pela tela (`x-so-tela`). O assistente recebe 403.
- Só vaga reservada (`aplicacaoManual`, em `aplicar`).
- Sem o envio no build: 404.

**A janela:**
- Visível, com a mesma trava da janela de login: enquanto ela está aberta, nenhuma página do envio automático abre.
- Abrir espera o ciclo atual do executor terminar.

**O domínio:**
- A pessoa vê o domínio **antes** de qualquer dado entrar na página.
- O domínio é conferido de novo na hora de preencher. Se mudou, 409 `site-mudou` e nada é preenchido.
- Só https. Abas do LinkedIn não contam como site da empresa.

**O que o app preenche** (`executor.PlanejarAssistido`):
- contato do perfil: nome (ou nome e sobrenome separados), e-mail, telefone e cidade;
- os links de LinkedIn e GitHub do perfil, só se forem https no site certo;
- respostas padrão cuja pergunta tem **o mesmo texto**, porque o app nunca inventa nem aproxima;
- o currículo da reserva, conferido pelo sha256.

**O que o app nunca faz:**
- sobrescrever campo que a página trouxe preenchido;
- preencher campo sensível (senha, RG, documento, dado bancário; a lista do `MotivoDoRotulo` e do detector). O CPF
  é a exceção desde o contrato 0.22.0: só em site brasileiro e só depois de a pessoa confirmar naquele domínio
  (cpf-no-preencher.md);
- marcar caixa (termos, newsletter, "seguir");
- preencher pretensão (a moeda e o período variam por site);
- **avançar ou enviar**. Nenhum clique no formulário: o único clique do app é o "Candidatar-se" do LinkedIn.

**A cópia do currículo** fica numa pasta própria da janela e é apagada quando ela fecha. O site lê o arquivo no envio, então ela não pode sumir antes.

**A resposta e o log** levam só números (`preenchidos`, `anexou`, `faltando`), o domínio vai só na resposta do passo 3, e nenhum texto da página vai para o log. A tela usa frases fixas por motivo.

## Riscos e respostas

| Risco | Resposta |
|---|---|
| Vaga falsa leva a um site de golpe, que recebe nome, e-mail, telefone e currículo | A pessoa vê e confirma o domínio antes; nada é preenchido sem isso. O app não envia, mas um site pode ler campos preenchidos: por isso a confirmação vem antes de preencher, não antes de enviar. |
| A página troca de site entre conferir e preencher | O domínio é conferido de novo na lista de abas e na própria aba. |
| O app aceita termos ou envia sem a pessoa | Caixas nunca são marcadas, e nada é clicado no formulário. O teste com o navegador de verdade confere que o servidor não recebeu envio. |
| Campo sensível preenchido | Os mesmos detectores do envio, mais a regra "na dúvida, em branco". |
| O envio automático roda ao mesmo tempo | A janela usa a trava do login. |

## Fica para depois

- Formulário dentro de um quadro (iframe) de outro domínio, comum com Greenhouse embutido no site da empresa: hoje o app só lê a página principal, e a tela diz "preenchi 0 campos".
- Formulário em várias etapas: o app preenche a etapa atual. A pessoa avança e clica de novo em "Preencher esta página".
- Envio automático nos sistemas mais comuns (Greenhouse, Lever, Ashby), quando houver dados de quais aparecem mais.
