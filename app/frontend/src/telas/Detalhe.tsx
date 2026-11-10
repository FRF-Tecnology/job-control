// Gaveta com o detalhe da vaga: dados, mover de etapa, notas e linha do tempo (porte de web/telas/detalhe.mjs).
import { useEffect, useRef, useState } from 'react';
import { useApp } from '../app/contexto';
import { ETAPAS } from '../dominio/modelo';
import { EtapaChip, LinkExterno } from '../componentes';
import { dataCurta, encode, nomeAssistente, nomeEtapa, nomePlataforma, NOMES_SUB } from '../util';
import { extrasPara } from './Quadro';

const OPCOES_PENDENCIA = ['Digitar CPF e enviar', 'Fazer login no portal', 'Gravar vídeo', 'Fazer teste online'];

export function Detalhe({ id, fechar }: { id: string; fechar: () => void }) {
  const app = useApp();
  const v = app.estado.vagas.find((x: any) => x.id === id);
  const primeiroBotao = useRef<HTMLButtonElement>(null);
  const [novaNota, setNovaNota] = useState('');
  const [tipoNota, setTipoNota] = useState('nota');
  useEffect(() => { primeiroBotao.current?.focus(); }, []);
  if (!v) return null;

  const eventos = app.estado.eventos.filter((e: any) => e.vagaId === id).sort((a: any, b: any) => b.data.localeCompare(a.data) || b.id.localeCompare(a.id));
  const caminho = `/api/v1/vagas/${encode(id)}`;

  const moverPara = async (etapa: string) => {
    const extras = await extrasPara(app, etapa, OPCOES_PENDENCIA);
    if (extras) app.mover(v.id, etapa, extras, `Movida para ${nomeEtapa(etapa)}`);
  };

  const salvarNotas = async (minha: string) => {
    if ((v.notas ?? '') === minha) return;
    try {
      await app.api(caminho, 'PATCH', { notas: minha, notasAntes: v.notas ?? '' });
      app.aviso('Notas salvas');
      app.limpar();
      return app.recarregar();
    } catch (e) {
      if (!/mudaram/.test((e as Error).message)) return app.aviso((e as Error).message, true);
    }
    // Alguém (ex.: o Claude) mudou as notas enquanto você digitava: junte as duas versões.
    const atual = (await app.api('/api/v1/estado')).vagas.find((x: any) => x.id === id)?.notas ?? '';
    const juntas = await app.perguntar({ titulo: 'As notas mudaram enquanto você editava', texto: 'Juntei as duas versões. Ajuste e salve.', campo: 'Notas', valor: `${atual}\n\n${minha}`.trim(), multilinha: true, confirmar: 'Salvar notas' });
    if (juntas == null) { app.aviso('Suas notas não foram salvas', true); return; }
    await app.agir(() => app.api(caminho, 'PATCH', { notas: juntas, notasAntes: atual }), 'Notas salvas');
  };

  const dados = ([
    ['Plataforma', nomePlataforma(v.plataforma, app.estado.plataformas)],
    v.aderencia && ['Aderência', `Grupo ${v.aderencia}`],
    v.contrato && ['Contrato', v.contrato],
    v.nivel && ['Nível', v.nivel],
    v.idioma === 'en' && ['Idioma', 'Inglês'],
    ['Encontrada', dataCurta(v.descobertaEm)],
    v.envio?.data && ['Enviada', `${dataCurta(v.envio.data)}${v.envio.rodada ? `, rodada ${v.envio.rodada}` : ''}`],
    v.envio?.pretensao && ['Pretensão', `R$ ${Number(v.envio.pretensao).toLocaleString('pt-BR')}`],
    v.envio?.curriculo && ['Currículo', v.envio.curriculo === 'EN' ? 'Inglês' : 'Português'],
    v.envio?.respostas && ['Respostas', v.envio.respostas],
    v.followUp?.proximo && v.etapa === 'enviada' && ['Follow-up', `${dataCurta(v.followUp.proximo)}${v.followUp.feitos ? ` (${v.followUp.feitos} feito)` : ''}`],
    v.subEtapa && ['Etapa do processo', NOMES_SUB[v.subEtapa]],
    v.motivo && ['Motivo', v.motivo],
    v.pendencia && ['Falta você', v.pendencia],
  ].filter(Boolean)) as [string, string][];

  return (
    <>
      <div className="topo">
        <div>
          <div className="mudo">{v.empresa}</div>
          <h2 style={{ fontSize: '1.5rem', lineHeight: 1.2 }}>{v.titulo}</h2>
          <div style={{ marginTop: '6px' }}><EtapaChip etapa={v.etapa} /></div>
        </div>
        <button className="botao-fantasma p" type="button" aria-label="Fechar detalhe" ref={primeiroBotao} onClick={fechar}>Fechar</button>
      </div>
      {v.motivoClaude && <div className="ficha" style={{ padding: '12px 14px', boxShadow: 'none', background: 'var(--superficie-2)' }}><div><b>Observação do Claude: </b>{v.motivoClaude}</div></div>}
      <div className="acoes">
        <LinkExterno url={v.link} className="botao">Abrir vaga</LinkExterno>
        <label className="campo" style={{ minWidth: '200px' }}><span>Mover para</span>
          <select value="" onChange={ev => { const e = ev.target.value; if (e) moverPara(e); }}>
            <option value="">{nomeEtapa(v.etapa)}</option>
            {ETAPAS.filter(e => e !== v.etapa).map(e => <option key={e} value={e}>{nomeEtapa(e)}</option>)}
          </select>
        </label>
      </div>
      <dl>{dados.map(([k, val]) => <div key={k} style={{ display: 'contents' }}><dt>{k}</dt><dd>{val}</dd></div>)}</dl>
      <label className="campo"><span>Notas</span>
        {/* Recomeça do salvo quando as notas mudam por fora (com nada por salvar, a tela recarrega). */}
        <textarea key={v.notas ?? ''} id="notas-vaga" placeholder="Anotações: recrutador, dúvidas, próximos passos…" defaultValue={v.notas ?? ''} onBlur={ev => salvarNotas(ev.target.value)} />
      </label>
      <section>
        <h3 style={{ marginBottom: '10px' }}>Linha do tempo</h3>
        <form className="acoes" style={{ marginBottom: '14px', flexWrap: 'nowrap' }} onSubmit={async ev => {
          ev.preventDefault();
          const texto = novaNota.trim();
          if (!texto) return;
          const ok = await app.agir(() => app.api(`${caminho}/eventos`, 'POST', { tipo: tipoNota, texto }), 'Registrado');
          if (ok) setNovaNota('');
        }}>
          <select aria-label="Tipo" value={tipoNota} onChange={ev => setTipoNota(ev.target.value)}>
            {['nota', 'retorno', 'teste', 'entrevista', 'follow-up'].map(t => <option key={t} value={t}>{t}</option>)}
          </select>
          <input id="nova-nota" type="text" placeholder="Registrar algo (ex.: recrutador respondeu)" value={novaNota} onChange={ev => setNovaNota(ev.target.value)} />
          <button className="botao-fantasma" type="submit">Registrar</button>
        </form>
        {eventos.length ? (
          <ol className="linha-tempo">
            {eventos.map((e: any) => (
              <li key={e.id} className={`e-${e.tipo}`}>
                <div><b>{nomeEtapa(e.tipo)}</b>{e.texto ? ` — ${e.texto}` : ''}</div>
                <div className="q">{`${dataCurta(e.data)} · ${e.autor === 'claude' ? (e.canal ? nomeAssistente(e.canal) : 'Claude') : 'você'}`}</div>
              </li>
            ))}
          </ol>
        ) : <p className="mudo">Sem eventos.</p>}
      </section>
      <p className="pequeno mudo">{`id: ${v.id}`}</p>
    </>
  );
}
