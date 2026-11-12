package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Só consulta o host público da integração que forneceu o anúncio. Nunca segue
// redirecionamentos para formulários externos nem envia dados do candidato.
func revalidarAnuncioRadar(ctx context.Context, cliente *http.Client, v Objeto, agora time.Time) error {
	if t, err := time.Parse(time.RFC3339, dominio.Texto(v["expiraEm"])); err == nil && !t.After(agora) {
		return falha(409, "a vaga expirou; escolha outra no Radar")
	}
	version, _ := dominio.NumeroDe(v["radarVersao"])
	if version != 1 {
		return nil
	}
	host := map[string]string{"Gupy": "gupy.io", "Himalayas": "himalayas.app", "Arbeitnow": "arbeitnow.com", "We Work Remotely": "weworkremotely.com", "Jobicy": "jobicy.com"}[dominio.Texto(v["fonteBusca"])]
	if host == "" {
		return nil
	}
	endereco, err := url.Parse(dominio.Texto(v["link"]))
	if err != nil || endereco.Scheme != "https" || endereco.User != nil || endereco.Port() != "" || (endereco.Hostname() != host && !strings.HasSuffix(endereco.Hostname(), "."+host)) {
		return nil
	}
	metodo := http.MethodHead
	var idJobicy int64
	if host == "jobicy.com" && strings.HasPrefix(endereco.Path, "/jobs/") {
		trecho, _, _ := strings.Cut(strings.TrimPrefix(endereco.Path, "/jobs/"), "-")
		idJobicy, _ = strconv.ParseInt(trecho, 10, 64)
		if idJobicy > 0 {
			endereco.Path = "/api/jobs/status"
			endereco.RawQuery = "ids=" + strconv.FormatInt(idJobicy, 10)
			metodo = http.MethodGet
		}
	}
	if host == "himalayas.app" {
		partes := strings.Split(strings.Trim(endereco.Path, "/"), "/")
		if len(partes) == 4 && partes[0] == "companies" && partes[2] == "jobs" {
			endereco.Path = "/jobs/api/" + strings.Join(partes, "/")
			endereco.RawQuery = ""
			metodo = http.MethodGet
		}
	}
	prazo, cancelar := context.WithTimeout(ctx, 4*time.Second)
	defer cancelar()
	s := *cliente
	s.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(prazo, metodo, endereco.String(), nil)
	if err != nil {
		return err
	}
	resp, err := s.Do(req)
	if err != nil {
		return falha(503, "não foi possível verificar se a vaga continua aberta; tente novamente ou confira o anúncio original")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return falha(409, "a fonte informa que a vaga não está mais disponível")
	}
	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		return falha(503, "a fonte está indisponível para verificar a vaga; tente novamente ou confira o anúncio original")
	}
	if idJobicy > 0 {
		var resposta struct {
			Jobs []struct {
				ID     int64
				Status string
			}
		}
		if resp.StatusCode == 200 && json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&resposta) == nil {
			for _, j := range resposta.Jobs {
				if j.ID != idJobicy {
					continue
				}
				switch j.Status {
				case "active":
					return nil
				case "closed":
					return falha(409, "a fonte informa que a vaga não está mais disponível")
				}
			}
		}
		return falha(503, "a fonte ainda não confirmou a disponibilidade; confira o anúncio original ou tente novamente")
	}
	// Fontes que não aceitam HEAD ou levam ao formulário deixam a confirmação final no navegador.
	return nil
}

func (a *API) verificarAntesDeAbrir(ctx context.Context, id string) error {
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return err
	}
	// A retirada de um anúncio não deve impedir confirmar um envio já iniciado.
	for _, c := range e.Candidaturas {
		if c["vagaId"] == id && c["estado"] != dominio.CandCancelada {
			return nil
		}
	}
	for _, v := range e.Vagas {
		if v["id"] == id {
			return revalidarAnuncioRadar(ctx, http.DefaultClient, v, a.relogio())
		}
	}
	return nil
}
