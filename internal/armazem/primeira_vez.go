package armazem

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// PrepararPrimeiraVez deixa uma conta nova pronta: cria a pasta de currículos e, se o banco nunca foi
// gravado, coloca as plataformas iniciais. Devolve true quando era a primeira vez.
func (a *Armazem) PrepararPrimeiraVez(ctx context.Context, plataformas []byte) (bool, error) {
	if a.pasta != "" {
		if err := os.MkdirAll(filepath.Join(a.pasta, "curriculos"), 0o700); err != nil {
			return false, err
		}
	}
	v, err := a.Versao(ctx)
	if err != nil || v > 0 {
		return false, err
	}
	var lista []dominio.Objeto
	if err := json.Unmarshal(plataformas, &lista); err != nil {
		return false, fmt.Errorf("plataformas iniciais: %w", err)
	}
	err = a.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		if len(e.Plataformas) == 0 {
			e.Plataformas = lista
		}
		return e, nil
	})
	return err == nil, err
}
