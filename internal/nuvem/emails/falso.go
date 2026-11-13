package emails

// Garante em compilação que os dois enviadores cumprem a interface.
var (
	_ Enviador = (*Falso)(nil)
	_ Enviador = SMTP{}
)
