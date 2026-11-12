import { useEffect, useRef, useState, type ReactNode } from 'react';
import { IconeUI } from '../app/IconeUI';
import { useApp } from '../app/contexto';
import { SeletorPerfil } from './SeletorPerfil';
import { ConexaoOnboarding } from './ConexaoOnboarding';
import { lerArquivo } from './PrimeiroAcesso';
import arteCV from '../../../../site/artes/curriculo-verificado.svg';
import '../estilo/perfil-espaco.css';

const texto=(v:unknown)=> typeof v==='string'?v:'';
const lista=(v:unknown):string[]=>Array.isArray(v)?v.filter(x=>typeof x==='string'): [];
const separar=(s:string)=>[...new Set(s.split(/[,;\n]/).map(x=>x.trim()).filter(Boolean))];
const marcaIA:Record<string,string>={Claude:'anthropic',OpenAI:'openai',Gemini:'google',Grok:'xai'};
function leitura(p:any){
  const f=p.filtros||{}, faixa=p.faixaSalarial ?? {min:p.pretensaoUSD};
  return {nome:texto(p.nome), cargo:texto(p.cargo), ingles:texto(p.ingles), foco:p.foco||'ambos',
    principal:lista(p.stackPrincipal).join(', '), secundaria:lista(p.stackSecundaria).join(', '),
    niveis:lista(f.niveis), modelos:lista(f.modelo), pular:lista(f.pularSe).join(', '),
    moeda:faixa.moeda||'USD',min:String(faixa.min??''),max:String(faixa.max??'')};
}
function Cartao({titulo,icone,children}:{titulo:string;icone:string;children:ReactNode}){
  return <section className="panel pr-card"><div className="pr-card-title"><span className="pr-tile"><IconeUI nome={icone} tamanho={19}/></span><h2>{titulo}</h2></div>{children}</section>;
}
function Opcoes({titulo,base,valor,mudar}:{titulo:string;base:string[];valor:string[];mudar:(v:string[])=>void}){
  return <fieldset className="pr-levels pr-wide"><legend>{titulo}</legend><div>{[...new Set([...base,...valor])].map(v=><label key={v}><input type="checkbox" checked={valor.includes(v)} onChange={e=>mudar(e.target.checked?[...valor,v]:valor.filter(x=>x!==v))}/><span>{v}</span></label>)}</div></fieldset>;
}
export function PerfilDoEspaco(){
  const app=useApp(), id=app.estado.perfilBuscaAtivo || 'original';
  const [dados,setDados]=useState(()=>leitura(app.estado.perfil||{})), [sujo,alterado]=useState(false);
  const [salvando,salvarEstado]=useState(false), [erro,falhar]=useState(''), [alterandoIA,alterarIA]=useState(false);
  const conectado=app.conexaoIA?.verificada && !alterandoIA ? app.conexaoIA.nome : '';
  const [cvStatus,cvMensagem]=useState(''), [cvOcupado,cvOcupar]=useState(false), [cvErro,cvFalha]=useState('');
  const pdf=useRef<HTMLInputElement>(null), ia=useRef<HTMLElement>(null), ativa=useRef(true);
  useEffect(()=>{ativa.current=true;return()=>{ativa.current=false}},[]);
  useEffect(()=>{
    const foco=()=>{if(location.hash.endsWith('?aba=conexao')){ia.current?.scrollIntoView?.({block:'center'});ia.current?.focus({preventScroll:true})}};
    foco();window.addEventListener('hashchange',foco);return()=>window.removeEventListener('hashchange',foco);
  },[]);
  const set=<K extends keyof typeof dados>(k:K,v:typeof dados[K])=>{setDados(antes=>({...antes,[k]:v}));alterado(true);falhar('')};
  const url='/api/v1/perfil?perfilBuscaId='+encodeURIComponent(id);
  async function salvar(){
    if(salvando) return;
    if(!dados.cargo.trim()){falhar('Informe o cargo que deseja buscar.');document.getElementById('f-cargo')?.focus();return;}
    if(dados.min && dados.max && Number(dados.max)<Number(dados.min)){falhar('O máximo não pode ser menor que o mínimo.');return;}
    salvarEstado(true);falhar('');
    try {
      await app.api(url,'PATCH',{nome:dados.nome.trim(),cargo:dados.cargo.trim(),ingles:dados.ingles,foco:dados.foco,
        stackPrincipal:separar(dados.principal),stackSecundaria:separar(dados.secundaria),
        filtros:{niveis:dados.niveis,modelo:dados.modelos,pularSe:separar(dados.pular)},pretensaoUSD:null,
        faixaSalarial:(dados.min||dados.max)?{moeda:dados.moeda,...dados.min&&{min:Number(dados.min)},...dados.max&&{max:Number(dados.max)}}:null});
      app.limpar();await app.recarregar();if(ativa.current) {alterado(false);app.aviso('Perfil salvo.');}
    }catch(e){if(ativa.current) falhar((e as Error).message||'Não foi possível salvar o perfil.')}
    finally{if(ativa.current) salvarEstado(false)}
  }
  async function selecionar(f?:File){
    if(!f||cvOcupado)return;
    cvFalha('');cvMensagem('');
    if(!/\.pdf$/i.test(f.name)||!f.size||f.size>20*1024*1024){cvFalha('Escolha um PDF de até 20 MB.');return}
    cvOcupar(true);
    try{
      const curriculoBase64 = await lerArquivo(f,true);
      await app.api('/api/v1/perfil/curriculo?perfilBuscaId='+encodeURIComponent(id),'POST',{curriculoBase64,nomeArquivo:f.name},{'X-Applyer-Onboarding':'1'});
      await app.recarregar();if(ativa.current)cvMensagem(f.name+' salvo neste perfil.');
    }catch(e){if(ativa.current)cvFalha((e as Error).message||'Não foi possível salvar o PDF.')}
    finally{if(ativa.current)cvOcupar(false)}
  }
  type Campo='nome'|'cargo'|'principal'|'secundaria'|'ingles'|'foco'|'pular'|'moeda'|'min'|'max';
  function campo(k:Campo,label:string,op?:{lista?:[string,string][];wide?:boolean;tipo?:'number';dica?:string}){
    return <div className={'pr-field'+(op?.wide?' pr-wide':'')}><label htmlFor={'f-'+k}>{label}</label>{op?.lista?
      <select form="form-perfil-ativo" id={'f-'+k} value={dados[k]} onChange={e=>set(k,e.target.value)}>{op.lista.map(([v,r])=><option key={v} value={v}>{r}</option>)}</select>:
      <input id={'f-'+k} form="form-perfil-ativo" maxLength={k==='principal'||k==='secundaria'||k==='pular'?350:120} type={op?.tipo||'text'} min={op?.tipo?'.01':undefined} step={op?.tipo?'.01':undefined} placeholder={op?.dica} value={dados[k]} onChange={e=>set(k,e.target.value)}/>}</div>;
  }
  const caminho=texto(app.estado.perfil?.curriculos?.PT || app.estado.perfil?.curriculos?.EN);
  const rotulo=app.estado.perfil?.curriculoExibicao;
  const cv=(rotulo?.arquivo===caminho && texto(rotulo?.nome)) || caminho.replace(/^.*[/\\]/,'');
  return <div className="pr-page" data-perfil-alterado={sujo}>
    <header className="pr-heading"><div><h1>Perfil de busca</h1><p>Ajuste seu perfil profissional, as condições de trabalho e os dados usados na triagem.</p></div></header>
    <SeletorPerfil/>
    <div className="pr-edit-grid"><div className="pr-stack">
      <Cartao titulo="Perfil profissional" icone="user"><div className="pr-fields">
        {campo('nome','Nome')}{campo('cargo','Cargo de interesse',{dica:'Ex.: Engenharia de IA'})}
        <Opcoes titulo="Níveis de experiência aceitos" base={['Estágio','Júnior','Pleno','Sênior','Liderança']} valor={dados.niveis} mudar={v=>set('niveis',v)}/>
        {campo('principal','Habilidades principais',{wide:true,dica:'Ex.: Java, Spring Boot'})}
        {campo('secundaria','Habilidades complementares')}
        {campo('ingles','Nível de inglês',{lista:[['','Selecione'],...[...new Set(['Não falo inglês','Básico','Intermediário','Avançado','Fluente',dados.ingles].filter(Boolean))].map(v=>[v,v] as [string,string])]})}
      </div></Cartao>
      <Cartao titulo="Filtros de busca" icone="sliders"><div className="pr-fields">
        {campo('foco','Mercado',{lista:[['nacional','Brasil'],['internacional','Exterior'],['ambos','Brasil e exterior']]})}
        <Opcoes titulo="Modelo de trabalho" base={['Remoto','Híbrido','Presencial']} valor={dados.modelos} mudar={v=>set('modelos',v)}/>
        {campo('pular','Pular vagas com estes termos',{wide:true})}
        <fieldset className="pr-pay pr-wide"><legend>Faixa salarial mensal <small>Opcional</small></legend><div>
          {campo('moeda','Moeda',{lista:[['USD','USD'],['BRL','BRL'],['EUR','EUR']]})}{campo('min','Mínimo',{tipo:'number'})}{campo('max','Máximo',{tipo:'number'})}
        </div></fieldset>
      </div></Cartao>
    </div><div className="pr-stack"><section ref={ia} tabIndex={-1} aria-label="Configuração de IA">
      <Cartao titulo="Conexão de IA" icone="sparkles">{conectado?<><p className={'pr-connected pr-marca-ia '+(marcaIA[conectado]||'')}>{marcaIA[conectado]&&<span aria-hidden="true" className="pa-logotipo"/>}{conectado} conectado</p><button type="button" className="btn secondary" onClick={()=>alterarIA(true)}>Alterar conexão</button></>:
        <ConexaoOnboarding aoConectar={()=>alterarIA(false)}/>}</Cartao></section>
      <Cartao titulo="Currículo" icone="file">
        <div className="pr-drop" onDragOver={e=>e.preventDefault()} onDrop={e=>{e.preventDefault();void selecionar(e.dataTransfer.files[0]);}}>
          <img src={arteCV} alt="" width="42" height="42"/><div className="pr-file-meta"><strong>{cv?(/^curriculo-[A-Z2-7]{26,}\.pdf$/.test(cv)?'Currículo do perfil':cv):'Arraste seu currículo para cá'}</strong><small>{cv?'PDF salvo neste computador':'PDF de até 20 MB'}</small></div>
          <button type="button" className="btn secondary" disabled={cvOcupado} onClick={()=>pdf.current?.click()}>{cvOcupado?'Salvando…':cv?'Trocar PDF':'Escolher PDF'}</button>
          <input hidden type="file" aria-label="Currículo em PDF" ref={pdf} accept=".pdf,application/pdf" onChange={e=>{void selecionar(e.target.files?.[0]);e.target.value='';}}/>
        </div>{cvErro&&<p role="alert">{cvErro}</p>}{cvStatus&&<p className="pr-connected" role="status">{cvStatus}</p>}
      </Cartao>
    </div></div>
    <form id="form-perfil-ativo" className="pr-footer pr-end" onSubmit={e=>{e.preventDefault();void salvar();}}>
      {erro&&<p role="alert">{erro}</p>}<button className="btn primary" type="submit" disabled={salvando}>{salvando?'Salvando…':'Salvar perfil'}</button>
    </form>
  </div>;
}
