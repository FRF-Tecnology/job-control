import { useEffect, useRef, useState, type ReactNode } from 'react';
import { IconeUI } from '../app/IconeUI';
import { useApp, useTrava } from '../app/contexto';
import { SeletorPerfil } from './SeletorPerfil';
import { aceitaTrabalhoLocal, cargoIgualAoNome, paises } from '../dominio/localDeTrabalho';
import { DDI } from '../dominio/ddi';
import { ConexaoOnboarding } from './ConexaoOnboarding';
import { SecaoConta } from './Conta';
import { ContaEmail } from './ContaEmail';
import { lerArquivo } from './PrimeiroAcesso';
import arteCV from '../../../../site/artes/curriculo-verificado.svg';
import '../estilo/perfil-espaco.css';

const texto=(v:unknown)=> typeof v==='string'?v:'';
const lista=(v:unknown):string[]=>Array.isArray(v)?v.filter(x=>typeof x==='string'): [];
const separar=(s:string)=>[...new Set(s.split(/[,;\n]/).map(x=>x.trim()).filter(Boolean))];
const marcaIA:Record<string,string>={Claude:'anthropic',OpenAI:'openai',Gemini:'google',Grok:'xai'};
// Sem faixa salva, a moeda segue o mercado (como no primeiro acesso); a pretensão antiga é sempre em USD.
const moedaDoMercado=(foco:string,moedaAtual:string)=>foco==='nacional'?'BRL':foco==='internacional'?'USD':moedaAtual;
const emailValido=(v:string)=>!v.trim()||/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v.trim());
// O endereço do perfil no site (com ou sem https://), no próprio site ou num subdomínio dele, com o caminho do perfil.
// A mesma regra do servidor (dominio.LinkDePerfil).
export const linkDoSite=(v:string,site:string)=>!v.trim()||new RegExp(`^(https?://)?([a-z0-9-]+\\.)*${site.replace('.','\\.')}/[^\\s<>"']+$`,'i').test(v.trim());
// Perfis antigos: só o nome completo (primeira palavra e o resto) e a cidade com o estado junto ("Recife, PE").
function nomeSeparado(p:any):[string,string]{
  if(texto(p.primeiroNome)||texto(p.sobrenome)) return [texto(p.primeiroNome),texto(p.sobrenome)];
  const partes=texto(p.nome).trim().split(/\s+/).filter(Boolean);
  return [partes[0]??'',partes.slice(1).join(' ')];
}
function cidadeSeparada(p:any):[string,string]{
  const cidade=texto(p.cidade).trim();
  if(texto(p.estado)) return [cidade,texto(p.estado)];
  const m=/^(.+?)\s*(?:,|-|–|\/)\s*([A-Za-z]{2})$/.exec(cidade);
  return m?[m[1],m[2].toUpperCase()]:[cidade,''];
}
// O telefone em duas partes: o país do código (a sigla, porque vários países usam +1) e o número. Perfis antigos
// guardavam só o número, às vezes já com o "+": o código é tirado do começo (o mais longo que existir).
const codigosDDI=[...new Set(Object.values(DDI))].sort((a,b)=>b.length-a.length);
const opcoesDDI:[string,string][]=paises.filter(([s])=>DDI[s]).map(([s,n])=>[s,`${DDI[s]} ${n}`]);
function telefoneSeparado(p:any):[string,string]{
  const tel=texto(p.telefone).trim(), casa=texto(p.paisResidencia)||'BR';
  const paisDoCodigo=(ddi:string)=>DDI[casa]===ddi?casa:(DDI.BR===ddi?'BR':Object.keys(DDI).find(s=>DDI[s]===ddi)??casa);
  if(texto(p.ddi)) return [paisDoCodigo(texto(p.ddi)),tel];
  if(tel.startsWith('+')){
    const ddi=codigosDDI.find(c=>tel.replace(/[\s().-]/g,'').startsWith(c));
    if(ddi) return [paisDoCodigo(ddi),tel.slice(tel.indexOf(ddi.slice(1))+ddi.length-1).replace(/^[\s.-]+/,'')];
  }
  return [DDI[casa]?casa:'BR',tel];
}
function leitura(p:any){
  const f=p.filtros||{}, faixa=p.faixaSalarial ?? {min:p.pretensaoUSD, moeda:p.pretensaoUSD?'USD':moedaDoMercado(p.foco,'USD')};
  const [primeiroNome,sobrenome]=nomeSeparado(p), [cidade,estado]=cidadeSeparada(p), [paisTelefone,telefone]=telefoneSeparado(p);
  return {nome:texto(p.nome), primeiroNome, sobrenome, email:texto(p.email), paisTelefone, telefone, cidade, estado, paisResidencia:texto(p.paisResidencia)||'BR', linkedin:texto(p.linkedin), github:texto(p.github), cargo:texto(p.cargo), ingles:texto(p.ingles), foco:p.foco||'ambos',
    paisTrabalho:texto(p.paisTrabalho), cidadeTrabalho:texto(p.cidadeTrabalho),
    titulos:lista(p.titulosEquivalentes).join(', '),
    principal:lista(p.stackPrincipal).join(', '), secundaria:lista(p.stackSecundaria).join(', '),
    niveis:lista(f.niveis), modelos:lista(f.modelo), pular:lista(f.pularSe).join(', '),
    moeda:faixa.moeda||'USD',min:String(faixa.min??''),max:String(faixa.max??'')};
}
// CPF: os dígitos verificadores, como o servidor confere (dominio.CPFValido).
export function cpfValido(v:string){
  if(/[^\d.\s-]/.test(v)) return false;
  const d=v.replace(/\D/g,'');
  if(d.length!==11||/^(\d)\1{10}$/.test(d)) return false;
  return [9,10].every(n=>{let s=0;for(let i=0;i<n;i++)s+=Number(d[i])*(n+1-i);return s*10%11%10===Number(d[n]);});
}
// O cofre do sistema tem nome diferente em cada um: Chaves no Mac, Gerenciador de Credenciais no Windows.
export const nomeDoCofre=(ua=typeof navigator==='undefined'?'':navigator.userAgent)=>
  /Windows/i.test(ua)?'no Gerenciador de Credenciais do Windows':/Mac OS|Macintosh/i.test(ua)?'nas Chaves do Mac':'no cofre do sistema';
const lerCPF=(r:any)=>({salvo:r?.salvo===true,mascarado:texto(r?.mascarado),automatico:r?.automatico===true});
// O CPF não vai com o perfil: tem rota e botão próprios e fica no cofre do sistema (nomeDoCofre). A tela só vê a
// máscara; o número digitado sai da tela assim que é salvo.
function CampoCPF(){
  const app=useApp(), trava=useTrava();
  const [estado,setEstado]=useState<{salvo:boolean;mascarado:string;automatico:boolean}|null>(null);
  const [editando,editar]=useState(false), [valor,setValor]=useState(''), [erro,setErro]=useState(''), [ocupado,ocupar]=useState(false);
  useEffect(()=>{let vivo=true;app.api('/api/v1/perfil/cpf').then(r=>{if(vivo)setEstado(lerCPF(r))}).catch(()=>{});return()=>{vivo=false}},[]); // eslint-disable-line react-hooks/exhaustive-deps
  if(!estado) return null;
  const salvar=async()=>{
    if(!cpfValido(valor)){setErro('Confira os 11 números do CPF.');return;}
    ocupar(true);setErro('');
    try{const r=await app.api('/api/v1/perfil/cpf','PUT',{cpf:valor});setEstado(lerCPF(r));setValor('');editar(false);app.aviso('CPF salvo.');}
    catch{setErro('Não deu para salvar o CPF. Tente de novo.')}finally{ocupar(false)}
  };
  const automatico=async(ligar:boolean)=>{
    ocupar(true);setErro('');
    try{const r=await app.api('/api/v1/perfil/cpf','PATCH',{automatico:ligar});setEstado(lerCPF(r));}
    catch{setErro('Não deu para mudar a opção. Tente de novo.')}finally{ocupar(false)}
  };
  const apagar=async()=>{
    ocupar(true);setErro('');
    try{await app.api('/api/v1/perfil/cpf','DELETE');setEstado({salvo:false,mascarado:'',automatico:false});app.aviso('CPF apagado.');}
    catch{setErro('Não deu para apagar o CPF. Tente de novo.')}finally{ocupar(false)}
  };
  return <div className="pr-field pr-wide pr-cpf">
    <label htmlFor="f-cpf">CPF</label>
    {estado.salvo&&!editando
      ? <div className="pr-cpf-salvo"><span>Salvo: <strong>{estado.mascarado}</strong></span>
          <button type="button" className="btn quiet" {...trava(ocupado)} onClick={()=>editar(true)}>Trocar</button>
          <button type="button" className="btn quiet" {...trava(ocupado)} onClick={apagar}>Apagar</button></div>
      : <div className="pr-cpf-novo">
          <input id="f-cpf" type="text" inputMode="numeric" autoComplete="off" maxLength={20} placeholder="000.000.000-00" value={valor} aria-invalid={!!erro||undefined} onChange={e=>{setValor(e.target.value);setErro('')}}/>
          <button type="button" className="btn secondary" {...trava(ocupado||!valor.trim())} onClick={salvar}>Salvar CPF</button>
          {estado.salvo&&<button type="button" className="btn quiet" disabled={ocupado} onClick={()=>{editar(false);setValor('');setErro('')}}>Cancelar</button>}
        </div>}
    {estado.salvo&&!editando&&<label className="pr-cpf-auto"><input type="checkbox" checked={estado.automatico} {...trava(ocupado)} onChange={e=>void automatico(e.target.checked)}/>
      <span>Preencher o CPF sem perguntar</span></label>}
    {erro&&<p className="pr-erro" role="alert">{erro}</p>}
    <p className="pr-nota">Fica {nomeDoCofre()}, fora do banco do app, e aqui aparece só em parte. O app só preenche o CPF em site brasileiro, no campo de CPF, {estado.automatico?'junto com o resto, depois que você confere o site':'e pergunta antes, a cada site'}. Nunca vai para a IA nem para o envio automático.</p>
  </div>;
}
function Cartao({titulo,icone,children}:{titulo:string;icone:string;children:ReactNode}){
  return <section className="panel pr-card"><div className="pr-card-title"><span className="pr-tile"><IconeUI nome={icone} tamanho={19}/></span><h2>{titulo}</h2></div>{children}</section>;
}
function Opcoes({titulo,base,valor,mudar}:{titulo:string;base:string[];valor:string[];mudar:(v:string[])=>void}){
  return <fieldset className="pr-levels pr-wide"><legend>{titulo}</legend><div>{[...new Set([...base,...valor])].map(v=><label key={v}><input type="checkbox" checked={valor.includes(v)} onChange={e=>mudar(e.target.checked?[...valor,v]:valor.filter(x=>x!==v))}/><span>{v}</span></label>)}</div></fieldset>;
}
export function PerfilDoEspaco(){
  const app=useApp(), id=app.estado.perfilBuscaAtivo || 'original', trava=useTrava();
  const [dados,setDados]=useState(()=>leitura(app.estado.perfil||{})), [sujo,alterado]=useState(false);
  // Estados e cidades do IBGE: um módulo à parte, carregado só nesta tela.
  const [geo,setGeo]=useState<typeof import('../dominio/municipios')|null>(null);
  useEffect(()=>{let vivo=true;void import('../dominio/municipios').then(m=>{if(vivo)setGeo(m)});return()=>{vivo=false}},[]);
  const noBrasil=dados.paisResidencia==='BR', cidades=noBrasil&&geo&&dados.estado?geo.cidadesDoEstado(dados.estado):[];
  const [salvando,salvarEstado]=useState(false), [erro,falhar]=useState(''), [alterandoIA,alterarIA]=useState(false);
  const conectado=app.conexaoIA?.verificada && !alterandoIA ? app.conexaoIA.nome : '';
  const [cvStatus,cvMensagem]=useState(''), [cvOcupado,cvOcupar]=useState(false), [cvErro,cvFalha]=useState('');
  const pdf=useRef<HTMLInputElement>(null), ia=useRef<HTMLElement>(null), ativa=useRef(true);
  useEffect(()=>{ativa.current=true;return()=>{ativa.current=false}},[]);
  useEffect(()=>{
    const foco=()=>{if(location.hash.endsWith('?aba=conexao')){ia.current?.scrollIntoView?.({block:'center'});ia.current?.focus({preventScroll:true})}};
    foco();window.addEventListener('hashchange',foco);return()=>window.removeEventListener('hashchange',foco);
  },[]);
  const moedaEditada=useRef(false);
  const set=<K extends keyof typeof dados>(k:K,v:typeof dados[K])=>{
    if(k==='moeda') moedaEditada.current=true;
    setDados(antes=>{
      const novo={...antes,[k]:v};
      // Trocar o mercado ajusta a moeda enquanto a pessoa não escolheu uma nem digitou a faixa.
      if(k==='foco' && !moedaEditada.current && !antes.min && !antes.max) novo.moeda=moedaDoMercado(String(v),antes.moeda);
      return novo;
    });
    alterado(true);falhar('')};
  const url='/api/v1/perfil?perfilBuscaId='+encodeURIComponent(id);
  async function salvar(){
    if(salvando||trava.travado) return;
    if(!dados.cargo.trim()){falhar('Informe o cargo que deseja buscar.');document.getElementById('f-cargo')?.focus();return;}
    if(dados.min && dados.max && Number(dados.max)<Number(dados.min)){falhar('O máximo não pode ser menor que o mínimo.');return;}
    if(!emailValido(dados.email)){falhar('Informe um e-mail válido, como nome@exemplo.com.');document.getElementById('f-email')?.focus();return;}
    if(!linkDoSite(dados.linkedin,'linkedin.com')){falhar('Informe o endereço do seu perfil no LinkedIn, como linkedin.com/in/seu-nome.');document.getElementById('f-linkedin')?.focus();return;}
    if(!linkDoSite(dados.github,'github.com')){falhar('Informe o endereço do seu perfil no GitHub, como github.com/seu-usuario.');document.getElementById('f-github')?.focus();return;}
    salvarEstado(true);falhar('');
    try {
      await app.api(url,'PATCH',{nome:[dados.primeiroNome.trim(),dados.sobrenome.trim()].filter(Boolean).join(' '),primeiroNome:dados.primeiroNome.trim(),sobrenome:dados.sobrenome.trim(),
        email:dados.email.trim(),ddi:DDI[dados.paisTelefone]??'',telefone:dados.telefone.trim(),cidade:dados.cidade.trim(),estado:dados.estado.trim(),paisResidencia:dados.paisResidencia,linkedin:dados.linkedin.trim(),github:dados.github.trim(),cargo:dados.cargo.trim(),ingles:dados.ingles,foco:dados.foco,
        paisTrabalho:dados.paisTrabalho, cidadeTrabalho:dados.cidadeTrabalho.trim(),
        titulosEquivalentes:separar(dados.titulos),
        stackPrincipal:separar(dados.principal),stackSecundaria:separar(dados.secundaria),
        filtros:{niveis:dados.niveis,modelo:dados.modelos,pularSe:separar(dados.pular)},pretensaoUSD:null,
        faixaSalarial:(dados.min||dados.max)?{moeda:dados.moeda,...dados.min&&{min:Number(dados.min)},...dados.max&&{max:Number(dados.max)}}:null});
      app.limpar();await app.recarregar();if(ativa.current) {alterado(false);app.aviso('Perfil salvo.');}
    }catch(e){if(ativa.current) falhar((e as Error).message||'Não foi possível salvar o perfil.')}
    finally{if(ativa.current) salvarEstado(false)}
  }
  async function selecionar(f?:File){
    if(!f||cvOcupado||trava.travado)return;
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
  type Campo='paisTelefone'|'nome'|'primeiroNome'|'sobrenome'|'estado'|'paisResidencia'|'email'|'telefone'|'cidade'|'linkedin'|'github'|'cargo'|'principal'|'secundaria'|'ingles'|'foco'|'pular'|'moeda'|'min'|'max'|'paisTrabalho'|'cidadeTrabalho'|'titulos';
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
      <Cartao titulo="Dados de contato" icone="profile"><div className="pr-fields">
        <p className="pr-wide pr-nota">Vão no pacote de cada candidatura e no "Preencher no site da empresa". Valem para todos os perfis de busca. Deixe em branco o que não quer mandar.</p>
        {campo('primeiroNome','Nome',{dica:'Ex.: Maria Clara'})}{campo('sobrenome','Sobrenome',{dica:'Ex.: Souza Lima'})}
        {campo('email','E-mail',{dica:'nome@exemplo.com'})}
        <div className="pr-field pr-campo-telefone"><label htmlFor="f-telefone">Telefone</label><div className="pr-telefone">
          <select form="form-perfil-ativo" id="f-paisTelefone" aria-label="Código do país do telefone" value={dados.paisTelefone} onChange={e=>set('paisTelefone',e.target.value)}>
            {opcoesDDI.map(([s,r])=><option key={s} value={s}>{r}</option>)}</select>
          <input id="f-telefone" form="form-perfil-ativo" type="tel" maxLength={40} autoComplete="tel-national" placeholder={dados.paisTelefone==='BR'?'DDD e número, ex.: 11 91234-5678':'Número, sem o código do país'} value={dados.telefone} onChange={e=>set('telefone',e.target.value)}/>
        </div></div>
        {campo('paisResidencia','País onde você mora',{lista:[['','Selecione'],...paises]})}
        {noBrasil ? campo('estado','Estado',{lista:[['','Selecione o estado'],...(geo?.ESTADOS ?? (dados.estado?[[dados.estado,dados.estado] as [string,string]]:[]))]}) : campo('estado','Estado ou província')}
        <div className="pr-field pr-wide"><label htmlFor="f-cidade">Cidade</label>
          <input id="f-cidade" form="form-perfil-ativo" maxLength={120} autoComplete="off" list={cidades.length?'cidades-do-estado':undefined}
            placeholder={noBrasil && !dados.estado ? 'Escolha o estado para ver as cidades' : 'Comece a digitar o nome da cidade'} value={dados.cidade} onChange={e=>set('cidade',e.target.value)}/>
          {cidades.length>0 && <datalist id="cidades-do-estado">{cidades.map(c=><option key={c} value={c}/>)}</datalist>}
        </div>
        {campo('linkedin','LinkedIn',{dica:'linkedin.com/in/seu-nome'})}{campo('github','GitHub',{dica:'github.com/seu-usuario'})}
        <CampoCPF/>
      </div></Cartao>
      <Cartao titulo="Perfil profissional" icone="user"><div className="pr-fields">
        {campo('cargo','Cargo de interesse',{wide:true})}
        {cargoIgualAoNome(dados.cargo,dados.nome) && <p className="pr-wide">O cargo coincide com seu nome. Informe a função que deseja buscar.</p>}
        <Opcoes titulo="Níveis de experiência aceitos" base={['Estágio','Júnior','Pleno','Sênior','Liderança']} valor={dados.niveis} mudar={v=>set('niveis',v)}/>
        {campo('titulos','Títulos equivalentes',{wide:true,dica:'Separe os títulos por vírgulas (opcional)'})}
        {campo('principal','Competências principais',{wide:true})}
        {campo('secundaria','Competências complementares')}
        {campo('ingles','Nível de inglês',{lista:[['','Selecione'],...[...new Set(['Não falo inglês','Básico','Intermediário','Avançado','Fluente',dados.ingles].filter(Boolean))].map(v=>[v,v] as [string,string])]})}
      </div></Cartao>
      <Cartao titulo="Filtros de busca" icone="sliders"><div className="pr-fields">
        {campo('foco','Mercado',{lista:[['nacional','Brasil'],['internacional','Exterior'],['ambos','Brasil e exterior']]})}
        <Opcoes titulo="Modelo de trabalho" base={['Remoto','Híbrido','Presencial']} valor={dados.modelos} mudar={v=>set('modelos',v)}/>
        {campo('paisTrabalho','País de trabalho',{lista:[['','Selecione de onde vai trabalhar'],...paises]})}
        {aceitaTrabalhoLocal(dados.modelos) && campo('cidadeTrabalho','Cidade de trabalho')}
        {campo('pular','Pular vagas com estes termos',{wide:true})}
        <fieldset className="pr-pay pr-wide"><legend>Faixa salarial mensal <small>Opcional</small></legend><div>
          {campo('moeda','Moeda',{lista:[['USD','USD'],['BRL','BRL'],['EUR','EUR']]})}{campo('min','Mínimo',{tipo:'number'})}{campo('max','Máximo',{tipo:'number'})}
        </div></fieldset>
      </div></Cartao>
    </div><div className="pr-stack"><section ref={ia} tabIndex={-1} aria-label="Configuração de IA">
      <Cartao titulo="Conexão de IA" icone="sparkles">{conectado?<><p className={'pr-connected pr-marca-ia '+(marcaIA[conectado]||'')}>{marcaIA[conectado]&&<span aria-hidden="true" className="pa-logotipo"/>}{conectado} conectado</p><button type="button" className="btn secondary" onClick={()=>alterarIA(true)}>Alterar conexão</button></>:
        <ConexaoOnboarding aoConectar={()=>alterarIA(false)}/>}</Cartao></section>
      <ContaEmail/>
      <Cartao titulo="Currículo" icone="file">
        <div className="pr-drop" onDragOver={e=>e.preventDefault()} onDrop={e=>{e.preventDefault();void selecionar(e.dataTransfer.files[0]);}}>
          <img src={arteCV} alt="" width="42" height="42"/><div className="pr-file-meta"><strong>{cv?(/^curriculo-[A-Z2-7]{26,}\.pdf$/.test(cv)?'Currículo do perfil':cv):'Arraste seu currículo para cá'}</strong><small>{cv?'PDF salvo neste computador':'PDF de até 20 MB'}</small></div>
          <button type="button" className="btn secondary" {...trava(cvOcupado)} onClick={()=>pdf.current?.click()}>{cvOcupado?'Salvando…':cv?'Trocar PDF':'Escolher PDF'}</button>
          <input hidden type="file" aria-label="Currículo em PDF" ref={pdf} accept=".pdf,application/pdf" onChange={e=>{void selecionar(e.target.files?.[0]);e.target.value='';}}/>
        </div>{cvErro&&<p role="alert">{cvErro}</p>}{cvStatus&&<p className="pr-connected" role="status">{cvStatus}</p>}
      </Cartao>
    </div></div>
    <form id="form-perfil-ativo" className="pr-footer pr-end" onSubmit={e=>{e.preventDefault();void salvar();}}>
      {erro&&<p role="alert">{erro}</p>}<button className="btn primary" type="submit" {...trava(salvando)}>{salvando?'Salvando…':'Salvar perfil'}</button>
    </form>
    {/* Conta (sair e excluir): só com a conta ligada neste build e com entrada. */}
    <SecaoConta />
  </div>;
}
