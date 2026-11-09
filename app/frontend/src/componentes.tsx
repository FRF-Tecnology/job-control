// Peças pequenas usadas em várias telas.
import type { ReactNode } from 'react';
import { nomeEtapa, safeHref } from './util';

// Link para fora do app, em outra janela. Sem endereço http(s), mostra `semLink` (ou nada).
export function LinkExterno({ url, className, children, semLink = null }: { url: unknown; className?: string; children: ReactNode; semLink?: ReactNode }) {
  const href = safeHref(url);
  if (!href) return <>{semLink}</>;
  return <a className={className} href={href} target="_blank" rel="noopener noreferrer">{children}</a>;
}

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

// azul: destaque (ação principal); ambar: algo pede a pessoa (ex.: IA em uso não pronta).
export function Convite({ titulo, texto, children, azul, ambar }: { titulo: ReactNode; texto?: ReactNode; children?: ReactNode; azul?: boolean; ambar?: boolean }) {
  return (
    <div className={`convite${azul ? ' azul' : ''}${ambar ? ' ambar' : ''}`}>
      <div>
        <strong>{titulo}</strong>
        {texto != null && <p style={{ margin: 0 }}>{texto}</p>}
      </div>
      {children}
    </div>
  );
}
