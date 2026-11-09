//go:build !windows

package navegador

import "github.com/chromedp/chromedp"

// opcoesDoSistema: fora do Windows, fica o padrão do chromedp (no Linux, ele mata o navegador quando o app sai;
// trocar o ModifyCmdFunc tiraria isso). A prioridade baixa no Linux e no macOS fica para depois da .6.5.
func opcoesDoSistema() []chromedp.ExecAllocatorOption { return nil }
