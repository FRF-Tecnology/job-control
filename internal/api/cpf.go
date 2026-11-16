package api

import (
	"log"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// CPF da pessoa (decisão da PO; docs/propostas/cpf-no-preencher.md). Só da tela:
//   - GET /perfil/cpf: {salvo, mascarado} (só o miolo: ***.456.789-**);
//   - PUT /perfil/cpf {cpf}: confere os dígitos e grava no cofre do sistema;
//   - DELETE /perfil/cpf: apaga do cofre.
// O CPF nunca vai para o banco, o log, a IA ou o assistente; a resposta nunca traz o número inteiro.

const chaveCofreCPF = "cpf"

func (a *API) rotaCPF(m string, c Objeto, autor string) (Resposta, error) {
	if autor != "eu" || a.canal != "" {
		return Resposta{}, falha(403, msgForaDoAssistente)
	}
	switch m {
	case "GET":
		return a.estadoCPF()
	case "PUT":
		if err := soCampos(c, "cpf"); err != nil {
			return Resposta{}, err
		}
		texto, _ := c["cpf"].(string)
		d, valido := dominio.CPFValido(texto)
		if !valido {
			return Resposta{}, erroCampo("cpf: confira os 11 números do CPF")
		}
		if err := a.cofre.Gravar(chaveCofreCPF, d); err != nil {
			log.Printf("cpf: não gravou (%s)", tipoDoErro(err))
			return Resposta{}, err
		}
		log.Printf("cpf: salvo no cofre")
		return ok(Objeto{"salvo": true, "mascarado": dominio.MascararCPF(d)})
	case "DELETE":
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		if err := a.cofre.Apagar(chaveCofreCPF); err != nil {
			return Resposta{}, err
		}
		log.Printf("cpf: apagado do cofre")
		return ok(Objeto{"salvo": false})
	}
	return Resposta{}, falha(404, "rota não encontrada")
}

func (a *API) estadoCPF() (Resposta, error) {
	d, err := a.cpfSalvo()
	if err != nil {
		return Resposta{}, err
	}
	if d == "" {
		return ok(Objeto{"salvo": false})
	}
	return ok(Objeto{"salvo": true, "mascarado": dominio.MascararCPF(d)})
}

// cpfSalvo: os 11 dígitos do cofre, ou "" (sem CPF, ou um valor que não é CPF).
func (a *API) cpfSalvo() (string, error) {
	s, err := a.cofre.Ler(chaveCofreCPF)
	if err != nil || s == "" {
		return "", err
	}
	d, valido := dominio.CPFValido(s)
	if !valido {
		return "", nil
	}
	return d, nil
}
