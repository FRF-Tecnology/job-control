// Formulário externo na Fila (jc-a2y.7.2.4; contrato 0.13.0, docs/propostas/formularios.md). Para candidatura de
// plataforma "formulario": "Ler formulário" (o app abre a página sem janela e lê as perguntas, sem interagir), as
// perguntas lidas (texto da página: dado não confiável, mostrado só como texto), o que veio das respostas padrão, um
// campo para cada pendente e a frase do pré-preenchimento ignorado. Pergunta sensível nunca ganha campo. Typeform
// ainda é "envie à mão". Nada aqui envia: aprovar continua sendo o botão da Fila, que só libera sem pendentes.
import { useState } from 'react';
import { perguntaSensivel } from '../app/sensivel';
import type { EstruturaFormulario, PerguntaFormulario } from '../app/impressao';

type Resposta = { pergunta: string; resposta: string; origem: string; perguntaId?: string };
export type CandFormulario = {
  id: string; url: string; estado: string; versao: number;
  conteudo: { respostas: Resposta[]; contato?: { email?: string; telefone?: string } | null } & Record<string, unknown>;
  estrutura?: EstruturaFormulario | null; pendentes?: string[]; preenchimentoIgnorado?: boolean;
};

const TIPO: Record<string, string> = {
  'texto-curto': 'Resposta curta', 'texto-longo': 'Parágrafo', 'escolha-unica': 'Escolha única', 'multipla-escolha': 'Caixas de seleção',
  lista: 'Lista', data: 'Data', numero: 'Número', email: 'E-mail', telefone: 'Telefone', escala: 'Escala', outro: 'Outro',
};
const ORIGEM: Record<string, string> = { respostasPadrao: 'Respostas padrão', pessoa: 'Você', ia: 'IA' };
// Escolha de uma opção só: a resposta é o texto da opção, então a tela oferece só as opções lidas.
const DE_OPCAO = new Set(['escolha-unica', 'lista', 'escala']);

export const ehTypeform = (url: string) => { try { return /(^|\.)typeform\.com$/.test(new URL(url).hostname); } catch { return false; } };
// O que trava o aprovar de um formulário: sem leitura, ou com pendência (o servidor dá 409 "faltam respostas").
export const formularioSemAprovar = (c: CandFormulario) => !c.estrutura || (c.pendentes?.length ?? 0) > 0;

export function FormularioCand({ c, ocupado, onLer, onResponder }: {
  c: CandFormulario; ocupado: boolean; onLer: () => void; onResponder: (conteudo: CandFormulario['conteudo']) => void;
}) {
  const [valores, setValores] = useState<Record<string, string>>({});
  const typeform = ehTypeform(c.url);
  const respostaDe = (p: PerguntaFormulario) => c.conteudo.respostas.find(r => r.perguntaId === p.id);
  const pendentes = new Set(c.pendentes ?? []);
  const preparada = c.estado === 'preparada';
  // Só as pendentes que podem ganhar campo (sensível nunca).
  const respondiveis = (c.estrutura?.perguntas ?? []).filter(p => pendentes.has(p.id) && !perguntaSensivel(p.texto));
  const prontas = respondiveis.filter(p => (valores[p.id] ?? '').trim());

  const salvar = (ev: React.FormEvent) => {
    ev.preventDefault();
    if (!prontas.length) return;
    const novos = new Set(prontas.map(p => p.id));
    const respostas = [
      ...c.conteudo.respostas.filter(r => !r.perguntaId || !novos.has(r.perguntaId)),
      ...prontas.map(p => ({ pergunta: p.texto, resposta: valores[p.id].trim(), origem: 'pessoa', perguntaId: p.id })),
    ];
    onResponder({ ...c.conteudo, respostas });
    setValores({});
  };

  return (
    <section className="form-ext" aria-label="Formulário da vaga">
      {c.preenchimentoIgnorado && <p className="pequeno aviso-cv">O link trazia respostas pré-preenchidas; o app ignorou.</p>}
      {typeform ? (
        // Typeform ainda é à mão: ler confere a página e a deixa bloqueada ("envie à mão"), com "Marcar como enviada".
        <div className="formulario-ler">
          <p className="pequeno aviso-cv"><strong>Typeform: envie à mão.</strong> O app ainda não preenche Typeform. Abra o link da vaga e envie você; depois, marque como enviada.</p>
          {preparada && <button className="botao-fantasma p" type="button" disabled={ocupado} onClick={onLer}>{ocupado ? 'Conferindo…' : 'Conferir e enviar à mão'}</button>}
        </div>
      ) : !c.estrutura ? (
        preparada && (
          <div className="formulario-ler">
            <p className="pequeno mudo">Leia o formulário para ver as perguntas antes de aprovar. O app abre a página sem janela e só lê: não preenche nem envia nada.</p>
            <button className="botao p" type="button" disabled={ocupado} onClick={onLer}>{ocupado ? 'Lendo…' : 'Ler formulário'}</button>
          </div>
        )
      ) : (
        <>
          <div className="formulario-topo">
            <h3>{`Perguntas do formulário (${c.estrutura.perguntas.length})`}</h3>
            {preparada && <button className="botao-fantasma p" type="button" disabled={ocupado} onClick={onLer}>Ler de novo</button>}
          </div>
          {pendentes.size > 0
            ? <p className="pequeno formulario-pendentes" role="status">{`${pendentes.size === 1 ? 'Falta responder 1 pergunta obrigatória' : `Faltam responder ${pendentes.size} perguntas obrigatórias`} para aprovar.`}</p>
            : <p className="pequeno mudo">Todas as obrigatórias têm resposta.</p>}
          <form onSubmit={salvar} noValidate>
            <ol className="formulario-perguntas">
              {c.estrutura.perguntas.map(p => {
                const r = respostaDe(p);
                const sensivel = perguntaSensivel(p.texto);
                const pendente = pendentes.has(p.id);
                const idCampo = `${c.id}-${p.id}`;
                return (
                  <li key={p.id} className={pendente ? 'pendente' : undefined}>
                    <span className="pergunta">{p.texto}</span>
                    <span className="pequeno mudo">{`${TIPO[p.tipo] ?? 'Outro'}${p.obrigatoria ? ' · obrigatória' : ''}${p.opcoes?.length ? ` · opções: ${p.opcoes.join(', ')}` : ''}`}</span>
                    {r ? (
                      <span className="resposta"><strong>{r.resposta}</strong><span className="chip">{ORIGEM[r.origem] ?? r.origem}</span></span>
                    ) : sensivel ? (
                      <span className="pequeno erro-resposta">Pergunta sensível: o app não responde. Envie este formulário à mão.</span>
                    ) : pendente && preparada ? (
                      DE_OPCAO.has(p.tipo) && p.opcoes?.length ? (
                        <select id={idCampo} aria-label={`Resposta: ${p.texto}`} value={valores[p.id] ?? ''} onChange={e => setValores(v => ({ ...v, [p.id]: e.target.value }))}>
                          <option value="">Escolha</option>
                          {p.opcoes.map(o => <option key={o} value={o}>{o}</option>)}
                        </select>
                      ) : (
                        <input id={idCampo} type="text" aria-label={`Resposta: ${p.texto}`} autoComplete="off" maxLength={500}
                          value={valores[p.id] ?? ''} onChange={e => setValores(v => ({ ...v, [p.id]: e.target.value }))} />
                      )
                    ) : (
                      <span className="pequeno mudo">{pendente ? 'Pendente' : 'Sem resposta (opcional)'}</span>
                    )}
                  </li>
                );
              })}
            </ol>
            {respondiveis.length > 0 && preparada && (
              <div className="acoes">
                <button className="botao p" type="submit" disabled={ocupado || !prontas.length}>{prontas.length > 1 ? `Salvar ${prontas.length} respostas` : 'Salvar resposta'}</button>
              </div>
            )}
          </form>
        </>
      )}
    </section>
  );
}
