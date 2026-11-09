package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// A janela pede tudo ao mesmo handler: /api/ vai para a API, o resto para as telas embutidas.
func TestRotearMandaApiParaApiETelasParaTelas(t *testing.T) {
	if _, err := os.Stat("frontend/dist/index.html"); err != nil {
		t.Skip("frontend/dist ainda não foi gerado (rode wails3 build)")
	}
	banco, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer banco.Fechar()
	h := rotear(api.Nova(banco, api.Opcoes{}).Handler(), application.AssetFileServerFS(telas))

	pedir := func(caminho string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, caminho, nil))
		corpo, _ := io.ReadAll(rec.Body)
		return rec.Code, string(corpo)
	}

	if s, c := pedir("/api/v1/saude"); s != 200 || !strings.Contains(c, `"ok":true`) {
		t.Errorf("/api/v1/saude = %d %q", s, c)
	}
	if s, c := pedir("/"); s != 200 || !strings.Contains(c, "<title>Applyer</title>") {
		t.Errorf("/ = %d; não veio o index.html das telas", s)
	}
}
