package nuvem

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ipDe é o IP do cliente usado nos limites por IP e nos logs.
func (s *Servidor) ipDe(r *http.Request) string { return ipDoCliente(r, s.cfg.ProxyConfiavel) }

// ipDoCliente devolve o IP do cliente. Por padrão é o da conexão (RemoteAddr). Só se a conexão vem de um proxy
// confiável (APPLYER_PROXY_CONFIAVEL) os cabeçalhos valem: CF-Connecting-IP, se for um IP válido; senão o
// X-Forwarded-For lido da direita para a esquerda, até o primeiro IP que não é de proxy confiável (o que está à
// esquerda disso o cliente pode ter forjado). Cabeçalho inválido ou sem resposta: o IP do próprio proxy.
func ipDoCliente(r *http.Request, confiaveis []netip.Prefix) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	remoto, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	remoto = remoto.Unmap().WithZone("")
	if !confia(confiaveis, remoto) {
		return remoto.String()
	}
	if ip, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))); err == nil {
		return ip.Unmap().WithZone("").String()
	}
	var cadeia []string
	for _, linha := range r.Header.Values("X-Forwarded-For") {
		cadeia = append(cadeia, strings.Split(linha, ",")...)
	}
	for i := len(cadeia) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(cadeia[i]))
		if err != nil {
			break // entrada ilegível: não dá para saber quem veio daqui para a esquerda
		}
		ip = ip.Unmap().WithZone("")
		if !confia(confiaveis, ip) {
			return ip.String()
		}
	}
	return remoto.String()
}

func confia(confiaveis []netip.Prefix, ip netip.Addr) bool {
	for _, p := range confiaveis {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// lerProxies lê APPLYER_PROXY_CONFIAVEL: CIDRs ou IPs soltos separados por vírgula.
func lerProxies(valor string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, item := range strings.Split(valor, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if p, err := netip.ParsePrefix(item); err == nil {
			out = append(out, p.Masked())
			continue
		}
		ip, err := netip.ParseAddr(item)
		if err != nil {
			return nil, errProxy
		}
		out = append(out, netip.PrefixFrom(ip, ip.BitLen()))
	}
	return out, nil
}
