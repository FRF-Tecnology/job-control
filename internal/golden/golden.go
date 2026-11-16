// Package golden lê e regrava os arquivos testdata/golden*.json dos testes.
//
// Os golden começaram como retrato do Node, mas agora são do Go: quando uma regra muda de propósito, rode o teste
// do pacote com -atualizar e confira o diff no git antes de commitar:
//
//	go test ./internal/dominio -run Golden -atualizar
//	go test ./internal/ia -atualizar
//	go test ./internal/api -run Golden -atualizar
//
// A regravação mantém a ordem das chaves e o formato do arquivo (1 espaço de recuo), e os testes só trocam os
// valores que mudaram, para o diff mostrar só o que mudou.
package golden

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// Atualizar liga a regravação dos golden com a saída atual do Go (go test ... -atualizar).
var Atualizar = flag.Bool("atualizar", false, "regrava os testdata/golden*.json com a saída atual do Go")

// Obj é um objeto JSON que lembra a ordem das chaves.
type Obj struct {
	Chaves  []string
	Valores map[string]any
}

// NovoObj monta um objeto com os pares chave, valor na ordem dada.
func NovoObj(pares ...any) *Obj {
	o := &Obj{Valores: map[string]any{}}
	for i := 0; i+1 < len(pares); i += 2 {
		o.Set(pares[i].(string), pares[i+1])
	}
	return o
}

func (o *Obj) Get(k string) any { return o.Valores[k] }

// Set troca o valor mantendo a posição da chave; chave nova vai para o fim.
func (o *Obj) Set(k string, v any) {
	if _, tem := o.Valores[k]; !tem {
		o.Chaves = append(o.Chaves, k)
	}
	o.Valores[k] = v
}

// Trocar é o Set para a saída nova do Go: as chaves que já existiam no valor antigo ficam na mesma ordem
// (o JSON do Go sai em ordem alfabética), para o diff mostrar só o valor que mudou.
func (o *Obj) Trocar(k string, novo any) {
	o.Set(k, ordenarComo(Valor(novo), o.Valores[k]))
}

func ordenarComo(novo, antigo any) any {
	switch n := novo.(type) {
	case *Obj:
		a, ok := antigo.(*Obj)
		if !ok {
			return n
		}
		r := NovoObj()
		for _, k := range a.Chaves {
			if v, tem := n.Valores[k]; tem {
				r.Set(k, ordenarComo(v, a.Valores[k]))
			}
		}
		for _, k := range n.Chaves {
			if _, tem := r.Valores[k]; !tem {
				r.Set(k, n.Valores[k])
			}
		}
		return r
	case []any:
		a, _ := antigo.([]any)
		for i := range n {
			if i < len(a) {
				n[i] = ordenarComo(n[i], a[i])
			}
		}
		return n
	}
	return novo
}

func (o *Obj) Del(k string) {
	if _, tem := o.Valores[k]; !tem {
		return
	}
	delete(o.Valores, k)
	for i, c := range o.Chaves {
		if c == k {
			o.Chaves = append(o.Chaves[:i:i], o.Chaves[i+1:]...)
			break
		}
	}
}

// Lista devolve o valor da chave como lista.
func (o *Obj) Lista(k string) []any { l, _ := o.Valores[k].([]any); return l }

// Item devolve o i-ésimo objeto da lista da chave.
func (o *Obj) Item(k string, i int) *Obj { return o.Lista(k)[i].(*Obj) }

// Ler lê um golden mantendo a ordem das chaves.
func Ler(t testing.TB, caminho string) *Obj {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	v, err := decodificar(b)
	if err != nil {
		t.Fatalf("%s: %v", caminho, err)
	}
	return v.(*Obj)
}

// Gravar regrava o golden no mesmo formato em que foi lido.
func Gravar(t testing.TB, caminho string, raiz *Obj) {
	t.Helper()
	if err := GravarArquivo(caminho, raiz); err != nil {
		t.Fatal(err)
	}
	t.Logf("golden regravado: %s (confira o diff no git)", caminho)
}

// GravarArquivo é o Gravar para quem não tem um *testing.T (TestMain).
func GravarArquivo(caminho string, raiz *Obj) error {
	var buf bytes.Buffer
	if err := escrever(&buf, raiz, 0); err != nil {
		return err
	}
	buf.WriteByte('\n')
	return os.WriteFile(caminho, buf.Bytes(), 0o644)
}

// LerArquivo é o Ler para quem não tem um *testing.T.
func LerArquivo(caminho string) (*Obj, error) {
	b, err := os.ReadFile(caminho)
	if err != nil {
		return nil, err
	}
	v, err := decodificar(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", caminho, err)
	}
	return v.(*Obj), nil
}

// Valor converte uma saída do Go para o formato do golden (passa por JSON).
func Valor(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	r, err := decodificar(b)
	if err != nil {
		panic(err)
	}
	return r
}

func decodificar(b []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, err := lerValor(d)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("conteúdo depois do JSON")
	}
	return v, nil
}

func lerValor(d *json.Decoder) (any, error) {
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		o := NovoObj()
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			v, err := lerValor(d)
			if err != nil {
				return nil, err
			}
			o.Set(k.(string), v)
		}
		_, err := d.Token()
		return o, err
	case json.Delim('['):
		l := []any{}
		for d.More() {
			v, err := lerValor(d)
			if err != nil {
				return nil, err
			}
			l = append(l, v)
		}
		_, err := d.Token()
		return l, err
	}
	return tok, nil
}

// escrever segue o JSON.stringify(v, null, 1) do Node, que gravou os arquivos.
func escrever(w *bytes.Buffer, v any, nivel int) error {
	recuo := func(n int) { w.WriteByte('\n'); w.WriteString(strings.Repeat(" ", n)) }
	switch x := v.(type) {
	case *Obj:
		if len(x.Chaves) == 0 {
			w.WriteString("{}")
			return nil
		}
		w.WriteByte('{')
		for i, k := range x.Chaves {
			if i > 0 {
				w.WriteByte(',')
			}
			recuo(nivel + 1)
			escreverTexto(w, k)
			w.WriteString(": ")
			if err := escrever(w, x.Valores[k], nivel+1); err != nil {
				return err
			}
		}
		recuo(nivel)
		w.WriteByte('}')
	case []any:
		if len(x) == 0 {
			w.WriteString("[]")
			return nil
		}
		w.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				w.WriteByte(',')
			}
			recuo(nivel + 1)
			if err := escrever(w, item, nivel+1); err != nil {
				return err
			}
		}
		recuo(nivel)
		w.WriteByte(']')
	case string:
		escreverTexto(w, x)
	case json.Number:
		w.WriteString(x.String())
	case bool:
		fmt.Fprint(w, x)
	case nil:
		w.WriteString("null")
	default:
		return fmt.Errorf("tipo fora do golden: %T (use golden.Valor)", v)
	}
	return nil
}

func escreverTexto(w *bytes.Buffer, s string) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.Encode(s)
	t := strings.TrimSuffix(b.String(), "\n")
	// O Node não escapa U+2028 e U+2029.
	t = strings.ReplaceAll(t, ` `, " ")
	t = strings.ReplaceAll(t, ` `, " ")
	w.WriteString(t)
}
