//go:build radar_publico

package api

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/triagem"
)

// Ensaio opt-in do coletor real e das evidências, sem usar o cofre, o PDF,
// a base de produção. A confirmação pela assinatura Claude é opt-in separado,
// com um máximo de duas avaliações por cargo e sem dados particulares.
func TestAmostraPublicaRadar(t *testing.T) {
	arquivo, saida := os.Getenv("RADAR_PERFIS_QA"), os.Getenv("RADAR_EVIDENCIA_QA")
	if arquivo == "" || saida == "" {
		t.Skip("informe um JSON de perfis de teste e um destino de evidência")
	}
	dados, err := os.ReadFile(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	var perfis []Objeto
	if err = json.Unmarshal(dados, &perfis); err != nil || len(perfis) == 0 {
		t.Fatal("perfis de teste inválidos", err)
	}
	var relatorio []Objeto
	cache := map[string]busca.Coleta{}
	cachePath := os.Getenv("RADAR_AMOSTRA_QA")
	if cachePath != "" {
		if b, err := os.ReadFile(cachePath); err == nil {
			if err = json.Unmarshal(b, &cache); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, p := range perfis {
		// O cache é exclusivo deste ensaio, criado para comparar a mesma amostra.
		criterios, _ := json.Marshal(p)
		chave := string(criterios)
		r, ok := cache[chave]
		if !ok {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
			r = busca.Consultar(ctx, p)
			cancel()
			cache[chave] = r
		}
		grupos := map[string]int{"forte": 0, "conferir": 0, "excluir": 0}
		var amostra []Objeto
		avaliadas := 0
		for _, v := range r.Vagas {
			classe, motivo := triagem.QualificarRadar(p, v, time.Now())
			grupos[classe]++
			item := Objeto{"classe": classe, "motivo": motivo, "titulo": v["titulo"], "empresa": v["empresa"], "link": v["link"], "publicadaEm": v["publicadaEm"], "fonte": v["fonteBusca"], "local": v["local"]}
			if os.Getenv("RADAR_IA_QA") == "claude-cli" && classe == "forte" && avaliadas < 2 {
				avaliadas++
				motor := ia.Nova(ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "claude-cli"}}}, func(string) string { return "" }, nil)
				anon := Objeto{}
				for _, k := range []string{"cargo", "stackPrincipal", "ingles", "paisTrabalho"} {
					anon[k] = p[k]
				}
				f, _ := p["filtros"].(map[string]any)
				anon["filtros"] = Objeto{"niveis": f["niveis"], "modelo": f["modelo"]}
				ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
				nota, err := triagem.AvaliarRadar(ctx, motor, anon, v)
				cancel()
				if err != nil {
					item["ia"] = "indisponível"
				} else {
					item["ia"], item["decisao"], item["motivoIA"] = nota.Aderencia, nota.Decisao, nota.Motivo
				}
			}
			amostra = append(amostra, item)
		}
		relatorio = append(relatorio, Objeto{"cargo": p["cargo"], "amostra": amostra, "grupos": grupos, "avisos": r.Avisos, "encontradas": len(r.Vagas), "data": time.Now().UTC()})
		t.Logf("%v: %d anúncios; %v; avisos=%v", p["cargo"], len(r.Vagas), grupos, r.Avisos)
	}
	if cachePath != "" {
		b, err := json.Marshal(cache)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(cachePath, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, err := json.MarshalIndent(relatorio, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(saida, b, 0600); err != nil {
		t.Fatal(err)
	}
}
