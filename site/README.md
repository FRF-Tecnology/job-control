# Landing do Applyer

A pasta `site/` é a raiz do site estático. Publique o conteúdo completo num servidor HTTPS, preservando os caminhos relativos. A página também funciona em um subdiretório. Não há etapa de build.

## O que está disponível

A demonstração usa vagas fictícias. A fila reflete as vagas selecionadas e só avança quando a pessoa interage. Os preços, as opções de plataforma, as respostas às perguntas e o e-mail de suporte ficam no HTML. O tema inicial é escuro. A página restaura o tema claro quando a pessoa o escolhe. O seletor de plataforma, o tema e os painéis de demonstração usam JavaScript local.

| Cobrança | Total do período | Desconto sobre o mensal |
| --- | --- | --- |
| Mensal | R$ 29,90 | — |
| Semestral | R$ 161,46 | 10% |
| Anual | R$ 287,04 | 20% |

## Publicação e ativação comercial

- Use `index.html` como entrada. Habilite compressão HTTP para HTML, CSS, JS e SVG. Como esses arquivos não têm nomes versionados, configure revalidação de cache ao publicar uma atualização.
- Após publicar, valide HTTPS no domínio final, os recursos na página, a navegação por âncoras e o endereço `suporte@applyer.dev` no serviço de e-mail.
- Os botões de download e de aviso estão desativados por decisão de lançamento. Quando houver instaladores oficiais, use links diretos para as plataformas correspondentes e atualize os rótulos em ambos os menus e cartões.
- A página informa os preços aprovados. A ativação de pagamento e de avisos depende de checkout e de cadastro reais. Vincule os documentos oficiais de termos e privacidade quando estiverem disponíveis.

## Verificação

Rode `node --test testes/site.test.mjs` ou `npm test` na raiz do repositório. Para a revisão visual, sirva esta pasta com um servidor estático. Confira desktop e celular, os dois temas, teclado, movimento reduzido e conteúdo sem JavaScript.

## Arquivos

- `index.html` contém o conteúdo, os preços e as perguntas em HTML nativo.
- `estilo.css` define o layout responsivo e os temas.
- `tema.js` lê a preferência local e sincroniza o tema. `app.js` controla a demonstração e o seletor de plataforma.
- `marcas/` e `artes/` contêm os SVGs locais. Os créditos das marcas estão nos arquivos correspondentes.

Os recursos da landing são locais. A política de conteúdo bloqueia conexões de dados externas. A demonstração não transmite o que é selecionado na página.
