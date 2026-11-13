package conta

import "github.com/denisbrodbeck/machineid"

// Dispositivo devolve o identificador deste computador: HMAC-SHA256 em hex (64 caracteres)
// do id da máquina, sem expor o id original.
func Dispositivo() (string, error) {
	return machineid.ProtectedID("applyer")
}
