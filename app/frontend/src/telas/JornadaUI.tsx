import { useEffect, useId, useRef, type ReactNode } from 'react';
import { useApp } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import { MarcaEmpresa as SimboloEmpresa } from '../app/MarcaEmpresa';
import '../estilo/jornada.css';

export type VagaJornada = {
  id: string; titulo: string; empresa: string; etapa: string; subEtapa?: string; motivo?: string;
  notas?: string; local?: string; modelo?: string; perfilBuscaId?: string; plataforma?: string;
  aplicacaoManual?: boolean; candidaturaManualId?: string; urlCandidaturaManual?: string; envio?: {data?: string}; atualizadoEm?: string;
};

export function BotaoRadar() {
  return <a className="btn secondary" href="#/radar"><IconeUI nome="arrow-left" tamanho={16} />Voltar ao radar</a>;
}

export function CabecalhoJornada({ titulo, descricao, acao }: { titulo: string; descricao: string; acao?: ReactNode }) {
  return <header className="j-header"><div><h1>{titulo}</h1><p>{descricao}</p></div>{acao}</header>;
}

export function MarcaEmpresa({ empresa, tamanho }: { empresa: string; tamanho: number }) {
  return <SimboloEmpresa classe="j-marca" nome={empresa} tamanho={tamanho}/>;
}

export function TituloVaga({ vaga }: { vaga: VagaJornada }) {
  const { estado } = useApp();
  const nome = estado.perfisBusca?.find((p: { id: string }) => p.id === (vaga.perfilBuscaId ?? 'original'))?.nome
    ?? (!vaga.perfilBuscaId || vaga.perfilBuscaId === estado.perfilBuscaAtivo ? estado.perfil?.cargo : '');
  return <div className="j-job-heading"><MarcaEmpresa empresa={vaga.empresa} tamanho={42} />
    <div><span className="j-company">{[vaga.empresa, nome].filter(Boolean).join(' · ')}</span><h3>{vaga.titulo}</h3></div></div>;
}

export function VazioJornada({ icone, titulo, texto, acao }: { icone: string; titulo: string; texto: string; acao: ReactNode }) {
  return <div className="j-empty"><span className="j-empty-icon"><IconeUI nome={icone} tamanho={25} /></span>
    <h3>{titulo}</h3><p>{texto}</p>{acao}</div>;
}

export function DialogoJornada({ titulo, fechar, children }: { titulo: string; fechar: () => void; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  const id = useId();
  useEffect(() => {
    const d = ref.current!;
    const origem = document.activeElement as HTMLElement | null;
    if (d.showModal) d.showModal(); else d.setAttribute('open', '');
    return () => { if (d.open) d.close?.(); origem?.focus({preventScroll:true}); };
  }, []);
  return <dialog className="j-modal" ref={ref} aria-labelledby={id} onCancel={ev => {ev.preventDefault(); fechar();}}>
    <header><h2 id={id}>{titulo}</h2><button type="button" aria-label="Fechar" className="btn quiet" onClick={fechar}><IconeUI nome="x" tamanho={18} /></button></header>
    <div className="j-dialog">{children}</div></dialog>;
}
