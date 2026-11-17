package conta

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

const arquivoLicencaNome = "licenca.json"

// desvioMaximo limita a diferença guardada entre o relógio da nuvem e o do PC. Um PC mais desajustado que isso
// fica com a licença vencida até o relógio ser acertado.
const desvioMaximo = 24 * time.Hour

// toleranciaAtras: quanto o relógio corrigido pode ficar antes de emitida_em. Mais que isso quer dizer que o
// relógio do PC voltou ou que o desvio do arquivo foi editado, e a licença deixa de valer.
const toleranciaAtras = 5 * time.Minute

// arquivoLicenca é o licenca.json: o documento assinado como veio da nuvem e o desvio do relógio
// (emitida_em − relógio do PC no recebimento). Nenhum token vai para este arquivo.
type arquivoLicenca struct {
	Documento licenca.Documento `json:"documento"`
	DesvioNs  int64             `json:"desvio_ns"`
}

func limitarDesvio(d time.Duration) time.Duration {
	return max(-desvioMaximo, min(desvioMaximo, d))
}

// vale diz se a licença (já com a assinatura conferida) vale agora pelo relógio do PC corrigido pelo desvio.
// Risco que fica: o PC é de confiança. Quem edita o desvio ou atrasa o relógio ganha no máximo os 30 min da
// licença mais a tolerância; o estado é assinado e não muda.
func vale(l licenca.Licenca, agora time.Time, desvio time.Duration) bool {
	t := agora.Add(limitarDesvio(desvio))
	return !t.After(l.ValidaAte) && !t.Before(l.EmitidaEm.Add(-toleranciaAtras))
}

// GravarLicenca grava o documento em licenca.json na pasta, sem desvio de relógio conhecido.
func GravarLicenca(pasta string, d licenca.Documento) error {
	return gravarLicenca(pasta, d, 0)
}

// gravarLicenca grava de forma atômica (temporário + rename), com permissão 0600.
func gravarLicenca(pasta string, d licenca.Documento, desvio time.Duration) error {
	bruto, err := json.Marshal(arquivoLicenca{Documento: d, DesvioNs: int64(limitarDesvio(desvio))})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(pasta, "licenca-*.tmp")
	if err != nil {
		return err
	}
	nome := tmp.Name()
	falhou := func(err error) error {
		tmp.Close()
		os.Remove(nome)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		return falhou(err)
	}
	if _, err := tmp.Write(bruto); err != nil {
		return falhou(err)
	}
	if err := tmp.Sync(); err != nil {
		return falhou(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(nome)
		return err
	}
	if err := os.Rename(nome, filepath.Join(pasta, arquivoLicencaNome)); err != nil {
		os.Remove(nome)
		return err
	}
	return nil
}

// lerArquivoLicenca lê licenca.json e confere só a assinatura (a validade fica para quem chama).
func lerArquivoLicenca(pasta string, pub ed25519.PublicKey) (licenca.Licenca, time.Duration, licenca.Documento, error) {
	bruto, err := os.ReadFile(filepath.Join(pasta, arquivoLicencaNome))
	if err != nil {
		return licenca.Licenca{}, 0, licenca.Documento{}, err
	}
	var a arquivoLicenca
	if err := json.Unmarshal(bruto, &a); err != nil {
		return licenca.Licenca{}, 0, licenca.Documento{}, licenca.ErrAssinatura
	}
	l, err := conferirAssinatura(pub, a.Documento)
	if err != nil {
		return licenca.Licenca{}, 0, licenca.Documento{}, err
	}
	return l, limitarDesvio(time.Duration(a.DesvioNs)), a.Documento, nil
}

// conferirAssinatura confere a assinatura sem olhar a validade (o tempo zero nunca passa de valida_ate).
func conferirAssinatura(pub ed25519.PublicKey, d licenca.Documento) (licenca.Licenca, error) {
	return licenca.Conferir(pub, d, time.Time{})
}

// LerLicencaDoDisco lê licenca.json e confere assinatura e validade em agora (relógio do PC) corrigido pelo
// desvio guardado. Sem arquivo: erro com fs.ErrNotExist; editado: licenca.ErrAssinatura; vencida:
// licenca.ErrVencida. Usada pela CLI e pelo MCP.
func LerLicencaDoDisco(pasta string, pub ed25519.PublicKey, agora time.Time) (licenca.Licenca, error) {
	_, desvio, doc, err := lerArquivoLicenca(pasta, pub)
	if err != nil {
		return licenca.Licenca{}, err
	}
	l, err := licenca.Conferir(pub, doc, agora.Add(desvio))
	if err != nil {
		return licenca.Licenca{}, err
	}
	if !vale(l, agora, desvio) {
		return licenca.Licenca{}, licenca.ErrVencida
	}
	return l, nil
}

func apagarLicenca(pasta string) error {
	if err := os.Remove(filepath.Join(pasta, arquivoLicencaNome)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
