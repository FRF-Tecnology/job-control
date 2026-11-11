// Assinatura (Task 7): a situação da conta, Assinar (abre o Checkout no navegador; o app confere sozinho até a
// assinatura aparecer), Gerenciar (portal) e as notas fiscais. Fica liberada com a trava: é o caminho para sair dela.
import { useEffect, useRef, useState } from 'react';
import { useApp } from '../app/contexto';
import { LinkExterno } from '../componentes';
import { TextosLegais } from './Conta';

const CADENCIA = 3000; // enquanto o pagamento é feito no navegador, relê a conta a cada 3 s
const PRAZO = 10 * 60_000; // por até 10 min (o mesmo prazo do servidor)

const data = (iso?: string | null) => (iso ? new Date(iso).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' }) : '');

export function TelaAssinatura() {
  const app = useApp();
  const c = app.conta;
  const [notas, setNotas] = useState<any[] | null>(null);
  const [erro, setErro] = useState('');
  const [mensagem, setMensagem] = useState('');
  const [ocupado, setOcupado] = useState(false);
  const [conferindo, setConferindo] = useState(false);
  const assinante = c?.estado === 'assinante' && !c.motivo;
  const vivo = useRef(true);
  useEffect(() => () => { vivo.current = false; }, []);

  useEffect(() => {
    if (!c?.ativa || !c.entrada) return;
    app.api('/api/v1/assinatura').then(r => { if (vivo.current) setNotas(Array.isArray(r?.notas) ? r.notas : []); })
      .catch(e => { if (vivo.current) setErro((e as Error).message); });
  }, [c?.ativa, c?.entrada, assinante]); // eslint-disable-line react-hooks/exhaustive-deps

  // Depois de Assinar: relê a conta até virar assinante (a casca tira a faixa e libera as telas).
  useEffect(() => {
    if (!conferindo) return;
    if (assinante) { setConferindo(false); setMensagem('Assinatura ativa. Obrigado!'); return; }
    const fim = Date.now() + PRAZO;
    const t = setInterval(() => { if (Date.now() > fim) { setConferindo(false); return; } app.releConta?.(); }, CADENCIA);
    return () => clearInterval(t);
  }, [conferindo, assinante]); // eslint-disable-line react-hooks/exhaustive-deps

  const abrir = async (rota: 'assinar' | 'gerenciar') => {
    setOcupado(true); setErro('');
    try {
      const r = await app.api(`/api/v1/assinatura/${rota}`, 'POST', {});
      setMensagem(r?.mensagem ?? '');
      if (rota === 'assinar') { setConferindo(true); await app.releConta?.(); }
    } catch (e) { setErro((e as Error).message); } finally { if (vivo.current) setOcupado(false); }
  };

  if (!c?.ativa) return <div className="convite"><div><strong>Assinatura</strong><p>A conta não está ligada nesta versão do app.</p></div></div>;
  const situacao = assinante
    ? `Assinante${c.periodo_fim ? ` até ${data(c.periodo_fim)}` : ''}.`
    : c.estado === 'teste' && !c.motivo
      ? `Teste grátis${c.teste_fim ? ` até ${data(c.teste_fim)}` : ''}.`
      : 'Sem assinatura ativa.';

  return (
    <section className="formulario" aria-labelledby="assinatura-titulo">
      <header className="cabecalho"><div><h1 id="assinatura-titulo">Assinatura</h1><p>{c.email}</p></div></header>
      <fieldset>
        <p style={{ margin: 0 }}><b>{situacao}</b></p>
        {mensagem && <p className="mudo" role="status" style={{ margin: 0 }}>{mensagem}</p>}
        <div aria-live="assertive">{erro && <p className="erro-resposta" role="alert">{erro}</p>}</div>
        <div className="acoes">
          {!assinante && <button className="botao" type="button" disabled={ocupado} onClick={() => abrir('assinar')}>Assinar</button>}
          {(assinante || c.periodo_fim) && <button className="botao-fantasma" type="button" disabled={ocupado} onClick={() => abrir('gerenciar')}>Gerenciar assinatura</button>}
        </div>
        <p className="pequeno mudo" style={{ margin: 0 }}>O pagamento é feito pela Stripe, no navegador. Cancele quando quiser em "Gerenciar assinatura".</p>
      </fieldset>
      {notas && notas.length > 0 && (
        <fieldset><legend>Notas fiscais</legend>
          <ul>{notas.map((n, i) => (
            <li key={n.numero ?? i}>
              <LinkExterno url={n.link_pdf} semLink={<span>{`Nota ${n.numero ?? ''}`}</span>}>{`Nota ${n.numero ?? ''}`}</LinkExterno>
              {` · ${data(n.criada_em)}${n.estado ? ` · ${n.estado}` : ''}`}
            </li>
          ))}</ul>
        </fieldset>
      )}
      <fieldset><legend>Termos</legend><TextosLegais /></fieldset>
    </section>
  );
}
