// A conta do app (Task 7): a situação de GET /api/v1/conta, o texto da faixa e a leitura que a casca refaz a cada
// 60 s e depois de um 402 (evento applyer:trava, disparado por chamarApi). Build sem a nuvem: {ativa: false} e o app
// fica como antes (ruling A11), sem porta, sem faixa e sem trava.
import { useCallback, useEffect, useRef, useState } from 'react';

// Versão dos termos que a tela mostra e manda ao criar a conta (a mesma de internal/conta.TermosVersao).
export const TERMOS_VERSAO = '0-rascunho';

// Evento de janela que chamarApi dispara quando uma chamada dá 402 (trava das ações); detail: o motivo.
export const EVENTO_TRAVA = 'applyer:trava';

export interface Situacao {
  ativa: boolean;
  entrada?: boolean;
  email?: string;
  estado?: string; // teste, assinante ou bloqueado
  teste_fim?: string | null;
  periodo_fim?: string | null;
  motivo?: string; // vazio: pode agir
  google?: string; // vazio, aguardando ou erro:<frase>
  google_motivo?: string;
  google_versao?: string;
  erro_cofre?: string;
  google_disponivel?: boolean; // false: o build não tem o login com Google (a tela esconde o botão)
  verificando?: boolean; // o app acabou de abrir e a primeira conferência com a nuvem ainda não acabou
}

// Texto neutro da faixa enquanto a primeira conferência não acaba (no lugar de "Sem conexão").
export const TEXTO_VERIFICANDO = 'Conferindo sua conta…';

// Cadência da leitura de GET /conta: 2 s conferindo ao abrir, 15 s sem internet, 60 s no resto.
export const cadenciaConta = (s: Situacao | null | undefined) =>
  s?.ativa && s.verificando ? 2_000 : s?.ativa && s.motivo === 'sem-internet' ? 15_000 : 60_000;

// As frases fixas da trava (as mesmas do 402 em internal/api/trava.go).
export const FRASES_TRAVA: Record<string, string> = {
  'teste-acabou': 'Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.',
  'sem-assinatura': 'Sua assinatura não está ativa. Você ainda vê tudo; para buscar e se candidatar, assine.',
  'sem-internet': 'Sem conexão: dá para ver, mas não para buscar ou se candidatar.',
  'sem-conta': 'Entre na sua conta para buscar e se candidatar.',
};

// Motivo desconhecido (ex.: licenca-vencida, que só a CLI e o MCP veem) fica com a frase de quem não assinou.
export const fraseDaTrava = (motivo: string) => FRASES_TRAVA[motivo] ?? FRASES_TRAVA['sem-assinatura'];

// podeAgir: sem conta neste build (ou antes de saber), tudo liberado como antes; o servidor confere de novo.
export const podeAgir = (s: Situacao | null | undefined) => !s?.ativa || !s.motivo;

// Texto da faixa no topo: quanto falta do teste, a frase da trava ou nada (assinante, sem conta no build, sem entrada).
export function textoFaixa(s: Situacao | null | undefined, agora: Date): string | null {
  if (!s?.ativa || !s.entrada) return null;
  if (s.verificando && s.motivo === 'sem-internet') return TEXTO_VERIFICANDO;
  if (s.motivo) return fraseDaTrava(s.motivo);
  if (s.estado === 'teste' && s.teste_fim) {
    const min = Math.max(0, Math.floor((new Date(s.teste_fim).getTime() - agora.getTime()) / 60_000));
    return `Seu teste acaba em ${Math.floor(min / 60)} h ${min % 60} min · Assinar`;
  }
  return null;
}

// useFoco (ruling A16): o foco é um filtro de ver. Com a trava ligada, muda só na tela (sem PATCH no perfil); quando
// o acesso volta, vale de novo o foco salvo, e mudar salva (salvar).
export function useFoco(salvo: string, liberado: boolean, salvar: (foco: string) => Promise<void>) {
  const [local, setLocal] = useState<string | null>(null);
  useEffect(() => { if (liberado) setLocal(null); }, [liberado]);
  const foco = (!liberado && local) || salvo;
  const mudarFoco = useCallback(async (novo: string) => {
    if (novo === foco) return;
    if (!liberado) { setLocal(novo); return; }
    await salvar(novo);
  }, [foco, liberado, salvar]);
  return { foco, mudarFoco };
}

type Api = (caminho: string, metodo?: string, corpo?: unknown) => Promise<any>;

// useConta: lê GET /api/v1/conta ao abrir, a cada 60 s (2 s conferindo ao abrir, 15 s sem internet; cadenciaConta)
// e a cada 402. atualizar ("Tentar de novo"): POST /conta/atualizar, que pede a licença à nuvem agora. undefined: ainda não leu. Se a leitura falhar
// (API sem a rota, erro de rede), a casca segue como antes ({ativa: false}): a trava de verdade é a do servidor.
export function useConta(api: Api) {
  const [situacao, setSituacao] = useState<Situacao | null | undefined>(undefined);
  const vivo = useRef(true);
  const ler = useCallback(async (atualizar: boolean) => {
    let s: Situacao;
    try {
      const r = atualizar ? await api('/api/v1/conta/atualizar', 'POST', {}) : await api('/api/v1/conta');
      s = r && typeof r === 'object' ? { ...r, ativa: r.ativa === true } : { ativa: false };
    } catch {
      s = { ativa: false };
    }
    if (vivo.current) setSituacao(antes => (antes?.ativa && !s.ativa ? antes : s)); // um erro passageiro não tira a trava
  }, [api]);
  const reler = useCallback(() => ler(false), [ler]);
  const atualizar = useCallback(() => ler(true), [ler]);
  const cadencia = cadenciaConta(situacao);
  useEffect(() => {
    vivo.current = true;
    const aoTravar = (ev: Event) => {
      const motivo = (ev as CustomEvent).detail;
      // Mostra a faixa na hora, com o motivo do 402; a leitura logo depois traz a situação inteira.
      if (typeof motivo === 'string' && motivo) setSituacao(antes => (antes?.ativa ? { ...antes, motivo } : antes));
      reler();
    };
    window.addEventListener(EVENTO_TRAVA, aoTravar);
    reler();
    return () => { vivo.current = false; window.removeEventListener(EVENTO_TRAVA, aoTravar); };
  }, [reler]);
  useEffect(() => {
    const t = setInterval(reler, cadencia);
    return () => clearInterval(t);
  }, [reler, cadencia]);
  return { situacao, reler, atualizar };
}
