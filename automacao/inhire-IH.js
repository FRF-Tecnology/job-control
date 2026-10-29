// Gere com: node ferramentas/cli.mjs script inhire-IH (preenche nome, e-mail, telefone e pretensões do perfil).
await new Promise(r=>setTimeout(r,1500));
const src=`window.IH={sl:ms=>new Promise(r=>setTimeout(r,ms)),
setv(el,v){const p=el.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype;Object.getOwnPropertyDescriptor(p,'value').set.call(el,v);el.dispatchEvent(new Event('input',{bubbles:true}));el.dispatchEvent(new Event('change',{bubbles:true}));el.dispatchEvent(new Event('blur',{bubbles:true}))},
async base(sal){const q=n=>document.querySelector('[name='+n+']');const r=[];
 const m={name:'{{nome}}',email:'{{email}}',phone:'{{telefoneSoNumeros}}',linkedinUsername:'{{linkedin}}'};
 for(const[k,v]of Object.entries(m)){const e=q(k);if(e){e.focus();this.setv(e,v);await this.sl(150);r.push(k)}}
 const s=q('salaryExpectation');if(s&&sal){s.focus();this.setv(s,sal);await this.sl(150);r.push('sal='+s.value)}
 return r.join(' ; ')},
form(){const f=document.querySelector('[name=email]')?.closest('form')||document.body;return [...f.querySelectorAll('input,textarea,select')].filter(e=>!/recaptcha|checkbox|hidden/.test(e.name+e.type)).map((e,i)=>i+':'+e.type+':'+(e.name||'')+':'+(e.value||'').slice(0,20)+':'+(e.checked?'X':'')+':'+(e.closest('label,div')?.innerText||'').replace(/\\s+/g,' ').slice(0,60)).join('\\n')},
pos(n){const c=document.querySelector('[name="'+n+'"]');if(!c)return 'no '+n;c.scrollIntoView({block:'center'});const r=c.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2].map(v=>Math.round(v*1568/innerWidth)).join(',')},
async submit(){const b=[...document.querySelectorAll('form button')].find(b=>/Candidatar-se|Continuar inscri/.test(b.innerText));b.click();await this.sl(5000);const f=[...document.querySelectorAll('iframe')].find(f=>/form-app/.test(f.src));return f?'FORM '+f.src:(/sucesso/i.test(document.body.innerText)?'SUCESSO':'?? '+(document.body.innerText.match(/[^\\n]*(obrigat|inv[aá]lid|erro)[^\\n]*/gi)||[]).join('|'))}};`;
const s=document.createElement('script');s.textContent=src;document.head.appendChild(s);await new Promise(r=>setTimeout(r,300));
const lab=document.body.innerText.match(/Pretens[^\n]*/)?.[0]||'';(document.title+' | '+lab+'\n'+(await window.IH.base(/PJ|cooperad/i.test(lab)?'{{pretensaoPJ}}':'{{pretensaoCLT}}'))+'\n'+window.IH.form()+'\n'+[...document.querySelectorAll('form button')].map(b=>b.innerText.trim()).filter(Boolean).join('/')+'\npos:'+IH.pos('country')).replace(/[?&=@]/g,' ')