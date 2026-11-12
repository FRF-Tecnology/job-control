package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

const limitePerfilOnboarding = 64 << 10

func erroMultipart(err error) error {
	var excedido *http.MaxBytesError
	if errors.As(err, &excedido) {
		return falha(413, "o arquivo ou o formulário excede o limite")
	}
	return falha(400, "formulário de primeiro acesso inválido")
}

// Leitura limitada em memória. O nome e o caminho do computador de origem
// não são usados no disco. Só os dois campos conhecidos são aceitos.
func lerOnboarding(w http.ResponseWriter, r *http.Request) (Objeto, error) {
	tipo, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err == nil && tipo == "application/json" {
		// A janela lê o arquivo selecionado e envia seus bytes em base64. O mesmo
		// documento serve à extração e à gravação, inclusive após mudar de etapa.
		maxCodificado := base64.StdEncoding.EncodedLen(int(curriculos.MaxBytes))
		limite := int64(maxCodificado + limitePerfilOnboarding + 1024)
		if r.ContentLength > limite {
			return nil, falha(413, "o arquivo ou o formulário excede o limite")
		}
		var entrada struct {
			Perfil      json.RawMessage `json:"perfil"`
			Curriculo   string          `json:"curriculoBase64"`
			ArquivoID   string          `json:"arquivoId,omitempty"`
			Revisao     *int64          `json:"revisao,omitempty"`
			NomeArquivo string          `json:"nomeArquivo,omitempty"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limite))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&entrada); err != nil {
			return nil, erroMultipart(err)
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			return nil, erroMultipart(err)
		}
		if len(entrada.Curriculo) > maxCodificado || len(entrada.Perfil) > limitePerfilOnboarding {
			return nil, falha(413, "o arquivo ou o perfil excede o limite")
		}
		pdf, err := base64.StdEncoding.DecodeString(entrada.Curriculo)
		if err != nil {
			return nil, falha(400, "não foi possível transferir o PDF. Selecione o arquivo novamente")
		}
		dados := Objeto{"curriculo": pdf}
		if entrada.ArquivoID != "" {
			if len(pdf) != 0 || entrada.NomeArquivo != "" {
				return nil, erroCampo("escolha uma referência salva ou envie o PDF")
			}
			dados["arquivoId"] = entrada.ArquivoID
			delete(dados, "curriculo")
		}
		if entrada.Revisao != nil {
			dados["revisao"] = float64(*entrada.Revisao)
		}
		if nome := nomeDeExibicao(entrada.NomeArquivo); nome != "" {
			dados["nomeArquivo"] = nome
		}
		if entrada.Perfil != nil {
			var perfil Objeto
			if json.Unmarshal(entrada.Perfil, &perfil) != nil || perfil == nil {
				return nil, falha(400, "o perfil de busca deve ser um objeto JSON")
			}
			dados["perfil"] = perfil
		}
		return dados, nil
	}
	if err != nil || tipo != "multipart/form-data" {
		return nil, falha(415, "envie o currículo como JSON ou multipart/form-data")
	}
	const limite = curriculos.MaxBytes + limitePerfilOnboarding + (64 << 10)
	if r.ContentLength > limite {
		return nil, falha(413, "o arquivo ou o formulário excede o limite")
	}
	r.Body = http.MaxBytesReader(w, r.Body, limite)
	partes, err := r.MultipartReader()
	if err != nil {
		return nil, erroMultipart(err)
	}
	dados := Objeto{}
	for {
		parte, err := partes.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, erroMultipart(err)
		}
		campo := parte.FormName()
		if _, duplicado := dados[campo]; duplicado {
			return nil, falha(400, "campo repetido no primeiro acesso")
		}
		switch campo {
		case "perfil":
			if parte.FileName() != "" {
				return nil, falha(400, "perfil deve conter um objeto JSON")
			}
			b, err := io.ReadAll(io.LimitReader(parte, limitePerfilOnboarding+1))
			if err != nil {
				return nil, erroMultipart(err)
			}
			if len(b) > limitePerfilOnboarding {
				return nil, falha(413, "o perfil excede o limite de 64 KiB")
			}
			var perfil Objeto
			if json.Unmarshal(b, &perfil) != nil || perfil == nil {
				return nil, falha(400, "perfil deve conter um objeto JSON")
			}
			dados[campo] = perfil
		case "curriculo":
			if !strings.EqualFold(filepath.Ext(parte.FileName()), ".pdf") {
				return nil, falha(400, "escolha um arquivo PDF")
			}
			b, err := io.ReadAll(io.LimitReader(parte, curriculos.MaxBytes+1))
			if err != nil {
				return nil, erroMultipart(err)
			}
			if int64(len(b)) > curriculos.MaxBytes {
				return nil, falha(413, "o currículo excede 20 MB")
			}
			dados[campo] = b
			dados["nomeArquivo"] = nomeDeExibicao(parte.FileName())
		default:
			return nil, falha(400, "campo desconhecido no primeiro acesso")
		}
	}
	return dados, nil
}

func listaOnboarding(x any, obrigatoria bool) bool {
	itens, ok := x.([]any)
	if !ok || (obrigatoria && len(itens) == 0) {
		return false
	}
	for _, item := range itens {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}

func validarOnboarding(p Objeto) error {
	if err := soCampos(p, "cargo", "stackPrincipal", "stackSecundaria", "filtros", "foco", "ingles", "pretensaoUSD", "faixaSalarial"); err != nil {
		return err
	}
	if err := exigirPerfilOk(p); err != nil {
		return err
	}
	for _, k := range []string{"cargo", "ingles", "foco"} {
		s, ok := p[k].(string)
		if !ok || strings.TrimSpace(s) == "" {
			return erroCampo("preencha o campo " + k)
		}
	}
	filtros := dominio.ObjetoDe(p["filtros"])
	if err := soCampos(filtros, "niveis", "modelo", "pularSe"); err != nil {
		return err
	}
	if !listaOnboarding(p["stackPrincipal"], true) || !listaOnboarding(filtros["niveis"], true) || !listaOnboarding(filtros["modelo"], true) {
		return erroCampo("escolha ao menos uma habilidade principal, um nível e um modelo de trabalho")
	}
	if (p["stackSecundaria"] != nil && !listaOnboarding(p["stackSecundaria"], false)) || (filtros["pularSe"] != nil && !listaOnboarding(filtros["pularSe"], false)) {
		return erroCampo("informe apenas texto nas habilidades e exclusões")
	}
	if valor, tem := p["pretensaoUSD"]; tem {
		n, valido := valor.(float64)
		if !valido || n < 0 {
			return erroCampo("a pretensão em USD deve ser um número maior ou igual a zero")
		}
	}
	return nil
}

func pdfDoOnboarding(dados Objeto) ([]byte, error) {
	pdf, _ := dados["curriculo"].([]byte)
	if int64(len(pdf)) > curriculos.MaxBytes {
		return nil, falha(413, "o currículo excede 20 MB")
	}
	if len(pdf) == 0 {
		return nil, falha(400, "o PDF não chegou ao app. Volte à etapa Currículo e selecione o arquivo novamente")
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, falha(400, "o arquivo recebido não tem o cabeçalho de um PDF")
	}
	if !bytes.Contains(pdf[max(0, len(pdf)-1024):], []byte("%%EOF")) {
		return nil, falha(400, "o PDF chegou incompleto. Selecione o arquivo novamente")
	}
	return pdf, nil
}

func (a *API) concluirOnboarding(ctx context.Context, autor string, dados Objeto) (Resposta, error) {
	if autor != "eu" {
		return Resposta{}, falha(403, msgForaDoAssistente)
	}
	ref, err := referenciaDoPedido(ctx, a, dados)
	if err != nil {
		return Resposta{}, err
	}
	var pdf []byte
	if ref == nil {
		pdf, err = pdfDoOnboarding(dados)
		if err != nil {
			return Resposta{}, err
		}
	}
	perfil, okPerfil := dados["perfil"].(map[string]any)
	if !okPerfil {
		return Resposta{}, falha(400, "o perfil de busca não chegou ao app. Revise as preferências e tente novamente")
	}
	if err := validarOnboarding(perfil); err != nil {
		return Resposta{}, err
	}
	if _, err := a.arm.BackupDiario(ctx, a.hoje()); err != nil {
		return Resposta{}, err
	}
	montar := func(atual Objeto, nome, exibicao string) (Objeto, error) {
		if err := a.verificarProvaDoPerfil(ctx); err != nil {
			return nil, err
		}
		novo := MesclarPerfil(atual, perfil)
		cvs := dominio.ObjetoDe(novo["curriculos"])
		if cvs == nil {
			cvs = Objeto{}
		}
		cvs["PT"] = "curriculos/" + nome
		novo["curriculos"] = cvs
		novo["curriculoExibicao"] = Objeto{"arquivo": cvs["PT"], "nome": exibicao}
		return novo, exigirPerfilOk(novo)
	}
	if ref != nil {
		assinatura, _ := json.Marshal([]any{ref.ArquivoID, ref.Revisao, perfil})
		ref.Assinatura = curriculos.SHA256(assinatura)
		err = a.arm.ConcluirRascunho(ctx, *ref, montar)
	} else {
		err = a.arm.ConcluirOnboarding(ctx, pdf, func(atual Objeto, nome string) (Objeto, error) {
			return montar(atual, nome, dominio.Texto(dados["nomeArquivo"]))
		})
	}
	if errors.Is(err, armazem.ErrRevisaoRascunho) {
		return Resposta{}, falha(409, armazem.ErrRevisaoRascunho.Error())
	}
	if errors.Is(err, armazem.ErrOnboardingConcluido) {
		return Resposta{}, falha(409, armazem.ErrOnboardingConcluido.Error())
	}
	if err != nil {
		return Resposta{}, err
	}
	return ok(Objeto{"ok": true, "onboardingConcluido": true})
}
