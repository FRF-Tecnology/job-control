# Applyer

> **Nome novo.** O produto agora se chama **Applyer** e está virando um app de PC (veja `docs/status.md`). A versão em uso hoje é a **Central** (versão em Node, sai na P8), descrita abaixo. A tela dela ainda diz "Central", e os nomes internos (`dados/`, o MCP "central") continuam iguais.

Um painel local para organizar a busca de emprego: o que foi encontrado, o que você decidiu, o que foi enviado, o que travou e quando fazer follow-up.
O Claude Code busca e aplica. Você decide e acompanha na tela.

Cada pessoa roda a sua cópia, no próprio computador e com os próprios dados. Nada sai da máquina.

## Abrir
1. Instale o [Node.js](https://nodejs.org) 18 ou mais novo (só uma vez).
2. Dê dois cliques em **`iniciar.bat`** (Windows) ou rode `./iniciar.sh` (Mac/Linux).
3. O navegador abre em http://localhost:4321. Para desligar, feche a janela preta.

## Primeira vez (computador novo)
1. Abra a Central. Ela cria a pasta `dados/` com as plataformas conhecidas.
2. Em **Perfil e regras**, preencha nome, contato, pretensões, inglês, formação, filtros e respostas padrão, e clique em **Salvar perfil**.
3. Coloque seus currículos em `dados/curriculos/` (`curriculo-pt.pdf` e, se tiver, `resume-en.pdf`).
4. Abra o Claude Code **nesta pasta** e peça `/rodada`.

## As telas
| Tela | Para quê |
|---|---|
| **Hoje** | O que fazer agora: o que travou esperando você, follow-ups do dia, processos andando, quantas vagas esperam triagem |
| **Triagem** | Uma vaga por vez: `A` aplica, `P` pula (com motivo), `→` deixa para depois |
| **Quadro** | Todas as vagas por etapa; arraste os cartões entre colunas |
| **Vagas** | Tabela com busca e filtros. Clique para ver o detalhe: respostas dadas, linha do tempo, notas |
| **Números** | Funil, envios por semana, retorno por plataforma e por aderência, motivos de pulo |
| **Plataformas** | Onde procurar, como aplicar e as limitações de cada site |
| **Rodadas** | O histórico de cada rodada do Claude |
| **Perfil e regras** | Seus dados e as regras que o Claude segue |

## Comandos do Claude Code (nesta pasta)
| Comando | O que faz |
|---|---|
| `/buscar [plataforma] [dias]` | Procura vagas novas e coloca na Triagem |
| `/triagem` | Resume o que espera decisão |
| `/aplicar [máx]` | Aplica as vagas aprovadas; o que travar vira "Precisa de você" |
| `/status` | Confere retornos e encerramentos |
| `/followup` | Lista os follow-ups vencidos com mensagem sugerida (só envia com seu ok) |
| `/rodada [máx]` | Tudo acima, em ordem, e atualiza o histórico |
| `/pedidos` | Faz o que você digitou na caixa "O que vamos fazer hoje?" da tela Hoje |
| `/emails` | Lê os retornos por email (só leitura) e muda a etapa das vagas sozinho |

## Usar a Central de dentro do Claude (MCP)
A Central tem um servidor MCP local (`mcp/servidor.mjs`). Com ele, o Claude Desktop ou o Claude Code
listam, movem e registram vagas com ferramentas próprias, sem rodar comandos no terminal.
Ele grava pela mesma API da tela e da CLI, com as mesmas regras (CPF recusado, sem duplicar) e não tem ferramenta de enviar mensagem.

- **Claude Code:** o arquivo `.mcp.json` desta pasta já registra o servidor. Na próxima vez que abrir o Claude Code aqui,
  aprove o servidor "central" quando ele perguntar.
- **Claude Desktop:** em Configurações → Desenvolvedor → Editar configuração, acrescente (troque o caminho pelo desta pasta):
  ```json
  { "mcpServers": { "central": { "command": "node", "args": ["C:\\caminho\\para\\job-control\\mcp\\servidor.mjs"] } } }
  ```
  Reinicie o Claude Desktop. As ferramentas aparecem como "central".

Ferramentas: `resumo`, `vagas_listar`, `vaga_detalhe`, `vaga_duplicada`, `vaga_adicionar`, `vaga_mover`, `vaga_registrar_envio`,
`vaga_editar`, `evento_registrar`, `perfil_ler`, `resposta_padrao`, `followups_vencidos`, `followup_registrar`, `rodada_registrar`,
`pedidos_listar`, `pedido_concluir`, `emails_registrar`, `emails_sem_vaga`.

## Regras de segurança
- CPF, RG, senhas e dados bancários nunca são preenchidos pelo Claude nem guardados na Central.
- O Claude não inventa experiência e não envia mensagens em seu nome sem um "pode mandar".
- A Central só atende o próprio computador. Os scripts de busca usam uma chave local (`dados/.chave`).
- Na primeira busca, o Chrome pode perguntar se o LinkedIn, a Gupy ou a InHire podem **acessar dispositivos da rede local**. Permitir deixa as vagas irem direto para a Central. Se você negar, o Claude copia as vagas em partes (mais lento, mas funciona).

## Testar o app novo no Mac (versão de teste)
O CI gera o Applyer para Mac com chip Apple (M1 ou mais novo) a cada commit no branch `reescrita-go`. É só para teste: **não tem assinatura da Apple** (isso fica para a P8).
1. No GitHub, abra **Actions > CI**, filtre pelo branch **`reescrita-go`** e escolha uma execução verde do tipo **push**. Baixe o artefato **`Applyer-macos-arm64.zip`**. É preciso estar logado; o artefato some depois de 7 dias.
   **Não use** um `.app` de execução de pull request nem de outro repositório: esses não têm artefato, e um `.app` de outra origem pode ter qualquer coisa dentro.
2. Confira o arquivo antes de abrir. No Terminal, rode `shasum -a 256 ~/Downloads/Applyer-macos-arm64.zip`. O código precisa ser igual ao `sha256` do resumo da execução (e ao digest ao lado do artefato). Se for diferente, apague o arquivo e não abra.
3. Abra o zip: sai o `Applyer.app`. Mova-o para **Aplicativos**, se quiser.
4. Na primeira vez, o macOS bloqueia, porque o app não é de um desenvolvedor identificado:
   - **macOS 15 (Sequoia) ou mais novo:** tente abrir, feche o aviso e vá em **Ajustes do Sistema > Privacidade e Segurança**. Lá embaixo, clique em **Abrir Mesmo Assim** e confirme.
   - **macOS 14 ou mais antigo:** clique com o botão direito no `Applyer.app`, escolha **Abrir** e depois **Abrir** de novo.

   Se aparecer que o app "está danificado", rode no Terminal: `xattr -dr com.apple.quarantine /Applications/Applyer.app` (ajuste o caminho se ele estiver em outra pasta).

## Para quem mexe no código
- `nucleo/`: regras (validação, deduplicação, funil, follow-up). Puro e testado.
  - `nucleo/armazens/`: onde os dados moram (`arquivos` no modo local, `memoria` nos testes; Postgres na nuvem depois).
  - `nucleo/ia/`: adaptador de IA (Claude, GPT ou Gemini) com as regras de segurança validadas no código.
- `servidor/api.mjs`: API v1 (`/api/v1/...`). A CLI, a tela e, depois, a extensão e o MCP usam só ela.
- `servidor/app.mjs`: servidor HTTP local (`node:http`, sem dependências) em volta da API.
- `mcp/`: servidor MCP local (stdio) com as ferramentas da Central, também em cima da API v1.
- `web/`: interface (HTML/CSS/JS puros, módulos ES).
- `ferramentas/cli.mjs`: o que o Claude usa para gravar.
- `automacao/`: scripts de navegador e notas por plataforma.
- `dados/`: **seus dados, fora do git**. Backup diário automático em `dados/backups/`.

Testes: `npm test`.
