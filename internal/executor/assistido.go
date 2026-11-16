package executor

import (
	"regexp"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Preenchimento assistido no site da empresa (docs/propostas/preencher-site-externo.md). Num site que o app não
// conhece, a PESSOA confere o domínio, o app preenche o que reconhece e a pessoa revisa, completa e envia. Por isso a
// regra é mais simples que a do envio (planejar), mas nunca mais solta no que importa:
//   - campo que a página já trouxe preenchido fica como está (o app não sobrescreve nada);
//   - campo sensível (senha, CPF, documento, dado bancário, termos, aceite) nunca é preenchido nem marcado;
//   - pergunta só com a resposta padrão de mesmo texto (nunca inventa);
//   - nada clica: nem avançar, nem enviar.
// O que fica em branco e é obrigatório conta em Faltando, para a tela avisar.

// Tipos que só o preenchimento assistido usa (sites próprios costumam separar o nome e pedir o LinkedIn).
const (
	CampoPrimeiroNome = "primeiro-nome"
	CampoSobrenome    = "sobrenome"
	CampoLinkedIn     = "linkedin"
	CampoGitHub       = "github"
	CampoEstado       = "estado"
	CampoPais         = "pais"
	CampoLocal        = "local" // cidade e estado num campo só ("Localização", "Location")
)

var (
	rotuloPrimeiroNome = regexp.MustCompile(`^(primeiro nome|first name|given name|nome \(primeiro\)|nombre de pila)$`)
	rotuloSoNome       = regexp.MustCompile(`^(nome|name|nombre)$`)
	rotuloSobrenome    = regexp.MustCompile(`sobrenome|ultimo nome|last name|surname|family name|apellido`)
	rotuloLinkedIn     = regexp.MustCompile(`linkedin`)
	rotuloGitHub       = regexp.MustCompile(`github`)
	rotuloEstado       = regexp.MustCompile(`^(estado|uf|state|province|provincia|state/province|estado/provincia|region|regiao)$`)
	rotuloPais         = regexp.MustCompile(`^(pais|country|pais de residencia|country of residence|pais onde mora)$`)
	rotuloLocal        = regexp.MustCompile(`localizacao|location|onde (voce )?mora|where do you live|cidade e estado|city and state|city, state`)
)

// Preenchimento é uma ação do preenchimento assistido. Alternativas: outros jeitos de escrever o mesmo valor, para
// uma lista de opções que não tenha o primeiro ("Pernambuco" ou "PE"; "Brasil" ou "Brazil").
type Preenchimento struct {
	Campo        Campo
	Valor        string
	Alternativas []string
	Anexar       bool
}

// tipoAssistido refina o tipo do classificador do envio para os campos de site próprio. temSobrenome: a página tem
// um campo de sobrenome, então "Nome" sozinho é o primeiro nome.
func tipoAssistido(c Campo, temSobrenome bool) string {
	rotulo := strings.TrimSpace(strings.TrimRight(dominio.Normalizar(c.Rotulo), " *:"))
	switch {
	case c.Tipo == CampoCidade && rotuloLocal.MatchString(rotulo):
		return CampoLocal
	case c.Tipo != CampoPergunta && c.Tipo != CampoNome && c.Tipo != CampoOutro:
		return c.Tipo // e-mail, telefone, cidade, currículo, sensíveis...: o classificador do envio decide
	case rotuloSobrenome.MatchString(rotulo):
		return CampoSobrenome
	case rotuloPrimeiroNome.MatchString(rotulo), temSobrenome && rotuloSoNome.MatchString(rotulo):
		return CampoPrimeiroNome
	case rotuloLinkedIn.MatchString(rotulo):
		return CampoLinkedIn
	case rotuloGitHub.MatchString(rotulo):
		return CampoGitHub
	case rotuloEstado.MatchString(rotulo):
		return CampoEstado
	case rotuloPais.MatchString(rotulo):
		return CampoPais
	case rotuloLocal.MatchString(rotulo):
		return CampoLocal
	}
	return c.Tipo
}

// PlanejarAssistido decide o que preencher na página. conteudo é o pacote (currículo e respostas); perfil dá o
// contato, o LinkedIn e o GitHub. temCurriculo: há uma cópia conferida do currículo para anexar.
func PlanejarAssistido(conteudo, perfil dominio.Objeto, campos []Campo, temCurriculo bool) (plano []Preenchimento, faltando int) {
	temSobrenome := false
	for _, c := range campos {
		if c.Tipo == CampoPergunta && rotuloSobrenome.MatchString(dominio.Normalizar(c.Rotulo)) {
			temSobrenome = true
		}
	}
	primeiro, sobrenome := nomeESobrenome(perfil)
	// O código do país sai do número quando a página tem um campo para ele, ou num site brasileiro (o número
	// nacional, como nos formulários daqui). Nos outros sites, o número vai completo, com o "+55".
	semDDI := temCampoDDI(campos) || conteudo["siteNacional"] == true
	contato := dominio.ObjetoDe(conteudo["contato"])
	for _, c := range campos {
		if c.Preenchido {
			continue // a página já trouxe: fica como está
		}
		var valor string
		var alternativas []string
		switch tipoAssistido(c, temSobrenome) {
		case CampoCurriculo:
			if temCurriculo {
				plano = append(plano, Preenchimento{Campo: c, Anexar: true})
				continue
			}
		case CampoPrimeiroNome:
			valor = primeiro
		case CampoSobrenome:
			valor = sobrenome
		case CampoLocal:
			valor = dominio.Texto(contato["local"])
		case CampoEstado:
			uf := dominio.Texto(contato["estado"])
			if nome := dominio.EstadosDoBrasil[uf]; nome != "" && dominio.Texto(contato["paisResidencia"]) == "BR" {
				valor, alternativas = nome, []string{uf}
			} else {
				valor = uf
			}
		case CampoPais:
			if dominio.Texto(contato["paisResidencia"]) == "BR" {
				valor, alternativas = "Brasil", []string{"Brazil", "BR"}
			}
		case CampoLinkedIn:
			valor = linkPublico(perfil["linkedin"], "linkedin.com")
		case CampoGitHub:
			valor = linkPublico(perfil["github"], "github.com")
		case CampoPretensao, CampoSeguir, CampoOutro:
			// pretensão: a moeda e o período variam por site; a pessoa preenche. Caixas: nunca marca.
		default:
			var motivo string
			valor, motivo = valorDoCampo(conteudo, c)
			if motivo != "" {
				valor = "" // sensível ou sem resposta: a pessoa preenche
			} else if c.Tipo == CampoTelefone && semDDI {
				valor = telefoneSemDDI(valor)
			}
		}
		if strings.TrimSpace(valor) == "" {
			if c.Obrigatorio {
				faltando++
			}
			continue
		}
		plano = append(plano, Preenchimento{Campo: c, Valor: valor, Alternativas: alternativas})
	}
	return plano, faltando
}

// nomeESobrenome: os campos separados do perfil; perfis antigos só têm o nome completo (primeira palavra e o resto).
func nomeESobrenome(perfil dominio.Objeto) (string, string) {
	primeiro, sobrenome := strings.TrimSpace(dominio.Texto(perfil["primeiroNome"])), strings.TrimSpace(dominio.Texto(perfil["sobrenome"]))
	if primeiro != "" || sobrenome != "" {
		return primeiro, sobrenome
	}
	nome := strings.Fields(dominio.Texto(perfil["nome"]))
	if len(nome) == 0 {
		return "", ""
	}
	return nome[0], strings.Join(nome[1:], " ")
}

// linkPublico: o endereço do perfil da pessoa, só se for https no site esperado (nunca outro texto).
func linkPublico(v any, host string) string {
	return dominio.LinkDePerfil(dominio.Texto(v), host)
}
