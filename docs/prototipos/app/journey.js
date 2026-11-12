(() => {
  'use strict';
  const A = window.A;
  const stages = [
    { name: 'Enviada', className: 'sent' },
    { name: 'Entrevista', className: 'interview' },
    { name: 'Proposta', className: 'offer' },
    { name: 'Conquistada', className: 'won' }
  ];
  let lastSent = [];
  const h = value => A.h(String(value ?? ''));
  const key = id => String(id);
  const same = (first, second) => key(first) === key(second);
  const jobById = id => A.jobs.find(job => same(job.id, id));
  const entryById = id => A.state.board.find(entry => same(entry.id, id));
  const isInDay = id => A.state.day.some(value => same(id, value)) && !entryById(id);
  const activeJobs = () => Array.from(new Set(A.state.day.map(key))).map(jobById).filter(job => job && !entryById(job.id));
  const packet = id => A.state.packets[id] || {};
  const labelCount = (n, singular, plural) => `${n} ${n === 1 ? singular : plural}`;
  const time = () => new Intl.DateTimeFormat('pt-BR', {day:'2-digit', month:'2-digit', hour:'2-digit', minute:'2-digit'}).format(new Date());
  const stageName = value => (stages.find(stage => stage.name.toLowerCase() === String(value).toLowerCase()) || stages[0]).name;

  function btn(action, label, className, options = {}) {
    const { id, icon, disabled, aria, pressed } = options;
    return `<button type="button" class="${className}" data-act="j-${action}"${id !== undefined ? ` data-id="${h(id)}"` : ''}${disabled ? ' disabled' : ''}${aria ? ` aria-label="${h(aria)}"` : ''}${pressed !== undefined ? ` aria-pressed="${pressed}"` : ''}>${icon ? A.icon(icon, 16) : ''}${h(label)}</button>`;
  }
  const radarButton = () => btn('radar', 'Voltar ao radar', 'btn secondary', {icon:'arrow-left'});
  function header(title, desc, action) {
    return `<header class="j-header"><div><h1>${title}</h1><p>${desc}</p></div>${action || ''}</header>`;
  }
  function noteToast(message) {
    A.save();
    A.render();
    A.toast(message);
  }
  function setPacket(id, props) {
    A.state.packets[id] = {...packet(id), ...props};
  }
  function focusRow(action,id) {
    Array.from(document.querySelectorAll?.(`[data-act="j-${action}"]`) || [])
      .find(button => same(button.dataset.id,id))?.focus({preventScroll:true});
  }
  function letterFor(job) {
    return packet(job.id).letter ?? `Olá, equipe ${job.company}!\n\nTenho interesse na vaga de ${job.title} e gostaria de conversar sobre a oportunidade. Encaminho meu currículo para avaliação.\n\n${A.profileForJob(job.id).profile.name || 'Agradeço pela atenção.'}`;
  }
  function jobHeading(job) {
    return `<div class="j-job-heading">${A.mark(job, 42)}<div><span class="j-company">${h(job.company)} · ${h(A.profileForJob(job.id).label)}</span><h3>${h(job.title)}</h3></div></div>`;
  }
  function listRow(job) {
    const approved = packet(job.id).approved === true;
    return `<li class="j-row"><div class="j-row-top">${jobHeading(job)}<span class="j-status ${approved ? 'j-approved' : ''}"><i aria-hidden="true"></i>${approved ? 'Aprovado' : 'Em revisão'}</span></div>
      <div class="j-row-actions">${btn('review','Revisar pacote','btn secondary j-small',{id:job.id,icon:'file',aria:`Revisar pacote de ${job.title} na ${job.company}`})}
      ${btn('approve',approved ? 'Retirar aprovação' : 'Aprovar pacote','btn quiet j-small j-approve',{id:job.id,icon:'check',pressed:approved,aria:`${approved ? 'Retirar aprovação' : 'Aprovar pacote'} de ${job.title} na ${job.company}`})}</div></li>`;
  }
  function emptyState(icon, title, text, action) {
    return `<div class="j-empty"><span class="j-empty-icon" aria-hidden="true">${A.icon(icon,25)}</span><h3>${title}</h3><p>${text}</p>${action || ''}</div>`;
  }
  function summary(total, ready) {
    return `<aside class="panel j-summary" aria-label="Resumo do lote">
      <span class="j-summary-icon" aria-hidden="true">${A.icon('send',22)}</span><h2>Resumo do lote</h2>
      <div class="j-ratio"><strong>${ready}</strong><span>/ ${total}</span></div>
      <div class="j-ratio-caption">${ready === 1 ? 'pacote aprovado' : 'pacotes aprovados'}</div>
      <div class="j-meter" aria-hidden="true"><i style="width:${total ? ready * 100 / total : 0}%"></i></div>
      <div class="j-summary-line"><span>Para revisar</span><b>${total-ready}</b></div>
      <div class="j-summary-line"><span>Prontos para envio</span><b>${ready}</b></div>
      <div class="j-summary-bottom">${btn('send','Simular envio do lote','btn primary',{icon:'send',disabled:ready === 0})}
      <p>Somente os pacotes aprovados entram na simulação.</p></div></aside>`;
  }
  function dayView() {
    const jobs = activeJobs();
    const ready = jobs.filter(job => packet(job.id).approved === true).length;
    const sent = lastSent.filter(entryById);
    const receipt = sent.length ? `<div class="j-receipt" role="status"><span aria-hidden="true">${A.icon('check',20)}</span><div><b>Envio simulado</b><p>${labelCount(sent.length,'candidatura registrada no quadro.','candidaturas registradas no quadro.')}</p></div>${btn('board','Ver no quadro','btn quiet',{icon:'arrow-right'})}</div>` : '';
    const emptyTitle = sent.length ? 'Lista concluída' : 'Sua lista está vazia';
    const emptyText = sent.length ? 'Você pode acompanhar as candidaturas no quadro ou escolher mais vagas no radar.' : 'Adicione uma vaga no radar para revisar o pacote de candidatura aqui.';
    return `<section class="j-page">${header('Lista do dia','Revise os pacotes das vagas que você escolheu no radar.',jobs.length ? radarButton() : '')}${receipt}
      <div class="j-list-layout"><section class="panel j-list-panel"><div class="j-panel-heading"><h2>Pacotes para envio <span class="j-count">${jobs.length}</span></h2>
      ${jobs.length > 1 && ready < jobs.length ? btn('approve-all','Aprovar todos','btn quiet j-small') : ''}</div>
      ${jobs.length ? `<ul class="j-list">${jobs.map(listRow).join('')}</ul>` : emptyState('list',emptyTitle,emptyText,radarButton())}</section>${summary(jobs.length,ready)}</div></section>`;
  }
  function boardCard(entry) {
    const job = jobById(entry.id);
    if (!job) return '';
    const detail = entry.note || [job.mode,job.location].filter((v,i,list) => v && list.indexOf(v) === i).join(' · ');
    return `<button type="button" class="j-board-card" data-act="j-card" data-id="${h(job.id)}" aria-label="Abrir candidatura de ${h(job.title)} na ${h(job.company)}">
      <span class="j-card-company">${A.mark(job,28)}<span>${h(job.company)}</span></span><strong>${h(job.title)}</strong>
      ${detail ? `<span class="j-card-meta">${A.icon(entry.note ? 'edit' : 'map-pin',13)}<span>${h(detail)}</span></span>` : ''}</button>`;
  }
  function boardView() {
    const entries = A.state.board.filter((entry,index,all) => jobById(entry.id) && all.findIndex(other => same(other.id,entry.id)) === index);
    const columns = stages.map(stage => {
      const list = entries.filter(entry => stageName(entry.stage) === stage.name);
      return `<section class="j-column j-${stage.className}" aria-label="${stage.name}">
        <header><span class="j-stage-dot" aria-hidden="true"></span><h2>${stage.name}</h2><span class="j-count">${list.length}</span></header>
        <div class="j-column-body">${list.length ? list.map(boardCard).join('') : '<div class="j-stage-empty">Nenhuma candidatura</div>'}</div></section>`;
    }).join('');
    return `<section class="j-page">${header('Quadro de candidaturas','Acompanhe as conversas, registre atualizações e mova cada vaga de etapa.',radarButton())}
      ${entries.length ? '' : `<div class="panel j-board-empty">${emptyState('board','Comece pela lista do dia','Depois de simular um envio, sua candidatura aparece na primeira coluna.',btn('day','Abrir lista do dia','btn secondary',{icon:'list'}))}</div>`}
      <div class="j-board-grid">${columns}</div></section>`;
  }
  function review(job) {
    const curriculum = A.profileForJob(job.id).profile.cv || 'Nenhum currículo escolhido';
    A.appModal('Revisar pacote', `<div class="j-dialog">${jobHeading(job)}
      <div class="j-document"><span aria-hidden="true">${A.icon('file',19)}</span><div><strong>Currículo</strong><p>${h(curriculum)}</p></div></div>
      <label class="j-field" for="j-letter">Carta de apresentação <span>Opcional</span></label>
      <textarea class="j-input" id="j-letter" rows="6" maxlength="6000" placeholder="Escreva uma mensagem para a empresa">${h(letterFor(job))}</textarea>
      <div class="j-editor-actions">${btn('return','Devolver ao radar','btn quiet j-return',{id:job.id,icon:'arrow-left'})}
      ${btn('save-letter','Salvar pacote','btn primary',{id:job.id,icon:'check'})}</div></div>`);
  }
  function showBoardEntry(job,entry) {
    const stage = stageName(entry.stage);
    const options = stages.map(item => `<option${stage === item.name ? ' selected' : ''}>${item.name}</option>`).join('');
    const history = Array.isArray(entry.history) ? entry.history : [];
    const records = history.slice().reverse().map(line => `<li>${h(line)}</li>`).join('');
    A.appModal('Detalhes da candidatura', `<div class="j-dialog">${jobHeading(job)}
      <label class="j-field" for="j-next-stage">Etapa atual</label><select class="j-input" id="j-next-stage">${options}</select>
      <label class="j-field" for="j-entry-note">Anotação</label><textarea class="j-input" rows="3" maxlength="1500" id="j-entry-note" placeholder="Ex.: combinar horário da entrevista">${h(entry.note || '')}</textarea>
      <section class="j-history"><h3>Histórico</h3>${records ? `<ol>${records}</ol>` : '<p>As próximas atualizações aparecerão aqui.</p>'}</section>
      <div class="j-editor-actions">${btn('close','Fechar','btn secondary')}${btn('update','Salvar alterações','btn primary',{id:job.id,icon:'check'})}</div></div>`);
  }
  function sendBatch() {
    const ready = activeJobs().filter(job => packet(job.id).approved === true);
    if (!ready.length) return;
    const ids = new Set(ready.map(item => key(item.id)));
    const stamp = time();
    for (const job of ready) {
      const id = job.id;
      setPacket(id,{letter:letterFor(job),approved:true});
      A.state.board.unshift({id,profileId:A.profileForJob(id).id,stage:stages[0].name,note:'',history:[`${stamp} · Envio simulado para a empresa.`]});
    }
    A.state.day = A.state.day.filter(id => !ids.has(key(id)));
    lastSent = ready.map(item => item.id);
    noteToast(labelCount(ready.length,'candidatura registrada no quadro.','candidaturas registradas no quadro.'));
    document.querySelector('.j-receipt button')?.focus({preventScroll:true});
  }
  const actions = A.actions;
  actions['j-radar'] = () => A.go('radar');
  actions['j-day'] = () => A.go('lista');
  actions['j-board'] = () => A.go('quadro');
  actions['j-close'] = () => A.closeModal();
  actions['j-send'] = sendBatch;
  actions['j-review'] = el => {
    const job = jobById(el.dataset.id);
    if (job && isInDay(job.id)) review(job);
  };
  actions['j-save-letter'] = el => {
    const job = jobById(el.dataset.id);
    const field = document.getElementById('j-letter');
    if (!job || !field || !isInDay(job.id)) return;
    const letter = field.value.trim();
    const changed = letter !== letterFor(job);
    setPacket(job.id, {letter,approved:!changed && packet(job.id).approved === true});
    A.closeModal();
    noteToast(changed ? 'Pacote salvo. Revise a nova versão antes de aprovar.' : 'Pacote salvo.');
    focusRow('review',job.id);
  };
  actions['j-approve'] = el => {
    const job = jobById(el.dataset.id);
    if (!job || !isInDay(job.id)) return;
    const approved = packet(job.id).approved !== true;
    setPacket(job.id,{approved, letter:letterFor(job)});
    noteToast(approved ? 'Pacote aprovado para o envio simulado.' : 'Aprovação retirada.');
    focusRow('approve',job.id);
  };
  actions['j-approve-all'] = () => {
    const jobs = activeJobs();
    if (!jobs.length) return;
    for (const job of jobs) setPacket(job.id,{approved:true,letter:letterFor(job)});
    noteToast('Todos os pacotes da lista foram aprovados.');
  };
  actions['j-return'] = el => {
    const id = el.dataset.id;
    if (!isInDay(id)) return;
    A.state.day = A.state.day.filter(value => !same(value,id));
    A.restoreJob(id);


    setPacket(id,{approved:false});
    lastSent = [];
    A.closeModal();
    noteToast('Vaga devolvida ao radar de ' + A.profileForJob(id).label + '.');
  };
  actions['j-card'] = el => {
    const job = jobById(el.dataset.id);
    const entry = entryById(el.dataset.id);
    if (job && entry) showBoardEntry(job,entry);
  };
  actions['j-update'] = el => {
    const entry = entryById(el.dataset.id);
    const select = document.getElementById('j-next-stage');
    const input = document.getElementById('j-entry-note');
    if (!entry || !input || !stages.some(stage => stage.name === select?.value)) return;
    const stage = select.value;
    const note = input.value.trim();
    const previous = stageName(entry.stage);
    if (stage === previous && note === (entry.note || '')) return A.closeModal();
    if (!Array.isArray(entry.history)) entry.history = [];
    const stamp = time();
    if (stage !== previous) entry.history.push(`${stamp} · Etapa alterada de ${previous} para ${stage}.`);
    if (note !== (entry.note || '')) entry.history.push(`${stamp} · ${note ? 'Anotação: '+note : 'Anotação removida.'}`);
    entry.note = note;
    entry.stage = stage;
    A.closeModal();
    noteToast('Candidatura atualizada.');
    focusRow('card',entry.id);
  };
  window.JourneyView = () => A.route() === 'quadro' ? boardView() : dayView();
})();
