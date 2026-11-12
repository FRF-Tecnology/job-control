(() => {
  'use strict';

  const A = window.A;
  const grades = ['all', 'A', 'B', 'C'];
  const gradeLabels = { A: 'Alta compatibilidade', B: 'Compatibilidade média', C: 'Baixa compatibilidade' };
  const mobile = window.matchMedia('(max-width: 760px)');
  let filter = 'all';
  let selectedId = null;
  let mobileDetail = false;
  let lastSkipped = null;

  function resultJobs() {
    const jobs = new Map(A.jobs.map(job => [job.id, job]));
    return [...new Set(A.state.results)].map(id => jobs.get(id)).filter(Boolean);
  }

  function availableJobs(results) {
    const reviewed = new Set([...A.state.day, ...A.state.skipped, ...A.state.board.map(entry => entry.id)]);
    return results.filter(job => !reviewed.has(job.id));
  }

  function filteredJobs(jobs) {
    return filter === 'all' ? jobs : jobs.filter(job => job.grade === filter);
  }

  function redraw(focusSelector, scrollToSection = false) {
    A.render();
    const target = document.querySelector(focusSelector) || document.querySelector('#rd-title');
    target?.focus({ preventScroll: true });
    if (scrollToSection && mobile.matches) {
      document.querySelector('.rd-results')?.scrollIntoView({ block: 'start' });
    } else if (target?.classList.contains('rd-row')) {
      target.scrollIntoView({ block: 'nearest' });
    }
  }

  function gradeMark(job) {
    return `<span class="rd-grade rd-grade-${A.h(job.grade)}" aria-label="Nota ${A.h(job.grade)}, ${A.h(gradeLabels[job.grade])}">${A.h(job.grade)}</span>`;
  }

  function row(job) {
    const active = job.id === selectedId;
    return `<li><button type="button" class="rd-row${active ? ' rd-selected' : ''}" data-act="rd-select" data-id="${A.h(job.id)}" aria-controls="rd-detail"${active ? ' aria-current="true"' : ''}>
      <span class="rd-row-mark" aria-hidden="true">${A.mark(job, 37)}</span>
      <span class="rd-row-info"><span class="rd-company">${A.h(job.company)}</span><strong>${A.h(job.title)}</strong><span class="rd-location">${A.icon('pin', 13)} ${A.h(job.location)}</span></span>
      <span class="rd-row-end">${gradeMark(job)}<span class="rd-open-icon" aria-hidden="true">${A.icon('chevron-right', 14)}</span></span>
    </button></li>`;
  }

  function filterButtons() {
    return `<div class="rd-filters" role="group" aria-label="Filtrar vagas pela nota de compatibilidade">
      ${grades.map(grade => `<button type="button" data-act="rd-filter" data-grade="${grade}" aria-pressed="${filter === grade}" aria-label="${grade === 'all' ? 'Todas as notas' : 'Nota ' + grade}">${grade === 'all' ? 'Todas' : grade}</button>`).join('')}
    </div>`;
  }

  function undoButton() {
    if (!lastSkipped || !A.state.skipped.includes(lastSkipped.id) || !A.state.results.includes(lastSkipped.id)) return '';
    return `<div class="rd-undo"><span>Vaga de ${A.h(lastSkipped.company)} pulada.</span><button type="button" data-act="rd-undo">Desfazer</button></div>`;
  }

  function fact(icon, label, value) {
    return `<div class="rd-fact"><dt>${A.icon(icon, 15)} ${label}</dt><dd>${A.h(value || 'Não informado')}</dd></div>`;
  }

  function detail(job) {
    return `<article class="rd-detail" id="rd-detail" aria-labelledby="rd-job-heading">
      <button type="button" class="rd-back" data-act="rd-back">${A.icon('chevron-left', 16)} Voltar às vagas</button>
      <div class="rd-detail-head">
        <div class="rd-employer"><span aria-hidden="true">${A.mark(job, 35)}</span><span>${A.h(job.company)}</span></div>
        <h3 id="rd-job-heading" tabindex="-1">${A.h(job.title)}</h3>
      </div>
      <dl class="rd-facts">${fact('pin', 'Localização', job.location)}${fact('briefcase', 'Modalidade', job.mode)}${fact('wallet', 'Salário mensal', job.salary)}</dl>
      <div class="rd-actions"><button type="button" class="btn primary" data-act="rd-add" data-id="${A.h(job.id)}">${A.icon('plus', 17)} Adicionar à lista</button><button type="button" class="btn quiet rd-skip" data-act="rd-skip" data-id="${A.h(job.id)}">Pular ${A.icon('chevron-right', 15)}</button></div>
      <div class="rd-analysis"><div class="rd-analysis-head"><span class="rd-analysis-name">${A.icon('sparkles', 16)} Análise da IA</span><span class="rd-grade-label rd-grade-${A.h(job.grade)}">Nota ${A.h(job.grade)}</span></div><p>${A.h(job.reason)}</p></div>
      ${job.description ? `<div class="rd-description"><h4>Sobre a vaga</h4><p>${A.h(job.description)}</p></div>` : ''}
      ${job.tags?.length ? `<ul class="rd-tags" aria-label="Competências e requisitos">${job.tags.map(tag => `<li>${A.h(tag)}</li>`).join('')}</ul>` : ''}
      ${undoButton()}
    </article>`;
  }

  function emptyState(icon, title, message, actions) {
    return `<div class="rd-empty" id="rd-detail"><span class="rd-empty-icon" aria-hidden="true">${A.icon(icon, 24)}</span><h3 tabindex="-1" id="rd-job-heading">${title}</h3><p>${message}</p><div class="rd-empty-actions">${actions}</div>${undoButton()}</div>`;
  }

  function loadingState() {
    return `<div class="rd-loading" aria-busy="true"><p role="status">Buscando vagas para o seu perfil…</p><div class="rd-skeletons" aria-hidden="true">${Array.from({ length: 3 }, () => '<div><i></i><span><b></b><b></b></span></div>').join('')}</div></div>`;
  }

  let renderedProfile;
  window.RadarView = function RadarView() {
    if (renderedProfile !== A.state.activeProfileId) {
      renderedProfile = A.state.activeProfileId;
      selectedId = null; filter = 'all'; mobileDetail = false; lastSkipped = null;
    }
    const results = resultJobs();
    const remaining = availableJobs(results);
    const visible = filteredJobs(remaining);
    const current = visible.find(job => job.id === selectedId) || visible[0];
    selectedId = current?.id ?? null;
    const ready = A.state.searchStatus === 'ready';
    const count = visible.length;
    let content;

    if (A.state.searchStatus === 'loading') {
      mobileDetail = false;
      content = loadingState();
    } else if (A.state.searchStatus === 'error') {
      content = emptyState('search', 'A busca foi interrompida', 'Tente novamente para carregar as vagas.', '<button type="button" class="btn secondary" data-act="rd-search">Tentar novamente</button>');
    } else if (!results.length) {
      content = emptyState('search', 'Nenhuma vaga encontrada', 'Ajuste suas preferências para ampliar a busca.', '<button type="button" class="btn secondary" data-act="rd-preferences">Ajustar preferências</button>');
    } else if (!remaining.length) {
      const skipped = results.some(job => A.state.skipped.includes(job.id));
      const buttons = '<button type="button" class="btn primary" data-act="rd-list">Ver lista</button>' + (skipped ? '<button type="button" class="btn secondary" data-act="rd-resume">Rever vagas puladas</button>' : '');
      content = emptyState('check', 'Você revisou todas as vagas', 'Acompanhe na lista as vagas que você escolheu.', buttons);
    } else if (!current) {
      content = emptyState('search', `Nenhuma vaga com nota ${A.h(filter)}`, 'Veja as vagas disponíveis nas outras notas.', '<button type="button" class="btn secondary" data-act="rd-filter" data-grade="all">Ver todas as vagas</button>');
    } else {
      content = `<div class="rd-workbench${mobileDetail ? ' rd-show-detail' : ''}"><ul class="rd-list" aria-label="Vagas para revisão">${visible.map(row).join('')}</ul>${detail(current)}</div>`;
    }

    return `<section class="rd-results" aria-labelledby="rd-title"><div class="rd-toolbar"><div class="rd-heading"><h2 id="rd-title" tabindex="-1">Vagas encontradas</h2>${ready ? `<span class="rd-count" aria-label="${count} ${count === 1 ? 'vaga disponível' : 'vagas disponíveis'}">${count}</span>` : ''}</div>${ready && remaining.length ? filterButtons() : ''}</div>${content}</section>`;
  };

  function selectJob(el) {
    selectedId = el.dataset.id;
    mobileDetail = mobile.matches;
    redraw(mobileDetail ? '#rd-job-heading' : '.rd-row[aria-current="true"]', mobileDetail);
  }

  function review(el, action) {
    const available = filteredJobs(availableJobs(resultJobs()));
    const index = available.findIndex(job => job.id === el.dataset.id);
    if (index < 0) return;
    const job = available[index];
    selectedId = (available[index + 1] || available[index - 1])?.id ?? null;
    if (action === 'add') {
      A.state.day.push(job.id);
      A.state.packets[job.id]={profileId:A.state.activeProfileId,approved:false};
    } else {
      A.state.skipped.push(job.id);
      lastSkipped = job;
    }
    A.save();
    redraw('#rd-job-heading', true);
    A.toast(action === 'add' ? 'Vaga adicionada à lista.' : 'Vaga pulada.');
  }

  Object.assign(A.actions, {
    'rd-select': selectJob,
    'rd-filter'(el) {
      if (!grades.includes(el.dataset.grade)) return;
      filter = el.dataset.grade;
      selectedId = null;
      mobileDetail = false;
      redraw(`.rd-filters [data-grade="${filter}"]`);
    },
    'rd-back'() {
      mobileDetail = false;
      redraw('.rd-row[aria-current="true"]');
    },
    'rd-add': el => review(el, 'add'),
    'rd-skip': el => review(el, 'skip'),
    'rd-undo'() {
      if (!lastSkipped || !A.state.skipped.includes(lastSkipped.id)) return;
      A.state.skipped = A.state.skipped.filter(id => id !== lastSkipped.id);
      selectedId = lastSkipped.id;
      if (filter !== 'all' && filter !== lastSkipped.grade) filter = 'all';
      lastSkipped = null;
      mobileDetail = mobile.matches;
      A.save();
      redraw('#rd-job-heading', true);
      A.toast('Vaga recuperada.');
    },
    'rd-resume'() {
      const currentIds = new Set(A.state.results);
      A.state.skipped = A.state.skipped.filter(id => !currentIds.has(id));
      filter = 'all';
      selectedId = null;
      lastSkipped = null;
      mobileDetail = false;
      A.save();
      redraw('.rd-row[aria-current="true"]');
    },
    'rd-search': () => A.search(),
    'rd-list': () => A.go('lista'),
    'rd-preferences': () => A.go('perfil')
  });

  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && mobile.matches && mobileDetail && event.target.closest('.rd-detail')) {
      event.preventDefault();
      A.actions['rd-back']();
      return;
    }
    const row = event.target.closest('.rd-row');
    if (!row || !['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
    const rows = [...row.closest('.rd-list').querySelectorAll('.rd-row')];
    const index = rows.indexOf(row);
    const next = { ArrowDown: Math.min(index + 1, rows.length - 1), ArrowUp: Math.max(0, index - 1), Home: 0, End: rows.length - 1 }[event.key];
    event.preventDefault();
    if (mobile.matches) rows[next].focus();
    else selectJob(rows[next]);
  });
})();
