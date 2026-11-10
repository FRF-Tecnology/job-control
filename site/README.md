# Landing do Applyer (applyer.dev)

Página de apresentação e de download do app. É HTML, CSS e JavaScript puros: não tem build nem dependência.

## Arquivos

- `index.html`: a página inteira, já com todo o texto.
- `estilo.css`: o visual (vidro, bolhas de cor, botões). Tema claro e escuro.
- `app.js`: a prévia do app, o botão de baixar, a escolha da IA, o preço e as perguntas. Só troca textos e classes do que já está no HTML.
- `tema.js`: aplica o tema salvo antes de a página aparecer.
- `icone.svg`: o ícone da aba.
- `marcas/` contém os ícones usados nos downloads e na seleção de IA, com os créditos das marcas.
- `artes/` reúne as ilustrações do Applyer para os cartões de segurança e os métodos de conexão da IA.

## Ver no computador

Abra `index.html` no navegador, ou sirva a pasta:

```
npx serve site
```

## Antes de publicar

1. **Links de download.** Em `app.js`, preencha `LINKS` com o endereço do instalador de cada sistema. Com o campo vazio, o botão só mostra a animação de exemplo e não baixa nada.
2. **Status de cada sistema.** Em `app.js` (`SISTEMAS`) e nos cartões e menus do `index.html`: macOS e Windows aparecem como disponíveis. Linux, iPhone e Android exibem “Avisar quando sair”. O aviso é uma demonstração local à página.
3. **Textos por preencher**, marcados com colchetes no `index.html`: `[PREÇO]`, `[E-MAIL DE CONTATO]`, `[TERMOS]` e `[PRIVACIDADE]`.
4. **Comando do terminal** na seção "Sua IA" (`applyer ia conectar …`): é ilustrativo. Troque pelo comando real quando ele existir.

## Testes

`npm test` na raiz do repositório roda `testes/site.test.mjs`: âncoras, textos ligados pelo script, recursos de fora e restos do protótipo.

## Segurança

A página não envia nada a servidor nenhum (`connect-src 'none'`) e só roda script do próprio site. As fontes vêm do Google Fonts.
