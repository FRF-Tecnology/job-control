// Package legado importa os dados da Central antiga (Node, pasta dados/ com arquivos JSON) para o app novo.
// Os ids continuam os mesmos, então importar de novo não duplica nada: o estado importado substitui o atual.
package legado

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// arquivos da Central antiga, por coleção.
var arquivos = map[string]string{
	"vagas": "vagas.json", "eventos": "eventos.json", "plataformas": "plataformas.json", "rodadas": "rodadas.json",
	"perfil": "perfil.json", "pedidos": "pedidos.json", "emails": "emails.json",
}

// Relatorio diz o que foi lido e o que impede ou merece atenção na importação.
type Relatorio struct {
	Contagem   map[string]int // itens por coleção
	Curriculos []string       // PDFs encontrados em dados/curriculos
	Bloqueios  []string       // impedem a importação (ex.: CPF/RG)
	Avisos     []string       // não impedem (ex.: vaga antiga sem campo obrigatório)
}

// Ler lê a pasta dados/ da Central antiga. Arquivo ausente vira coleção vazia.
func Ler(dir string) (dominio.Estado, Relatorio, error) {
	e := dominio.EstadoVazio()
	rel := Relatorio{Contagem: map[string]int{}}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return e, rel, fmt.Errorf("pasta de dados não encontrada: %s", dir)
	}
	for colecao, nome := range arquivos {
		b, err := os.ReadFile(filepath.Join(dir, nome))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return e, rel, err
		}
		if colecao == "perfil" {
			if err := json.Unmarshal(b, &e.Perfil); err != nil {
				return e, rel, fmt.Errorf("%s corrompido: %w", nome, err)
			}
			if e.Perfil == nil {
				e.Perfil = dominio.Objeto{}
			}
			rel.Contagem[colecao] = 1
			continue
		}
		var l []dominio.Objeto
		if err := json.Unmarshal(b, &l); err != nil {
			return e, rel, fmt.Errorf("%s corrompido: %w", nome, err)
		}
		if l == nil {
			l = []dominio.Objeto{}
		}
		switch colecao {
		case "vagas":
			e.Vagas = l
		case "eventos":
			e.Eventos = l
		case "plataformas":
			e.Plataformas = l
		case "rodadas":
			e.Rodadas = l
		case "pedidos":
			e.Pedidos = l
		case "emails":
			e.Emails = l
		}
		rel.Contagem[colecao] = len(l)
	}
	if itens, err := os.ReadDir(filepath.Join(dir, "curriculos")); err == nil {
		for _, i := range itens {
			if !i.IsDir() && strings.EqualFold(filepath.Ext(i.Name()), ".pdf") {
				rel.Curriculos = append(rel.Curriculos, i.Name())
			}
		}
		sort.Strings(rel.Curriculos)
	}
	limparLinks(e, &rel)
	verificar(e, &rel)
	return e, rel, nil
}

var reHTTP = regexp.MustCompile(`(?i)^https?://`)

// limparLinks tira o link de vaga que não comece com http(s):// (ex.: javascript:, data:). Na janela
// do app ele viraria script com acesso à API local (jc-a2y.29). A vaga entra; o aviso não repete o link.
func limparLinks(e dominio.Estado, rel *Relatorio) {
	for _, v := range e.Vagas {
		bruto, tem := v["link"]
		if !tem || bruto == nil || bruto == "" {
			continue
		}
		if s, ok := bruto.(string); ok && reHTTP.MatchString(s) {
			continue
		}
		delete(v, "link")
		rel.Avisos = append(rel.Avisos, dominio.CampoJS(v, "id")+": link fora de http(s) removido")
	}
}

// verificar aplica as mesmas regras da Central: documento (CPF/RG) bloqueia; o resto vira aviso.
func verificar(e dominio.Estado, rel *Relatorio) {
	ids := map[string]bool{}
	for _, v := range e.Vagas {
		id := dominio.CampoJS(v, "id")
		if ids[id] {
			rel.Avisos = append(rel.Avisos, id+": id repetido")
		}
		ids[id] = true
		for _, erro := range dominio.ValidarVaga(v) {
			msg := id + ": " + erro
			if strings.Contains(erro, "CPF/RG") {
				rel.Bloqueios = append(rel.Bloqueios, msg)
			} else {
				rel.Avisos = append(rel.Avisos, msg)
			}
		}
	}
	if erros := dominio.ValidarPerfil(e.Perfil); len(erros) > 0 {
		for _, erro := range erros {
			rel.Avisos = append(rel.Avisos, "perfil: "+erro)
		}
	}
	if dominio.PerfilSensivel(e.Perfil) {
		rel.Bloqueios = append(rel.Bloqueios, "perfil: guarda CPF, RG, senha ou dados bancários")
	}
	for nome, l := range map[string][]dominio.Objeto{"eventos": e.Eventos, "plataformas": e.Plataformas, "rodadas": e.Rodadas, "pedidos": e.Pedidos, "emails": e.Emails} {
		for i, o := range l {
			if dominio.ContemDocumentoEm(o) {
				rel.Bloqueios = append(rel.Bloqueios, fmt.Sprintf("%s[%d]: parece conter CPF/RG", nome, i))
			}
		}
	}
	sort.Strings(rel.Bloqueios)
}

// CopiarCurriculos copia os PDFs de dir/curriculos para destino (sem sobrescrever os que já existem).
func CopiarCurriculos(dir, destino string, nomes []string) (int, error) {
	if err := os.MkdirAll(destino, 0o700); err != nil {
		return 0, err
	}
	copiados := 0
	for _, n := range nomes {
		alvo := filepath.Join(destino, n)
		if _, err := os.Stat(alvo); err == nil {
			continue
		}
		if err := copiar(filepath.Join(dir, "curriculos", n), alvo); err != nil {
			return copiados, err
		}
		copiados++
	}
	return copiados, nil
}

func copiar(de, para string) error {
	o, err := os.Open(de)
	if err != nil {
		return err
	}
	defer o.Close()
	d, err := os.OpenFile(para, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	// Cópia pela metade não fica na pasta de currículos: com erro ao copiar ou ao fechar, o destino é apagado
	// (jc-a2y.15, gosec G104).
	_, err = io.Copy(d, o)
	if errFechar := d.Close(); err == nil {
		err = errFechar
	}
	if err != nil {
		os.Remove(para)
	}
	return err
}
