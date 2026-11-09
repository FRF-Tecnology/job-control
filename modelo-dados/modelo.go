// Package modelodados guarda os dados iniciais de uma conta nova (as plataformas onde procurar vagas).
// O mesmo arquivo serve à Central antiga (Node copia para dados/) e ao app novo (vai embutido no binário).
package modelodados

import _ "embed"

// Plataformas é o conteúdo de plataformas.json.
//
//go:embed plataformas.json
var Plataformas []byte
