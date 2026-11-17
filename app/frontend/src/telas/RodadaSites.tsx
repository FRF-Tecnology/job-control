// Rodada dos sites das empresas (pedido da PO: "cada rodada com um play"). As vagas da Lista do dia que vão para o
// site da empresa (sem Candidatura simplificada, ou de outro site) entram numa fila; o ▶ passa por elas uma por vez:
//   1. reserva a vaga para a pessoa ("Aplicar no site", iniciar-manual) e abre na janela do app, que clica em
//      Candidatar-se (abrir-para-preencher; a vaga seguinte reaproveita a janela e fecha as abas da anterior);
//   2. espera o site da empresa abrir e mostra o domínio: a pessoa confere e clica em Preencher (a única conferência
//      antes de os dados entrarem num site de terceiro; com o CPF automático, ele vai junto);
//   3. a pessoa revisa, completa e ENVIA no site (o app nunca envia em site de terceiro) e clica em "Já enviei", ou pula.
// Nada é enviado pelo app aqui.
import { useEffect, useRef, useState } from 'react';
import { ErroApi, useApp, useTrava } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import { foiEnviada } from '../dominio/modelo';
import { encode, plural } from '../util';
import type { Candidatura } from './Fila';
import { explicarPreencher, resumoPreencher } from './Radar';

const COM_ENVIO = new Set(['linkedin', 'gupy', 'inhire', 'formulario']); // plataformas que o app envia sozinho
type Vaga = { id: string; empresa: string; titulo: string; etapa: string; aplicacaoManual?: boolean; envio?: { data?: string } };

// As vagas da rodada: reservadas para a pessoa, paradas pelo envio porque levam ao site da empresa, ou de um site
// que o app não envia (Jobicy, Himalayas, sites próprios).
export function vagasDaRodadaSites(vagas: Vaga[], candidaturas: Candidatura[]): { vaga: Vaga; c: Candidatura }[] {
  const ativa = new Map(candidaturas.filter(c => c.estado !== 'cancelada').map(c => [c.vagaId, c]));
  return vagas.flatMap(v => {
    const c = ativa.get(v.id);
    if (!c || foiEnviada(v)) return [];
    const reservada = v.etapa === 'aplicar' && v.aplicacaoManual && c.estado === 'bloqueada';
    const siteExterno = ['aplicar', 'bloqueada'].includes(v.etapa) && c.estado === 'bloqueada' && c.bloqueio?.motivo === 'site-externo';
    const semEnvio = v.etapa === 'aplicar' && ['preparada', 'aprovada'].includes(c.estado) && !COM_ENVIO.has(c.plataforma);
    return reservada || siteExterno || semEnvio ? [{ vaga: v, c }] : [];
  }).sort((a, b) => a.c.criadaEm.localeCompare(b.c.criadaEm));
}

type Passo = 'abrindo' | 'esperando' | 'conferir' | 'preenchendo' | 'feito' | 'erro';

export function RodadaSites({ candidaturas, atualizar }: { candidaturas: Candidatura[]; atualizar: () => Promise<void> }) {
  const app = useApp();
  const trava = useTrava();
  const disponiveis = vagasDaRodadaSites(app.estado.vagas ?? [], candidaturas);
  const [fila, setFila] = useState<Vaga[] | null>(null); // a rodada em curso (fixa ao começar)
  const [i, setI] = useState(0);
  const [passo, setPasso] = useState<Passo>('abrindo');
  const [dominio, setDominio] = useState('');
  const [texto, setTexto] = useState('');
  const [cpf, setCPF] = useState<{ salvo: boolean } | null>(null);
  const [conta, setConta] = useState({ enviadas: 0, puladas: 0 });
  const [fim, setFim] = useState(false);
  const vez = useRef(0); // muda a cada vaga: a espera da vaga anterior para sozinha

  const atual = fila?.[i];
  const rota = (v: Vaga, acao: string) => `/api/v1/vagas/${encode(v.id)}/${acao}`;

  async function abrir(v: Vaga) {
    const minha = ++vez.current;
    setPasso('abrindo'); setTexto(''); setDominio(''); setCPF(null);
    try {
      if (!v.aplicacaoManual) await app.api(rota(v, 'iniciar-manual'), 'POST', {});
      await app.api(rota(v, 'abrir-para-preencher'), 'POST', {});
      setPasso('esperando');
      // Espera o site da empresa abrir (o app clicou em Candidatar-se; se não deu, a pessoa clica na janela).
      for (let t = 0; t < 45 && vez.current === minha; t++) {
        try {
          const r = await app.api(rota(v, 'preencher'), 'POST', {});
          if (vez.current !== minha) return;
          if (typeof r?.dominio === 'string' && r.dominio) { setDominio(r.dominio); setPasso('conferir'); return; }
        } catch (e) {
          if (!(e instanceof ErroApi && e.dados?.motivo === 'sem-site')) throw e;
        }
        await new Promise(ok => setTimeout(ok, 2000));
      }
      if (vez.current === minha) { setTexto('O site da empresa não abriu. Clique em Candidatar-se na janela do app e tente de novo, ou pule esta vaga.'); setPasso('erro'); }
    } catch (e) {
      if (vez.current === minha) { setTexto(explicarPreencher(e)); setPasso('erro'); }
    }
  }
  useEffect(() => { if (atual && !fim) void abrir(atual); }, [atual?.id, fim]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => () => { vez.current++; }, []); // saiu da tela: a espera para

  async function preencher(comCPF = false) {
    if (!atual) return;
    setPasso('preenchendo');
    try {
      const r = await app.api(rota(atual, 'preencher'), 'POST', comCPF ? { dominio, cpf: true } : { dominio });
      setTexto(comCPF ? 'Preenchi o CPF. Confira tudo, avance e envie no site.' : resumoPreencher(r ?? {}).replace(' Depois, clique em Já enviei.', ''));
      setCPF(!comCPF && Number(r?.pedeCPF) > 0 ? { salvo: r?.cpfSalvo === true } : null);
      setPasso('feito');
    } catch (e) { setTexto(explicarPreencher(e)); setPasso('erro'); }
  }
  async function proxima(enviou: boolean) {
    if (!atual || !fila) return;
    vez.current++;
    if (enviou) {
      try { await app.api(rota(atual, 'confirmar-envio-manual'), 'POST', {}); }
      catch { app.aviso('Não deu para registrar o envio. Tente de novo.', true); return; }
    }
    const nova = { enviadas: conta.enviadas + (enviou ? 1 : 0), puladas: conta.puladas + (enviou ? 0 : 1) };
    setConta(nova);
    await atualizar();
    if (i + 1 >= fila.length) { setFim(true); app.aviso(`Rodada dos sites concluída: ${plural(nova.enviadas, 'enviada', 'enviadas')}, ${plural(nova.puladas, 'pulada', 'puladas')}.`); }
    else setI(i + 1);
  }
  function parar() { vez.current++; setFila(null); setI(0); setFim(false); setConta({ enviadas: 0, puladas: 0 }); }
  function comecar() { setFila(disponiveis.map(d => d.vaga)); setI(0); setFim(false); setConta({ enviadas: 0, puladas: 0 }); }

  if (!fila && !disponiveis.length) return null;
  const nome = (v: Vaga) => `${v.empresa} — ${v.titulo}`;
  return (
    <section className="j-rodada j-rodada-sites" aria-labelledby="rodada-sites">
      <div className="j-rodada-topo">
        <div>
          <h2 id="rodada-sites"><span className="j-rodada-marca j-rodada-marca-site" aria-hidden="true"><IconeUI nome="globe" tamanho={14} /></span>Rodada dos sites das empresas</h2>
          <p>O app abre cada vaga no site da empresa e preenche o que souber. Você confere, completa e envia; depois, a próxima.</p>
        </div>
        <div className="j-rodada-acoes">
          {!fila || fim
            ? <button type="button" className="btn primary" {...trava(!disponiveis.length)} onClick={comecar}><IconeUI nome="send" tamanho={16} />{`Começar rodada (${disponiveis.length})`}</button>
            : <button type="button" className="btn quiet" onClick={parar}>Parar rodada</button>}
        </div>
      </div>
      {!fila && <p className="j-rodada-lista">Vão nesta rodada: {disponiveis.slice(0, 5).map(d => nome(d.vaga)).join(' · ')}{disponiveis.length > 5 ? ` e mais ${disponiveis.length - 5}` : ''}.</p>}
      {fim && <p className="j-rodada-aviso" role="status">{`Rodada concluída: ${plural(conta.enviadas, 'enviada', 'enviadas')}, ${plural(conta.puladas, 'pulada', 'puladas')}.`}</p>}
      {atual && !fim && <div className="j-rodada-vaga" role="group" aria-label="Vaga da rodada">
        <p className="j-rodada-agora">{`Vaga ${i + 1} de ${fila!.length}: `}<strong>{nome(atual)}</strong></p>
        {passo === 'abrindo' && <p role="status">Abrindo a vaga na janela do app…</p>}
        {passo === 'esperando' && <p role="status">Esperando o site da empresa. Se ele não abrir sozinho, clique em Candidatar-se na janela do app.</p>}
        {passo === 'conferir' && <>
          <p>Preencher em <strong>{dominio}</strong>? Confira se é mesmo o site da empresa.</p>
          <div className="j-rodada-acoes">
            <button type="button" className="btn primary" {...trava()} onClick={() => void preencher()}>Preencher</button>
            <button type="button" className="btn quiet" onClick={() => void proxima(false)}>Não é este site: pular</button>
          </div>
        </>}
        {passo === 'preenchendo' && <p role="status">Preenchendo…</p>}
        {(passo === 'feito' || passo === 'erro') && <p role="status">{texto}</p>}
        {passo === 'feito' && cpf && (cpf.salvo
          ? <div className="j-rodada-acoes"><span>Este formulário pede CPF.</span><button type="button" className="btn secondary" {...trava()} onClick={() => void preencher(true)}>Preencher CPF em {dominio}</button></div>
          : <p>Este formulário pede CPF: digite no site, ou salve o seu em <a href="#/perfil">Perfil e IA</a>.</p>)}
        {(passo === 'feito' || passo === 'erro') && <div className="j-rodada-acoes">
          {passo === 'feito' && <button type="button" className="btn primary" {...trava()} onClick={() => void proxima(true)}><IconeUI nome="check" tamanho={16} />Já enviei: próxima</button>}
          {passo === 'erro' && <button type="button" className="btn secondary" {...trava()} onClick={() => void abrir(atual)}>Tentar de novo</button>}
          <button type="button" className="btn quiet" onClick={() => void proxima(false)}>Pular esta vaga</button>
        </div>}
      </div>}
    </section>
  );
}
