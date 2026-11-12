package triagem

import (
	"context"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

func TestAvaliarRadarEnviaOAlvoSemPendenciaPorSalarioAusente(t *testing.T) {
	p, v := perfilJavaRadar(), anuncioRadar()
	p["titulosEquivalentes"] = []string{"Desenvolvedor de serviços"}
	v["paisesElegiveis"] = []string{"BR"}
	provedor := &provedorTeste{resposta: `{"aderencia":"A","decisao":"aplicar","motivo":"Atribuições de desenvolvimento em Java e Spring Boot."}`}
	r, err := AvaliarRadar(context.Background(), configurar(provedor), p, v)
	if err != nil || r.Aderencia != "A" || len(r.Conferir) > 0 || len(provedor.chamadas) != 1 {
		t.Fatal(r, err)
	}
	msg := provedor.chamadas[0].Prompt.Mensagem
	for _, s := range []string{"Desenvolvedor de serviços", "paisTrabalho", "paisesElegiveis", "Java"} {
		if !strings.Contains(msg, s) {
			t.Errorf("faltou %q na comparação pela IA", s)
		}
	}
}

var diaDoRadar = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

func perfilJavaRadar() dominio.Objeto {
	return dominio.Objeto{"cargo": "Engenheiro Java", "stackPrincipal": []any{"Java", "Spring Boot"}, "paisTrabalho": "Brasil",
		"cidadeTrabalho": "Vitória", "faixaSalarial": dominio.Objeto{"moeda": "USD", "min": 4000},
		"filtros": dominio.Objeto{"niveis": []any{"Sênior"}, "modelo": []any{"Remoto"}}}
}

func anuncioRadar() dominio.Objeto {
	return dominio.Objeto{"titulo": "Senior Backend Engineer", "empresa": "Loja Exemplo", "descricao": "Build backend APIs in Java and Spring Boot.",
		"modelo": "Remoto", "local": "Remote worldwide", "publicadaEm": "2026-11-10", "mercado": "internacional"}
}

func TestRadarFuncaoDoPerfilAtivo(t *testing.T) {
	java := perfilJavaRadar()
	ml := maps.Clone(java)
	ml["cargo"], ml["stackPrincipal"] = "Engenheira de IA", []string{"Python", "Machine Learning", "LLM"}
	marketing := maps.Clone(java)
	marketing["cargo"], marketing["stackPrincipal"] = "Analista de Marketing", []string{"SEO"}
	design := maps.Clone(java)
	design["cargo"], design["stackPrincipal"] = "Product Designer", []string{"Figma"}
	for _, tt := range []struct {
		perfil                               dominio.Objeto
		titulo, descricao, classe, evidencia string
	}{
		{java, "Senior Backend Engineer", "Build backend APIs in Java and Spring Boot.", "forte", "Java"},
		{java, "Desenvolvedora Java Sênior", "Desenvolver microsserviços em Java com Spring.", "forte", "Java"},
		{java, "Senior Machine Learning Engineer", "Train models using Python and PyTorch.", "excluir", ""},
		{ml, "Senior Machine Learning Engineer", "Train and deploy models using Python and PyTorch.", "forte", "Python"},
		{ml, "Senior Backend Engineer", "Build backend APIs in Java and Spring Boot.", "excluir", ""},
		{java, "Senior Customer Support Engineer", "Support customers of our Java and Spring Boot analytics product.", "excluir", ""},
		{ml, "Especialista de Atendimento Sênior", "Atender clientes que usam nossa plataforma de inteligência artificial.", "excluir", ""},
		{java, "Senior Frontend JavaScript Engineer", "Build frontend features using JavaScript and React.", "excluir", ""},
		{java, "Senior Backend Engineer", "Our product is written in Java. We are looking for motivated people.", "conferir", ""},
		{java, "Senior Backend Engineer", "Build APIs for our customers.", "conferir", ""},
		{marketing, "Senior Marketing Analyst", "Analyze marketing campaigns and optimize SEO.", "forte", "SEO"},
		{design, "Senior Product Designer", "Design prototypes and test interfaces in Figma.", "forte", "Figma"},
	} {
		v := anuncioRadar()
		v["titulo"], v["descricao"], v["categorias"] = tt.titulo, tt.descricao, []any{"Engineering", "Java", "Machine Learning"}
		classe, motivo := QualificarRadar(tt.perfil, v, diaDoRadar)
		if classe != tt.classe || motivo == "" || !strings.Contains(motivo, tt.evidencia) {
			t.Errorf("%s × %s: %s (%s), esperado %s e evidência %q", tt.perfil["cargo"], tt.titulo, classe, motivo, tt.classe, tt.evidencia)
		}
	}
}

func TestRadarPublicacaoExpiracaoEExclusoes(t *testing.T) {
	for _, tt := range []struct {
		nome, classe string
		campos       dominio.Objeto
	}{
		{"30 dias atrás", "forte", dominio.Objeto{"publicadaEm": "2026-10-13"}},
		{"31 dias atrás", "excluir", dominio.Objeto{"publicadaEm": "2026-10-12", "coletadaEm": "2026-11-12"}},
		{"sem publicação", "conferir", dominio.Objeto{"publicadaEm": "", "criadaEm": "2026-11-12"}},
		{"publicação futura", "conferir", dominio.Objeto{"publicadaEm": "2026-12-12"}},
		{"encerrada", "excluir", dominio.Objeto{"etapa": "encerrada", "publicadaEm": ""}},
		{"fim agora", "excluir", dominio.Objeto{"expiraEm": "2026-11-12T12:00:00Z", "publicadaEm": ""}},
		{"recrutadora intermediária", "excluir", dominio.Objeto{"empresa": "Exemplo Staffing Agency"}},
		{"intermediação explícita", "excluir", dominio.Objeto{"descricao": "Estamos recrutando para nosso cliente. Desenvolver APIs com Java e Spring Boot."}},
		{"serviço de RH para clientes não é intermediação", "forte", dominio.Objeto{"empresa": "Exemplo Tecnologia RH", "descricao": "Build our recruiting software in Java and Spring Boot."}},
		{"nível divergente", "excluir", dominio.Objeto{"nivel": "Pleno"}},
		{"nível a esclarecer", "conferir", dominio.Objeto{"titulo": "Backend Engineer"}},
		{"salário não publicado", "forte", dominio.Objeto{}},
		{"salário anunciado em outra moeda", "forte", dominio.Objeto{"moeda": "EUR", "periodoSalario": "annual", "salarioMin": 60000}},
		{"regra explícita", "excluir", dominio.Objeto{"descricao": "Desenvolver APIs com Java e Spring Boot. A função exige plantão obrigatório."}},
	} {
		t.Run(tt.nome, func(t *testing.T) {
			p, v := perfilJavaRadar(), anuncioRadar()
			dominio.ObjetoDe(p["filtros"])["pularSe"] = []string{"plantão obrigatório"}
			maps.Copy(v, tt.campos)
			if classe, motivo := QualificarRadar(p, v, diaDoRadar); classe != tt.classe || motivo == "" {
				t.Errorf("%s (%s), esperado %s", classe, motivo, tt.classe)
			}
		})
	}
}

func TestRadarAceitaCargosEDescricoesDeQualquerArea(t *testing.T) {
	for _, tt := range []struct {
		cargo, equivalente, habilidade, titulo, descricao, classe string
	}{
		{"Atendimento ao cliente", "Customer Support Specialist", "Zendesk", "Customer Support Specialist", "Resolve customer tickets using Zendesk.", "forte"},
		{"Enfermeira", "Registered Nurse", "curativos", "Enfermeira", "Realizar curativos e aferir sinais vitais.", "forte"},
		{"Analista de RH", "People Operations Specialist", "admissão", "People Operations Specialist", "Conduzir processos de admissão de funcionários.", "forte"},
		{"Luthier", "Restauradora de cordas", "archetaria", "Restauradora de cordas", "Restaurar instrumentos com técnicas de archetaria.", "forte"},
		{"Marisqueira", "", "", "Marisqueira", "Coletar mariscos nos bancos autorizados.", "forte"},
		{"Gem Cutter", "", "lapidary tools", "Gem Cutter", "Facet gemstones using lapidary tools.", "forte"},
		{"Analista de RH", "", "Excel", "Analista financeiro", "Preparar demonstrativos financeiros com Excel para a empresa de RH.", "conferir"},
		{"Enfermeira", "", "triagem clínica", "Customer Support Engineer", "Resolve tickets about our software for triagem clínica.", "excluir"},
		{"Designer", "", "Figma", "Engenheira de Software", "Desenvolver nosso produto de design integrado ao Figma.", "excluir"},
		{"Técnica em Radiologia", "", "radiografia", "Assistente de Vendas", "Vender equipamentos de radiografia a profissionais de radiologia.", "conferir"},
		{"Técnica de Logística", "", "Excel", "Técnica de laboratório", "Registrar resultados de ensaios em Excel.", "conferir"},
		{"Auxiliar de laboratório", "", "Excel", "Auxiliar de logística", "Registrar cargas e expedições em Excel.", "conferir"},
	} {
		t.Run(tt.cargo+"/"+tt.titulo, func(t *testing.T) {
			p, v := perfilJavaRadar(), anuncioRadar()
			p["cargo"], p["titulosEquivalentes"] = tt.cargo, []string{tt.equivalente}
			p["stackPrincipal"] = []string{}
			if tt.habilidade != "" {
				p["stackPrincipal"] = []string{tt.habilidade}
			}
			v["titulo"], v["nivel"], v["descricao"], v["empresa"] = tt.titulo, "Sênior", tt.descricao, "Grupo Exemplo de Saúde, RH e Design"
			classe, motivo := QualificarRadar(p, v, diaDoRadar)
			if classe != tt.classe {
				t.Errorf("%s (%s), esperado %s", classe, motivo, tt.classe)
			}
		})
	}
}

func TestRadarElegibilidadeGeografica(t *testing.T) {
	for _, tt := range []struct {
		nome, pais, cidade, modelo, local string
		elegiveis                         []string
		mundial                           bool
		classe                            string
	}{
		{"remoto mundial", "Brasil", "", "Remoto", "Remote worldwide", nil, false, "forte"},
		{"remoto mundial estruturado", "Brasil", "", "Remoto", "", nil, true, "forte"},
		{"só outro país", "Brasil", "", "Remoto", "Remote, US only", nil, false, "excluir"},
		{"outro perfil pode trabalhar nos EUA", "Estados Unidos", "", "Remoto", "Remote, US only", nil, false, "forte"},
		{"mundial com exceção", "Brasil", "", "Remoto", "Worldwide except Brazil", nil, false, "excluir"},
		{"remoto ambíguo", "Brasil", "", "Remoto", "Remote", nil, false, "conferir"},
		{"região que inclui o país", "Brasil", "", "Remoto", "", []string{"LATAM"}, false, "forte"},
		{"restrição explícita estruturada", "Brasil", "", "Remoto", "", []string{"US", "CA"}, false, "excluir"},
		{"país em outro idioma", "Alemanha", "", "Remoto", "", []string{"Germany"}, false, "forte"},
		{"nome composto de país na lista", "Trinidad and Tobago", "", "Remoto", "", []string{"Canada", "Trinidad and Tobago"}, false, "forte"},
		{"sem país no perfil", "", "São Paulo", "Remoto", "", []string{"Brazil"}, false, "conferir"},
		{"híbrido na cidade", "Brasil", "Vitória", "Híbrido", "Vitoria, ES", []string{"Brazil"}, false, "forte"},
		{"híbrido em outra cidade", "Brasil", "Vitória", "Híbrido", "Salvador, BA", []string{"Brazil"}, false, "excluir"},
		{"presencial sem cidade", "Brasil", "", "Presencial", "Vitória, ES", []string{"Brazil"}, false, "conferir"},
		{"cidade sem país", "Brasil", "Vitória", "Presencial", "Vitória", nil, false, "conferir"},
		{"país sem cidade", "Brasil", "Vitória", "Presencial", "Brasil", []string{"Brazil"}, false, "conferir"},
	} {
		t.Run(tt.nome, func(t *testing.T) {
			p, v := perfilJavaRadar(), anuncioRadar()
			p["paisTrabalho"], p["cidadeTrabalho"], p["cidade"] = tt.pais, tt.cidade, "Outro local"
			dominio.ObjetoDe(p["filtros"])["modelo"] = []string{tt.modelo}
			v["modelo"], v["local"], v["paisesElegiveis"], v["remotoMundial"] = tt.modelo, tt.local, tt.elegiveis, tt.mundial
			if classe, motivo := QualificarRadar(p, v, diaDoRadar); classe != tt.classe || motivo == "" {
				t.Errorf("%s (%s), esperado %s", classe, motivo, tt.classe)
			}
		})
	}
}

func TestRadarCruzaAtribuicoesComRequisitosDeclarados(t *testing.T) {
	for _, tt := range []struct {
		titulo, descricao string
		stack             []string
		classe            string
	}{
		{"Senior Backend Engineer", "Build and maintain backend APIs. Requirements: Java and Spring Boot.", nil, "forte"},
		{"Senior Backend Engineer", "Desenvolver APIs para a loja.", []string{"Java", "Spring Boot"}, "forte"},
		{"Senior Backend Engineer", "Resolve customer tickets using Java and Spring Boot.", nil, "conferir"},
		{"Senior Backend Engineer", "Atender clientes que usam Java. Requisitos: Java e Spring Boot.", nil, "conferir"},
		{"Senior JavaScript Engineer", "Build backend services in JavaScript.", nil, "excluir"},
		{"Senior Backend Engineer", "Build backend APIs. Some customers use Java.", nil, "conferir"},
	} {
		v := anuncioRadar()
		v["titulo"], v["descricao"], v["stack"] = tt.titulo, tt.descricao, tt.stack
		if classe, motivo := QualificarRadar(perfilJavaRadar(), v, diaDoRadar); classe != tt.classe || classe == "forte" && !strings.Contains(motivo, "Java") {
			t.Errorf("%s: %s (%s), esperado %s", tt.descricao, classe, motivo, tt.classe)
		}
	}
}

func TestRadarRespeitaSenioridadeDoCargoOuDoFiltroExplicito(t *testing.T) {
	for _, tt := range []struct {
		alvo, titulo string
		aceitos      []string
		classe       string
	}{
		{"Senior Java Engineer", "Junior Java Engineer", nil, "excluir"},
		{"Senior Java Engineer", "Junior Java Engineer", []string{"Júnior", "Pleno"}, "forte"},
		{"Java Engineer", "Staff Java Engineer", []string{"Staff"}, "forte"},
		{"Java Engineer", "Staff Java Engineer", []string{"Pleno"}, "excluir"},
	} {
		p, v := perfilJavaRadar(), anuncioRadar()
		p["cargo"], dominio.ObjetoDe(p["filtros"])["niveis"], v["titulo"] = tt.alvo, tt.aceitos, tt.titulo
		if classe, motivo := QualificarRadar(p, v, diaDoRadar); classe != tt.classe {
			t.Errorf("%s × %s (%v): %s (%s), esperado %s", tt.alvo, tt.titulo, tt.aceitos, classe, motivo, tt.classe)
		}
	}
}

func TestRadarGeografiaComprovadaNoCorpoDoAnuncio(t *testing.T) {
	for _, tt := range []struct {
		frase, local string
		mundial      bool
		classe       string
	}{
		{"Trabalho remoto em todo o Brasil.", "", false, "forte"},
		{"Candidates must be based in the United States.", "Remote worldwide", true, "excluir"},
		{"Our headquarters is in Brazil and our customers are worldwide.", "", false, "conferir"},
		{"The job is remote worldwide.", "", false, "forte"},
		{"This role is remote from anywhere in the world except Brazil.", "", false, "excluir"},
		{"", "Remote, US only", true, "excluir"},
	} {
		v := anuncioRadar()
		v["descricao"], v["local"], v["remotoMundial"] = texto(v["descricao"])+" "+tt.frase, tt.local, tt.mundial
		if c, m := QualificarRadar(perfilJavaRadar(), v, diaDoRadar); c != tt.classe {
			t.Errorf("%q / %q: %s (%s), esperado %s", tt.frase, tt.local, c, m, tt.classe)
		}
	}
}

func TestRadarDistingueAtividadePrincipalDeContextoIncidental(t *testing.T) {
	testes := []struct {
		alvo, titulo      string
		stack             []string
		descricao, classe string
	}{
		{"Java Engineer", "Senior Software Engineer", []string{"Java", "Spring Boot"}, "You'll work primarily in Python while collaborating across the Java ecosystem, building models for forecasting.", "conferir"},
		{"Java Engineer", "Senior Software Engineer", []string{"Java"}, "Build and operate services primarily in Python, while integrating with Java APIs as needed. Requirements: maintain interoperability with Java when needed.", "conferir"},
		{"Java Engineer", "Senior Android Engineer", []string{"Java"}, "Build our welcoming engineering culture " + strings.Repeat("together with people from many different backgrounds ", 6) + "and meet members of our Java community.", "conferir"},
		{"Enfermeira de UTI", "Enfermeira", []string{"UTI"}, "Atuar principalmente na vacinação enquanto colabora com a equipe de UTI do hospital.", "conferir"},
		{"Analista de Compras", "Analista", []string{"negociação"}, "Conferir notas fiscais em parceria com o time de compras que cuida da negociação.", "conferir"},
		{"Product Designer", "Senior Product Designer", []string{"Figma"}, "Design newsletter campaigns in Canva. Requirements: consult with the Figma team and follow its handoff process.", "conferir"},
		{"Java Engineer", "Senior Software Engineer", []string{"Java"}, "Develop backend services in Java while collaborating with the team responsible for Python models.", "forte"},
		{"Enfermeira de UTI", "Enfermeira", []string{"UTI"}, "Atuar principalmente na UTI enquanto colabora com o setor de vacinação.", "forte"},
	}
	for _, tt := range testes {
		p, v := perfilJavaRadar(), anuncioRadar()
		p["cargo"], p["stackPrincipal"], v["titulo"], v["descricao"], v["nivel"] = tt.alvo, tt.stack, tt.titulo, tt.descricao, "Sênior"
		if classe, motivo := QualificarRadar(p, v, diaDoRadar); classe != tt.classe {
			t.Errorf("%s × %s: %s (%s), esperado %s", tt.alvo, tt.descricao, classe, motivo, tt.classe)
		}
	}
}

func TestRadarCitaAAtividadeECompetenciaNoMesmoTrecho(t *testing.T) {
	p, v := perfilJavaRadar(), anuncioRadar()
	v["descricao"] = "We are a welcoming workplace " + strings.Repeat("with a longstanding commitment to our shared values ", 8) + " What you will do: Build payment APIs in Java and Spring Boot."
	c, m := QualificarRadar(p, v, diaDoRadar)
	if c != "forte" || !strings.Contains(m, "Build payment APIs in Java") || strings.Contains(m, "welcoming workplace") {
		t.Fatalf("citação não mostra o trabalho com Java: %s %s", c, m)
	}
}

func TestRadarRegiaoDeContratacaoSobrepoeRotuloMundial(t *testing.T) {
	for _, tt := range []struct{ titulo, pais, descricao, classe string }{
		{"Senior Backend Engineer - EU", "Brazil", "For this role we can hire candidates based in the UK, Portugal or the Netherlands About the team We work together.", "excluir"},
		{"Senior Backend Engineer - EU", "United Kingdom", "For this role we can hire candidates based in the UK, Portugal or the Netherlands About the team We work together.", "forte"},
		{"Senior Backend Engineer - EU", "Brasil", "", "excluir"},
		{"Senior Backend Engineer - EU", "Portugal", "", "forte"},
		{"Senior Backend Engineer", "Brasil", "We sell products to customers based in Europe.", "forte"},
	} {
		p, v := perfilJavaRadar(), anuncioRadar()
		p["paisTrabalho"], v["titulo"], v["descricao"], v["remotoMundial"] = tt.pais, tt.titulo, texto(v["descricao"])+" "+tt.descricao, true
		if c, m := QualificarRadar(p, v, diaDoRadar); c != tt.classe {
			t.Errorf("%s × %s: %s (%s), esperado %s", tt.pais, tt.titulo, c, m, tt.classe)
		}
	}
}
