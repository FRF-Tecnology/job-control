// Markdown mínimo dos textos legais (docs/legal): títulos (#), citação (>), listas (- ou *), parágrafos e **negrito**.
// Só vira elementos React (texto como texto): nada do arquivo chega ao DOM como HTML.
import { Fragment, type ReactNode } from 'react';

function emLinha(t: string): ReactNode[] {
  return t.split(/(\*\*[^*]+\*\*)/g).filter(Boolean).map((p, i) =>
    p.length > 4 && p.startsWith('**') && p.endsWith('**')
      ? <strong key={i}>{p.slice(2, -2)}</strong>
      : <Fragment key={i}>{p.replace(/\*\*/g, '')}</Fragment>);
}

const TITULOS = ['h3', 'h4', 'h5'] as const;

export function Markdown({ texto }: { texto: string }) {
  const saida: ReactNode[] = [];
  let paragrafo: string[] = [];
  let lista: string[] = [];
  let citacao: string[] = [];
  const fechar = () => {
    const k = saida.length;
    if (paragrafo.length) saida.push(<p key={k}>{emLinha(paragrafo.join(' '))}</p>);
    if (lista.length) saida.push(<ul key={k}>{lista.map((l, i) => <li key={i}>{emLinha(l)}</li>)}</ul>);
    if (citacao.length) saida.push(<blockquote key={k}>{emLinha(citacao.join(' '))}</blockquote>);
    paragrafo = []; lista = []; citacao = [];
  };
  for (const linha of texto.replace(/\r\n?/g, '\n').split('\n')) {
    const t = linha.trim();
    const titulo = /^(#{1,6})\s+(.*)$/.exec(t);
    const item = /^[-*]\s+(.*)$/.exec(t);
    const cita = /^>\s?(.*)$/.exec(t);
    if (!t) fechar();
    else if (titulo) {
      fechar();
      const Tag = TITULOS[Math.min(titulo[1].length, TITULOS.length) - 1];
      saida.push(<Tag key={saida.length}>{emLinha(titulo[2])}</Tag>);
    } else if (item) {
      if (paragrafo.length || citacao.length) fechar();
      lista.push(item[1]);
    } else if (cita) {
      if (paragrafo.length || lista.length) fechar();
      citacao.push(cita[1]);
    } else {
      if (lista.length || citacao.length) fechar();
      paragrafo.push(t);
    }
  }
  fechar();
  return <>{saida}</>;
}
