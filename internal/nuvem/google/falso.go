package google

import "context"

// Falso é o Trocador dos testes: cada code conhecido vira a identidade do mapa; os outros falham.
type Falso struct {
	Identidades map[string]Identidade // chave = code
}

func (f Falso) Trocar(_ context.Context, code, _, redirectURI string) (Identidade, error) {
	if !RedirectValido(redirectURI) {
		return Identidade{}, ErrRedirect
	}
	id, ok := f.Identidades[code]
	if !ok {
		return Identidade{}, ErrGoogle
	}
	return id, nil
}
