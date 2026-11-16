package navegador

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Ler a descrição de uma vaga do Radar ("Ler a descrição da vaga"). A pessoa clica, e o app abre a página da vaga
// numa aba do envio (sem janela, com a sessão dela), como ela mesma abriria. SEM INTERAGIR: nada é clicado,
// digitado nem enviado. O destino é conferido (a página tem de parar na mesma vaga, na plataforma); o texto da página
// é dado não confiável: vem como texto (innerText), com teto, e não vai para o log.

const (
	tempoDescricao    = 60 * time.Second
	esperaDescricao   = 12 * time.Second // o LinkedIn carrega a descrição depois da página
	maxTextoDescricao = 20000
)

// scriptDescricao devolve o texto do primeiro bloco de descrição conhecido com algum corpo, ou "".
const scriptDescricao = `(() => {
  const seletores = ['#job-details', '.jobs-description__content', '.jobs-description-content__text',
    '.jobs-box__html-content', '.show-more-less-html__markup', '.description__text', '[class*="jobs-description"]'];
  for (const s of seletores) {
    for (const el of document.querySelectorAll(s)) {
      const t = (el.innerText || '').trim();
      if (t.length >= 80) return t.slice(0, ` + "20000" + `);
    }
  }
  return '';
})()`

// LerDescricao abre a vaga e devolve o texto da descrição. bloqueio é a categoria do destino quando a página não
// parou na vaga (dominio.EnderecoFinal: login, outro-host, outra-vaga, about:blank) ou "sem-descricao".
func (n *Navegador) LerDescricao(ctx context.Context, plataforma, endereco string) (texto, bloqueio string, err error) {
	ctx, cancelar := context.WithTimeout(ctx, tempoDescricao)
	defer cancelar()
	pg, err := n.NovaPagina(ctx)
	if err != nil {
		return "", "", err
	}
	defer pg.Fechar()
	p := pg.(*pagina)
	final, err := p.Abrir(ctx, endereco)
	if err != nil {
		return "", "", err
	}
	if b := dominio.EnderecoFinal(plataforma, endereco, final); b != "" {
		return "", b, nil
	}
	fim := time.Now().Add(esperaDescricao)
	for {
		t, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](scriptDescricao))
		if err != nil {
			return "", "", err
		}
		if t != "" {
			if r := []rune(t); len(r) > maxTextoDescricao {
				t = string(r[:maxTextoDescricao])
			}
			// A página pode ter redirecionado durante a espera: confere de novo.
			if agora, err := p.URLAtual(ctx); err != nil {
				return "", "", err
			} else if b := dominio.EnderecoFinal(plataforma, endereco, agora); b != "" {
				return "", b, nil
			}
			return t, "", nil
		}
		if time.Now().After(fim) {
			return "", "sem-descricao", nil
		}
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(700 * time.Millisecond):
		}
	}
}
