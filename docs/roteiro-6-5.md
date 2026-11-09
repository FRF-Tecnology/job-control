# Roteiro técnico da `.6.5`: primeira candidatura real, supervisionada

Bead `jc-a2y.6.5`. Segue os pré-requisitos e o roteiro do relatório do CyberSec (`Shared/seguranca/p5-revisao-integrada/RELATORIO.md`). É **uma** candidatura, numa vaga escolhida pelo Vinicius, com o Vinicius e o CyberSec acompanhando.

> **Regras do dia**
> - Só o **app compilado** com a tag `envio_dev`. Ele não abre porta TCP: a API só fala com a janela do Wails. O `central-dev` nunca envia: ele não tem o executor, e um teste trava isso.
> - O binário `envio_dev` fica **só na máquina do teste**: nunca é distribuído nem vai para o git, e é apagado no fim (passo 8).
> - O fluxo antigo (Claude in Chrome, `/aplicar` do Node) fica **parado** nessa vaga, para não candidatar duas vezes.
> - O login no LinkedIn é feito pela **própria pessoa**, no perfil do app. Nenhum agente digita senha.
> - **Só o Vinicius** aprova a candidatura e aperta Retomar. Nenhum agente controla a janela do app nem a do navegador, e nenhum agente roda a CLI (`central-cli`, `cli.mjs`) na pasta do teste durante o dia.
> - **A pasta do teste e o log ficam em `%LOCALAPPDATA%`**, nunca na raiz de `C:\`. Na raiz, as pastas herdam acesso de "Usuários autenticados", e outro usuário da máquina leria ou mudaria a sessão do LinkedIn (revisão do CyberSec no #62).

## 0. Antes do dia

- [ ] Integrados em `reescrita-go`:
  - `jc-a2y.49`: apagar meus dados e sair das plataformas;
  - `jc-a2y.50`: teste integrado;
  - `jc-a2y.51`: pausar, desfechos e a tela do #60;
  - #53: bloqueio de popups.
- [ ] `jc-a2y.52` (QA): conferir a ACL da pasta do perfil na máquina do teste (passo 3).
- [ ] Se a vaga for da InHire, rever a regra do `form-app` com o formulário real (id curto, nota do #49). LinkedIn é o caso esperado.
- [ ] Chrome ou Edge instalado num local padrão: `C:\Program Files\Google\Chrome\Application\chrome.exe` (ou `Program Files (x86)`, ou a instalação só do usuário em `%LOCALAPPDATA%`), ou o Edge em `Program Files (x86)\Microsoft\Edge\Application\msedge.exe`. O app **não** usa o PATH.
- [ ] **Ensaiar o log antes do dia.** O app é compilado como janela (`-H windowsgui`), então não tem console. O log vai para a saída de erro do processo. Ensaiar a captura do passo 5 com o build `envio_dev` e uma pasta de ensaio, e confirmar que o arquivo recebe a linha `envio: executor LIGADO`. Se o `Start-Process` não servir, a alternativa vai no passo 5.

## 1. Compilar o app com o envio (Windows, PowerShell, na raiz do repositório)

```powershell
git fetch origin; git checkout reescrita-go; git pull
cd app
wails3 build EXTRA_TAGS=envio_dev      # gera app\bin\central.exe com -tags production,envio_dev
cd ..
go version -m app\bin\central.exe | Select-String -Pattern "-tags|chromedp"
```

Tem de aparecer `build -tags=production,envio_dev` e `dep github.com/chromedp/chromedp v0.21.1`. Sem isso, o binário não é o do teste e não envia. Conferido em 2026-11-09: o build padrão (`wails3 build`) mostra só `-tags=production`, sem chromedp.

## 2. Pasta de dados nova, só para o teste

A pasta normal do app fica intacta. Tudo do teste vai para uma pasta própria, pelo `CENTRAL_PASTA`:

```powershell
$env:CENTRAL_PASTA = "$env:LOCALAPPDATA\central-teste-6.5"   # nunca na raiz de C:\
New-Item -ItemType Directory -Force $env:CENTRAL_PASTA | Out-Null
icacls $env:CENTRAL_PASTA   # não pode listar "Usuários autenticados", "Usuários" nem "Todos"
go run ./cmd/central-cli importar-legado --dir dados --dry-run   # mostra o que viria da Central antiga
go run ./cmd/central-cli importar-legado --dir dados             # importa vagas, perfil e currículos
```

- O importador recusa CPF e RG, e só sobrescreve com `--substituir`.
- Faça a importação **antes** do passo 4: no passo 4, a vaga sai de `aplicar` no Node.
- Use a mesma janela do PowerShell, com o `CENTRAL_PASTA`, em todos os passos seguintes.
- Se o `icacls` listar algum desses grupos, pare e chame o CyberSec antes de seguir: a pasta tem de ser só do usuário.

## 3. Login no LinkedIn no perfil do app (a pessoa)

Antes, **abra o app uma vez e feche**, com o `CENTRAL_PASTA` do passo 2 (o comando do passo 5). Assim ele cria o perfil já com as preferências: sem salvar senha, sem autofill, sem downloads. Depois, o Vinicius entra no LinkedIn **no perfil próprio do app**, nunca no perfil pessoal:

```powershell
& "C:\Program Files\Google\Chrome\Application\chrome.exe" --user-data-dir="$env:CENTRAL_PASTA\navegador" https://www.linkedin.com/login
```

- Se a tela já tiver o botão **"Entrar nas plataformas"** (`POST /navegador/entrar`, `jc-a2y.6.7`), use-o no lugar do comando acima. Ele abre o login do LinkedIn no perfil do app numa janela visível, e o envio espera até a janela fechar.
- A pessoa entra e resolve o 2FA, se houver.
- Quando o Chrome oferecer **salvar a senha**, escolha **"Nunca"**.
- **Não** entre com conta Google nem ligue a sincronização nesse perfil.
- Feche **todas** as janelas desse Chrome e confira que ele não ficou rodando em segundo plano: nenhum ícone do Chrome perto do relógio e nenhum `chrome.exe` desse perfil no Gerenciador de Tarefas. O app não abre o perfil se ele estiver em uso.
- O app mantém os cookies e regrava as preferências dele ao abrir.
- ACL (`jc-a2y.52`): `icacls "$env:CENTRAL_PASTA\navegador"`. Só podem aparecer o usuário, SYSTEM e Administradores; "Usuários autenticados", "Usuários" ou "Todos" não podem aparecer.

## 4. Parar o fluxo antigo na vaga escolhida (Central Node)

```powershell
node ferramentas/cli.mjs dup "<empresa>" "<título>" --excluir <id>      # não pode listar "enviada <data>"
node ferramentas/cli.mjs mover <id> bloqueada --pendencia "Teste .6.5 no app novo: não aplicar por aqui"
```

- Com a vaga em `bloqueada` no Node, o `/aplicar` e o Claude in Chrome não a pegam.
- Durante o teste, ninguém roda `/aplicar`, `/skill-job` nem o `linkedin-auto`.

## 5. Abrir o app e preparar a candidatura

```powershell
$log = "$env:LOCALAPPDATA\central-teste-6.5-log.txt"
Start-Process -FilePath .\app\bin\central.exe -RedirectStandardError $log
```

O log fica **fora** da pasta de dados, para não ser apagado no passo 8, e em `%LOCALAPPDATA%`, nunca na raiz de `C:\`. Se o ensaio do passo 0 mostrar que o `Start-Process` não grava o log, use `cmd /c "app\bin\central.exe 2> %LOCALAPPDATA%\central-teste-6.5-log.txt"`.

| Passo | O que conferir |
|---|---|
| Abrir | O painel do envio diz **ligado e pausado**. Abrir o app sempre pausa. No log: `envio: executor LIGADO`. |
| Triagem | A vaga está em `aplicar`, aprovada pela pessoa (aperte A, se não estiver na Fila). |
| Limite | A Fila mostra o limite de hoje (LinkedIn 30). Não há tela para mudá-lo, e não precisa: só **uma** candidatura é aprovada no dia. |
| Preparar | A Fila mostra o pacote inteiro: currículo (só o nome do PDF), contato, respostas e mensagem. Nada de CPF, RG, senha ou banco; o pacote recusa isso com 400. |
| Aprovar | A impressão mostrada é a que vai. A aprovação vale uma vez e até 23:59:59 de hoje. |

## 6. Retomar e acompanhar o envio

- O envio roda **sem janela** por padrão (`jc-a2y.6.7`): não pega o foco, o mouse nem o teclado. Para acompanhar no dia, o Vinicius liga **"Ver o envio"** antes de Retomar (`POST /envio/janela`, no log `envio: janela visível (eu)`). Assim a candidatura abre numa janela visível.
- O Vinicius aperta **Retomar**. No log: `envio: retomado (eu)`.
- Em até 2 minutos, o executor pega a aprovada. Com "Ver o envio" ligado, o Chrome do app abre **visível**; sem ele, nada aparece, e o desfecho sai no painel.
- Conferir também se o LinkedIn aceita o envio sem janela (o padrão). Se recusar, anotar no bead `jc-a2y.6.7`: a próxima etapa é a janela que não pega o foco.
- **O que conferir na tela do navegador, em cada etapa (com "Ver o envio"):**
  - o endereço continua em `linkedin.com` e na mesma vaga (`/jobs/view/<id>`, ou `currentJobId=<id>`);
  - os campos preenchidos são só os do pacote;
  - nada é digitado em campo sensível;
  - ao aparecer CPF, senha, captcha, login, termos obrigatórios ou pergunta fora do pacote, o executor para em `bloqueada`, com o motivo e a pendência.
- **Para interromper:**
  - **Pausar** impede a próxima candidatura, mas a que está em envio termina;
  - **fechar o app** no meio deixa a candidatura em `incerta` ao reabrir, e ela nunca é reenviada sozinha.
- **O CyberSec confere o log:** só id, estado, motivo e tipo de erro. Nenhum conteúdo, URL, rótulo, nome nem pasta.

## 7. Depois do desfecho

| Desfecho | O que fazer |
|---|---|
| `enviada` | Conferir no LinkedIn ("Candidaturas") que chegou **uma** vez. Registrar no Node: `node ferramentas/cli.mjs mover <id> enviada`. |
| `bloqueada` | Ler o motivo e a pendência. A pessoa termina à mão no LinkedIn, se quiser, e usa "Concluir manualmente" no app. |
| `incerta` | Conferir no LinkedIn se chegou. "Concluir manualmente" se sim; "Reabrir" (e aprovar de novo) se não. |
| `falhou` | Ler o detalhe (currículo mudou ou erro técnico) e decidir com o CyberSec antes de tentar outra vez. |

O desfecho também aparece no painel do envio ("Últimos envios").

## 8. Fechar o dia

1. No app, **Pausar** e depois "Sair das plataformas", que apaga a sessão do LinkedIn do perfil do app.
2. Decidir com o Vinicius se a pasta de teste fica, para rever, ou sai com **"Apagar meus dados"**. Apagar também testa o `jc-a2y.49` de verdade: o app fecha e a pasta fica sem os arquivos dele.
3. Recompilar o padrão, que apaga o binário de envio: `cd app; wails3 build`. Depois, `go version -m app\bin\central.exe` não pode mostrar `envio_dev` nem chromedp.
4. Na Central Node, se a vaga não foi enviada, voltar com ela para onde estava (`mover <id> aplicar` só pela pessoa, na Triagem).
5. O CyberSec registra o resultado no bead `jc-a2y.6.5`, a partir do log em `%LOCALAPPDATA%\central-teste-6.5-log.txt`, sem conteúdo. **Depois do registro, apague o log** (`Remove-Item $log`).
