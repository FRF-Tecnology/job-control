package busca

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

func weWorkRemotely(ctx context.Context, c *http.Client, termos []string) ([]dominio.Objeto, error) {
	dados, err := baixar(ctx, c, "https://weworkremotely.com/remote-jobs.rss", "application/rss+xml, application/xml")
	if err != nil {
		return nil, err
	}
	var feed struct {
		XMLName xml.Name `xml:"rss"`
		Channel *struct {
			Items []struct {
				Titulo    string `xml:"title"`
				Link      string `xml:"link"`
				Regiao    string `xml:"region"`
				Pais      string `xml:"country"`
				Descricao string `xml:"description"`
				Tipo      string `xml:"type"`
				Pub       string `xml:"pubDate"`
				Fim       string `xml:"expires_at"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(dados, &feed); err != nil {
		return nil, err
	}
	if feed.Channel == nil {
		return nil, errors.New("formato inesperado")
	}
	var vagas []dominio.Objeto
	for _, item := range feed.Channel.Items {
		empresa, titulo, ok := strings.Cut(item.Titulo, ": ")
		empresa, titulo = texto(empresa, 180), texto(titulo, 300)
		descricao := texto(item.Descricao, 6000)
		if !ok || empresa == "" || !cargoNasAtribuicoes(termos, titulo, descricao) || !linkDaFonte(item.Link, "weworkremotely.com") {
			continue
		}
		id := sha256.Sum256([]byte(item.Link))
		v := dominio.Objeto{"id": fmt.Sprintf("wwr-%x", id[:12]), "titulo": titulo, "empresa": empresa, "fonteBusca": "We Work Remotely", "fonteURL": item.Link, "link": item.Link, "plataforma": "externo", "mercado": "internacional", "modelo": "Remoto", "local": texto(item.Regiao, 250), "descricao": descricao, "contrato": texto(item.Tipo, 100)}
		if fim, err := time.Parse(time.RFC1123Z, item.Fim); err == nil {
			if !fim.After(time.Now()) {
				continue
			}
			v["expiraEm"] = fim.UTC().Format(time.RFC3339)
		}
		if data, err := time.Parse(time.RFC1123Z, item.Pub); err == nil {
			v["publicadaEm"] = data.UTC().Format(time.RFC3339)
		}
		if pais := texto(item.Pais, 250); pais != "" {
			v["paisesElegiveis"] = []string{pais}
			v["local"] = pais
		} else if strings.EqualFold(item.Regiao, "Anywhere in the World") || strings.EqualFold(item.Regiao, "Worldwide") {
			v["remotoMundial"] = true
		}
		vagas = append(vagas, v)
	}
	return vagas, nil
}
