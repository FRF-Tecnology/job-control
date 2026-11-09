// Diálogo genérico: pergunta com opções e/ou campo, ou a folha "Mais" do celular (porte de perguntar() e abrirMais()).
import { useEffect, useRef, useState } from 'react';
import type { Pergunta } from './contexto';

export type PedidoDialogo =
  | { tipo: 'pergunta'; pergunta: Pergunta; resolver: (v: string | null) => void }
  | { tipo: 'mais'; telas: { rota: string; nome: string }[] };

export function Dialogo({ pedido, fechar }: { pedido: PedidoDialogo | null; fechar: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (pedido && !d.open) d.showModal?.();
    if (!pedido && d.open) d.close();
  }, [pedido]);
  return (
    <dialog id="dialogo" ref={ref} onCancel={ev => { ev.preventDefault(); responder(null); }}>
      {pedido?.tipo === 'pergunta' && <FormPergunta key={pedido.pergunta.titulo + (pedido.pergunta.valor ?? '')} p={pedido.pergunta} responder={v => responder(v)} />}
      {pedido?.tipo === 'mais' && (
        <form method="dialog" onSubmit={ev => { ev.preventDefault(); fechar(); }}>
          <h2>Mais</h2>
          <nav className="folha-mais" aria-label="Outras seções">
            {pedido.telas.map(t => <a key={t.rota} href={`#/${t.rota}`} onClick={fechar}>{t.nome}<span className="mudo">›</span></a>)}
          </nav>
          <div className="acoes" style={{ justifyContent: 'flex-end' }}><button className="botao-fantasma" type="submit">Fechar</button></div>
        </form>
      )}
    </dialog>
  );

  function responder(v: string | null) {
    if (pedido?.tipo === 'pergunta') pedido.resolver(v);
    fechar();
  }
}

function FormPergunta({ p, responder }: { p: Pergunta; responder: (v: string | null) => void }) {
  const { titulo, texto, opcoes = [], campo, valor = '', confirmar = 'Confirmar', multilinha = false } = p;
  const [entrada, setEntrada] = useState(valor);
  const campoRef = useRef<HTMLInputElement & HTMLTextAreaElement>(null);
  const primeiro = useRef<HTMLButtonElement>(null);
  useEffect(() => { (campoRef.current ?? primeiro.current)?.focus(); }, []);

  const teclar = (ev: React.KeyboardEvent) => {
    const n = Number(ev.key);
    if (!campo && n >= 1 && n <= opcoes.length && !(ev.target as Element).matches('input,textarea')) { ev.preventDefault(); responder(opcoes[n - 1]); }
  };
  const enviar = (ev: React.FormEvent) => {
    ev.preventDefault();
    const v = campo ? entrada.trim() : 'ok';
    if (!v) return campoRef.current?.focus();
    responder(v);
  };
  const Campo = multilinha ? 'textarea' : 'input';
  return (
    <form method="dialog" onSubmit={enviar} onKeyDown={teclar}>
      <h2>{titulo}</h2>
      {texto && <p className="mudo" style={{ margin: 0 }}>{texto}</p>}
      {opcoes.length > 0 && (
        <div className="motivos">
          {opcoes.map((o, i) => (
            <button key={o} className="botao-fantasma" type="button" ref={i === 0 && !campo ? primeiro : undefined}
              onClick={() => { if (campo) { setEntrada(o); campoRef.current?.focus(); } else responder(o); }}>
              {i < 9 && <kbd>{i + 1}</kbd>}{o}
            </button>
          ))}
        </div>
      )}
      {campo && <Campo ref={campoRef} type="text" placeholder={campo} value={entrada} onChange={e => setEntrada(e.target.value)} />}
      <div className="acoes" style={{ justifyContent: 'flex-end' }}>
        <button className="botao-fantasma" type="button" ref={!opcoes.length && !campo ? primeiro : undefined} onClick={() => responder(null)}>Cancelar</button>
        {(campo || !opcoes.length) && <button className="botao" type="submit">{confirmar}</button>}
      </div>
    </form>
  );
}
