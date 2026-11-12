window.searchProfileStore = function searchProfileStore(seed,saved){
  const personalSeed=seed(), firstId='original';
  const makeTrack=(id,label,profile,source={})=>({id,label,profile:{...profile},results:source.results||[],skipped:source.skipped||[],ratings:source.ratings||{},lastSearch:source.lastSearch||0});
  const validSaved=saved&&Array.isArray(saved.day)&&Array.isArray(saved.board);
  const legacy=validSaved&&saved.profile?saved:personalSeed;
  const first=makeTrack(firstId,legacy.profile.role||'Meu perfil',legacy.profile,legacy);
  const validTrack=t=>t&&typeof t.id==='string'&&typeof t.label==='string'&&t.profile&&Array.isArray(t.results)&&Array.isArray(t.skipped);
  const tracks=validSaved&&Array.isArray(saved.profiles)?saved.profiles.filter(validTrack):[];
  if(!tracks.length)tracks.push(first);
  const shared=validSaved?saved:personalSeed;
  const state={day:shared.day,board:shared.board,packets:shared.packets||{},activeProfileId:tracks.find(t=>t.id===saved?.activeProfileId)?.id||tracks[0].id,searchStatus:'ready'};
  const active=()=>tracks.find(t=>t.id===state.activeProfileId);
  for(const field of ['profile','results','skipped','ratings','lastSearch']){
    Object.defineProperty(state,field,{enumerable:true,get:()=>active()[field],set:value=>{active()[field]=value}});
  }
  for(const id of [...state.day,...state.board.map(b=>b.id)]){
    const pkt=state.packets[id]||{};
    state.packets[id]={profileId:firstId,...pkt};
  }
  const presets={
    ai:{role:'AI Engineer',primary:'Python, Machine Learning, LLM',secondary:'PyTorch, MLOps'},
    java:{role:'Java Engineer',primary:'Java, Spring Boot',secondary:'SQL, APIs'}
  };
  function create(label,preset){
    label=String(label??'').trim().slice(0,72);
    if(!label)return null;
    const existing=tracks.find(t=>t.label.localeCompare(label,'pt-BR',{sensitivity:'base'})===0);
    if(existing)return null;
    const id=globalThis.crypto?.randomUUID?.()||`profile-${Date.now()}-${Math.random().toString(36).slice(2,8)}`;
    const data={...state.profile,...(presets[preset]||{role:label,primary:'',secondary:''})};
    tracks.push(makeTrack(id,label,data));return id;
  }
  function use(id){
    if(!tracks.some(t=>t.id===id))return false;
    state.activeProfileId=id;state.searchStatus='ready';return true;
  }
  function owner(jobId){return tracks.find(t=>t.id===state.packets[jobId]?.profileId)||active()}
  function release(jobId){const track=owner(jobId);track.skipped=track.skipped.filter(id=>id!==jobId);if(!track.results.includes(jobId)) track.results.unshift(jobId)}
  return {state,create,use,owner,release,list:()=>tracks.map(t=>({id:t.id,label:t.label,role:t.profile.role})),snapshot:()=>({version:2,activeProfileId:state.activeProfileId,profiles:tracks,day:state.day,board:state.board,packets:state.packets})};
};
