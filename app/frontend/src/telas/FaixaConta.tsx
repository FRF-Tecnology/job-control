// Faixa da conta no topo das telas (Task 7): quanto falta do teste ou a frase da trava, com o caminho para assinar.
// "Tentar de novo" (sem internet) chama reler: na casca, o POST /conta/atualizar (pede a licença à nuvem).
// Os botões de escrita travados apontam para o texto dela (aria-describedby = ID_FAIXA; ruling A13).
import { useEffect, useState } from 'react';
import { TEXTO_VERIFICANDO, textoFaixa, type Situacao } from '../app/conta';
import { ID_FAIXA } from '../app/contexto';

export function FaixaConta({ situacao, agora, reler }: { situacao: Situacao | null | undefined; agora?: Date; reler: () => Promise<void> }) {
  const [relogio, setRelogio] = useState(() => new Date());
  useEffect(() => {
    if (agora) return;
    const t = setInterval(() => setRelogio(new Date()), 30_000);
    return () => clearInterval(t);
  }, [agora]);
  const texto = textoFaixa(situacao, agora ?? relogio);
  if (!texto) return null;
  const sufixo = ' · Assinar';
  const conferindo = texto === TEXTO_VERIFICANDO; // ao abrir: neutro, sem "Tentar de novo"
  const travada = !!situacao?.motivo && !conferindo;
  return (
    <div className={`faixa-conta${travada ? ' travada' : ''}`} role="status">
      {texto.endsWith(sufixo)
        ? <><span id={ID_FAIXA}>{texto.slice(0, -sufixo.length)}</span> · <a href="#/assinatura">Assinar</a></>
        : <><span id={ID_FAIXA}>{texto}</span>{conferindo ? null : situacao?.motivo === 'sem-internet'
          ? <> <button className="botao-fantasma p" type="button" onClick={() => { reler(); }}>Tentar de novo</button></>
          : situacao?.motivo !== 'sem-conta' && <> <a href="#/assinatura">Ver a assinatura</a></>}</>}
    </div>
  );
}
