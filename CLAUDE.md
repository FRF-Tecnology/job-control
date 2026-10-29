# Central de Candidaturas — instruções para o Claude

Este projeto organiza a busca de emprego de **uma pessoa por computador**. O perfil dela fica em `dados/perfil.json`.
Leia-o antes de qualquer rodada. Ele traz nome, pretensões, inglês, formação, filtros, respostas padrão e modo de trabalho.

## Como gravar (obrigatório)
- **Nunca edite `dados/*.json` à mão.** Toda alteração passa pela CLI, que valida, deduplica e registra na linha do tempo:
  `node ferramentas/cli.mjs <comando>`. Rode `node ferramentas/cli.mjs` sem argumentos para ver a ajuda.
- A pessoa acompanha tudo ao vivo em http://localhost:4321 (aberto com `iniciar.bat` / `iniciar.sh`).
  Os scripts de busca precisam da Central aberta.
- Antes de aplicar qualquer vaga, rode `cli.mjs dup "<empresa>" "<título>"`. Se der "JÁ EXISTE", não aplique de novo.
- Fim de cada rodada: `cli.mjs rodada --json '{...}'` e `cli.mjs exportar-md ../historico-candidaturas.md`
  (ou outro caminho que a pessoa preferir).

## Regras que não mudam
1. **Nunca preencher CPF, RG, senhas, dados bancários** e nunca guardá-los. Se o formulário exigir, preencha todo o resto,
   deixe a aba aberta e mova para `bloqueada` com a pendência ("digitar CPF e enviar") e o link.
2. **Nunca inventar experiência.** Se não está no currículo nem em `respostasPadrao`, a resposta é "não" ou "sem experiência".
3. Criar conta, fazer login com senha, resolver captcha e fazer testes pessoais (comportamentais, cognitivos, vídeo, entrevista por IA)
   ficam com a pessoa: vaga `bloqueada` com a pendência certa.
4. Só vagas dentro dos `filtros` do perfil (nível, modelo, o que pular). Na dúvida, a vaga vai para Descoberta com o alerta em `motivoClaude`,
   e não é aplicada.
5. Pretensão: CLT ou PJ conforme o rótulo do campo (cooperado = PJ). Campo só numérico: apenas dígitos.
6. Mensagens de follow-up: só sugerir o texto. Enviar mensagem em nome da pessoa exige um "pode mandar" explícito no chat.
7. Currículo: `dados/curriculos/` (PT ou EN pelo idioma da vaga, caminhos em `perfil.curriculos`).

## Etapas de uma vaga
`descoberta` → `aplicar` → `enviada` → `em-processo` (sub: teste, entrevista-rh, entrevista-tecnica, proposta) → `oferta`.
Saídas: `pulada` (motivo obrigatório), `encerrada` (motivo obrigatório: Reprovada, Sem retorno, Vaga fechada, Desisti), `bloqueada` (pendência obrigatória).
Motivos de pulo padronizados: Sênior, Inglês, Diploma, Stack, Presencial/Local, Documento, Duplicada, Vaga fechada, Outro.

## Modo de trabalho (`perfil.modo`)
- `revisar`: aplicar só o que está na etapa `aplicar` (aprovado pela pessoa na Triagem).
- `direto`: também pode aplicar as vagas do **grupo A** em `descoberta`. B e C sempre esperam a Triagem.

## Ferramentas de navegador
Use o Claude in Chrome. Scripts prontos em `automacao/`. Gere a versão preenchida com
`node ferramentas/cli.mjs script <nome> [opções]` e injete com `javascript_tool`. Detalhes técnicos por plataforma
(InHire, Gupy, LinkedIn, dropdowns, questionários): `automacao/como-aplicar.md`.
Os scripts de busca rodam em segundo plano. Consulte `window.__CENTRAL_RESULTADO` até deixar de ser `'rodando'`.
Se o resultado começar com `PENDENTE`, o Chrome bloqueou o acesso a localhost (permissão "rede local" ainda não concedida).
Nesse caso:
- leia `window.__CENTRAL_PARTE(0)`, `(1)`… (uma vaga por parte) e grave cada uma com `cli.mjs add --uri "<parte>"`;
- avise a pessoa que permitir o acesso à rede local naquele site deixa as próximas buscas automáticas.

Nunca clique em prompts de permissão do navegador por ela.

## Comandos
`/buscar`, `/triagem`, `/aplicar`, `/status`, `/followup` e `/rodada` estão em `.claude/commands/`.
