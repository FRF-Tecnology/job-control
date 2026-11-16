package navegador

import (
	"context"
	"errors"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/executor"
)

// Preencher no site da empresa (docs/propostas/preencher-site-externo.md). Caminho SEPARADO do envio:
//   - só por ação da pessoa, numa janela VISÍVEL (a mesma trava da janela de login: nenhum envio abre enquanto ela
//     estiver aberta);
//   - o app abre a vaga e tenta clicar no "Candidatar-se" do LinkedIn, que abre o site da empresa; se não der, a
//     pessoa clica;
//   - o app só preenche depois que a pessoa confere o domínio, e o domínio é conferido de novo na hora de preencher;
//   - preenche o que reconhece (executor.PlanejarAssistido) e NUNCA avança nem envia: a pessoa revisa e envia;
//   - a cópia do currículo vive numa pasta própria, apagada quando a janela fecha (o site lê o arquivo no envio).

// Erros do preenchimento, para a API responder com motivo fixo.
var (
	ErrSemJanela   = errors.New("navegador: a janela de preenchimento não está aberta")
	ErrSemSite     = errors.New("navegador: nenhuma aba no site da empresa")
	ErrSiteMudou   = errors.New("navegador: a aba do site mudou de domínio")
	tempoPreencher = 90 * time.Second
)

// ResultadoPreenchimento: o tipo do domínio.
type ResultadoPreenchimento = dominio.ResultadoPreenchimento

// O botão "Candidatar-se" do LinkedIn que leva ao site da empresa (não o da Candidatura simplificada). O script só
// clica depois de devolver (setTimeout), para a navegação não cortar a resposta.
const scriptCandidatarFora = `(() => {
  // O site da empresa abre NESTA aba, não numa aba nova: a aba nova nasce fora do controle do app (pausada para o
  // DevTools). Vale também para o clique da própria pessoa.
  window.open = (u) => { if (u) location.assign(u); return window; };
  for (const a of document.querySelectorAll('a[target]')) a.removeAttribute('target');
  const norm = s => (s || '').normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().trim();
  for (const b of document.querySelectorAll('button.jobs-apply-button, a.jobs-apply-button, button[aria-label], a[aria-label]')) {
    const t = norm(b.innerText) + ' ' + norm(b.getAttribute('aria-label'));
    if (/candidatura simplificada|easy apply|solicitud sencilla/.test(t)) continue;
    if (/^(candidatar-se|candidate-se|apply|aplicar|postularse|postular)\b/.test(norm(b.innerText))) { setTimeout(() => b.click(), 0); return true; }
  }
  return false;
})()`

var hostLinkedInPreencher = regexp.MustCompile(`^(www\.|[a-z]{2}\.)?linkedin\.com$`)

// AbrirParaPreencher abre a vaga numa janela visível e tenta abrir o site da empresa. Como o AbrirLogin: fecha antes
// o navegador sem janela (sem página do envio aberta) e, até a pessoa fechar a janela, nenhum envio abre.
func (n *Navegador) AbrirParaPreencher(endereco string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.emLogin() {
		return ErrLoginAberto
	}
	if n.abertas > 0 {
		return ErrEmUso
	}
	n.fecharSemTrava()
	if err := n.iniciarSemTrava(true); err != nil {
		return err
	}
	navCtx := n.navCtx
	n.loginCtx, n.preencherCtx = navCtx, navCtx
	pasta, err := os.MkdirTemp("", "applyer-preencher-")
	if err != nil {
		n.fecharSemTrava()
		n.loginCtx, n.preencherCtx = nil, nil
		return err
	}
	n.pastaPreencher = pasta
	// Uma aba nova, como as do envio: a primeira aba do navegador perde a conexão do DevTools quando a página troca
	// de site (o processo do site muda), e o app não conseguiria mais ler nem preencher.
	aba, _ := chromedp.NewContext(navCtx) // vive o tempo da janela
	n.abaPreencher = aba
	go func() {
		if err := chromedp.Do(aba); err != nil {
			return
		}
		if c := chromedp.FromContext(aba); c != nil && c.Target != nil {
			_, _ = chromedp.CallBrowser(aba, target.ActivateTarget, target.ActivateTargetParams{TargetID: c.Target.TargetID})
		}
		ctx, cancelar := context.WithTimeout(aba, 2*time.Minute)
		defer cancelar()
		if _, err := chromedp.Run(ctx, chromedp.Navigate(endereco)); err != nil {
			return
		}
		// O botão aparece depois da página: tenta por alguns segundos; sem ele, a pessoa clica.
		for i := 0; i < 10 && ctx.Err() == nil; i++ {
			if achou, err := chromedp.Run(ctx, chromedp.Evaluate[bool](scriptCandidatarFora)); err == nil && achou {
				return
			}
			time.Sleep(time.Second)
		}
	}()
	go func() {
		<-navCtx.Done() // a pessoa fechou a janela, ou o app fechou o navegador
		n.mu.Lock()
		if n.navCtx == navCtx {
			n.fecharSemTrava()
		}
		if n.loginCtx == navCtx {
			n.loginCtx = nil
		}
		if n.preencherCtx == navCtx {
			n.preencherCtx, n.abaPreencher = nil, nil
		}
		if n.pastaPreencher == pasta {
			n.pastaPreencher = ""
		}
		n.mu.Unlock()
		_ = os.RemoveAll(pasta) // a cópia do currículo some com a janela
	}()
	return nil
}

// abaDoSite acha a aba da janela de preenchimento que está fora do LinkedIn (a última, se houver mais de uma).
func (n *Navegador) abaDoSite(ctx context.Context) (navCtx context.Context, id target.ID, host string, err error) {
	// A aba principal (a da vaga, onde o site da empresa abre) vem primeiro; outra aba fora do LinkedIn, depois.
	n.mu.Lock()
	navCtx, principal := n.preencherCtx, n.abaPreencher
	n.mu.Unlock()
	if navCtx == nil || navCtx.Err() != nil {
		return nil, "", "", ErrSemJanela
	}
	c, cancelar := context.WithTimeout(navCtx, tempoAcao)
	defer cancelar()
	parar := context.AfterFunc(ctx, cancelar)
	defer parar()
	alvos, err := chromedp.Targets(c)
	if err != nil {
		return nil, "", "", err
	}
	for _, a := range alvos {
		if a.Type != "page" {
			continue
		}
		u, err := url.Parse(a.URL)
		if err != nil || u.Scheme != "https" || u.User != nil {
			continue
		}
		h := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		if h == "" || hostLinkedInPreencher.MatchString(h) {
			continue
		}
		id, host = a.TargetID, h
		if c := chromedp.FromContext(principal); c != nil && c.Target != nil && c.Target.TargetID == a.TargetID {
			break // a aba da vaga vence
		}
	}
	if id == "" {
		return nil, "", "", ErrSemSite
	}
	return navCtx, id, host, nil
}

// SiteParaPreencher devolve o domínio da aba do site da empresa, para a pessoa conferir antes de preencher.
func (n *Navegador) SiteParaPreencher(ctx context.Context) (string, error) {
	_, _, host, err := n.abaDoSite(ctx)
	return host, err
}

// PreencherSite preenche a aba do site, se ela ainda está no domínio que a pessoa conferiu. curriculo: o PDF já
// conferido (nome e bytes); a cópia fica na pasta da janela.
func (n *Navegador) PreencherSite(ctx context.Context, dominioConferido string, conteudo, perfil dominio.Objeto, nomeCurriculo string, curriculo []byte) (ResultadoPreenchimento, error) {
	ctx, cancelar := context.WithTimeout(ctx, tempoPreencher)
	defer cancelar()
	navCtx, id, host, err := n.abaDoSite(ctx)
	if err != nil {
		return ResultadoPreenchimento{}, err
	}
	if host != dominioConferido {
		return ResultadoPreenchimento{}, ErrSiteMudou
	}
	// A aba da vaga já está conectada; outra aba (que a pessoa abriu) é conectada sem ser fechada depois.
	n.mu.Lock()
	aba := n.abaPreencher
	n.mu.Unlock()
	if c := chromedp.FromContext(aba); aba == nil || c == nil || c.Target == nil || c.Target.TargetID != id {
		var fechar context.CancelFunc
		aba, fechar = chromedp.NewContext(navCtx, chromedp.WithTargetID(id), chromedp.WithDetachOnCancel())
		defer fechar() // solta a aba sem fechar: a pessoa continua nela
	}
	p := &pagina{aba: aba, fechar: func() {}}
	// A página pode ter mudado entre achar e conectar: confere o domínio da própria aba.
	atual, err := p.URLAtual(ctx)
	if err != nil {
		return ResultadoPreenchimento{}, err
	}
	if u, err := url.Parse(atual); err != nil || strings.TrimSuffix(strings.ToLower(u.Hostname()), ".") != dominioConferido {
		return ResultadoPreenchimento{}, ErrSiteMudou
	}
	_, _ = chromedp.CallBrowser(aba, target.ActivateTarget, target.ActivateTargetParams{TargetID: id}) // só visual

	campos, err := p.Campos(ctx)
	if err != nil {
		return ResultadoPreenchimento{}, err
	}
	copia := ""
	if nomeCurriculo != "" && len(curriculo) > 0 {
		n.mu.Lock()
		pasta := n.pastaPreencher
		n.mu.Unlock()
		if pasta != "" {
			copia = filepath.Join(pasta, filepath.Base(nomeCurriculo))
			if err := os.WriteFile(copia, curriculo, 0o600); err != nil {
				copia = ""
			}
		}
	}
	// Site brasileiro: o telefone vai no formato nacional (sem o "+55"), como os formulários daqui pedem.
	conteudo = maps.Clone(conteudo)
	conteudo["siteNacional"] = siteNacional(host)
	plano, faltando := executor.PlanejarAssistido(conteudo, perfil, campos, copia != "")
	var r ResultadoPreenchimento
	r.Faltando = faltando
	for _, a := range plano {
		var err error
		if a.Anexar {
			err = p.Anexar(ctx, a.Campo, copia)
		} else {
			err = p.Preencher(ctx, a.Campo, a.Valor)
			for _, alt := range a.Alternativas { // a lista do site não tem o primeiro jeito: tenta os outros
				if !errors.Is(err, executor.ErrSemOpcao) {
					break
				}
				err = p.Preencher(ctx, a.Campo, alt)
			}
		}
		switch {
		case err == nil && a.Anexar:
			r.Anexou = true
		case err == nil:
			r.Preenchidos++
		case ctx.Err() != nil:
			return r, ctx.Err()
		default:
			r.Faltando++ // opção que não existe no campo, campo que sumiu: a pessoa preenche
		}
	}
	return r, nil
}

// siteNacional: o domínio é brasileiro (.br) ou de um sistema de vagas brasileiro.
func siteNacional(host string) bool {
	return host == "br" || strings.HasSuffix(host, ".br") || host == "gupy.io" || strings.HasSuffix(host, ".gupy.io")
}
