package dominio

import "strings"

// Preencher no site da empresa (docs/propostas/preencher-site-externo.md): o pacote e o resultado, compartilhados
// pela API, pelo envio e pelo navegador.

// ResultadoPreenchimento: o que o preenchimento fez, só números.
type ResultadoPreenchimento struct {
	Preenchidos int  `json:"preenchidos"`
	Anexou      bool `json:"anexou"`
	Faltando    int  `json:"faltando"`
}

// ConteudoAssistido monta o pacote do preenchimento: o contato do perfil e as respostas padrão, mais o currículo
// da candidatura reservada.
func ConteudoAssistido(perfil, pacote Objeto) Objeto {
	contato := Objeto{}
	for _, k := range CamposDeContato {
		if s := strings.TrimSpace(texto(perfil[k])); s != "" {
			contato[k] = s
		}
	}
	if local := LocalDoPerfil(perfil); local != "" {
		contato["local"] = local // "Recife, PE": para o campo que pede cidade e estado juntos
	}
	return Objeto{"contato": contato, "respostas": lista(perfil["respostasPadrao"]), "curriculo": pacote["curriculo"]}
}
