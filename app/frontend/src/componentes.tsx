// Peças pequenas usadas em várias telas.
import type { ReactNode } from 'react';
import { nomeEtapa } from './util';

// Ícone de linha (24x24) a partir dos caminhos SVG.
export function Icone({ d, children }: { d?: string; children?: ReactNode }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      {d ? <path d={d} /> : children}
    </svg>
  );
}

export function EtapaChip({ etapa }: { etapa: string }) {
  return <span className={`etapa e-${etapa}`}>{nomeEtapa(etapa)}</span>;
}

export function Convite({ titulo, texto, children, azul }: { titulo: ReactNode; texto?: ReactNode; children?: ReactNode; azul?: boolean }) {
  return (
    <div className={`convite${azul ? ' azul' : ''}`}>
      <div>
        <strong>{titulo}</strong>
        {texto != null && <p style={{ margin: 0 }}>{texto}</p>}
      </div>
      {children}
    </div>
  );
}
