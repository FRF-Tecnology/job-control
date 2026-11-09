package dominio

import "testing"

// Endereços fictícios. A regra só aperta o URLDaPlataforma: host da lista fechada e o id da vaga aprovado.
func TestMesmaVaga(t *testing.T) {
	const (
		li = "https://www.linkedin.com/jobs/view/9000000001"
		gp = "https://empresa-teste.gupy.io/jobs/9000001"
		ih = "https://empresa-teste.inhire.app/vagas/9000002/dev-go"
	)
	casos := []struct {
		plataforma, aprovada, atual string
		ok                          bool
	}{
		// LinkedIn.
		{"linkedin", li, li, true},
		{"linkedin", li, li + "/apply?origem=teste", true},
		{"linkedin", li, "https://br.linkedin.com/jobs/view/9000000001/", true},
		{"linkedin", li, "https://linkedin.com/jobs/view/9000000001", true},
		{"linkedin", li, "https://WWW.LinkedIn.com./jobs/view/9000000001", true},
		{"linkedin", li, "https://www.linkedin.com/jobs/view/9000000002", false},
		{"linkedin", li, "https://www.linkedin.com/jobs/view/90000000011", false},
		{"linkedin", li, "https://www.linkedin.com/feed/", false},
		{"linkedin", li, "https://evil-linkedin.com/jobs/view/9000000001", false},
		{"linkedin", li, "https://linkedin.com.evil.io/jobs/view/9000000001", false},
		{"linkedin", li, "https://linkedin.com@evil.io/jobs/view/9000000001", false},
		{"linkedin", li, "https://ads.mkt.linkedin.com/jobs/view/9000000001", false},
		{"linkedin", li, "https://xn--linkedn-v2a.com/jobs/view/9000000001", false},
		{"linkedin", li, "http://www.linkedin.com/jobs/view/9000000001", false},
		{"linkedin", li, "https://www.linkedin.com:8443/jobs/view/9000000001", false},
		{"linkedin", "https://www.linkedin.com/feed/", li, false}, // aprovado sem id de vaga
		// currentJobId= numa página de /jobs/, igual ao aprovado (pedido do PO).
		{"linkedin", li, "https://www.linkedin.com/jobs/search/?currentJobId=9000000001&keywords=go", true},
		{"linkedin", li, "https://www.linkedin.com/jobs/collections/recommended/?currentJobId=9000000001", true},
		{"linkedin", "https://www.linkedin.com/jobs/search/?currentJobId=9000000001", li + "/apply", true},
		{"linkedin", li, "https://www.linkedin.com/jobs/search/?currentJobId=9000000002", false},
		{"linkedin", li, "https://www.linkedin.com/jobs/search/?currentJobId=90000000011", false},
		{"linkedin", li, "https://www.linkedin.com/jobs/search/?currentJobId=9000000001x", false},
		{"linkedin", li, "https://www.linkedin.com/feed/?currentJobId=9000000001", false},
		{"linkedin", li, "https://www.linkedin.com/jobs/view/9000000001?currentJobId=9000000002", false},
		{"linkedin", li, "https://www.linkedin.com/jobs/search/?currentJobId=9000000001&currentJobId=9000000002", false},
		{"linkedin", li, "https://evil.com/jobs/search/?currentJobId=9000000001", false},
		// Gupy: a mesma empresa e a mesma vaga.
		{"gupy", gp, gp + "/apply", true},
		{"gupy", gp, "https://empresa-teste.gupy.io/jobs/9000001?jobBoardSource=teste", true},
		{"gupy", gp, "https://outra-empresa.gupy.io/jobs/9000001", false},
		{"gupy", gp, "https://empresa-teste.gupy.io/jobs/9000099", false},
		{"gupy", gp, "https://portal.gupy.io/jobs/9000001", false},
		{"gupy", gp, "https://empresa-teste.gupy.io.evil.com/jobs/9000001", false},
		{"gupy", gp, "https://empresa-testegupy.io/jobs/9000001", false},
		{"gupy", "https://portal.gupy.io/jobs/9000001", "https://portal.gupy.io/jobs/9000001", false},
		// InHire: a mesma empresa e a mesma vaga, ou o form-app com o id aprovado inteiro no endereço.
		{"inhire", ih, ih + "/questionario", true},
		{"inhire", ih, "https://empresa-teste.inhire.app/vagas/9000002", true},
		{"inhire", ih, "https://form-app.inhire.app/?jobId=9000002&flow=returnMessageToParent", true},
		{"inhire", ih, "https://form-app.inhire.app/9000002/perguntas", true},
		{"inhire", ih, "https://form-app.inhire.app/?jobId=9000003", false},
		{"inhire", ih, "https://form-app.inhire.app/?jobId=90000021", false},
		{"inhire", ih, "https://form-app.inhire.app/?jobId=x9000002", false},
		{"inhire", ih, "https://form-app.inhire.app/", false},
		{"inhire", ih, "https://form-app.inhire.app/90000021/perguntas", false},
		{"inhire", ih, "https://form-app.inhire.app/x9000002", false},
		{"inhire", ih, "https://outra-empresa.inhire.app/vagas/9000002/dev-go", false},
		{"inhire", ih, "https://empresa-teste.inhire.app/vagas/9000003/dev-go", false},
		{"inhire", ih, "https://inhire.app.evil.com/vagas/9000002", false},
		{"inhire", ih, "https://evilinhire.app/vagas/9000002", false},
		{"inhire", ih, "https://form-app.inhire.app.evil.com/?jobId=9000002", false},
		{"inhire", ih, "https://xn--nhire-9ua.app/vagas/9000002", false},
		{"inhire", ih, "https://form-app.inhire.app@evil.com/?jobId=9000002", false},
		{"inhire", ih, "http://form-app.inhire.app/?jobId=9000002", false},
		// Outra plataforma ou desconhecida: nunca.
		{"outra", "https://carreiras.exemplo.com/vaga/1", "https://carreiras.exemplo.com/vaga/1", false},
		{"gupy", li, li, false},
	}
	for _, c := range casos {
		if got := MesmaVaga(c.plataforma, c.aprovada, c.atual); got != c.ok {
			t.Errorf("MesmaVaga(%s, %s, %s) = %v; queria %v", c.plataforma, c.aprovada, c.atual, got, c.ok)
		}
	}
}
