import { useState } from 'react';
import { useApp } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import { dataCurta, encode, nomeEtapa, NOMES_SUB } from '../util';
import { TelaFila } from './Fila';
import { BotaoRadar, CabecalhoJornada, DialogoJornada, MarcaEmpresa, TituloVaga, VazioJornada, type VagaJornada } from './JornadaUI';

export function TelaListaDoDia() { return <TelaFila jornada />; }

const colunas = [
  { nome:'Enviada', classe:'sent' }, { nome:'Entrevista', classe:'interview' },
  { nome:'Proposta', classe:'offer' }, { nome:'Conquistada', classe:'won' }
];
function coluna(v: VagaJornada) {
  if (v.etapa === 'enviada') return 'Enviada';
  if (v.etapa === 'em-processo') return v.subEtapa === 'proposta' ? 'Proposta' : 'Entrevista';
  if (v.etapa === 'oferta') return 'Proposta';
  if (v.etapa === 'encerrada' && v.motivo === 'Proposta aceita') return 'Conquistada';
  return '';
}

function EditarCartao({ vaga, fechar }: { vaga: VagaJornada; fechar: () => void }) {
  const app = useApp();
  const [etapa, setEtapa] = useState(coluna(vaga));
  const [notas, setNotas] = useState(vaga.notas || '');
  const [ocupado, setOcupado] = useState(false);
  const [erro, setErro] = useState('');
  const eventos = (app.estado.eventos ?? []).filter(e => e.vagaId === vaga.id).slice().reverse()
    .sort((a,b) => (b.data ?? '').localeCompare(a.data ?? ''));
  async function salvar() {
    if (ocupado) return;
    if (etapa === coluna(vaga) && notas.trim() === (vaga.notas || '')) { fechar(); return; }
    setOcupado(true); setErro('');
    try {
      await app.api(`/api/v1/vagas/${encode(vaga.id)}/acompanhamento`, 'POST', {
        etapaAntes: coluna(vaga), notasAntes: vaga.notas || '', etapa, notas: notas.trim()
      });
      app.limpar(); await app.recarregar(); fechar(); app.aviso('Candidatura atualizada.');
    } catch(e) { setErro((e as Error).message); }
    finally { setOcupado(false); }
  }
  return <DialogoJornada titulo="Detalhes da candidatura" fechar={fechar}><TituloVaga vaga={vaga} />
    <form onSubmit={ev => {ev.preventDefault(); void salvar();}}>
      <label className="j-field" htmlFor="j-etapa">Etapa atual</label>
      <select className="j-input" id="j-etapa" value={etapa} onChange={ev => setEtapa(ev.target.value)}>{colunas.map(c => <option key={c.nome}>{c.nome}</option>)}</select>
      <label className="j-field" htmlFor="j-nota">Anotação</label>
      <textarea className="j-input" id="j-nota" rows={3} maxLength={1500} value={notas} placeholder="Ex.: combinar horário da entrevista" onChange={ev => setNotas(ev.target.value)} />
      <section className="j-history"><h3>Histórico</h3>{eventos.length ? <ol>{eventos.map(e => <li key={e.id}>{dataCurta(e.data)} · {e.texto || nomeEtapa(e.tipo)}</li>)}</ol> : <p>As próximas atualizações aparecerão aqui.</p>}</section>
      {erro && <p className="j-erro" role="alert">{erro}</p>}
      <div className="j-editor-actions"><button className="btn secondary" type="button" onClick={fechar}>Fechar</button><button className="btn primary" disabled={ocupado}><IconeUI nome="check" tamanho={16} />Salvar alterações</button></div>
    </form></DialogoJornada>;
}

export function TelaQuadroCandidaturas() {
  const app = useApp();
  const [aberta, setAberta] = useState<VagaJornada | null>(null);
  const vagas: VagaJornada[] = (app.estado.vagas ?? []).filter((v: VagaJornada) => coluna(v))
    .sort((a,b) => (b.atualizadoEm ?? '').localeCompare(a.atualizadoEm ?? ''));
  return <section className="j-page">
    <CabecalhoJornada titulo="Quadro de candidaturas" descricao="Acompanhe as conversas, registre atualizações e mova cada vaga de etapa." acao={<BotaoRadar />} />
    {!vagas.length && <div className="j-board-empty"><VazioJornada icone="board" titulo="Comece pela lista do dia" texto="Depois que um envio é confirmado, a candidatura aparece na primeira coluna."
      acao={<a className="btn secondary" href="#/lista"><IconeUI nome="list" tamanho={16} />Abrir lista do dia</a>} /></div>}
    <div className="j-board-grid">{colunas.map(c => {
      const itens = vagas.filter(v => coluna(v) === c.nome);
      return <section key={c.nome} className={`j-column j-${c.classe}`} aria-label={c.nome}><header><span className="j-stage-dot" /><h2>{c.nome}</h2><span className="j-count">{itens.length}</span></header>
        <div className="j-column-body">{itens.length ? itens.map(v => {
          const subEtapa = v.etapa === 'em-processo' && v.subEtapa && v.subEtapa !== 'proposta' ? NOMES_SUB[v.subEtapa] : null;
          const detalhe = v.notas || [...new Set([subEtapa, v.modelo, v.local].filter(Boolean))].join(' · ');
          return <button type="button" key={v.id} className="j-board-card" aria-label={`Abrir candidatura de ${v.titulo} na ${v.empresa}`} onClick={() => setAberta(v)}>
            <span className="j-card-company"><MarcaEmpresa empresa={v.empresa} tamanho={28} /><span>{v.empresa}</span></span><strong>{v.titulo}</strong>
            {detalhe && <span className="j-card-meta"><IconeUI nome={v.notas ? 'edit' : 'map-pin'} tamanho={13} /><span>{detalhe}</span></span>}</button>;
        }) : <div className="j-stage-empty">Nenhuma candidatura</div>}</div></section>;
    })}</div>
    {aberta && <EditarCartao vaga={aberta} fechar={() => {app.limpar(); setAberta(null);}} />}
  </section>;
}
