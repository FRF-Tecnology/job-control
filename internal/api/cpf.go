package api

import (
	"context"
	"log"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// CPF da pessoa (decisão da PO; docs/propostas/cpf-no-preencher.md). Só da tela:
//   - GET /perfil/cpf: {salvo, mascarado} (só o miolo: ***.456.789-**);
//   - PUT /perfil/cpf {cpf}: confere os dígitos e grava no cofre do sistema;
//   - PATCH /perfil/cpf {automatico}: com true, o CPF salvo vai junto no "Preencher" de site brasileiro, sem a pergunta
//     à parte (a pessoa já confere o domínio antes de preencher). É uma preferência, no banco; o CPF continua no cofre;
//   - DELETE /perfil/cpf: apaga do cofre e desliga o automático.
// O CPF nunca vai para o banco, o log, a IA ou o assistente; a resposta nunca traz o número inteiro.

const (
	chaveCofreCPF    = "cpf"
	chaveMetaCPFAuto = "cpf-automatico"
)

func (a *API) rotaCPF(ctx context.Context, m string, c Objeto, autor string) (Resposta, error) {
	if autor != "eu" || a.canal != "" {
		return Resposta{}, falha(403, msgForaDoAssistente)
	}
	switch m {
	case "GET":
		return a.estadoCPF(ctx)
	case "PATCH":
		if err := soCampos(c, "automatico"); err != nil {
			return Resposta{}, err
		}
		auto, valido := c["automatico"].(bool)
		if !valido {
			return Resposta{}, erroCampo("automatico: true ou false")
		}
		if auto {
			if d, err := a.cpfSalvo(); err != nil {
				return Resposta{}, err
			} else if d == "" {
				return Resposta{}, &dominio.ErroConflito{Msg: "salve o CPF antes", Motivo: "sem-cpf"}
			}
		}
		if err := a.arm.GravarMeta(ctx, chaveMetaCPFAuto, auto); err != nil {
			return Resposta{}, err
		}
		log.Printf("cpf: automático %t", auto)
		return a.estadoCPF(ctx)
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
		return a.estadoCPF(ctx)
	case "DELETE":
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		if err := a.cofre.Apagar(chaveCofreCPF); err != nil {
			return Resposta{}, err
		}
		if err := a.arm.GravarMeta(ctx, chaveMetaCPFAuto, false); err != nil {
			return Resposta{}, err
		}
		log.Printf("cpf: apagado do cofre")
		return ok(Objeto{"salvo": false, "automatico": false})
	}
	return Resposta{}, falha(404, "rota não encontrada")
}

func (a *API) estadoCPF(ctx context.Context) (Resposta, error) {
	d, err := a.cpfSalvo()
	if err != nil {
		return Resposta{}, err
	}
	if d == "" {
		return ok(Objeto{"salvo": false, "automatico": false})
	}
	auto, err := a.cpfAutomatico(ctx)
	if err != nil {
		return Resposta{}, err
	}
	return ok(Objeto{"salvo": true, "mascarado": dominio.MascararCPF(d), "automatico": auto})
}

// cpfAutomatico: a pessoa ligou "Preencher o CPF sem perguntar".
func (a *API) cpfAutomatico(ctx context.Context) (bool, error) {
	var auto bool
	if _, err := a.arm.LerMeta(ctx, chaveMetaCPFAuto, &auto); err != nil {
		return false, err
	}
	return auto, nil
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
