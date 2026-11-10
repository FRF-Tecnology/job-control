// "Ao vivo" (jc-a2y.6.9.1; contrato 0.8.0, docs/propostas/ao-vivo.md): a imagem do navegador do envio dentro do app,
// SÓ PARA VER. Abrir liga (POST {ligado:true}), aberto lê a cada segundo, fechar (ou sair da tela) desliga. O quadro só
// entra num <img src> e só se for um data:image/jpeg em base64; a etapa vem de uma lista fechada e vira frase fixa.
// Não há clique, tecla nem nada que controle o envio. Sem leitura por 10 s o app desliga sozinho; se a leitura voltar
// com ligado false e o painel aberto, a tela liga de novo.
import { useEffect, useRef, useState } from 'react';
import { ErroApi, useApp, type App } from '../app/contexto';

export type Quadro = { imagem: string; em: string; largura: number; altura: number };
export type EstadoAoVivo = { ligado: boolean; etapa: string; candidatura: string | null; quadro: Quadro | null };

export const ETAPA: Record<string, string> = {
  desligado: 'Ao vivo desligado.',
  'sem-envio': 'Nada sendo enviado agora.',
  abrindo: 'Abrindo a página da vaga…',
  preenchendo: 'Preenchendo o formulário…',
  revisando: 'Revisando antes de enviar…',
  enviando: 'Enviando…',
  confirmada: 'Enviada: a plataforma confirmou.',
  bloqueada: 'Parou: precisa de você. Veja na fila o que falta.',
  falhou: 'Falhou: nada foi enviado.',
  incerta: 'Incerta: confira na plataforma se foi enviada.',
  'login-aberto': 'A janela de login está aberta: o envio espera.',
  pausado: 'O envio está pausado.',
};
// Só a imagem que o contrato permite (e no tamanho máximo dele) vai para o src.
const IMAGEM_OK = /^data:image\/jpeg;base64,[A-Za-z0-9+/=]+$/;
export const imagemValida = (q: Quadro | null): q is Quadro => !!q && typeof q.imagem === 'string' && q.imagem.length <= 400_000 && IMAGEM_OK.test(q.imagem);
const ROTA = '/api/v1/envio/ao-vivo';

// O painel da Fila avisa quando abre e fecha: o navegador flutuante (NavegadorFlutuante.tsx) some enquanto o painel
// mostra a mesma imagem, para não haver dois lugares ligando e desligando o ao vivo ao mesmo tempo.
export const EVENTO_PAINEL_AO_VIVO = 'applyer:ao-vivo-painel';

// A leitura do ao vivo, usada pelo painel da Fila e pelo navegador flutuante. Com `aberto`, liga e lê a cada segundo;
// ao fechar (ou desmontar), desliga e esquece o estado. `fechar` é chamado quando não adianta insistir (404 e 403).
export function useAoVivo(api: App['api'], aberto: boolean, fechar: () => void) {
  const [estado, setEstado] = useState<EstadoAoVivo | null>(null);
  const [erro, setErro] = useState<string | null>(null);
  const abertoRef = useRef(false);
  abertoRef.current = aberto;
  const fecharRef = useRef(fechar);
  fecharRef.current = fechar;

  useEffect(() => {
    if (!aberto) return;
    let vivo = true;
    let religando = false;
    // Erros em frase fixa; 404 e 403 fecham (não adianta insistir).
    const falhar = (e: unknown) => {
      if (e instanceof ErroApi && e.status === 404) { setErro('Este app não traz o envio: não há o que ver.'); fecharRef.current(); return; }
      if (e instanceof ErroApi && e.status === 403) { setErro('O ao vivo só abre na janela do app.'); fecharRef.current(); return; }
      setErro('Não deu para ler o ao vivo agora. Tentando de novo…');
    };
    const ligar = async () => { const e: EstadoAoVivo = await api(ROTA, 'POST', { ligado: true }); if (vivo) { setEstado(e); setErro(null); } };
    const ler = async () => {
      if (document.visibilityState !== 'visible') return; // escondida: não lê (o app desliga sozinho em 10 s)
      try {
        const e: EstadoAoVivo = await api(ROTA);
        if (!vivo) return;
        setEstado(e); setErro(null);
        if (!e.ligado && abertoRef.current && !religando) { religando = true; try { await ligar(); } finally { religando = false; } }
      } catch (e) { if (vivo) falhar(e); }
    };
    setErro(null); setEstado(null);
    ligar().catch(e => { if (vivo) falhar(e); });
    const t = window.setInterval(ler, 1000);
    return () => {
      vivo = false; window.clearInterval(t);
      setEstado(null); // o quadro não fica guardado na tela depois de fechar
      api(ROTA, 'POST', { ligado: false }).catch(() => { /* sem leitura, o app desliga sozinho em 10 s */ });
    };
  }, [aberto]); // eslint-disable-line react-hooks/exhaustive-deps

  return { estado, erro, limparErro: () => setErro(null) };
}

export function AoVivo({ nome }: { nome: (id: string) => string }) {
  const app = useApp();
  const [aberto, setAberto] = useState(false);
  const { estado, erro, limparErro } = useAoVivo(app.api, aberto, () => setAberto(false));
  useEffect(() => {
    window.dispatchEvent(new CustomEvent(EVENTO_PAINEL_AO_VIVO, { detail: aberto }));
    return () => { if (aberto) window.dispatchEvent(new CustomEvent(EVENTO_PAINEL_AO_VIVO, { detail: false })); };
  }, [aberto]);

  const abrir = () => { limparErro(); setAberto(true); };
  const fechar = () => setAberto(false);
  const etapa = estado ? (ETAPA[estado.etapa] ?? ETAPA['sem-envio']) : 'Ligando…';
  const q = estado && imagemValida(estado.quadro) ? estado.quadro : null;

  return (
    <div className="ao-vivo">
      <div className="ao-vivo-topo">
        <div>
          <h3 id="ao-vivo-titulo">Ao vivo</h3>
          <p className="pequeno mudo">Mostra aqui o que o envio está fazendo, só para ver: daqui não dá para clicar nem digitar nada.</p>
        </div>
        <button className="botao-fantasma p" type="button" aria-expanded={aberto} aria-controls="ao-vivo-painel" onClick={aberto ? fechar : abrir}>
          {aberto ? 'Fechar o ao vivo' : 'Ver ao vivo'}
        </button>
      </div>
      {erro && <p className="pequeno erro-resposta" role="alert">{erro}</p>}
      {aberto && (
        <div id="ao-vivo-painel" className="ao-vivo-painel" aria-labelledby="ao-vivo-titulo">
          <div>
            <p role="status" className="ao-vivo-etapa">{etapa}</p>
            {estado?.candidatura && estado.etapa !== 'sem-envio' && <p className="pequeno mudo ao-vivo-cand">{nome(estado.candidatura)}</p>}
          </div>
          {q
            ? <img className="ao-vivo-quadro" src={q.imagem} width={Math.min(q.largura, 1280)} height={Math.min(q.altura, 800)}
                alt={estado?.candidatura ? `Tela do envio de ${nome(estado.candidatura)}` : 'Tela do envio'} draggable={false} />
            : <div className="ao-vivo-vazio pequeno mudo">{estado?.etapa === 'login-aberto' || estado?.etapa === 'pausado' ? 'Sem imagem enquanto isso.' : 'A imagem aparece quando uma candidatura estiver sendo enviada.'}</div>}
        </div>
      )}
    </div>
  );
}
