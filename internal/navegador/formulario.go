package navegador

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Leitura de formulários externos (jc-a2y.7.2, parte 3a, docs/propostas/formularios.md). Abre o formulário numa aba
// do envio (sem janela, como o NovaPagina), confere o destino na lista exata e lê só o que a página já carregou, SEM
// INTERAGIR: nada é clicado, digitado, marcado nem enviado. O que vem da página é dado não confiável: lido com
// JSON.stringify, com teto de tamanho, e validado no domínio; nada é executado. Nada da página vai para o log.
//
// Google Forms: pelo FB_PUBLIC_LOAD_DATA_, no formato PÚBLICO, não conferido contra um formulário real (decisão do
// PO). Typeform: bloqueada/outro ("envie à mão") até termos uma amostra real do formato da página.

const tempoLeitura = 60 * time.Second

// sinais da página, no texto já sem acento e em minúsculas.
var (
	textoLogin           = regexp.MustCompile(`fazer login|faca login|sign in to continue|you need permission|precisa de permissao|iniciar sesion|inicia sesion`)
	textoEmailRegistrado = regexp.MustCompile(`(seu e-?mail|o e-?mail da sua conta) sera registrad|will be recorded|se registrara (tu|el) correo`)
	hostsDoProprioGoogle = map[string]bool{"docs.google.com": true, "www.google.com": true, "accounts.google.com": true, "www.gstatic.com": true}
	hostTypeform         = regexp.MustCompile(`(^|\.)typeform\.com$`)
	maxRetratoFormulario = dominio.MaxEstruturaBruta + 100_000
)

// LerFormulario lê a estrutura do formulário do endereço (a URL canônica da candidatura). O navegador é o do envio;
// se a janela de login da pessoa está aberta, recusa como o NovaPagina.
func (n *Navegador) LerFormulario(ctx context.Context, endereco string) (dominio.LeituraFormulario, error) {
	ctx, cancelar := context.WithTimeout(ctx, tempoLeitura)
	defer cancelar()
	pg, err := n.NovaPagina(ctx)
	if err != nil {
		return dominio.LeituraFormulario{}, err
	}
	defer pg.Fechar()
	p := pg.(*pagina)
	final, err := p.Abrir(ctx, endereco)
	if err != nil {
		return dominio.LeituraFormulario{}, err
	}
	if b := bloqueioDoDestino(final); b != "" {
		log.Printf("navegador: leitura do formulário parou no destino (%s)", b) // só o código
		return dominio.LeituraFormulario{Bloqueio: b}, nil
	}
	canonica, _, _ := dominio.FormularioDoEndereco(final)
	if u, err := url.Parse(canonica); err == nil && hostTypeform.MatchString(u.Hostname()) {
		return dominio.LeituraFormulario{Bloqueio: "outro"}, nil // sem amostra real do formato: a pessoa envia
	}
	bruto, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](scriptFormulario))
	if err != nil {
		return dominio.LeituraFormulario{}, err
	}
	return leituraDoRetrato(bruto, canonica), nil
}

// bloqueioDoDestino: onde a página parou depois de abrir (o forms.gle redireciona). Fora da lista, site-externo; o
// login do Google, login.
func bloqueioDoDestino(final string) string {
	u, err := url.Parse(final)
	if err != nil {
		return "site-externo"
	}
	if strings.ToLower(u.Hostname()) == "accounts.google.com" {
		return "login"
	}
	canonica, _, ok := dominio.FormularioDoEndereco(final)
	if !ok || strings.HasPrefix(canonica, "https://forms.gle/") {
		return "site-externo"
	}
	return ""
}

// retratoFormulario: o que o script devolve. Só lê.
type retratoFormulario struct {
	Google  *string  `json:"google"`
	Texto   string   `json:"texto"`
	Iframes []string `json:"iframes"`
	Arquivo bool     `json:"arquivo"`
}

// leituraDoRetrato decide, só com o retrato da página: bloqueios primeiro, depois a estrutura.
func leituraDoRetrato(bruto, canonica string) dominio.LeituraFormulario {
	if len(bruto) > maxRetratoFormulario {
		return dominio.LeituraFormulario{Bloqueio: "outro"}
	}
	var r retratoFormulario
	if err := json.Unmarshal([]byte(bruto), &r); err != nil {
		return dominio.LeituraFormulario{Bloqueio: "outro"}
	}
	texto := semAcento(strings.ToLower(r.Texto))
	switch {
	case textoEmailRegistrado.MatchString(texto), textoLogin.MatchString(texto):
		return dominio.LeituraFormulario{Bloqueio: "login"} // nunca enviamos a identidade da conta Google
	case r.Arquivo:
		return dominio.LeituraFormulario{Bloqueio: "outro"} // upload: a pessoa envia
	}
	for _, h := range r.Iframes {
		if h != "" && !hostsDoProprioGoogle[strings.ToLower(h)] {
			return dominio.LeituraFormulario{Bloqueio: "site-externo"} // iframe ou embed de outro domínio
		}
	}
	if r.Google == nil {
		return dominio.LeituraFormulario{Bloqueio: "outro"} // a estrutura não dá para ler sem interagir
	}
	perguntas, bloqueio, err := dominio.EstruturaDoGoogle(*r.Google)
	switch {
	case err != nil:
		return dominio.LeituraFormulario{Bloqueio: "outro"}
	case bloqueio != "":
		return dominio.LeituraFormulario{Bloqueio: bloqueio}
	}
	return dominio.LeituraFormulario{Perguntas: perguntas, URLFinal: canonica}
}

// semAcento: o mínimo para os padrões acima (á, à, ã, â, é, ê, í, ó, õ, ô, ú, ç).
func semAcento(s string) string {
	return strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "õ", "o", "ô", "o", "ú", "u", "ç", "c", "ñ", "n").Replace(s)
}

// scriptFormulario só lê: o FB_PUBLIC_LOAD_DATA_ como JSON (teto de tamanho), o texto visível (teto), os hosts dos
// iframes e se há campo de arquivo. Nada é clicado, digitado nem executado do que veio da página.
const scriptFormulario = `(() => {
  let google = null;
  try {
    const d = window.FB_PUBLIC_LOAD_DATA_;
    if (d !== undefined && d !== null) {
      const j = JSON.stringify(d);
      if (typeof j === "string" && j.length <= 2000000) google = j;
    }
  } catch (e) { google = null; }
  const texto = (document.body ? document.body.innerText : "").slice(0, 20000);
  const iframes = Array.from(document.querySelectorAll("iframe")).slice(0, 50).map(f => {
    try { return new URL(f.getAttribute("src") || "", location.href).hostname; } catch (e) { return ""; }
  });
  const arquivo = !!document.querySelector('input[type="file"]');
  return JSON.stringify({google, texto, iframes, arquivo});
})()`
