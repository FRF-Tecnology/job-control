(() => {
  'use strict';

  const providers = ['Claude', 'OpenAI', 'Gemini', 'Grok'];
  const exampleFile = 'Curriculo-exemplo.pdf';
  const sample = {
    name: 'Marina Costa', role: 'Product Designer', level: 'Sênior',
    primary: 'Figma, Design de produto', secondary: 'Design Systems', english: 'Avançado'
  };
  const blank = {
    name: '', role: '', primary: '', secondary: '', level: '', english: '', skip: '',
    currency: 'USD', min: '', max: '', mode: 'Remoto', market: 'Brasil e exterior',
    provider: '', cv: '', cvExample: false
  };
  const icons = {
    user: '<circle cx="12" cy="8" r="3.5"/><path d="M5 21v-2a7 7 0 0 1 14 0v2"/>',
    filters: '<path d="M3 7h7m4 0h7M3 17h11m4 0h3"/><circle cx="12" cy="7" r="2"/><circle cx="16" cy="17" r="2"/>',
    plug: '<path d="m13 7 4 4m-7-1 4 4m-8 0 6-6 6 6-6 6a4.2 4.2 0 0 1-6-6Zm3 7-3 2m12-13 4-4m-8 0 4-4"/>',
    file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Zm0 0v6h6M8 13h8m-8 4h5"/>',
    upload: '<path d="M12 16V3m-5 5 5-5 5 5M4 16v4a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-4"/>',
    spark: '<path d="m12 3 1.9 5.3 5.1 1.9-5.1 1.9-1.9 5.2-1.9-5.2L5 10.2l5.1-1.9L12 3Zm7 12 .9 2.3L22 18l-2.1.8L19 21l-.8-2.2L16 18l2.2-.7L19 15Z"/>',
    arrow: '<path d="M4 12h15m-6-6 6 6-6 6"/>',
    back: '<path d="M20 12H5m6-6-6 6 6 6"/>',
    check: '<path d="m5 12 4 4L19 6"/>',
    restart: '<path d="M3 10a9 9 0 1 1 1 7M3 4v6h6"/>',
    shield: '<path d="m12 2 8 3v6c0 5-3.8 8.2-8 11-4.2-2.8-8-6-8-11V5Zm-3.5 9.7 2.2 2.3 4.8-5"/>'
  };
  const icon = (key, size = 18) => '<svg class="pr-icon" aria-hidden="true" width="' + size + '" height="' + size + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round">' + icons[key] + '</svg>';
  const app = () => window.A;
  const esc = value => app().h(String(value ?? ''));
  let session;
  let registered = false;

  function begin(route) {
    const saved = app().state.profile || {};
    const draft = route === 'inicio' ? { ...blank } : {
      ...blank, ...saved, cvExample: saved.cvExample ?? saved.cv === exampleFile
    };
    return { route, profileId: app().state.activeProfileId, draft, step: 1, issues: {}, reading: false, revision: 0, suggested: false, touched: new Set(), autofilled: new Set() };
  }

  function button(action, text, style = 'secondary', glyph = '', side = 'before') {
    const mark = glyph ? icon(glyph, 16) : '';
    return '<button class="btn ' + style + ' pr-button" type="button" data-act="pr-' + action + '">' +
      (side === 'before' ? mark : '') + text + (side === 'after' ? mark : '') + '</button>';
  }

  function title(mark, name) {
    return '<div class="pr-card-title"><span class="pr-tile">' + icon(mark) + '</span><h2>' + name + '</h2></div>';
  }

  function errorFor(key) {
    const message = session.issues[key];
    return '<span id="pr-error-' + key + '" class="pr-error" role="alert"' +
      (message ? '' : ' hidden') + '>' + esc(message) + '</span>';
  }

  function field(key, name, opts = {}) {
    const value = session.draft[key] ?? '';
    const id = 'pr-field-' + key;
    const attrs = ' id="' + id + '" data-pr-field="' + key + '" aria-describedby="pr-error-' + key + '"' +
      (session.issues[key] ? ' aria-invalid="true"' : '') + (opts.required ? ' required' : '');
    let control;
    if (opts.list) {
      const choices = [...new Set([...opts.list, ...(value ? [value] : [])])];
      control = '<select' + attrs + '><option value="">Selecione</option>' +
        choices.map(item => '<option value="' + esc(item) + '"' + (value === item ? ' selected' : '') + '>' + esc(item) + '</option>').join('') + '</select>';
    } else {
      control = '<input' + attrs + (opts.number ? ' type="number" min="0.01" step=".01" inputmode="decimal"' : ' type="text" maxlength="' + (opts.wide ? 350 : 120) + '"') +
        ' value="' + esc(value) + '" placeholder="' + esc(opts.placeholder || '') + '">';
    }
    return '<div class="pr-field' + (opts.wide ? ' pr-wide' : '') + '"><label for="' + id + '">' + name + '</label>' + control + errorFor(key) + '</div>';
  }

  function professional() {
    return '<section class="panel pr-card">' + title('user', 'Perfil profissional') +
      '<div class="pr-fields">' +
      field('name', 'Nome', { required: true, placeholder: 'Seu nome' }) +
      field('role', 'Cargo de interesse', { required: true, placeholder: 'Ex.: Product Designer' }) +
      levels() +
      field('primary', 'Habilidades principais', { required: true, wide: true, placeholder: 'Ex.: Figma, design de produto' }) +
      field('secondary', 'Habilidades complementares', { placeholder: 'Ex.: Design Systems' }) +
      field('english', 'Nível de inglês', { required: true, list: ['Não falo inglês', 'Básico', 'Intermediário', 'Avançado', 'Fluente'] }) +
      '</div></section>';
  }

  function levels() {
    const values = String(session.draft.level ?? '').split(',').map(item => item.trim()).filter(Boolean);
    const options = [...new Set(['Estágio', 'Júnior', 'Pleno', 'Sênior', 'Liderança', ...values])];
    return '<fieldset class="pr-levels pr-wide" aria-describedby="pr-error-level"><legend>Níveis de experiência aceitos</legend>' +
      '<div>' + options.map((item, index) => '<label><input type="checkbox" data-pr-level value="' + esc(item) + '"' +
        (index ? '' : ' id="pr-field-level"') + (values.includes(item) ? ' checked' : '') + '><span>' + esc(item) + '</span></label>').join('') +
      '</div>' + errorFor('level') + '</fieldset>';
  }

  function preferences() {
    return '<section class="panel pr-card">' + title('filters', 'Filtros de busca') +
      '<div class="pr-fields">' +
      field('market', 'Mercado', { required: true, list: ['Brasil', 'Exterior', 'Brasil e exterior'] }) +
      field('mode', 'Modelo de trabalho', { required: true, list: ['Remoto', 'Híbrido', 'Presencial', 'Todos'] }) +
      field('skip', 'Pular vagas com estes termos', { wide: true, placeholder: 'Separe os termos por vírgula' }) +
      '<fieldset class="pr-pay pr-wide"><legend>Faixa salarial mensal <small>Opcional</small></legend><div>' +
      field('currency', 'Moeda', { list: ['USD', 'BRL', 'EUR'] }) +
      field('min', 'Mínimo', { number: true, placeholder: '4.000' }) +
      field('max', 'Máximo', { number: true, placeholder: '6.000' }) + '</div></fieldset></div></section>';
  }

  function connection() {
    const active = session.draft.provider;
    return '<section class="panel pr-card">' + title('plug', 'Conexão de IA') +
      '<p class="pr-note">Escolha um provedor para simular a conexão.</p>' +
      '<div class="pr-providers" role="group" aria-label="Provedores de IA">' +
      providers.map(name => {
        const selected = name === active;
        return '<button type="button" data-act="pr-provider" data-provider="' + name + '" aria-pressed="' + selected +
          '" class="pr-provider"><span class="pr-brand" aria-hidden="true">' + app().brand(name) + '</span><span class="pr-provider-name">' +
          name + '</span><span class="pr-radio" aria-hidden="true">' + (selected ? icon('check', 12) : '') + '</span></button>';
      }).join('') + '</div>' + errorFor('provider') + (active ? '<p class="pr-connected"><i></i>Conexão simulada com ' + esc(active) + '</p>' : '') + '</section>';
  }

  function resume() {
    const { cv, cvExample } = session.draft;
    const helper = session.reading ? 'Verificando o PDF…' : cv ? (cvExample ? 'Currículo fictício de exemplo' : 'PDF verificado neste dispositivo') : 'PDF de até 20 MB';
    return '<section class="panel pr-card">' + title('file', 'Currículo') +
      '<div class="pr-drop' + (cv ? ' pr-has-file' : '') + '" data-pr-drop>' +
      '<span class="pr-file-icon" aria-hidden="true">' + icon(cv ? 'file' : 'upload', 23) + '</span>' +
      '<div class="pr-file-meta"><strong>' + esc(cv || 'Arraste seu currículo para cá') + '</strong><small role="status">' + helper + '</small></div>' +
      button('choose-file', cv ? 'Trocar PDF' : 'Escolher PDF', 'secondary') +
      '<input data-pr-file type="file" accept=".pdf,application/pdf" aria-label="Currículo em PDF" hidden></div>' + errorFor('cv') +
      '<div class="pr-file-actions">' + (!cvExample ? button('example', 'Usar currículo de exemplo', 'quiet') : '') +
      (cv ? button('remove-file', 'Remover', 'quiet') : '') + '</div>' +
      '<p class="pr-local">' + icon('shield', 16) + '<span>Ao escolher seu PDF, o protótipo guarda apenas o nome do arquivo neste navegador.</span></p></section>';
  }

  function suggestion() {
    if (!session.draft.cvExample) return '';
    return '<section class="pr-suggestion"><span class="pr-tile">' + icon('spark') +
      '</span><div><h2>Sugestões do currículo de exemplo</h2><p>' +
      (session.suggested ? 'Confira os campos preenchidos com o perfil fictício de Marina Costa.' : 'Simule o preenchimento dos campos profissionais com o perfil de Marina Costa.') +
      '</p></div>' + (session.suggested ? '<span class="pr-applied">' + icon('check', 15) + ' Aplicadas</span>' : button('suggest', 'Aplicar sugestões', 'secondary')) + '</section>';
  }

  function header(text, sub, action) {
    return '<header class="pr-heading"><div><h1 id="pr-heading" tabindex="-1">' + text + '</h1><p>' + sub + '</p></div>' + (action || '') + '</header>';
  }

  function wizard() {
    const first = session.step === 1;
    const art = first && app().landscape ? '<div class="pr-landscape" aria-hidden="true">' + app().landscape() + '</div>' : '';
    const intro = first
      ? header('Conecte sua IA e seu currículo', 'Escolha o provedor e o documento de referência para a busca.', art)
      : header('Defina seu perfil de busca', 'Revise seus dados e escolha as condições das vagas que deseja encontrar.');
    const progress = '<div class="pr-progress" aria-label="Etapa ' + session.step + ' de 2"><span class="' +
      (first ? 'pr-current' : 'pr-complete') + '"><i>' + (first ? '1' : icon('check', 13)) + '</i>IA e currículo</span><b></b><span class="' +
      (first ? '' : 'pr-current') + '"><i>2</i>Perfil e filtros</span></div>';
    return '<div class="pr-page pr-wizard">' + progress + intro +
      '<form novalidate data-pr-submit="' + (first ? 'next' : 'finish-search') + '">' +
      (first ? '<div class="pr-onboard-cards">' + connection() + resume() + '</div>' : suggestion() + '<div class="pr-onboard-cards">' + professional() + preferences() + '</div>') +
      '<div class="pr-footer">' + (first ? button('cancel', 'Voltar ao perfil', 'quiet', 'back') : button('back', 'Voltar', 'quiet', 'back')) +
      '<div class="pr-footer-main">' + (first ? button('next', 'Continuar', 'primary', 'arrow', 'after') :
        button('finish', 'Somente salvar', 'secondary') + button('finish-search', 'Salvar e buscar vagas', 'primary', 'arrow', 'after')) +
      '</div></div></form></div>';
  }

  function editor() {
    return '<div class="pr-page">' +
      header('Perfil de busca', 'Ajuste seu perfil profissional, as condições de trabalho e os dados usados na triagem.',
        button('start', 'Experimentar primeiro acesso', 'secondary', 'restart')) + window.MultiUI.selector() +
      '<form novalidate data-pr-submit="save"><div class="pr-edit-grid"><div class="pr-stack">' +
      professional() + preferences() + '</div><div class="pr-stack">' + connection() + resume() + '</div></div>' +
      '<div class="pr-footer pr-end">' + button('save', 'Salvar perfil', 'primary', 'check') + '</div></form></div>';
  }

  function redraw(focusSelector) {
    app().render();
    if (focusSelector) document.querySelector(focusSelector)?.focus();
  }

  function dropSuggestions() {
    for (const key of session.autofilled) {
      if (!session.touched.has(key)) session.draft[key] = '';
    }
    session.autofilled.clear();
    session.suggested = false;
  }

  function adoptFile(name, example) {
    if (!example && session.draft.cvExample) dropSuggestions();
    session.draft.cv = name;
    session.draft.cvExample = example;
  }

  async function readLocal(files) {
    if (!files?.length) return;
    const current = session;
    const version = ++current.revision;
    const pdf = files[0];
    current.reading = true;
    delete current.issues.cv;
    redraw();
    let issue = '';
    try {
      if (files.length > 1) issue = 'Escolha um único PDF por vez.';
      else if (!/\.pdf$/i.test(pdf.name)) issue = 'Escolha um arquivo com extensão .pdf.';
      else if (pdf.size > 20 * 1024 * 1024) issue = 'Escolha um PDF de até 20 MB.';
      else {
        const [start, end] = await Promise.all([pdf.slice(0, 5).text(), pdf.slice(-1024).text()]);
        if (start !== '%PDF-' || !end.includes('%%EOF')) issue = 'O arquivo não parece ser um PDF completo. Exporte o currículo em PDF e tente novamente.';
      }
    } catch {
      issue = 'Não foi possível abrir o PDF neste navegador. Escolha o arquivo novamente.';
    }
    if (session !== current || current.revision !== version) return;
    current.reading = false;
    if (issue) current.issues.cv = issue;
    else {
      delete current.issues.cv;
      adoptFile(pdf.name, false);
    }
    if (app().route() === current.route) redraw('[data-act="pr-choose-file"]');
  }

  function validate(includeFields) {
    const d = session.draft;
    const issues = {};
    if (!providers.includes(d.provider)) issues.provider = 'Selecione um provedor para continuar.';
    if (session.reading) issues.cv = 'Aguarde a verificação do PDF.';
    else if (session.issues.cv && d.cv) issues.cv = session.issues.cv;
    else if (!d.cv) issues.cv = 'Escolha um PDF ou use o currículo de exemplo.';
    if (includeFields) {
      const required = { name: 'Informe seu nome.', role: 'Informe o cargo de interesse.', level: 'Selecione a senioridade.',
        english: 'Selecione o nível de inglês.', primary: 'Informe pelo menos uma habilidade principal.',
        market: 'Escolha o mercado.', mode: 'Escolha o modelo de trabalho.' };
      Object.entries(required).forEach(([key, message]) => {
        if (!String(d[key] ?? '').trim() || (key === 'primary' && !String(d[key]).split(/[,;\n]/).some(x => x.trim()))) issues[key] = message;
      });
      for (const key of ['min', 'max']) {
        const field = document.querySelector('[data-pr-field="' + key + '"]');
        if (field?.validity.badInput || (d[key] !== '' && d[key] != null && (!Number.isFinite(Number(d[key])) || Number(d[key]) <= 0))) {
          issues[key] = 'Informe um valor positivo ou deixe em branco.';
        }
      }
      if (Number(d.min) > 0 && Number(d.max) > 0 && Number(d.max) < Number(d.min)) issues.max = 'O máximo deve ser maior ou igual ao mínimo.';
    }
    session.issues = issues;
    const key = Object.keys(issues)[0];
    if (!key) return true;
    if (session.route === 'inicio' && (key === 'cv' || key === 'provider')) session.step = 1;
    const target = key === 'cv' ? '[data-act="pr-choose-file"]' : key === 'provider' ? '[data-act="pr-provider"]' : '#pr-field-' + key;
    redraw(target);
    return false;
  }

  function save(search, exit) {
    if (!validate(true)) return;
    const result = { ...session.draft };
    for (const key of Object.keys(blank)) {
      if (typeof result[key] === 'string') result[key] = result[key].trim();
    }
    for (const key of ['min', 'max']) result[key] = result[key] === '' || result[key] == null ? '' : Number(result[key]);
    const state = app().state;
    if (state.profile.cv !== result.cv || state.profile.name !== result.name) {
      for (const id of state.day) if (state.packets[id]?.profileId === state.activeProfileId) state.packets[id].approved = false;
    }
    state.profile = result;
    app().save();
    session = undefined;
    app().toast('Perfil de busca salvo.');
    if (search) app().search();
    else if (exit) app().go('radar');
    else redraw('[data-act="pr-save"]');
  }

  function register() {
    if (registered) return;
    registered = true;
    const actions = app().actions;
    actions['pr-start'] = () => { session = begin('inicio'); app().go('inicio'); };
    actions['pr-cancel'] = () => { session = undefined; app().go('perfil'); };
    actions['pr-provider'] = el => {
      if (!providers.includes(el.dataset.provider)) return;
      session.draft.provider = el.dataset.provider;
      delete session.issues.provider;
      redraw('[data-provider="' + el.dataset.provider + '"]');
    };
    actions['pr-choose-file'] = () => document.querySelector('[data-pr-file]')?.click();
    actions['pr-remove-file'] = () => {
      session.revision++; session.reading = false;
      adoptFile('', false); delete session.issues.cv;
      redraw('[data-act="pr-choose-file"]');
    };
    actions['pr-example'] = () => {
      session.revision++; session.reading = false;
      adoptFile(exampleFile, true); delete session.issues.cv;
      redraw('[data-act="pr-choose-file"]');
    };
    actions['pr-next'] = () => {
      if (!validate(false)) return;
      session.step = 2;
      redraw('#pr-heading');
    };
    actions['pr-back'] = () => { session.step = 1; session.issues = {}; redraw('#pr-heading'); };
    actions['pr-suggest'] = () => {
      if (!session.draft.cvExample) return;
      Object.entries(sample).forEach(([key, value]) => {
        if (!session.touched.has(key) && !session.draft[key]) {
          session.draft[key] = value;
          session.autofilled.add(key);
          delete session.issues[key];
        }
      });
      session.suggested = true;
      redraw('#pr-field-name');
    };
    actions['pr-finish-search'] = () => save(true, true);
    actions['pr-finish'] = () => save(false, true);
    actions['pr-save'] = () => save(false, false);
  }

  function captureField(event) {
    const el = event.target;
    const isLevel = el.hasAttribute?.('data-pr-level');
    const key = isLevel ? 'level' : el.dataset?.prField;
    if (!session || !Object.hasOwn(blank, key)) return;
    session.draft[key] = isLevel ? Array.from(document.querySelectorAll('[data-pr-level]:checked'), item => item.value).join(', ') : el.value;
    session.touched.add(key);
    delete session.issues[key];
    el.removeAttribute('aria-invalid');
    const feedback = document.getElementById('pr-error-' + key);
    if (feedback) { feedback.hidden = true; feedback.textContent = ''; }
  }
  document.addEventListener('input', captureField);
  document.addEventListener('change', event => {
    if (event.target.matches('[data-pr-file]')) void readLocal(event.target.files);
    else captureField(event);
  });
  document.addEventListener('submit', event => {
    const action = event.target.dataset.prSubmit;
    if (!action) return;
    event.preventDefault();
    app().actions['pr-' + action]?.();
  });
  document.addEventListener('keydown', event => {
    const form = event.target.closest('[data-pr-submit]');
    if (event.key !== 'Enter' || event.isComposing || !form || !event.target.matches('input[data-pr-field]')) return;
    event.preventDefault();
    app().actions['pr-' + form.dataset.prSubmit]?.();
  });
  document.addEventListener('dragover', event => {
    const box = event.target.closest?.('[data-pr-drop]');
    if (!box) return;
    event.preventDefault();
    box.classList.add('pr-dragging');
  });
  document.addEventListener('dragleave', event => {
    const box = event.target.closest?.('[data-pr-drop]');
    if (box && !box.contains(event.relatedTarget)) box.classList.remove('pr-dragging');
  });
  document.addEventListener('drop', event => {
    const box = event.target.closest?.('[data-pr-drop]');
    if (!box) return;
    event.preventDefault();
    box.classList.remove('pr-dragging');
    void readLocal(event.dataTransfer?.files);
  });

  window.ProfileView = () => {
    register();
    const route = app().route();
    if (!session || session.route !== route || session.profileId !== app().state.activeProfileId) session = begin(route);
    return route === 'inicio' ? wizard() : editor();
  };
})();
