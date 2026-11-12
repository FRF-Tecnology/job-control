import { useEffect, useRef, useState, type ReactNode } from 'react';
import { IconeUI } from './IconeUI';
import palavra from '../arte/primeiro-acesso/applyer-inversa.svg';
import palavraClara from '../arte/primeiro-acesso/applyer.svg';
import claude from '../../../../site/marcas/claude.svg';

const rotas = [
  ['radar','Radar de vagas','radar'], ['lista','Lista do dia','list'], ['quadro','Quadro','board'],
  ['perfil','Perfil e IA','sliders'],
] as const;
type Props = {
  rota:string; titulo:string; pessoa:string; ferramentas:{rota:string;nome:string}[];
  quantidades:{radar:number;lista:number}; status:string; falha:boolean; envio:boolean;
  ia:{texto:string; verificada:boolean; claude:boolean}; tema:()=>void; children:ReactNode;
};

export function Espaco({rota,titulo,pessoa,ferramentas,quantidades,status,falha,envio,ia,tema,children}:Props) {
  const [menu,abrirMenu]=useState(false), [extras,abrirExtras]=useState(false);
  const trilho=useRef<HTMLElement>(null), botaoMenu=useRef<HTMLButtonElement>(null);
  const barra=useRef<HTMLDivElement>(null), ferramentasRef=useRef<HTMLButtonElement>(null);
  useEffect(()=>{
    const fechar=()=>{abrirMenu(false);abrirExtras(false);};
    window.addEventListener('hashchange',fechar);return ()=>window.removeEventListener('hashchange',fechar);
  },[]);
  useEffect(()=>{
    if (!menu && !extras) return;
    if(menu) trilho.current?.querySelector<HTMLElement>('[aria-current="page"], a')?.focus();
    const tecla=(e:KeyboardEvent)=>{
      if(e.key==='Escape'){abrirMenu(false);abrirExtras(false);(menu?botaoMenu:ferramentasRef).current?.focus();}
      if(e.key==='Tab' && menu){
        const elems=trilho.current?.querySelectorAll<HTMLElement>('a[href],button:not([disabled])');
        if(!elems?.length) return;
        if((!e.shiftKey && document.activeElement===elems[elems.length-1])||(e.shiftKey && document.activeElement===elems[0])){
          e.preventDefault();elems[e.shiftKey?elems.length-1:0].focus();
        }
      }
    };
    const fora=(e:PointerEvent)=>{if(extras && !barra.current?.contains(e.target as Node)) abrirExtras(false);};
    document.addEventListener('keydown',tecla);document.addEventListener('pointerdown',fora);
    return ()=>{document.removeEventListener('keydown',tecla);document.removeEventListener('pointerdown',fora);};
  },[menu,extras]);
  function destino([id,nome,icone]:typeof rotas[number]){
    const ativo=id===rota, total=id==='radar'||id==='lista'?quantidades[id]:null;
    return <a key={id} href={'#/'+id} className={'nav-link'+(ativo?' active':'')} aria-current={ativo?'page':undefined} onClick={()=>abrirMenu(false)}>
      <IconeUI nome={icone}/><span>{nome}</span>{total!==null && <span className="nav-count" aria-hidden="true">{total}</span>}</a>;
  }
  const palavras=pessoa.trim().split(/\s+/);
  const iniciais=palavras.length && palavras[0] ? palavras[0][0]+(palavras.length>1?palavras[palavras.length-1][0]:'') : 'A';
  return <div className="espaco">
    <a className="skip-link" href="#conteudo" onClick={e=>{e.preventDefault();document.getElementById('conteudo')?.focus();}}>Ir para o conteúdo</a>
    <aside ref={trilho} id="rail" className={'rail'+(menu?' open':'')} aria-label="Barra lateral">
      <a className="wordmark" href="#/radar" onClick={()=>abrirMenu(false)} aria-label="Applyer — início"><img src={palavra} alt="Applyer." className="logo-noturna"/><img src={palavraClara} alt="" className="logo-diurna"/></a>
      <div className="nav-group"><p className="nav-heading">Espaço de trabalho</p><nav aria-label="Navegação principal">{rotas.slice(0,3).map(destino)}</nav></div>
      <div className="nav-group nav-account"><p className="nav-heading">Preferências</p><nav aria-label="Preferências">{destino(rotas[3])}</nav></div>
      <div className="rail-end"><a className="connection-box" href="#/perfil?aba=conexao" onClick={()=>abrirMenu(false)}>
        <span id="connection-icon">{ia.claude?<img src={claude} alt="" width="23" height="23"/>:<IconeUI nome="sparkles" tamanho={19}/>}</span>
        <span><strong>{ia.texto}</strong><small><i className={'status-dot'+(ia.verificada?'':' pendente')} />{ia.verificada?'Verificada nesta sessão':'Verificar conexão'}</small></span>
      </a><div className="person"><span id="avatar" aria-hidden="true">{iniciais.toLocaleUpperCase('pt-BR')}</span><div><strong>{pessoa||'Meu espaço'}</strong><small>Espaço pessoal</small></div></div></div>
    </aside>
    {menu&&<button id="nav-scrim" type="button" aria-label="Fechar navegação" onClick={()=>{abrirMenu(false);botaoMenu.current?.focus();}}/>}
    <div className="workspace"><header className="topbar">
      <div className="breadcrumb"><button ref={botaoMenu} id="mobile-menu" className="icon-btn" type="button" aria-label="Abrir navegação" aria-expanded={menu} aria-controls="rail" onClick={()=>abrirMenu(true)}><IconeUI nome="menu"/></button><span id="areaicon"><IconeUI nome="area" tamanho={15}/></span><span className="crumb-home">Meu espaço</span><span className="crumb-divider">/</span><strong>{titulo}</strong></div>
      <div className="barra-opcoes" ref={barra}>
        {falha&&<span role="status" className="status-dados">{status}</span>}
        {envio&&<a href="#/fila" className="selo-envio">Envio ligado</a>}
        <button type="button" ref={ferramentasRef} className="demo-control" aria-expanded={extras} aria-controls="ferramentas-menu" onClick={()=>abrirExtras(!extras)}><IconeUI nome="sliders" tamanho={15}/>Ferramentas<IconeUI nome="chevron-down" tamanho={14}/></button>
        {extras && <div id="ferramentas-menu"><nav aria-label="Ferramentas avançadas">{ferramentas.map(t=><a href={'#/'+t.rota} key={t.rota} aria-current={t.rota===rota?'page':undefined} onClick={()=>abrirExtras(false)}>{t.nome}</a>)}</nav><button type="button" onClick={tema}>Alternar tema claro/escuro</button>{!falha && <span className="status-dados" role="status">{status}</span>}</div>}
      </div></header>{children}</div>
  </div>;
}
