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

// O robô do Applyer (só enfeite): flutua, pisca e olha para os lados. Os degradês ficam no próprio SVG.
export function Robo({ tamanho = 112, feliz = false }: { tamanho?: number; feliz?: boolean }) {
  return (
    <svg className="robo" viewBox="0 0 140 160" width={tamanho} height={Math.round(tamanho * 160 / 140)} aria-hidden="true">
      <defs>
        <linearGradient id="robo-casco" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#FFFFFF" /><stop offset="1" stopColor="#D5DBF2" /></linearGradient>
        <linearGradient id="robo-visor" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#3C40A8" /><stop offset="1" stopColor="#14163F" /></linearGradient>
        <linearGradient id="robo-orelha" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#B6F06A" /><stop offset="1" stopColor="#2FB86A" /></linearGradient>
        <linearGradient id="robo-fogo" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stopColor="#A8F4FF" /><stop offset="1" stopColor="#3FD8F5" stopOpacity="0" /></linearGradient>
      </defs>
      <ellipse className="r-sombra" cx="70" cy="154" rx="26" ry="4" fill="#3B3DBF" opacity="0.16" />
      <g className="r-flutua">
        <path className="r-fogo" d="M52 110 Q70 152 88 110 Z" fill="url(#robo-fogo)" />
        <g className="r-orelha">
          <path d="M22 52 L28 14 Q31 7 37 13 L56 36 Z" fill="url(#robo-orelha)" />
          <path d="M118 52 L112 14 Q109 7 103 13 L84 36 Z" fill="url(#robo-orelha)" />
          <path d="M30 40 L33 21 L46 35 Z" fill="#8A9BFF" />
          <path d="M110 40 L107 21 L94 35 Z" fill="#8A9BFF" />
        </g>
        <rect x="16" y="26" width="108" height="88" rx="40" fill="url(#robo-casco)" />
        <path d="M24 92 Q26 108 46 112 L40 100 Z" fill="#7ED957" opacity="0.9" />
        <path d="M116 92 Q114 108 94 112 L100 100 Z" fill="#7ED957" opacity="0.9" />
        <rect x="42" y="106" width="56" height="12" rx="6" fill="#E6EAF8" />
        <rect x="23" y="39" width="94" height="60" rx="27" fill="#A9B4FF" />
        <rect x="27" y="43" width="86" height="52" rx="23" fill="url(#robo-visor)" />
        <path d="M38 52 Q60 45 86 49" stroke="#FFFFFF" strokeOpacity="0.35" strokeWidth="4" strokeLinecap="round" fill="none" />
        <g className="r-olhar">
          {feliz
            ? <g className="r-olhos"><path d="M48 72 Q54 62 60 72" stroke="#5CF0FF" strokeWidth="5" strokeLinecap="round" fill="none" /><path d="M80 72 Q86 62 92 72" stroke="#5CF0FF" strokeWidth="5" strokeLinecap="round" fill="none" /></g>
            : <g className="r-olhos"><rect x="48" y="58" width="12" height="23" rx="6" fill="#5CF0FF" /><rect x="80" y="58" width="12" height="23" rx="6" fill="#5CF0FF" /></g>}
        </g>
      </g>
    </svg>
  );
}
