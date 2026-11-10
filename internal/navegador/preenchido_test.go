package navegador

import (
	"fmt"
	"net/http"
	"strings"
)

// Páginas fictícias do jc-a2y.6.5.1: candidatura simplificada em que a plataforma já traz campos preenchidos, como
// o LinkedIn faz (e-mail da conta, código do país, respostas de candidaturas anteriores). Empresa, vaga e e-mails
// inventados. Cada variante muda só o que a página traz pronto.
type variantePreenchida struct {
	id        string
	email     string // e-mail já marcado no select ("" = nenhum)
	ddi       bool   // código do país +55 já marcado
	tel       string // telefone já escrito pela página ("" = vazio)
	extra     string // HTML a mais dentro do formulário
	enviada   bool   // o que se espera do executor
	motivo    string // motivo esperado quando bloqueia
	descricao string
}

var variantesPreenchidas = []variantePreenchida{
	{id: "9000000005", email: "pessoa@exemplo.com", ddi: true, enviada: true, descricao: "e-mail e código do país iguais ao pacote: vai sem alterar"},
	{id: "9000000011", email: "pessoa@exemplo.com", ddi: false, enviada: true, descricao: "código do país não marcado: o app marca +55"},
	{id: "9000000006", email: "conta@exemplo.com", ddi: true, motivo: "pergunta-nova", descricao: "e-mail da conta diferente do pacote: bloqueia"},
	{id: "9000000007", email: "pessoa@exemplo.com", ddi: true, motivo: "pergunta-nova", descricao: "pergunta de triagem já marcada fora do pacote: bloqueia",
		extra: `<label for="mud">Disponível para mudança?</label><select id="mud" name="mud"><option value="">Selecione</option><option value="s" selected>Sim</option><option value="n">Não</option></select>`},
	{id: "9000000008", email: "pessoa@exemplo.com", ddi: true, motivo: "termos", descricao: "termos já marcados: bloqueia",
		extra: `<label><input type="checkbox" id="termos" name="termos" checked> Li e aceito os termos de uso</label>`},
	{id: "9000000009", email: "pessoa@exemplo.com", ddi: true, motivo: "pergunta-nova", descricao: "texto livre já preenchido: bloqueia",
		extra: `<label for="porque">Por que você quer trabalhar aqui?</label><textarea id="porque" name="porque">Texto de uma candidatura anterior</textarea>`},
	{id: "9000000012", email: "pessoa@exemplo.com", ddi: true, enviada: true, descricao: "Seguir a empresa já marcado: o app desmarca e envia sem ele",
		extra: `<label><input type="checkbox" id="seguir" name="seguir" checked> Seguir Empresa Teste</label>`},
	{id: "9000000013", email: "pessoa@exemplo.com", ddi: true, tel: "+55 (11) 90000-0000", enviada: true, descricao: "telefone já escrito igual ao do pacote, com o código do país: fica"},
	{id: "9000000014", email: "pessoa@exemplo.com", ddi: true, tel: "011900000000", motivo: "pergunta-nova", descricao: "telefone já escrito com dígito a mais: bloqueia"},
	{id: "9000000010", email: "pessoa@exemplo.com", ddi: true, motivo: "pergunta-nova", descricao: "currículo anterior já escolhido: bloqueia",
		extra: `<fieldset><legend>Escolha um currículo enviado antes</legend><label><input type="radio" name="cvantigo" value="cv-antigo.pdf" checked>cv-antigo.pdf</label><label><input type="radio" name="cvantigo" value="outro.pdf">outro.pdf</label></fieldset>`},
}

func paginaPreenchida(v variantePreenchida) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		opcao := func(valor, texto string, marcada bool) string {
			sel := ""
			if marcada {
				sel = " selected"
			}
			return fmt.Sprintf(`<option value="%s"%s>%s</option>`, valor, sel, texto)
		}
		email := opcao("", "Selecione", v.email == "") + opcao("pessoa@exemplo.com", "pessoa@exemplo.com", v.email == "pessoa@exemplo.com") +
			opcao("conta@exemplo.com", "conta@exemplo.com", v.email == "conta@exemplo.com")
		ddi := opcao("", "Selecione", !v.ddi) + opcao("BR", "Brasil (+55)", v.ddi) + opcao("PT", "Portugal (+351)", false) // valor por país, como no LinkedIn
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, strings.NewReplacer("{{ID}}", v.id, "{{TEL}}", v.tel, "{{EMAIL}}", email, "{{DDI}}", ddi, "{{EXTRA}}", v.extra).Replace(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>Dev Go - Empresa Teste</title></head>
<body>
  <h1>Dev Go</h1><p>Empresa Teste. Trabalho remoto.</p>
  <button type="button" onclick="document.getElementById('modal').style.display='block'; this.style.display='none'">Candidatura simplificada</button>
  <div id="modal" style="display:none">
    <form method="post" action="/jobs/view/{{ID}}/apply" enctype="multipart/form-data">
      <h2>Informações de contato</h2>
      <label for="nome">Nome completo</label><input id="nome" name="nome" required>
      <label for="email">E-mail</label><select id="email" name="email" required>{{EMAIL}}</select>
      <label for="ddi">Código do país</label><select id="ddi" name="ddi" required>{{DDI}}</select>
      <label for="tel">Telefone celular</label><input id="tel" name="tel" type="tel" value="{{TEL}}" required>
      <label for="cv">Currículo (PDF)</label><input id="cv" name="cv" type="file" accept=".pdf" required>
      {{EXTRA}}
      <button type="submit">Enviar candidatura</button>
    </form>
  </div>
</body></html>`))
	}
}
