// Limite diário por plataforma (jc-a2y.54): cada cartão mostra o uso de hoje e deixa ajustar o limite, de 1 a 40
// (padrão 30), com confirmação. PUT /candidaturas/limites/{plataforma} {limite}, só pela tela. A gravação não muda a
// /versao, então o cartão usa a resposta do PUT.
import { useEffect, useRef, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';

export type Limite = { limite: number; usadas: number; renovaEm: string };
export const MINIMO = 1, MAXIMO = 40, PADRAO = 30;
const FORA = `O limite vai de ${MINIMO} a ${MAXIMO} por dia.`;

// aprovadas: as que ainda vão sair hoje. O servidor conta com elas no limite (jc-a2y.6.13), então a tela também.
export function LimiteCartao({ plataforma, nome, na, limite, renova, onSalvo, aprovadas = 0 }: {
  plataforma: string; nome: string; na: string; limite: Limite; renova: string; onSalvo: (l: Limite) => void; aprovadas?: number;
}) {
  const sobra = Math.max(0, limite.limite - limite.usadas - aprovadas);
  const app = useApp();
  const [aberto, setAberto] = useState(false);
  const [valor, setValor] = useState(String(limite.limite));
  const [erro, setErro] = useState('');
  const [ocupado, setOcupado] = useState(false);
  const id = `limite-${plataforma}`;
  // Abrir leva o foco ao campo; fechar devolve ao botão (teclado e leitor de tela; jc-a2y.59).
  const campo = useRef<HTMLInputElement>(null);
  const botao = useRef<HTMLButtonElement>(null);
  const mexeu = useRef(false);
  useEffect(() => {
    if (!mexeu.current) return;
    (aberto ? campo.current : botao.current)?.focus();
  }, [aberto]);

  const abrir = () => { mexeu.current = true; setValor(String(limite.limite)); setErro(''); setAberto(true); };
  const fechar = () => { setAberto(false); setErro(''); app.limpar(); };

  async function salvar(ev: React.FormEvent) {
    ev.preventDefault();
    const n = Number(valor);
    if (!Number.isInteger(n) || n < MINIMO || n > MAXIMO) { setErro(FORA); return; }
    if (n === limite.limite) { fechar(); return; }
    const ok = await app.perguntar({
      titulo: `Mudar o limite ${na} para ${n} por dia?`,
      texto: `O app envia no máximo ${n} candidatura${n === 1 ? '' : 's'} por dia ${na}. Hoje já foram ${limite.usadas}. O padrão é ${PADRAO}.`,
      confirmar: 'Mudar limite',
    });
    if (!ok) return;
    setOcupado(true); setErro('');
    try {
      const novo: Limite = await app.api(`/api/v1/candidaturas/limites/${plataforma}`, 'PUT', { limite: n });
      onSalvo(novo);
      fechar();
      app.aviso(`Limite ${na}: ${novo.limite} por dia.`);
    } catch (e) {
      setErro(e instanceof ErroApi && e.status === 400 ? FORA
        : e instanceof ErroApi && e.status === 403 ? 'Só você pode mudar o limite, por esta tela.'
        : 'Não deu para salvar. Tente de novo.');
    } finally { setOcupado(false); }
  }

  return (
    <div className={`limite${sobra === 0 ? ' cheio' : ''}`}>
      <strong>{nome}</strong>
      <span className="num">{`${limite.usadas} de ${limite.limite}`}</span>
      {aprovadas > 0 && <span className="pequeno mudo">{`${aprovadas === 1 ? '1 aprovada esperando' : `${aprovadas} aprovadas esperando`} envio · ${sobra === 1 ? 'sobra 1' : `sobram ${sobra}`} hoje`}</span>}
      <span className="pequeno mudo">{renova}</span>
      {aberto ? (
        <form className="limite-form" onSubmit={salvar} noValidate onKeyDown={e => { if (e.key === 'Escape') { e.preventDefault(); fechar(); } }}>
          <label htmlFor={id}>{`Limite por dia, de ${MINIMO} a ${MAXIMO}`}</label>
          <input ref={campo} id={id} type="number" inputMode="numeric" min={MINIMO} max={MAXIMO} step={1} value={valor}
            aria-invalid={!!erro} aria-describedby={erro ? `${id}-dica ${id}-erro` : `${id}-dica`} onChange={e => { setValor(e.target.value); setErro(''); }} />
          <small id={`${id}-dica`} className="mudo">{`Padrão: ${PADRAO}.`}</small>
          {erro && <p id={`${id}-erro`} className="erro-resposta" role="alert">{erro}</p>}
          <div className="acoes">
            <button className="botao p" type="submit" disabled={ocupado}>Salvar</button>
            <button className="botao-fantasma p" type="button" onClick={fechar}>Cancelar</button>
          </div>
        </form>
      ) : (
        <div><button ref={botao} className="botao-fantasma p" type="button" onClick={abrir} aria-label={`Ajustar limite ${na}`}>Ajustar limite</button></div>
      )}
    </div>
  );
}
