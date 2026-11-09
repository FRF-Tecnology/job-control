package segredos

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/config"
)

// ServicoDaPasta é o nome no cofre do sistema das chaves de uma pasta de dados (jc-a2y.65). Antes, todas as pastas
// usavam o mesmo nome, e "apagar meus dados" numa pasta de teste (CENTRAL_PASTA) apagava as chaves da pessoa.
//   - pasta padrão (a de sempre, também quando CENTRAL_PASTA aponta para ela): o nome de sempre, sem migração;
//   - outra pasta: o nome de sempre + " - pasta " + 12 caracteres do sha256 do caminho (o caminho não vai para o
//     cofre, que outros programas do usuário listam);
//   - banco em memória (só testes): um nome próprio, para um teste nunca tocar nas chaves de verdade.
func ServicoDaPasta(pasta string) string {
	if pasta == "" {
		return servico + " - memoria"
	}
	if padrao, err := config.PastaPadrao(); err == nil && config.MesmaPasta(pasta, padrao) {
		return servico
	}
	caminho := filepath.Clean(pasta)
	if abs, err := filepath.Abs(caminho); err == nil {
		caminho = abs
	}
	if runtime.GOOS == "windows" {
		caminho = strings.ToLower(caminho) // o Windows não diferencia maiúsculas: a mesma pasta, o mesmo nome
	}
	h := sha256.Sum256([]byte(caminho))
	return servico + " - pasta " + hex.EncodeToString(h[:])[:12]
}
