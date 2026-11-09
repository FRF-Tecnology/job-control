# Roteiro técnico da `.6.5`: primeira candidatura real, supervisionada

Bead `jc-a2y.6.5`. Segue os pré-requisitos e o roteiro do relatório do CyberSec (`Shared/seguranca/p5-revisao-integrada/RELATORIO.md`). É **uma** candidatura, numa vaga escolhida pelo Vinicius, com o Vinicius e o CyberSec acompanhando.

> **Regras do dia**
> - Só o **app compilado** com a tag `envio_dev`. Ele não abre porta TCP: a API só fala com a janela do Wails. O `central-dev` nunca envia: ele não tem o executor, e um teste trava isso.
> - O binário `envio_dev` fica **só na máquina do teste**: nunca é distribuído nem vai para o git, e é apagado no fim (passo 8).
> - O fluxo antigo (Claude in Chrome, `/aplicar` do Node) fica **parado** nessa vaga, para não candidatar duas vezes.
> - O login no LinkedIn é feito pela **própria pessoa**, no perfil do app. Nenhum agente digita senha.

## 0. Antes do dia

- [ ] Integrados em `reescrita-go`:
  - `jc-a2y.49`: apagar meus dados e sair das plataformas;
  - `jc-a2y.50`: teste integrado;
  - `jc-a2y.51`: pausar, desfechos e a tela do #60;
  - #53: bloqueio de popups.
- [ ] `jc-a2y.52` (QA): conferir a ACL da pasta do perfil na máquina do teste (passo 3).
- [ ] Se a vaga for da InHire, rever a regra do `form-app` com o formulário real (id curto, nota do #49). LinkedIn é o caso esperado.
- [ ] Chrome ou Edge instalado num local padrão: `C:\Program Files\Google\Chrome\Application\chrome.exe` (ou `Program Files (x86)`, ou a instalação só do usuário em `%LOCALAPPDATA%`), ou o Edge em `Program Files (x86)\Microsoft\Edge\Application\msedge.exe`. O app **não** usa o PATH.
- [ ] **A conferir antes do dia: o log.** O app é compilado como janela (`-H windowsgui`), então não tem console. O log vai para a saída de erro do processo. Ensaiar a captura no passo 5, com o build padrão e o `CENTRAL_PASTA` de teste, e confirmar que o arquivo de log recebe a linha `envio: executor LIGADO`. Se não receber, o CyberSec decide outro jeito antes do dia.

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
$env:CENTRAL_PASTA = "C:\central-teste-6.5"
go run ./cmd/central-cli importar-legado --dir dados --dry-run   # mostra o que viria da Central antiga
go run ./cmd/central-cli importar-legado --dir dados             # importa vagas, perfil e currículos
```

- O importador recusa CPF e RG, e só sobrescreve com `--substituir`.
- Faça a importação **antes** do passo 4: no passo 4, a vaga sai de `aplicar` no Node.
- Use a mesma janela do PowerShell, com o `CENTRAL_PASTA`, em todos os passos seguintes.

## 3. Login no LinkedIn no perfil do app (a pessoa)

Antes de abrir o app, o Vinicius entra no LinkedIn **no perfil próprio do app**, nunca no perfil pessoal:

```powershell
& "C:\Program Files\Google\Chrome\Application\chrome.exe" --user-data-dir="$env:CENTRAL_PASTA\navegador" https://www.linkedin.com/login
```

- A pessoa entra, resolve o 2FA se houver e fecha **todas** as janelas desse Chrome. O app não abre o perfil se ele estiver em uso.
- O app mantém os cookies e regrava as preferências dele ao abrir (sem senhas, autofill nem downloads).
- ACL (`jc-a2y.52`): `icacls "$env:CENTRAL_PASTA\navegador"`. Só o usuário (e SYSTEM e Administradores, se for o padrão da máquina) pode ter acesso; outro usuário da máquina não.

## 4. Parar o fluxo antigo na vaga escolhida (Central Node)

```powershell
node ferramentas/cli.mjs dup "<empresa>" "<título>" --excluir <id>      # não pode listar "enviada <data>"
node ferramentas/cli.mjs mover <id> bloqueada --pendencia "Teste .6.5 no app novo: não aplicar por aqui"
```

- Com a vaga em `bloqueada` no Node, o `/aplicar` e o Claude in Chrome não a pegam.
- Durante o teste, ninguém roda `/aplicar`, `/skill-job` nem o `linkedin-auto`.

## 5. Abrir o app e preparar a candidatura

```powershell
Start-Process -FilePath .\app\bin\central.exe -RedirectStandardError "C:\central-teste-6.5-log.txt"
```

O log fica **fora** da pasta de dados, para não ser apagado no passo 8. Confira se a captura funciona (passo 0).

| Passo | O que conferir |
|---|---|
| Abrir | O painel do envio diz **ligado e pausado**. Abrir o app sempre pausa. No log: `envio: executor LIGADO`. |
| Triagem | A vaga está em `aplicar`, aprovada pela pessoa (aperte A, se não estiver na Fila). |
| Limite | Pôr o limite do LinkedIn em **1** para o dia. |
| Preparar | A Fila mostra o pacote inteiro: currículo (só o nome do PDF), contato, respostas e mensagem. Nada de CPF, RG, senha ou banco; o pacote recusa isso com 400. |
| Aprovar | A impressão mostrada é a que vai. A aprovação vale uma vez e até 23:59:59 de hoje. |

## 6. Retomar e acompanhar o envio

- O Vinicius aperta **Retomar**. No log: `envio: retomado (eu)`.
- Em até 2 minutos, o executor pega a aprovada, e o Chrome do app abre **visível**.
- **O que conferir na tela do navegador, em cada etapa:**
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
5. O CyberSec registra o resultado no bead `jc-a2y.6.5`, com o log de `C:\central-teste-6.5-log.txt`, sem conteúdo.
