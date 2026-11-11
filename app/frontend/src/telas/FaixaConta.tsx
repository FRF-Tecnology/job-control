// Faixa da conta no topo das telas (Task 7): quanto falta do teste ou a frase da trava, com o caminho para assinar.
// E a trava das telas: o conteúdo fica num <fieldset disabled> quando a licença não deixa agir (ver continua).
import { useEffect, useState, type ReactNode } from 'react';
import { textoFaixa, type Situacao } from '../app/conta';

// Telas que continuam liberadas com a trava: o perfil (sair e excluir a conta, apagar os dados), a IA e a assinatura.
export const LIVRES_DA_TRAVA = ['perfil', 'ia', 'assinatura'];

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
  const travada = !!situacao?.motivo;
  return (
    <div className={`faixa-conta${travada ? ' travada' : ''}`} role="status">
      {texto.endsWith(sufixo)
        ? <>{texto.slice(0, -sufixo.length)} · <a href="#/assinatura">Assinar</a></>
        : <>{texto}{situacao?.motivo === 'sem-internet'
          ? <> <button className="botao-fantasma p" type="button" onClick={() => { reler(); }}>Tentar de novo</button></>
          : situacao?.motivo !== 'sem-conta' && <> <a href="#/assinatura">Ver a assinatura</a></>}</>}
    </div>
  );
}

// ativa false (build sem a nuvem): nenhuma moldura, a tela fica como antes.
export function TravaDaTela({ rota, ativa, podeAgir, children }: { rota: string; ativa: boolean; podeAgir: boolean; children: ReactNode }) {
  if (!ativa || LIVRES_DA_TRAVA.includes(rota)) return <>{children}</>;
  return <fieldset className="trava" disabled={!podeAgir}>{children}</fieldset>;
}
