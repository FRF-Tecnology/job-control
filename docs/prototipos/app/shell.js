(function shell(){
'use strict';
const key='applyer.workspace.demo.v1';
const h=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const norm=s=>String(s??'').normalize('NFD').replace(/[\u0300-\u036f]/g,'').toLowerCase().trim();
const jobs=window.demoJobs, icon=window.uiIcon, assets=window.designAssets;
const seed=()=>({profile:{...window.demoProfile,level:'Pleno, Sênior'},results:['forma','hfdesign','nimbo','vertice','luma','onda'],day:['junco'],skipped:[],board:[{id:'atlas',stage:'Entrevista',note:'Conversa com o time de produto.',history:['Entrevista marcada com a equipe.']},{id:'prisma',stage:'Enviada',note:'',history:['Candidatura enviada na demonstração.']}],packets:{},searchStatus:'ready',lastSearch:0});
let saved;
try { saved=JSON.parse(localStorage.getItem(key)); } catch {}
const tracks=window.searchProfileStore(seed,saved);
const state=tracks.state;
const names={radar:'Radar de vagas',lista:'Lista do dia',quadro:'Quadro',perfil:'Perfil e IA',inicio:'Primeiro acesso'};
const actions={};let searchSeq=0,sessionResults=new Map(),toastTimer=0,returnFocus=null;
function selectProfile(id){
  if(!tracks.use(id))return false;
  ++searchSeq;
  sessionResults.clear();
  closeModal();
  save();
  render();
  return true;
}
function createProfile(label,preset){
  const id=tracks.create(label,preset);
  if(!id){toast('Escolha um nome diferente para o perfil.');return null}
  selectProfile(id);
  if(preset==='ai'||preset==='java')search();
  else go('perfil');
  return id;
}
function route(){let q=location.hash.slice(1);return Object.hasOwn(names,q)?q:'radar'}
function go(r){closeModal();if(!Object.hasOwn(names,r))return;if(route()===r){render();return}location.hash=r}
function save(){try{localStorage.setItem(key,JSON.stringify(tracks.snapshot()))}catch{}}
function toast(msg){const el=document.getElementById('notice');clearTimeout(toastTimer);el.textContent=msg;el.classList.add('show');toastTimer=setTimeout(()=>el.classList.remove('show'),3600)}
function closeModal(){const d=document.getElementById('dialog');if(d.open)d.close()}
function appModal(title,body){const d=document.getElementById('dialog');if(!d.open)returnFocus=document.activeElement;document.getElementById('modal-title').textContent=title;document.getElementById('modal-body').innerHTML=body;if(!d.open)d.showModal();d.querySelector('button')?.focus()}
function nav(r,glyph,count,active){return `<a href="#${r}" class="nav-link ${active===r?'active':''}" ${active===r?'aria-current="page"':''}>${icon(glyph)}<span>${names[r]}</span>${Number.isFinite(count)?`<span class="nav-count">${count}</span>`:''}</a>`}
function updateShell(r){
const count=state.results.filter(id=>!state.day.includes(id)&&!state.skipped.includes(id)&&!state.board.some(b=>b.id===id)).length;
const p=state.profile;
document.getElementById('railnav').innerHTML=nav('radar','radar',count,r)+nav('lista','list',state.day.length,r)+nav('quadro','board',undefined,r);
document.getElementById('accountnav').innerHTML=nav('perfil','sliders',undefined,r==='inicio'?'perfil':r);
document.getElementById('currentArea').textContent=names[r];
document.getElementById('rail-person').textContent=p.name||'Perfil de exemplo';
document.getElementById('avatar').textContent=(p.name||'Meu perfil').trim().split(/\s+/).filter((_,i,a)=>i===0||i===a.length-1).map(w=>w[0]).join('').toUpperCase().slice(0,2);
document.getElementById('connection').textContent=p.provider?`${p.provider} conectado`:'Conexão pendente';
document.title=`${names[r]} · Applyer.`;
}
function hero(){
const p=state.profile;
const details=[p.mode,p.market].filter(Boolean).map(h).join('<span aria-hidden="true"> · </span>');
return `<section class="radar-hero" data-search="${state.searchStatus}" aria-labelledby="hero-title"><div class="hero-art" aria-hidden="true">${assets.radar}</div><div class="hero-copy"><div class="hero-kicker"><span class="cross-star">✦</span> DESCOBERTA</div><h1 id="hero-title">Radar de vagas<span>.</span></h1><div class="hero-profile">${window.MultiUI.selector()}<p>${details||'Defina seu perfil para encontrar oportunidades.'}</p></div><div class="hero-buttons"><button class="btn primary" data-act="search-now" ${state.searchStatus==='loading'?'disabled':''}>${icon('radar',17)}${state.searchStatus==='loading'?'Buscando…':'Buscar vagas'}</button><a class="hero-edit" href="#perfil">${icon('sliders',15)} Ajustar perfil</a></div></div></section>`}
function render(){const r=route();updateShell(r);const area=document.getElementById('main');area.innerHTML=r==='radar'?hero()+window.RadarView():r==='perfil'||r==='inicio'?window.ProfileView():window.JourneyView();if(area.dataset.route && area.dataset.route!==r && !matchMedia('(prefers-reduced-motion: reduce)').matches)area.animate([{opacity:.8,transform:'translateY(3px)'},{opacity:1,transform:'translateY(0)'}],{duration:150,easing:'ease-out'});area.dataset.route=r;}
const fields=x=>String(Array.isArray(x)?x.join(','):x??'').split(/[,;|/]+/).map(norm).filter(Boolean);
const ranks={'nao falo ingles':0,'basico':1,'intermediario':2,'avancado':3,'fluente':4};
function matches(){
const p=state.profile, levels=fields(p.level), primary=fields(p.primary), stack=[...new Set([...primary,...fields(p.secondary)])],market=norm(p.market),skips=fields(p.skip);
const terms=s=>norm(s).replace(/\b(ia|artificial intelligence|inteligencia artificial|machine learning)\b/g,'ai').split(/[^a-z0-9+#]+/).filter(Boolean);
const general=new Set(['senior','junior','pleno','staff','de','da','do','para','engineer','engenheiro','engenheira','software','developer','desenvolvedor','desenvolvedora']);
const words=terms(p.role).filter(w=>!general.has(w));
const isAll=s=>!s||/ambos|todos|qualquer|exterior.*brasil|brasil.*exterior|dois|sem preferencia|indiferente/.test(s);
const ranked=[];window.demoJobs.forEach(j=>{
const bag=norm([j.title,j.company,j.level,j.mode,j.description,...j.tags].join(' '));
if(skips.some(s=>bag.includes(s)))return;
if(!isAll(market)&&(market==='brasil'||market==='br'||market==='nacional'?j.market!=='Brasil':/^(exterior|internacional|fora do brasil)$/.test(market)&&j.market!=='Exterior'))return;
if(!isAll(norm(p.mode))&&!norm(p.mode).split(/[,/]+/).some(m=>m.trim()===norm(j.mode)))return;
if(levels.length&&!levels.some(l=>isAll(l)||l===norm(j.level)))return;
if(p.currency===j.currency&&((Number(p.min)>0&&j.max<Number(p.min))||(Number(p.max)>0&&j.min>Number(p.max))))return;
const englishRequired=j.market==='Exterior'?(/Toronto|Austin/.test(j.location)?3:2):0;
if(ranks[norm(p.english)]!==undefined&&ranks[norm(p.english)]<englishRequired)return;
const title=terms(j.title),titleHit=words.some(w=>title.includes(w));
const tags=j.tags.map(t=>terms(t).join(' '));
const mentions=s=>tags.includes(terms(s).join(' '));
const hits=stack.filter(mentions).length;
const primaryMatches=primary.some(mentions);
if((words.length||primary.length)&&!titleHit&&!primaryMatches)return;
const grade= titleHit&&hits>=2&&p.currency===j.currency?'A':titleHit||hits>=2?'B':'C';
const reason=[titleHit?'O cargo corresponde à área que você definiu.':'A vaga se aproxima do perfil pelas habilidades exigidas.',hits?`O anúncio menciona ${stack.filter(mentions).map(s=>j.tags.find(t=>norm(t)===s)||s).join(', ')}.`:'Confira a experiência pedida antes de incluir na lista.',p.currency!==j.currency?'A remuneração está em outra moeda.':Number(p.min)||Number(p.max)?'A faixa anunciada cruza a sua pretensão.':''].filter(Boolean).join(' ');
ranked.push({...j,grade,reason});
});
ranked.sort((a,b)=>a.grade.localeCompare(b.grade));sessionResults=new Map(ranked.map(j=>[j.id,{grade:j.grade,reason:j.reason}]));return ranked.map(j=>j.id);
}
function search(scenario='normal'){if(typeof scenario!=='string')scenario='normal';if(state.searchStatus==='loading'&&scenario==='normal'){go('radar');return}const token=++searchSeq;state.searchStatus='loading';go('radar');setTimeout(()=>{if(token!==searchSeq)return;if(scenario==='error'){state.searchStatus='error';}else{state.results=scenario==='empty'?[]:matches();state.searchStatus='ready';state.lastSearch=Date.now();state.ratings=Object.fromEntries(sessionResults);save();if(scenario!=='empty')toast('Busca concluída. As vagas estão em ordem de compatibilidade.');}render()},750)}
function scenarios(){appModal('Explorar a demonstração',`<p class="modal-intro">Este espaço usa vagas fictícias. Suas escolhas ficam salvas neste navegador. Experimente também os estados do aplicativo.</p><div class="scenario-list">${[['begin','login','Percorrer o primeiro acesso'],['fail','refresh','Simular uma falha de busca'],['empty','search','Simular uma busca sem resultados'],['reset','undo','Restaurar os dados de exemplo']].map(([id,ic,label])=>`<button class="scenario" data-act="demo-${id}">${icon(ic)}<span>${label}</span>${icon('chevron-right',16)}</button>`).join('')}</div>`)}
Object.assign(actions,{'search-now':()=>search(),'demo-menu':scenarios,'demo-begin':()=>go('inicio'),'demo-fail':()=>search('error'),'demo-empty':()=>search('empty'),'demo-reset':()=>{searchSeq++;try{localStorage.removeItem(key)}catch{}closeModal();location.hash='radar';location.reload()},'modal-close':closeModal,'menu-toggle':()=>{const side=document.getElementById('rail');side.classList.toggle('open');document.getElementById('nav-scrim').hidden=!side.classList.contains('open');document.getElementById('mobile-menu').setAttribute('aria-expanded',String(side.classList.contains('open')));if(side.classList.contains('open'))side.querySelector('a')?.focus();}});
function hideNav(){document.getElementById('rail').classList.remove('open');document.getElementById('nav-scrim').hidden=true;document.getElementById('mobile-menu').setAttribute('aria-expanded','false')}
function start(){
window.MultiUI.install();
document.getElementById('realbrand').innerHTML=`<img src="${assets.logo}" alt="Applyer." width="136" height="30">`;
document.getElementById('mobile-menu').innerHTML=icon('menu',21);document.getElementById('areaicon').innerHTML=icon('area',16);document.getElementById('demobtn').innerHTML=`${icon('flask',15)}<span>Demonstração</span>${icon('chevron-down',13)}`;
document.querySelector('.dialog-close').innerHTML=icon('close');document.getElementById('connection-icon').innerHTML=icon('sparkles',16);
document.addEventListener('click',e=>{const el=e.target.closest('[data-act]');if(el&&!el.disabled&&actions[el.dataset.act]){e.preventDefault();actions[el.dataset.act](el,e)}if(e.target.closest('#rail a'))hideNav()});
document.getElementById('dialog').addEventListener('click',e=>{if(e.target===e.currentTarget){const r=e.currentTarget.getBoundingClientRect();if(e.clientX<r.left||e.clientX>r.right||e.clientY<r.top||e.clientY>r.bottom)closeModal()}});
document.getElementById('dialog').addEventListener('close',()=>{if(returnFocus?.isConnected)returnFocus.focus();returnFocus=null;});
document.getElementById('nav-scrim').addEventListener('click',hideNav);document.addEventListener('keydown',e=>{if(e.key==='Escape'&&document.getElementById('rail').classList.contains('open')){hideNav();document.getElementById('mobile-menu').focus()}});
window.addEventListener('hashchange',()=>{closeModal();hideNav();render();window.scrollTo(0,0);document.getElementById('main').focus({preventScroll:true})});
render();}
window.A={state,profiles:tracks.list,selectProfile,createProfile,profileForJob:tracks.owner,restoreJob:tracks.release,get jobs(){const m=state.ratings||{};return jobs.map(x=>({...x,...(m[x.id]||{})}))},go,route,save,render,search,h,toast,actions,appModal,closeModal,icon,mark:window.jobMark,brand(name){const n=norm(name);const k=/claude|anthropic/.test(n)?'claude':/openai|chatgpt|codex/.test(n)?'chatgpt':/grok|xai/.test(n)?'grok':'gemini';return `<img width="28" height="28" src="${assets[k]}" alt="">`},landscape:()=>`<img src="${assets.landscape}" alt="">`,start};})();
