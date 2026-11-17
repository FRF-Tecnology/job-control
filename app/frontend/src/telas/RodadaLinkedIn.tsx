// Rodada do LinkedIn (pedido da PO: "no LinkedIn, pode aplicar sem aprovação; cada rodada com um play"). Um bloco no
// topo da Lista do dia junta o que antes ficava espalhado (aprovar a lista, retomar o envio no painel da Fila, retomar a
// plataforma) num botão só:
//   - ▶ Começar: aprova de uma vez os pacotes do LinkedIn prontos (impressão conferida NESTA tela, dentro do limite de
//     hoje), retoma o envio e a plataforma. O clique é a aprovação do lote, uma vez por rodada.
//   - Ficam de fora (a pessoa revê): pacote com carta ou com resposta que não veio do perfil (regra do CyberSec no #102)
//     e pacote cuja impressão ainda não confere.
//   - ⏸ Pausar: o mesmo pausar do painel de envio.
// O envio é o de sempre (uma por vez, no limite do dia; sem Candidatura simplificada, para antes de enviar com
// "site externo"). Nada aqui clica no LinkedIn: o executor envia.
import { useCallback, useEffect, useRef, useState } from 'react';
import { ErroApi, useApp, useTrava } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import { plural } from '../util';
import type { Candidatura } from './Fila';
import type { Limite } from './LimiteDiario';
import { entrarNavegador, EVENTO_ENVIO, type EstadoEnvio } from './PainelEnvio';

const precisaRever = (c: Candidatura) => !!c.conteudo.mensagem || c.conteudo.respostas.some(r => r.origem !== 'respostasPadrao');
const MAXIMO_ITENS = 120; // o corpo do aprovar-lote aceita de 1 a 120

export type PlanoRodada = {
  prontas: Candidatura[]; // vão no ▶ Começar
  ficam: number; // prontas que passam do limite de hoje
  revisar: number; // preparadas que a pessoa revê (carta, resposta nova)
  conferindo: number; // impressão ainda não conferida nesta tela
  naFila: Candidatura[]; // aprovadas, esperando o envio
  enviadasHoje: number;
  paraVoce: number; // bloqueadas, falhas e incertas de hoje
  resta: number | null; // envios que ainda cabem hoje (null: sem limite lido)
};

export function planoDaRodada(candidaturas: Candidatura[], calculadas: Record<string, string>, limite: Limite | undefined, hoje: string): PlanoRodada {
  const doLinkedIn = candidaturas.filter(c => c.plataforma === 'linkedin');
  const naFila = doLinkedIn.filter(c => c.estado === 'aprovada');
  const preparadas = doLinkedIn.filter(c => c.estado === 'preparada').sort((a, b) => a.criadaEm.localeCompare(b.criadaEm));
  const conferidas = preparadas.filter(c => calculadas[c.id] !== undefined);
  const boas = conferidas.filter(c => calculadas[c.id] && calculadas[c.id] === c.impressao && !precisaRever(c));
  const resta = limite ? Math.max(0, limite.limite - limite.usadas - naFila.length) : null;
  const prontas = (resta === null ? boas : boas.slice(0, resta)).slice(0, MAXIMO_ITENS);
  const deHoje = (c: Candidatura) => (c.resultado?.em ?? c.bloqueio?.em ?? c.atualizadaEm ?? '').slice(0, 10) === hoje;
  return {
    prontas, ficam: boas.length - prontas.length,
    revisar: conferidas.length - boas.length, conferindo: preparadas.length - conferidas.length,
    naFila, resta,
    enviadasHoje: doLinkedIn.filter(c => c.estado === 'enviada' && deHoje(c)).length,
    paraVoce: doLinkedIn.filter(c => ['bloqueada', 'falhou', 'incerta'].includes(c.estado) && deHoje(c)).length,
  };
}

export function RodadaLinkedIn({ candidaturas, calculadas, limites, nome, atualizar }: {
  candidaturas: Candidatura[]; calculadas: Record<string, string>; limites: Record<string, Limite>;
  nome: (c: Candidatura) => string; atualizar: () => Promise<void>;
}) {
  const app = useApp();
  const trava = useTrava();
  const [envio, setEnvio] = useState<EstadoEnvio | null>(null);
  const [ocupado, setOcupado] = useState(false);
  const plano = planoDaRodada(candidaturas, calculadas, limites.linkedin, app.hoje);
  const linkedin = envio?.plataformas?.find(p => p.plataforma === 'linkedin');
  const enviando = envio?.enviandoAgora ? candidaturas.find(c => c.id === envio.enviandoAgora!.id && c.plataforma === 'linkedin') : undefined;
  const rodando = !!envio && !envio.pausado && linkedin?.estado !== 'pausada' && (plano.naFila.length > 0 || !!enviando);

  // O estado do envio: lido ao abrir, a cada gravação e, enquanto a rodada anda, a cada poucos segundos. Desfecho novo
  // relê a lista (a candidatura mudou de estado).
  const ultimo = useRef('');
  const ler = useCallback(async () => {
    try {
      const e: EstadoEnvio = await app.api('/api/v1/envio');
      setEnvio(e);
      const marca = `${e.enviandoAgora?.id ?? ''}|${e.desfechos.map(d => d.id + d.estado + d.em).join(',')}`;
      if (ultimo.current && marca !== ultimo.current) void atualizar();
      ultimo.current = marca;
    } catch { /* sem envio neste build ou fora do ar: o bloco diz isso */ }
  }, [app, atualizar]);
  useEffect(() => { void ler(); }, [app.estado]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const ouvir = (ev: Event) => setEnvio((ev as CustomEvent<EstadoEnvio>).detail);
    window.addEventListener(EVENTO_ENVIO, ouvir);
    return () => window.removeEventListener(EVENTO_ENVIO, ouvir);
  }, []);
  // Também com a janela de login aberta: quando a pessoa fecha, o botão volta sozinho.
  const login = !!envio?.login;
  useEffect(() => {
    if (!rodando && !login) return;
    const t = window.setInterval(() => void ler(), login ? 2000 : 4000);
    return () => window.clearInterval(t);
  }, [rodando, login, ler]);

  const doLinkedIn = candidaturas.some(c => c.plataforma === 'linkedin');
  if (!doLinkedIn && !envio?.ligado) return null;

  async function comecar() {
    if (ocupado) return;
    setOcupado(true);
    try {
      if (plano.prontas.length) {
        const r: { aprovadas?: string[]; recusadas?: { id: string }[] } = await app.api('/api/v1/candidaturas/aprovar-lote', 'POST',
          { itens: plano.prontas.map(c => ({ id: c.id, impressao: calculadas[c.id] })) });
        if (r.recusadas?.length) app.aviso(`${plural(r.recusadas.length, 'pacote ficou', 'pacotes ficaram')} de fora: mudou ou passou do limite. Veja na lista.`, true);
      }
      let e: EstadoEnvio | null = envio;
      if (e?.pausado) e = await app.api('/api/v1/envio/retomar', 'POST', {});
      if (e?.plataformas?.find(p => p.plataforma === 'linkedin')?.estado === 'pausada') e = await app.api('/api/v1/envio/plataformas/linkedin/retomar', 'POST', {});
      if (e) { setEnvio(e); window.dispatchEvent(new CustomEvent(EVENTO_ENVIO, { detail: e })); }
      app.aviso('Rodada do LinkedIn começou. O app envia uma por vez; você pode pausar quando quiser.');
      await atualizar();
    } catch (err) {
      app.aviso(err instanceof ErroApi && err.status === 403 ? 'Só você pode começar a rodada, por esta tela.' : 'Não deu para começar a rodada. Tente de novo.', true);
      await atualizar();
    } finally { setOcupado(false); }
  }
  async function pausar() {
    setOcupado(true);
    try {
      const e: EstadoEnvio = await app.api('/api/v1/envio/pausar', 'POST', {});
      setEnvio(e); window.dispatchEvent(new CustomEvent(EVENTO_ENVIO, { detail: e }));
      app.aviso(e.enviandoAgora ? 'Rodada pausada. A candidatura que está sendo enviada agora termina; nenhuma outra sai.' : 'Rodada pausada.');
    } catch { app.aviso('Não deu para pausar. Tente de novo.', true); }
    finally { setOcupado(false); }
  }
  async function entrar() {
    setOcupado(true);
    try { setEnvio(await entrarNavegador(app, { plataforma: 'linkedin' })); app.aviso('Abri o navegador do Applyer, separado do seu (pode parecer o mesmo, mas é outro perfil). Entre no LinkedIn nele e feche a janela quando terminar.'); }
    catch (e) { app.aviso((e as Error).message || 'Não deu para abrir a janela do LinkedIn.', true); }
    finally { setOcupado(false); }
  }

  const aComecar = plano.prontas.length > 0 || (plano.naFila.length > 0 && !rodando);
  const sessaoCaiu = linkedin?.estado === 'precisa-entrar';
  const paradaHoje = linkedin?.estado === 'parada-hoje';
  let aviso = '';
  if (!envio) aviso = 'Lendo o envio…';
  else if (!envio.ligado) aviso = 'Esta versão do app não envia sozinha: use "Aplicar no site" em cada vaga.';
  else if (envio.login) aviso = 'O navegador do Applyer está aberto para você entrar no LinkedIn. É um navegador separado do seu, com perfil próprio: entre nele e feche a janela. Se já fechou, espere alguns segundos.';
  else if (sessaoCaiu) aviso = 'A sessão do LinkedIn caiu. Entre de novo para a rodada continuar.';
  else if (paradaHoje) aviso = 'O LinkedIn ficou parado por hoje (aviso na conta, captcha ou limite de páginas). Volta amanhã.';
  else if (!aComecar && !rodando) aviso = plano.revisar || plano.conferindo ? 'Nenhum pacote pronto: os que estão na lista precisam de revisão.' : 'Adicione vagas do LinkedIn no Radar para montar a rodada.';

  return (
    <section className="j-rodada" aria-labelledby="rodada-linkedin">
      <div className="j-rodada-topo">
        <div>
          <h2 id="rodada-linkedin"><span className="j-rodada-marca" aria-hidden="true">in</span>Rodada do LinkedIn</h2>
          <p>{rodando ? 'Enviando, uma por vez. Pode pausar quando quiser.'
            : 'Um clique aprova os pacotes prontos e começa a enviar, uma por vez, dentro do limite de hoje.'}</p>
        </div>
        <div className="j-rodada-acoes">
          {rodando
            ? <button type="button" className="btn secondary" disabled={ocupado} onClick={() => void pausar()}><IconeUI nome="close" tamanho={15} />Pausar</button>
            : sessaoCaiu
              ? <button type="button" className="btn primary" {...trava(ocupado || !!envio?.login)} onClick={() => void entrar()}><IconeUI nome="login" tamanho={16} />Entrar no LinkedIn</button>
              : <button type="button" className="btn primary" {...trava(ocupado || !envio?.ligado || !!envio?.login || paradaHoje || !aComecar)} onClick={() => void comecar()}>
                <IconeUI nome="send" tamanho={16} />{plano.prontas.length ? `Começar rodada (${plano.prontas.length})` : plano.naFila.length ? 'Retomar rodada' : 'Começar rodada'}</button>}
          {!sessaoCaiu && envio?.ligado && <button type="button" className="btn quiet" {...trava(ocupado || !!envio.login)} onClick={() => void entrar()}>Entrar no LinkedIn</button>}
        </div>
      </div>
      <ul className="j-rodada-numeros" aria-label="Situação da rodada">
        <li><b>{plano.prontas.length}</b><span>{plano.prontas.length === 1 ? 'pronta para enviar' : 'prontas para enviar'}</span></li>
        <li><b>{plano.naFila.length}</b><span>na fila do envio</span></li>
        <li><b>{plano.enviadasHoje}</b><span>{plano.enviadasHoje === 1 ? 'enviada hoje' : 'enviadas hoje'}</span></li>
        <li><b>{plano.paraVoce}</b><span>{plano.paraVoce === 1 ? 'precisa de você' : 'precisam de você'}</span></li>
      </ul>
      {enviando && <p className="j-rodada-agora" role="status">Enviando agora: <strong>{nome(enviando)}</strong></p>}
      {plano.prontas.length > 0 && !rodando && <p className="j-rodada-lista">Vão nesta rodada: {plano.prontas.slice(0, 5).map(nome).join(' · ')}{plano.prontas.length > 5 ? ` e mais ${plano.prontas.length - 5}` : ''}.</p>}
      {(plano.revisar > 0 || plano.ficam > 0 || plano.paraVoce > 0) && <p className="j-rodada-nota">
        {[plano.revisar > 0 && `${plural(plano.revisar, 'pacote tem', 'pacotes têm')} carta ou resposta fora do perfil: revise e aprove na lista`,
          plano.ficam > 0 && `${plural(plano.ficam, 'fica', 'ficam')} para amanhã (limite de hoje${limites.linkedin ? `: ${limites.linkedin.limite}` : ''})`,
          plano.paraVoce > 0 && 'as que precisam de você estão na lista abaixo (sem Candidatura simplificada, use "Aplicar no site")',
        ].filter(Boolean).join('. ')}.</p>}
      {aviso && <p className="j-rodada-aviso" role="status">{aviso}</p>}
    </section>
  );
}
