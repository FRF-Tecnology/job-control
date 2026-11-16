# CPF no "Preencher no site da empresa"

Status: **decisão da PO (Maritza), implementada sem revisão prévia do CyberSec** (contrato 0.22.0). Muda a regra anterior
de nunca preencher CPF (preencher-site-externo.md); o envio automático continua sem CPF. Fica registrada aqui para
revisão posterior.

## O pedido

Muitas vagas brasileiras pedem CPF no site da empresa. Hoje o app deixa o campo em branco e a pessoa digita.

## O risco

- Golpe de vaga falsa pedindo CPF é comum no Brasil: o site imita uma empresa e junta nome, CPF e telefone.
- Nome, contato e CPF juntos num lugar só são um alvo maior que cada um separado.

## Como ficou

**Onde o CPF fica**
- Só no cofre do sistema (Chaves do Mac), com a chave `cpf`, como as chaves de IA. Nunca no banco, no pacote da
  candidatura, no log, na IA, no assistente ou na exportação.
- A tela tem rota própria (`/perfil/cpf`, só da tela) e mostra só a máscara (`***.456.789-**`). O número digitado sai
  da tela quando é salvo.
- O servidor confere os dígitos verificadores; CPF inválido não é gravado.
- "Apagar meus dados" apaga o CPF do cofre junto com as chaves.

**Quando o CPF é preenchido**
1. O primeiro "Preencher" nunca leva o CPF. Num site brasileiro (`.br`, Gupy) com campo de CPF em branco, a resposta
   traz `pedeCPF`.
2. A tela pergunta, com o domínio na frente: "Este formulário pede CPF. Preencher o seu CPF em carreiras.empresa.com.br?",
   e lembra do golpe.
3. Só o clique em "Preencher CPF" manda `cpf: true`. O servidor lê o cofre e o navegador preenche **só** os campos de
   CPF, no domínio conferido, de novo conferido na hora.

**Sem perguntar (opcional, também decisão da PO)**
- Com o CPF salvo, a caixa "Preencher o CPF sem perguntar" (PATCH `/perfil/cpf {automatico}`, preferência no banco)
  faz o CPF ir já no primeiro "Preencher", junto com o resto, no domínio que a pessoa acabou de conferir. A pergunta à
  parte some; a conferência do domínio, não.
- A resposta traz `comCPF`, e a tela diz "Preenchi N campos, incluindo o CPF".
- Apagar o CPF desliga a caixa.

**O que continua proibido**
- Fora de site brasileiro, o CPF é descartado mesmo se pedido.
- Campo que pode ser de empresa ou outro documento ("CPF/CNPJ", CNPJ, SSN, NIF, Tax ID): nunca.
- Campo que a página já trouxe preenchido: fica como está.
- Envio automático (LinkedIn): nunca CPF.
- RG, senha e dado bancário: nunca, como antes.

## Para revisar

- Se `.br` e Gupy bastam como "site brasileiro" ou se a lista deve crescer (Kenoby, Solides, Recrutei…).
- Se o aviso de golpe na pergunta é suficiente, ou se vale uma lista de domínios que nunca recebem CPF.
