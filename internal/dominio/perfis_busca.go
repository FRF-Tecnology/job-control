package dominio

import (
	"maps"
	"strings"
	"unicode/utf8"
)

const PerfilOriginal = "original"

// O perfil profissional fica materializado em Estado.Perfil para os consumidores
// existentes. Ao trocar, só os dados específicos da busca viajam com o perfil.
var camposDaBusca = []string{"nome", "cargo", "titulosEquivalentes", "cidade", "paisTrabalho", "cidadeTrabalho", "experienciaAnos", "formacao", "ingles", "stackPrincipal", "stackSecundaria", "foco", "filtros", "contratos", "pretensaoUSD", "pretensaoCLT", "pretensaoPJ", "faixaSalarial", "curriculos", "curriculoExibicao"}

func (e Estado) PerfilBuscaAtivo() string {
	for _, perfil := range e.PerfisBusca {
		if perfil["ativo"] == true {
			return texto(perfil["id"])
		}
	}
	return PerfilOriginal
}

func DonoDaBusca(item Objeto) string {
	if id := texto(item["perfilBuscaId"]); id != "" {
		return id
	}
	return PerfilOriginal
}

// Resultados do mesmo anúncio podem ter avaliações diferentes em dois
// perfis. A vaga que entrou na lista ou ganhou pacote pertence à conta toda.
func AnuncioReservado(e Estado, vaga Objeto) error {
	if len(e.PerfisBusca) == 0 {
		return nil
	}
	anuncio := texto(ou(vaga["anuncioId"], vaga["id"]))
	for _, outra := range e.Vagas {
		if DonoDaBusca(outra) == DonoDaBusca(vaga) || texto(ou(outra["anuncioId"], outra["id"])) != anuncio {
			continue
		}
		reservada := outra["etapa"] != "descoberta" && outra["etapa"] != "pulada"
		if !reservada {
			for _, cand := range e.Candidaturas {
				if cand["vagaId"] == outra["id"] && cand["estado"] != CandCancelada {
					reservada = true
					break
				}
			}
		}
		if reservada {
			return &ErroConflito{Msg: "este anúncio já está na lista ou no quadro de outro perfil", Existente: texto(outra["id"])}
		}
	}
	return nil
}

func criteriosDoPerfil(perfil Objeto) Objeto {
	dados := Objeto{}
	for _, chave := range camposDaBusca {
		if valor, ok := perfil[chave]; ok {
			dados[chave] = clonar(valor)
		}
	}
	return dados
}

func (e Estado) PerfilDeBusca(id string) Objeto {
	if id == e.PerfilBuscaAtivo() {
		return e.Perfil
	}
	for _, p := range e.PerfisBusca {
		if p["id"] == id {
			perfil := maps.Clone(e.Perfil)
			for _, chave := range camposDaBusca {
				delete(perfil, chave)
			}
			maps.Copy(perfil, obj(p["criterios"]))
			return perfil
		}
	}
	return nil
}

func nomeOriginal(perfil Objeto) string {
	if nome := strings.TrimSpace(texto(perfil["cargo"])); nome != "" && Normalizar(nome) != Normalizar(perfil["nome"]) {
		return nome
	}
	return "Perfil principal"
}

func resumoBusca(id, nome string, dados Objeto) Objeto {
	pacote := Objeto{}
	for _, chave := range []string{"curriculos", "curriculoExibicao", "nome", "email", "telefone", "cidade", "pretensaoUSD", "pretensaoCLT", "pretensaoPJ", "respostasPadrao"} {
		if valor, ok := dados[chave]; ok {
			pacote[chave] = valor
		}
	}
	return Objeto{"id": id, "nome": nome, "cargo": dados["cargo"], "curriculos": dados["curriculos"], "perfilPacote": pacote}
}

func (e Estado) ResumoPerfisBusca() []Objeto {
	if len(e.PerfisBusca) == 0 {
		return []Objeto{resumoBusca(PerfilOriginal, nomeOriginal(e.Perfil), e.Perfil)}
	}
	resumo := make([]Objeto, 0, len(e.PerfisBusca))
	for _, p := range e.PerfisBusca {
		dados := e.PerfilDeBusca(texto(p["id"]))
		resumo = append(resumo, resumoBusca(texto(p["id"]), texto(p["nome"]), dados))
	}
	return resumo
}

func AtivarPerfilBusca(e Estado, id string) (Estado, error) {
	if id == e.PerfilBuscaAtivo() {
		return e, nil
	}
	alvo := e.PerfilDeBusca(id)
	if alvo == nil {
		return e, &ErroNaoEncontrado{"perfil de busca não encontrado"}
	}
	novo := e.Clonar()
	for _, p := range novo.PerfisBusca {
		if p["ativo"] == true {
			p["criterios"] = criteriosDoPerfil(e.Perfil)
			delete(p, "ativo")
		}
		if p["id"] == id {
			p["ativo"] = true
			delete(p, "criterios")
		}
	}
	novo.Perfil = alvo
	return novo, nil
}

func CriarPerfilBusca(e Estado, id, nome, modelo string) (Estado, error) {
	return CriarPerfilComCargo(e, id, nome, modelo, "")
}

func CriarPerfilComCargo(e Estado, id, nome, modelo, cargo string) (Estado, error) {
	nome = strings.TrimSpace(nome)
	if nome == "" || utf8.RuneCountInString(nome) > 120 || ContemDocumento(nome) {
		return e, validacao("informe um nome de perfil válido com até 120 caracteres")
	}
	for _, p := range e.ResumoPerfisBusca() {
		if Normalizar(p["nome"]) == Normalizar(nome) {
			return e, &ErroConflito{Msg: "você já tem um perfil com esse nome"}
		}
	}
	dados := criteriosDoPerfil(e.Perfil)
	delete(dados, "titulosEquivalentes")
	switch modelo {
	case "ai":
		dados["cargo"], dados["stackPrincipal"], dados["stackSecundaria"] = "AI Engineer", []any{"Python", "Machine Learning", "LLM"}, []any{"PyTorch", "MLOps"}
	case "java":
		dados["cargo"], dados["stackPrincipal"], dados["stackSecundaria"] = "Java Engineer", []any{"Java", "Spring Boot"}, []any{"SQL", "APIs"}
	case "outro":
		cargo = strings.TrimSpace(cargo)
		if cargo == "" {
			cargo = nome
		}
		if utf8.RuneCountInString(cargo) > 180 || ContemDocumento(cargo) {
			return e, validacao("informe um cargo válido")
		}
		dados["cargo"], dados["stackPrincipal"], dados["stackSecundaria"] = cargo, []any{}, []any{}
	default:
		return e, validacao("escolha um cargo de interesse")
	}
	novo := e.Clonar()
	if len(novo.PerfisBusca) == 0 {
		novo.PerfisBusca = []Objeto{{"id": PerfilOriginal, "nome": nomeOriginal(e.Perfil), "ativo": true}}
	}
	novo.PerfisBusca = append(novo.PerfisBusca, Objeto{"id": id, "nome": nome, "criterios": dados})
	return AtivarPerfilBusca(novo, id)
}
