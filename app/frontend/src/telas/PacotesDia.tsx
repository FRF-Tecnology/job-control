import { useEffect, useState } from 'react';
import { useApp, useTrava } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import type { Conteudo } from '../app/pacote';
import { perfilDoPacote } from '../app/preparoAutomatico';
import { LinkExterno } from '../componentes';
import { encode } from '../util';
import type { Candidatura } from './Fila';
import type { Limite } from './LimiteDiario';
import { ListaDoDia } from './ListaDoDia';
import { BotaoRadar, CabecalhoJornada, DialogoJornada, TituloVaga, VazioJornada, type VagaJornada } from './JornadaUI';

type Dados = {
  candidaturas: Candidatura[]; calculadas: Record<string,string>; limites: Record<string,Limite>; ocupado: boolean;
  aprovar: (c: Candidatura) => Promise<unknown>; revogar: (c: Candidatura) => Promise<unknown>;
  salvar: (c: Candidatura, k: Conteudo) => Promise<boolean>; atualizar: () => Promise<void>; plataforma: (p: string) => string;
};
const rotulos: Record<Candidatura['estado'], string> = { preparada:'Em revisão', aprovada:'Aprovado', bloqueada:'Aguardando você',
  enviando:'Em envio', falhou:'Falhou', incerta:'Confira o envio', cancelada:'Cancelado', enviada:'Enviado' };
const enderecoFila = (v: VagaJornada) => `#/fila?preparar=${encode(v.id)}`;

function Revisar({ vaga, c, salvar, devolver, fechar, ocupado }: {
  vaga: VagaJornada; c?: Candidatura; salvar: Dados['salvar']; devolver: () => Promise<void>; fechar: () => void; ocupado: boolean;
}) {
  const {estado} = useApp();
  const trava = useTrava();
  const arquivo = c?.conteudo.curriculo.arquivo;
  const exibicao = perfilDoPacote(estado, vaga)?.curriculoExibicao;
  const nomeArquivo = arquivo && exibicao?.arquivo?.split(/[\\/]/).pop() === arquivo ? exibicao.nome || arquivo : arquivo;
  const [carta, setCarta] = useState(c?.conteudo.mensagem ?? '');
  const [pendente, setPendente] = useState(false);
  const editavel = c?.estado === 'preparada' || c?.estado === 'aprovada';
  const podeDevolver = !c || ['preparada','aprovada','bloqueada','falhou'].includes(c.estado);
  async function guardar() {
    if (!c || !editavel || ocupado) return;
    if (carta.trim() === (c.conteudo.mensagem || '')) { fechar(); return; }
    // Só a carta muda. Preserva respostas com perguntaId e todos os demais campos que a pessoa revisou.
    const ok = await salvar(c, {...c.conteudo, mensagem: carta.trim() || null} as Conteudo);
    if (ok) fechar();
    else setPendente(true);
  }
  return <DialogoJornada titulo="Revisar pacote" fechar={fechar}><TituloVaga vaga={vaga} />
    <div className="j-document"><span><IconeUI nome="file" tamanho={19} /></span><div><strong>Currículo</strong><p>{nomeArquivo || 'Pacote ainda não preparado'}</p></div></div>
    {c && <>
      <label className="j-field" htmlFor="j-carta">Carta de apresentação <span>Opcional</span></label>
      <textarea className="j-input" id="j-carta" rows={6} value={carta} readOnly={!editavel} onChange={ev => {setCarta(ev.target.value); setPendente(false);}} placeholder="Escreva uma mensagem para a empresa" />
      {pendente && <p className="j-erro" role="status">A alteração ainda está neste editor. Reabra o pacote para conferir a versão salva.</p>}
      <details className="j-dados-pacote"><summary>Conteúdo completo do pacote</summary><dl>
        {arquivo !== nomeArquivo && <><dt>Arquivo na pasta de currículos</dt><dd>{arquivo}</dd></>}
        <dt>Endereço da vaga</dt><dd>{c.url}</dd><dt>Idioma</dt><dd>{c.conteudo.idioma === 'en' ? 'Inglês' : 'Português'}</dd>
        <dt>Contato</dt><dd>{Object.values(c.conteudo.contato ?? {}).filter(Boolean).join(' · ') || 'Sem contato'}</dd>
        <dt>Pretensão</dt><dd>{c.conteudo.pretensao ? [c.conteudo.pretensao.moeda, c.conteudo.pretensao.valor?.toLocaleString('pt-BR'), c.conteudo.pretensao.tipo].filter(Boolean).join(' ') : 'Sem pretensão'}</dd>
        <dt>Respostas</dt><dd>{c.conteudo.respostas.length ? <ul>{c.conteudo.respostas.map((r,i) => <li key={i}>{r.pergunta} · {r.resposta} ({r.origem})</li>)}</ul> : 'Sem respostas'}</dd>
      </dl><a href={enderecoFila(vaga)}>Abrir pacote na fila</a></details>
    </>}
    <div className="j-editor-actions">{podeDevolver && <button type="button" className="btn quiet" {...trava(ocupado)} onClick={() => void devolver()}><IconeUI nome="arrow-left" tamanho={16} />Devolver ao radar</button>}
      {editavel ? <button type="button" className="btn primary" {...trava(ocupado)} onClick={() => void guardar()}><IconeUI nome="check" tamanho={16} />Salvar pacote</button>
        : <a className="btn primary" href={enderecoFila(vaga)}>{c ? 'Ver na fila' : 'Preparar pacote'}</a>}</div>
  </DialogoJornada>;
}

export function PacotesDia({ candidaturas, calculadas, limites, ocupado, aprovar, revogar, salvar, atualizar, plataforma }: Dados) {
  const app = useApp();
  const trava = useTrava();
  const [revisao, setRevisao] = useState<{ vaga: VagaJornada; c?: Candidatura } | null>(null);
  const [lote, setLote] = useState(false);
  const [manual, setManual] = useState<VagaJornada | null>(null);
  const [salvando, setSalvando] = useState(false);
  const [recibo, setRecibo] = useState(false);
  const vagas: VagaJornada[] = (app.estado.vagas ?? []).filter((v: VagaJornada) => v.etapa === 'aplicar' && !v.envio?.data);
  const porVaga = new Map(candidaturas.filter(c => c.estado !== 'cancelada').map(c => [c.vagaId, c]));
  const itens = vagas.map(vaga => ({vaga, c: porVaga.get(vaga.id)}));
  const reserva = manual && itens.find(({vaga,c}) => vaga.id === manual.id && vaga.aplicacaoManual
    && !!manual.candidaturaManualId && vaga.candidaturaManualId === manual.candidaturaManualId
    && vaga.urlCandidaturaManual === manual.urlCandidaturaManual && c?.id === vaga.candidaturaManualId && c.estado === 'bloqueada');
  useEffect(() => { if (manual && !reserva) setManual(null); }, [manual,reserva]);
  const prontas = itens.filter(i => i.c?.estado === 'aprovada').length;
  const paraRevisar = itens.filter(i => !i.c || i.c.estado === 'preparada').length;
  const retomada = itens.find(i => i.vaga.aplicacaoManual && i.c?.estado === 'bloqueada');
  const proxima = retomada || itens.find(i => i.c?.estado === 'aprovada');
  function permiteAprovar(c: Candidatura) {
    return !!calculadas[c.id] && calculadas[c.id] === c.impressao && c.estado === 'preparada'
      && (c.plataforma !== 'formulario' || (!!c.estrutura && !c.pendentes?.length));
  }
  const emAcao = ocupado || salvando;
  function fechar() { app.limpar(); setRevisao(null); setLote(false); setManual(null); }
  async function iniciarManual(vaga: VagaJornada) {
    if (emAcao) return;
    setSalvando(true);
    try {
      const reservada: VagaJornada = await app.api(`/api/v1/vagas/${encode(vaga.id)}/iniciar-manual`, 'POST', {});
      await atualizar();
      setManual(reservada);
    } catch(e) { app.aviso((e as Error).message,true); await atualizar(); }
    finally { setSalvando(false); }
  }
  async function agirVaga(vaga: VagaJornada, acao: 'devolver-ao-radar' | 'confirmar-envio-manual') {
    if (emAcao) return;
    setSalvando(true);
    try {
      await app.api(`/api/v1/vagas/${encode(vaga.id)}/${acao}`, 'POST', {});
      await atualizar(); fechar(); setRecibo(acao === 'confirmar-envio-manual');
      app.aviso(acao === 'devolver-ao-radar' ? 'Vaga devolvida ao radar.' : 'Envio registrado no quadro.');
    } catch(e) { app.aviso((e as Error).message,true); }
    finally { setSalvando(false); }
  }
  return <section className="j-page">
    <CabecalhoJornada titulo="Lista do dia" descricao="Revise os pacotes das vagas que você escolheu no radar." acao={itens.length ? <BotaoRadar /> : null} />
    {recibo && <div className="j-receipt" role="status"><IconeUI nome="check" tamanho={20} /><div><b>Envio confirmado</b><p>Candidatura registrada no quadro.</p></div><a className="btn quiet" href="#/quadro">Ver no quadro</a></div>}
    <div className="j-list-layout"><section className="j-list-panel" aria-label="Pacotes para envio">
      <div className="j-panel-heading"><h2>Pacotes para envio <span className="j-count">{itens.length}</span></h2>
        {itens.length > 1 && itens.some(i => i.c && permiteAprovar(i.c)) && <button className="btn quiet j-small" type="button" {...trava()} onClick={() => setLote(true)}>Aprovar todos</button>}</div>
      {itens.length ? <ul className="j-list">{itens.map(({vaga,c}) => <li className="j-row" key={vaga.id}>
        <div className="j-row-top"><TituloVaga vaga={vaga} /><span className={'j-status' + (c?.estado === 'aprovada' ? ' j-approved':'')}><i />{c ? rotulos[c.estado] : 'Para preparar'}</span></div>
        <div className="j-row-actions"><button type="button" className="btn secondary j-small" aria-label={`Revisar pacote de ${vaga.titulo} na ${vaga.empresa}`} onClick={() => setRevisao({vaga,c})}><IconeUI nome="file" tamanho={16} />Revisar pacote</button>
          {c && ['preparada','aprovada'].includes(c.estado) && <button type="button" className="btn quiet j-small" {...trava(emAcao || (c.estado === 'preparada' && !permiteAprovar(c)))} aria-pressed={c.estado === 'aprovada'}
            onClick={() => void (c.estado === 'aprovada' ? revogar(c) : aprovar(c))}><IconeUI nome="check" tamanho={16} />{c.estado === 'aprovada' ? 'Retirar aprovação' : 'Aprovar pacote'}</button>}
          {!c && <a className="btn quiet j-small" href={enderecoFila(vaga)}>Preparar pacote</a>}
          {c && (['preparada','aprovada'].includes(c.estado) || (vaga.aplicacaoManual && c.estado === 'bloqueada')) &&
            <button type="button" className="btn quiet j-small" {...trava(emAcao)} onClick={() => void iniciarManual(vaga)}>
              {vaga.aplicacaoManual ? 'Retomar candidatura' : 'Aplicar no site'}</button>}
          {c && ((!vaga.aplicacaoManual && ['bloqueada','incerta','falhou'].includes(c.estado)) || (c.plataforma === 'formulario' && !permiteAprovar(c) && c.estado === 'preparada')) && <a className="btn quiet j-small" href={enderecoFila(vaga)}>Revisar na fila</a>}
        </div></li>)}</ul> : <VazioJornada icone="list" titulo={recibo ? 'Lista concluída' : 'Sua lista está vazia'} texto="Adicione uma vaga no radar para revisar o pacote de candidatura aqui." acao={<BotaoRadar />} />}
    </section><aside className="j-summary" aria-label="Resumo do lote"><span className="j-summary-icon"><IconeUI nome="send" tamanho={22} /></span><h2>Resumo do lote</h2>
      <div className="j-ratio"><strong>{prontas}</strong><span>/ {itens.length}</span></div><div className="j-ratio-caption">{prontas === 1 ? 'pacote aprovado' : 'pacotes aprovados'}</div>
      <div className="j-meter" aria-hidden="true"><i style={{width: `${itens.length ? prontas*100/itens.length : 0}%`}} /></div>
      <div className="j-summary-line"><span>Para revisar</span><b>{paraRevisar}</b></div><div className="j-summary-line"><span>Prontos para envio</span><b>{prontas}</b></div>
      <div className="j-summary-bottom"><button type="button" className="btn primary" {...trava(!proxima || emAcao)} onClick={() => proxima && void iniciarManual(proxima.vaga)}><IconeUI nome="send" tamanho={16} />
        {retomada ? 'Retomar candidatura' : 'Aplicar na próxima vaga'}</button>
        <p>Conclua a candidatura no site da empresa e confirme aqui para acompanhar no quadro.</p></div></aside></div>
    {manual && reserva && <DialogoJornada titulo="Candidatura no site da empresa" fechar={fechar}><TituloVaga vaga={manual} />
      <p>Abra a vaga no navegador para enviar sua candidatura. Quando terminar, volte e confirme o envio.</p>
      <LinkExterno url={manual.urlCandidaturaManual} className="btn primary">Abrir candidatura no navegador</LinkExterno>
      <div className="j-editor-actions"><button type="button" className="btn quiet" onClick={fechar}>Continuar depois</button>
        <button type="button" className="btn secondary" {...trava(emAcao)} onClick={() => void agirVaga(manual,'confirmar-envio-manual')}><IconeUI nome="check" tamanho={16} />Já enviei</button></div>
    </DialogoJornada>}
    {revisao && <Revisar {...revisao} ocupado={emAcao} salvar={salvar} devolver={() => agirVaga(revisao.vaga,'devolver-ao-radar')} fechar={fechar} />}
    {lote && <DialogoJornada titulo="Aprovar pacotes" fechar={fechar}>
      <ListaDoDia candidaturas={candidaturas.filter(c => c.estado === 'aprovada' || itens.some(i => i.c?.id === c.id))} calculadas={calculadas} limites={limites}
        nome={c => {const i = itens.find(i => i.c?.id === c.id); return i ? `${i.vaga.empresa} — ${i.vaga.titulo}` : c.id;}}
        plataforma={plataforma} onFeito={() => {fechar(); void atualizar();}} onRevisar={id => {const i = itens.find(i => i.c?.id === id); if(i) {setLote(false); setRevisao(i);}}} />
    </DialogoJornada>}
  </section>;
}
