(() => {
  'use strict';

  const roles = { ai: 'AI Engineer', java: 'Java Engineer', custom: '' };
  let installed = false;

  function selector() {
    const app = window.A;
    const options = app.profiles().map(profile =>
      '<option value="' + app.h(profile.id) + '"' +
      (profile.id === app.state.activeProfileId ? ' selected' : '') + '>' +
      app.h(profile.label || profile.role) + '</option>'
    ).join('');
    return '<div class="multi-selector">' +
      '<div class="multi-field"><label for="multi-active">Perfil de busca</label>' +
      '<span class="multi-select-wrap"><select id="multi-active" data-multi-select>' +
      options + '</select>' + app.icon('chevron-down', 14) + '</span></div>' +
      '<button type="button" class="multi-create" data-act="multi-create" aria-haspopup="dialog">' +
      app.icon('plus', 15) + 'Criar perfil</button></div>';
  }

  function openCreate() {
    const app = window.A;
    app.appModal('Criar perfil de busca',
      '<form class="multi-form" id="multi-form">' +
      '<label for="multi-preset">Cargo de interesse</label>' +
      '<select id="multi-preset" name="preset"><option value="ai">AI Engineer</option>' +
      '<option value="java">Java Engineer</option><option value="custom">Outro cargo</option></select>' +
      '<label for="multi-name">Nome do perfil</label>' +
      '<input id="multi-name" name="label" type="text" required maxlength="120" ' +
      'value="AI Engineer" data-suggested="AI Engineer" autocomplete="off" aria-describedby="multi-help">' +
      '<p id="multi-help">Você pode ajustar os filtros depois.</p>' +
      '<div class="multi-form-actions"><button type="button" class="btn secondary" ' +
      'data-act="modal-close">Cancelar</button><button type="submit" class="btn primary">Criar perfil</button></div></form>'
    );
    document.getElementById('multi-preset').focus();
  }

  function updatePreset(form) {
    const preset = form.elements.preset.value;
    const name = form.elements.label;
    if (!name.value.trim() || name.value === name.dataset.suggested) name.value = roles[preset];
    name.dataset.suggested = roles[preset];
    name.setCustomValidity('');
    name.placeholder = preset === 'custom' ? 'Ex.: Analista de dados' : '';
    form.querySelector('#multi-help').textContent = preset === 'custom'
      ? 'Use o cargo desejado como nome. Em seguida, ajuste os filtros de busca.'
      : 'Você pode ajustar os filtros depois.';
    if (preset === 'custom') name.focus();
  }

  function install() {
    if (installed) return;
    installed = true;
    const app = window.A;
    app.actions['multi-create'] = openCreate;
    document.addEventListener('change', event => {
      const control = event.target;
      if (control.matches('[data-multi-select]')) {
        app.selectProfile(control.value);
        document.querySelector('[data-multi-select]')?.focus({ preventScroll: true });
      } else if (control.id === 'multi-preset') {
        updatePreset(control.form);
      }
    });
    document.addEventListener('input', event => {
      if (event.target.id === 'multi-name') event.target.setCustomValidity('');
    });
    document.addEventListener('submit', event => {
      const form = event.target;
      if (form.id !== 'multi-form') return;
      event.preventDefault();
      const name = form.elements.label;
      const label = name.value.trim();
      if (!label) {
        name.setCustomValidity('Digite o nome do perfil.');
        name.reportValidity();
        return;
      }
      if (app.profiles().some(profile => profile.label.localeCompare(label, 'pt-BR', { sensitivity: 'base' }) === 0)) {
        name.setCustomValidity('Você já tem um perfil com esse nome. Escolha outro nome.');
        name.reportValidity();
        return;
      }
      const preset = form.elements.preset.value;
      app.closeModal();
      app.createProfile(label, preset);
      if (document.activeElement === document.body) {
        document.querySelector('[data-multi-select]')?.focus({ preventScroll: true });
      }
    });
  }

  window.MultiUI = { selector, install };
})();
