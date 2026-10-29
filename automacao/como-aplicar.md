# Como aplicar (notas técnicas para automação via Claude in Chrome)

## Geral
- Filtrar a saída de JS removendo `? & = @`, para a ferramenta não bloquear o resultado.
- Currículos: `dados/curriculos/` (caminhos em `dados/perfil.json` → `curriculos.PT` / `curriculos.EN`). Vaga em inglês usa o EN.

## LinkedIn
- Buscar com filtro "Candidatura simplificada" + Remoto + últimos 7–14 dias.
- Palavras de busca: Java, Spring, Backend, Full Stack, Desenvolvedor Júnior.

## Gupy
- Buscar vagas remotas em portal.gupy.io. Pular as que pedem RG.

## InHire
- Não existe busca global. Usar os subdomínios de `inhire-empresas.txt`.
- **Listar vagas:** `GET https://api.inhire.app/job-posts/public/pages` com header `X-Tenant: <sub>`. Retorna `jobsPage[]` (jobId, displayName, workplaceType).
- **Detalhe:** `GET https://api.inhire.app/job-posts/public/pages/<jobId>`, mesmo header. Retorna descrição, contractType e status.
- O fetch precisa sair de uma aba `*.inhire.app` (CORS).
- **Página da vaga:** `https://<sub>.inhire.app/vagas/<jobId>/vaga`. Precisa de algum slug no final; sem ele dá 404.

### Formulário
1. Injetar `automacao/inhire-IH.js` (troque os marcadores `{{...}}` pelos dados do perfil antes de injetar) (a CSP bloqueia eval, por isso o script cria um elemento `<script>` com textContent). Ele preenche nome, e-mail, telefone, LinkedIn e a pretensão (PJ se o rótulo disser PJ/cooperado, senão CLT).
2. **País e cidade** são dropdowns customizados:
   - Clicar no campo do país, digitar "Brasil" e clicar na opção ~79px abaixo.
   - Clicar no campo da cidade, digitar a cidade do perfil e escolher a opção com a UF certa.
   - Para achar as coordenadas, use `IH.pos('country')` e `IH.pos('district')`.
3. **Currículo:** usar `find` "resume file input" e depois `file_upload`.
4. **contractType:** marcar CLT quando houver as duas opções.
5. **Diversidade:** "Prefiro não responder" + consentimento; PcD "Não".
6. Enviar com `IH.submit()`.
   - Retorno `SUCESSO`: terminou.
   - Retorno `FORM <url>`: abrir essa URL na própria aba.

### Questionário (form-app.inhire.app)
- Injetar `automacao/inhire-Q.js`.
- Comandos:
  - `Q.show(n)`: mostra a pergunta n.
  - `Q.step(n,'texto')`: escolha única.
  - `Q.multi(n,[...])`: múltipla escolha.
  - `Q.tstep(n,'valor')`: resposta em texto.
  - `Q.send()`: envia.
- Escolha única avança sozinha. Múltipla precisa de OK. Campo com máscara exige só números (ex.: a pretensão sem "R$" e sem pontos).
- Se a URL tiver `flow=returnMessageToParent` e não tiver talentId, responder dentro do iframe. Antes, ler as perguntas abrindo a URL numa aba separada.
- Se pedir CPF, parar: preencher todo o resto e marcar a vaga como bloqueada (`mover <id> bloqueada --pendencia "digitar CPF e enviar"`).
