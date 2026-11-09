// Formulário do pacote da candidatura (P5, jc-a2y.6.3.3): preparar e editar. O que está aqui é exatamente o que vai
// para a vaga. Resposta editada pela pessoa passa a ter origem "Você"; a da IA fica marcada até ser editada.
import { useState } from 'react';
import { conteudoParaEnviar, type Conteudo } from '../app/pacote';

const ORIGENS: Record<string, string> = { respostasPadrao: 'Respostas padrão', pessoa: 'Você', ia: 'IA' };

export function PacoteForm({ inicial, pdfs, rotulo, ocupado, onSalvar, onCancelar }: {
  inicial: Conteudo; pdfs: { nome: string }[]; rotulo: string; ocupado?: boolean;
  onSalvar: (c: Conteudo) => void; onCancelar: () => void;
}) {
  const [c, setC] = useState<Conteudo>(() => structuredClone(inicial));
  const mudar = (f: (x: Conteudo) => void) => setC(atual => { const n = structuredClone(atual); f(n); return n; });
  const nomes = pdfs.map(p => p.nome);
  if (c.curriculo.arquivo && !nomes.includes(c.curriculo.arquivo)) nomes.unshift(c.curriculo.arquivo);
  const semPDF = nomes.length === 0;
  const pretensao = c.pretensao ?? { tipo: '', moeda: 'BRL' };

  return (
    <form className="formulario pacote-form" aria-label="Pacote da candidatura" onSubmit={ev => { ev.preventDefault(); onSalvar(conteudoParaEnviar(c)); }}>
      <div className="duas">
        <label className="campo"><span>Currículo</span>
          <select value={c.curriculo.arquivo} disabled={semPDF} onChange={ev => { const v = ev.target.value; mudar(x => { x.curriculo = { arquivo: v }; }); }}>
            {nomes.map(n => <option key={n} value={n}>{n}</option>)}
          </select>
          {semPDF && <small className="destino-chave">Coloque um PDF na pasta de currículos (Perfil › Currículos).</small>}
        </label>
        <label className="campo"><span>Idioma</span>
          <select value={c.idioma} onChange={ev => { const v = ev.target.value; mudar(x => { x.idioma = v; }); }}>
            <option value="pt">Português</option><option value="en">Inglês</option>
          </select>
        </label>
      </div>

      <fieldset><legend>Contato</legend>
        <div className="duas">
          {([['nome', 'Nome', 'text'], ['email', 'E-mail', 'email'], ['telefone', 'Telefone', 'tel'], ['cidade', 'Cidade', 'text']] as const).map(([k, nome, tipo]) => (
            <label key={k} className="campo"><span>{nome}</span>
              <input type={tipo} value={c.contato?.[k] ?? ''} onChange={ev => { const v = ev.target.value; mudar(x => { x.contato = { ...(x.contato ?? {}), [k]: v }; }); }} />
            </label>
          ))}
        </div>
      </fieldset>

      <fieldset><legend>Pretensão</legend>
        <div className="duas">
          <label className="campo"><span>Tipo</span>
            <select value={pretensao.tipo ?? ''} onChange={ev => { const v = ev.target.value; mudar(x => { x.pretensao = v ? { ...(x.pretensao ?? { moeda: 'BRL' }), tipo: v } : null; }); }}>
              <option value="">Sem pretensão</option><option value="CLT">CLT</option><option value="PJ">PJ</option><option value="contractor">Contractor</option>
            </select>
          </label>
          {c.pretensao && (
            <>
              <label className="campo"><span>Moeda</span>
                <select value={pretensao.moeda ?? 'BRL'} onChange={ev => { const v = ev.target.value; mudar(x => { x.pretensao = { ...(x.pretensao ?? {}), moeda: v }; }); }}>
                  <option value="BRL">Real (BRL)</option><option value="USD">Dólar (USD)</option><option value="EUR">Euro (EUR)</option>
                </select>
              </label>
              <label className="campo"><span>Valor por mês</span>
                <input type="number" min="1" inputMode="decimal" value={pretensao.valor ?? ''} onChange={ev => { const v = ev.target.valueAsNumber; mudar(x => { x.pretensao = { ...(x.pretensao ?? {}), valor: Number.isFinite(v) ? v : undefined }; }); }} />
              </label>
            </>
          )}
        </div>
      </fieldset>

      <fieldset><legend>Respostas</legend>
        {c.respostas.length === 0 && <p className="pequeno mudo">Sem respostas. Só entra o que você escrever ou tiver nas respostas padrão do perfil.</p>}
        <ol className="respostas-form">
          {c.respostas.map((r, i) => (
            <li key={i}>
              <input type="text" aria-label={`Pergunta ${i + 1}`} value={r.pergunta} onChange={ev => { const v = ev.target.value; mudar(x => { x.respostas[i] = { ...x.respostas[i], pergunta: v, origem: 'pessoa' }; }); }} />
              <input type="text" aria-label={`Resposta ${i + 1}`} value={r.resposta} onChange={ev => { const v = ev.target.value; mudar(x => { x.respostas[i] = { ...x.respostas[i], resposta: v, origem: 'pessoa' }; }); }} />
              <span className={`chip origem-${r.origem}`}>{ORIGENS[r.origem] ?? r.origem}</span>
              <button className="botao-fantasma p" type="button" aria-label={`Remover resposta ${i + 1}`} onClick={() => mudar(x => { x.respostas.splice(i, 1); })}>Remover</button>
            </li>
          ))}
        </ol>
        <div><button className="botao-fantasma p" type="button" onClick={() => mudar(x => { x.respostas.push({ pergunta: '', resposta: '', origem: 'pessoa' }); })}>Adicionar resposta</button></div>
      </fieldset>

      <label className="campo"><span>Mensagem</span>
        <textarea value={c.mensagem ?? ''} placeholder="Opcional: texto para o recrutador" onChange={ev => { const v = ev.target.value; mudar(x => { x.mensagem = v; }); }} />
      </label>

      <div className="acoes">
        <button className="botao" type="submit" disabled={!!ocupado || semPDF}>{ocupado ? 'Salvando…' : rotulo}</button>
        <button className="botao-fantasma" type="button" onClick={onCancelar}>Cancelar</button>
      </div>
    </form>
  );
}
