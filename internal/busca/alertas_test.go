package busca

import (
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Alertas fictícios, no formato dos e-mails do LinkedIn e do Indeed (cartão com imagem, título e "empresa · local").

const alertaLinkedInHTML = `<html><head><title>Vagas</title><style>.x{color:red}</style></head><body>
<p>Fulana, 3 vagas novas para "Desenvolvedora iOS"</p>
<table><tr><td>
  <a href="https://www.linkedin.com/comm/jobs/view/4012345678/?trackingId=abc%3D%3D&amp;refId=xyz"><img src="https://media.licdn.com/logo.png" alt="Acme"></a>
</td><td>
  <a href="https://www.linkedin.com/comm/jobs/view/4012345678/?trackingId=abc%3D%3D&amp;refId=xyz">Engenheira iOS Sênior</a>
  <p>Acme Ltda · São Paulo, Brasil (Remoto)</p>
  <p>Candidatura simplificada</p>
  <a href="https://www.linkedin.com/comm/jobs/view/4012345678/">Ver vaga</a>
</td></tr>
<tr><td>
  <a href="https://br.linkedin.com/jobs/view/engenheira-mobile-at-beta-4098765432?refId=1">Engenheira Mobile &amp; iOS</a>
  <p>Beta S.A.</p><p>Belo Horizonte, MG</p>
</td></tr>
<tr><td>
  <a href="https://www.linkedin.com.golpe.example/jobs/view/4011111111/">Vaga falsa</a><p>Golpe</p>
</td></tr>
<tr><td>
  <a href="https://www.linkedin.com/jobs/view/4022222222/">Sem empresa</a>
</td></tr></table>
<a href="https://www.linkedin.com/comm/jobs/search">Ver todas as vagas</a>
</body></html>`

const alertaIndeedHTML = `<div>
<a href="https://cts.indeed.com/v3/H4sI?u=https%3A%2F%2Fbr.indeed.com%2Frc%2Fclk%2Fdl%3Fjk%3D1a2b3c4d5e6f7a8b%26from%3Dja"><b>Analista de QA</b></a><br>
Gama Tecnologia<br>
Curitiba, PR<br>
<a href="https://br.indeed.com/viewjob?jk=9f8e7d6c5b4a3f2e&amp;from=ja">Pessoa Desenvolvedora Go &lt;b&gt;</a><br>
Delta · Remoto
<a href="https://br.indeed.com/viewjob?jk=9f8e7d6c5b4a3f2e">https://br.indeed.com/viewjob?jk=9f8e7d6c5b4a3f2e</a>
</div>`

const alertaLinkedInTexto = `Seu alerta de vaga para desenvolvedora ios

Engenheira iOS Pleno
Épsilon Digital
Recife, PE
Ver vaga: https://www.linkedin.com/comm/jobs/view/4033333333/?trackingId=q

---------------------------------------------------------

Tech Lead Mobile
Zeta
Ver vaga: https://www.linkedin.com/comm/jobs/view/4044444444/

---------------------------------------------------------
Ver todas as vagas: https://www.linkedin.com/comm/jobs/search`

func TestFonteDoAlerta(t *testing.T) {
	for de, quer := range map[string]string{
		"jobalerts-noreply@linkedin.com":               "linkedin",
		"JOBS-LISTINGS@linkedin.com":                   "linkedin",
		"jobs-noreply@linkedin.com":                    "", // confirmação de candidatura: é resposta
		"messages-noreply@linkedin.com":                "",
		"jobalerts-noreply@linkedin.com.golpe.example": "",
		"jobalerts-noreply@golpe-linkedin.com":         "",
		"alert@indeed.com":                             "indeed",
		"jobalerts@indeed.com":                         "indeed",
		"donotreply@jobalert.indeed.com":               "indeed",
		"noreply@match.indeed.com":                     "indeed",
		"indeedapply@indeed.com":                       "", // confirmação de candidatura
		"alert@indeed.com.golpe.example":               "",
		"fulana@example.com":                           "",
		"":                                             "",
	} {
		if got := FonteDoAlerta(de); got != quer {
			t.Errorf("FonteDoAlerta(%q) = %q, quer %q", de, got, quer)
		}
	}
}

func conferirVaga(t *testing.T, v dominio.Objeto, id, titulo, empresa, local, link string) {
	t.Helper()
	for campo, quer := range map[string]string{"id": id, "titulo": titulo, "empresa": empresa, "local": local, "link": link, "fonteURL": link} {
		if got := dominio.Texto(v[campo]); got != quer {
			t.Errorf("%s: %s = %q, quer %q", id, campo, got, quer)
		}
	}
}

func TestVagasDoAlertaLinkedInHTML(t *testing.T) {
	vagas := VagasDoAlerta("linkedin", "", alertaLinkedInHTML)
	if len(vagas) != 2 {
		t.Fatalf("quero 2 vagas (a do host falso e a sem empresa ficam de fora), veio %d: %v", len(vagas), vagas)
	}
	conferirVaga(t, vagas[0], "linkedin-4012345678", "Engenheira iOS Sênior", "Acme Ltda", "São Paulo, Brasil (Remoto)", "https://www.linkedin.com/jobs/view/4012345678/")
	conferirVaga(t, vagas[1], "linkedin-4098765432", "Engenheira Mobile & iOS", "Beta S.A.", "Belo Horizonte, MG", "https://www.linkedin.com/jobs/view/4098765432/")
	if vagas[0]["plataforma"] != "linkedin" || vagas[0]["fonteBusca"] != "Alerta do LinkedIn (e-mail)" {
		t.Errorf("plataforma/fonte: %v", vagas[0])
	}
}

func TestVagasDoAlertaIndeedHTML(t *testing.T) {
	vagas := VagasDoAlerta("indeed", "", alertaIndeedHTML)
	if len(vagas) != 2 {
		t.Fatalf("quero 2 vagas, veio %d: %v", len(vagas), vagas)
	}
	// o link de rastreio vira o endereço da vaga, no host do país
	conferirVaga(t, vagas[0], "indeed-1a2b3c4d5e6f7a8b", "Analista de QA", "Gama Tecnologia", "Curitiba, PR", "https://www.indeed.com/viewjob?jk=1a2b3c4d5e6f7a8b")
	conferirVaga(t, vagas[1], "indeed-9f8e7d6c5b4a3f2e", "Pessoa Desenvolvedora Go", "Delta", "Remoto", "https://br.indeed.com/viewjob?jk=9f8e7d6c5b4a3f2e")
	if vagas[0]["plataforma"] != "outra" {
		t.Errorf("Indeed não tem envio automático: plataforma %v", vagas[0]["plataforma"])
	}
}

func TestVagasDoAlertaTextoSimples(t *testing.T) {
	// Sem vaga no HTML (ou sem HTML), o texto simples.
	vagas := VagasDoAlerta("linkedin", alertaLinkedInTexto, "<p>sem links</p>")
	if len(vagas) != 2 {
		t.Fatalf("quero 2 vagas, veio %d: %v", len(vagas), vagas)
	}
	conferirVaga(t, vagas[0], "linkedin-4033333333", "Engenheira iOS Pleno", "Épsilon Digital", "Recife, PE", "https://www.linkedin.com/jobs/view/4033333333/")
	conferirVaga(t, vagas[1], "linkedin-4044444444", "Tech Lead Mobile", "Zeta", "", "https://www.linkedin.com/jobs/view/4044444444/")
}

func TestVagasDoAlertaNaoGuardaTextoPerigoso(t *testing.T) {
	h := `<a href="https://www.linkedin.com/jobs/view/4055555555/">Vaga boa https://golpe.example/entrar</a><p>Empresa</p>
<a href="https://www.linkedin.com/jobs/view/4066666666/">Vaga com contato</a><p>fale com rh@golpe.example</p><p>Empresa Real</p>
<a href="https://www.linkedin.com/jobs/view/4077777777/">` + strings.Repeat("x", 300) + `</a><p>Empresa</p>`
	vagas := VagasDoAlerta("linkedin", "", h)
	if len(vagas) != 1 {
		t.Fatalf("quero só a vaga com contato (com a empresa da linha seguinte), veio %v", vagas)
	}
	conferirVaga(t, vagas[0], "linkedin-4066666666", "Vaga com contato", "Empresa Real", "", "https://www.linkedin.com/jobs/view/4066666666/")
	if VagasDoAlerta("gupy", alertaLinkedInTexto, alertaLinkedInHTML) != nil {
		t.Error("fonte desconhecida não tira vaga")
	}
}

func TestVagasDoAlertaTeto(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		b.WriteString(`<a href="https://www.linkedin.com/jobs/view/40` + strings.Repeat("0", 6) + string(rune('0'+i/10)) + string(rune('0'+i%10)) + `/">Vaga</a><p>Empresa</p>`)
	}
	if n := len(VagasDoAlerta("linkedin", "", b.String())); n != maxVagasPorAlerta {
		t.Errorf("teto: %d vagas, quer %d", n, maxVagasPorAlerta)
	}
}
