(() => {
  'use strict';
  const porId = id => document.getElementById(id);
  const todos = seletor => Array.from(document.querySelectorAll(seletor));
  const elementos = (pai, nome, texto, classe) => {
    const filho = document.createElement(nome);
    if (texto != null) filho.textContent = texto;
    if (classe) filho.className = classe;
    pai.append(filho);
    return filho;
  };
  const paginas = todos('[data-pagina]');
  const passos = todos('[data-etapa]');
  const campos = ['cargo','principal-stack','secundaria-stack','pular','ingles','usd'];
  const erros = ['erro-cv','erro-vagas','erro-condicoes'];
  const escolhas = {
    niveis: ['Estágio','Júnior','Pleno','Sênior','Liderança'],
    modelos: ['Remoto','Híbrido','Presencial'],
    mercado: ['Nacional','Internacional','Os dois']
  };
  let etapa = 0, liberada = 0, leitura = 0;
  let curriculo = null, perfilDeExemplo = false, filtro = 'Todas', resultados = [];

  for (const [grupo, valores] of Object.entries(escolhas)) {
    for (const valor of valores) {
      const label = elementos(porId(grupo),'label');
      const input = elementos(label,'input');
      input.type = grupo === 'mercado' ? 'radio' : 'checkbox';
      input.name = grupo;
      input.value = valor;
      elementos(label,'span',valor);
    }
  }
  const marcados = grupo => todos('input[name="' + grupo + '"]:checked').map(opcao => opcao.value);
  const termos = id => [...new Set(porId(id).value.split(/[,;\n]/).map(item => item.trim()).filter(Boolean))];
  function perfil() {
    const mercado = marcados('mercado')[0] || '';
    return {
      cargo: porId('cargo').value.trim(), niveis: marcados('niveis'), principal: termos('principal-stack'),
      secundaria: termos('secundaria-stack'), pular: termos('pular'), modelos: marcados('modelos'),
      mercado, ingles: porId('ingles').value,
      usd: mercado === 'Nacional' || !porId('usd').value ? null : porId('usd').valueAsNumber
    };
  }
  function mostrarArquivo() {
    porId('drop').classList.toggle('anexado',Boolean(curriculo));
    porId('arquivo-titulo').textContent = curriculo?.name || 'Arraste seu currículo para cá';
    porId('arquivo-info').textContent = curriculo
      ? (perfilDeExemplo ? 'Documento fictício' : 'PDF anexado') + ' · ' + Math.max(1,Math.ceil(curriculo.size / 1024)) + ' KB'
      : 'PDF de até 20 MB';
    porId('escolher').querySelector('span').textContent = curriculo ? 'Trocar PDF' : 'Escolher PDF';
    porId('remover').hidden = !curriculo;
    porId('usar-exemplo').closest('.exemplo').hidden = Boolean(curriculo);
  }
  function limparErro() {
    erros.forEach(id => porId(id).textContent = '');
    todos('[aria-invalid]').forEach(el => el.removeAttribute('aria-invalid'));
  }
  function erro(mensagem, alvo) {
    porId(erros[etapa]).textContent = mensagem;
    if (alvo) { alvo.setAttribute('aria-invalid','true'); alvo.focus(); }
    return false;
  }
  function limparCampos() {
    campos.forEach(id => porId(id).value = '');
    todos('.opcoes input').forEach(item => item.checked = false);
    porId('campo-usd').hidden = false;
  }
  async function anexar(arquivos) {
    const versao = ++leitura;
    limparErro();
    if (arquivos.length !== 1) return erro('Escolha um currículo por vez.',porId('escolher'));
    const arquivo = arquivos[0];
    if (!/\.pdf$/i.test(arquivo.name)) return erro('Escolha um arquivo em PDF.',porId('escolher'));
    if (arquivo.size > 20 * 1024 * 1024) return erro('O PDF deve ter até 20 MB.',porId('escolher'));
    let assinatura;
    try { assinatura = await arquivo.slice(0,5).text(); }
    catch { if (versao === leitura) erro('Não foi possível abrir este arquivo. Tente outro PDF.',porId('escolher')); return; }
    if (versao !== leitura) return;
    if (assinatura !== '%PDF-') return erro('Esse arquivo não tem o formato de um PDF.',porId('escolher'));
    if (perfilDeExemplo) { limparCampos(); liberada = 0; }
    curriculo = arquivo;
    perfilDeExemplo = false;
    mostrarArquivo(); atualizarPassos();
    porId('aviso').textContent = 'Currículo anexado. Você pode continuar.';
  }
  porId('escolher').addEventListener('click', () => porId('pdf').click());
  porId('pdf').addEventListener('change', ev => {
    const arquivos = Array.from(ev.target.files);
    ev.target.value = '';
    if (arquivos.length) anexar(arquivos);
  });
  const drop = porId('drop');
  document.addEventListener('dragover', ev => { if (ev.dataTransfer.types.includes('Files')) ev.preventDefault(); });
  document.addEventListener('drop', ev => { if (ev.dataTransfer.types.includes('Files')) ev.preventDefault(); });
  drop.addEventListener('dragover', ev => { ev.preventDefault(); drop.classList.add('arrastando'); });
  drop.addEventListener('dragleave', ev => { if (!drop.contains(ev.relatedTarget)) drop.classList.remove('arrastando'); });
  drop.addEventListener('drop', ev => {
    ev.preventDefault(); drop.classList.remove('arrastando'); anexar(Array.from(ev.dataTransfer.files));
  });
  porId('remover').addEventListener('click', () => {
    ++leitura;
    if (perfilDeExemplo) limparCampos();
    curriculo = null; perfilDeExemplo = false; liberada = 0;
    limparErro(); mostrarArquivo(); atualizarPassos(); porId('escolher').focus();
  });
  porId('usar-exemplo').addEventListener('click', () => {
    ++leitura;
    const bytes = Uint8Array.from(atob('{{PDF_BASE64}}'),c => c.charCodeAt(0));
    curriculo = new File([bytes],'Currículo de exemplo.pdf',{type:'application/pdf'});
    perfilDeExemplo = true;
    limparCampos();
    porId('cargo').value = 'Desenvolvimento de software';
    porId('principal-stack').value = 'Java, Spring Boot';
    porId('secundaria-stack').value = 'React';
    porId('pular').value = 'Sênior';
    porId('ingles').value = 'Avançado'; porId('usd').value = '3500';
    todos('.opcoes input').forEach(item => item.checked = ['Júnior','Pleno','Remoto','Os dois'].includes(item.value));
    limparErro(); mostrarArquivo(); atualizarPassos(); porId('continuar').focus();
    porId('aviso').textContent = 'Perfil de exemplo carregado. Você pode editar todos os critérios.';
  });
  function atualizarPassos() {
    passos.forEach((botao,indice) => {
      botao.disabled = indice > liberada;
      if (indice === etapa) botao.setAttribute('aria-current','step'); else botao.removeAttribute('aria-current');
      botao.classList.toggle('completa',indice < etapa);
      botao.querySelector('.numero').textContent = indice < etapa ? '✓' : String(indice+1);
      botao.querySelector('.numero').setAttribute('aria-hidden','true');
    });
  }
  function validar() {
    limparErro();
    const dados = perfil();
    if (etapa === 0 && !curriculo) return erro('Anexe um PDF ou use o perfil de exemplo para continuar.',porId('escolher'));
    if (etapa === 1) {
      if (!dados.cargo) return erro('Informe o cargo ou a área que você procura.',porId('cargo'));
      if (!dados.niveis.length) return erro('Escolha pelo menos um nível aceito.',porId('niveis').querySelector('input'));
      if (!dados.principal.length) return erro('Informe pelo menos uma ferramenta ou habilidade principal.',porId('principal-stack'));
    }
    if (etapa === 2) {
      if (!dados.mercado) return erro('Escolha o mercado em que quer procurar vagas.',porId('mercado').querySelector('input'));
      if (!dados.modelos.length) return erro('Selecione ao menos um modelo de trabalho.',porId('modelos').querySelector('input'));
      if (!dados.ingles) return erro('Selecione seu nível de inglês.',porId('ingles'));
      if (!porId('campo-usd').hidden && !porId('usd').validity.valid)
        return erro('Informe um valor mensal válido em dólares ou deixe em branco.',porId('usd'));
    }
    return true;
  }
  function preencherResumo(dl,dados) {
    const linhas = [
      ['Currículo',curriculo.name],['Cargo ou área',dados.cargo],['Níveis aceitos',dados.niveis.join(', ')],
      ['Stack principal',dados.principal.join(', ')],['Stack secundária',dados.secundaria.join(', ') || 'Não informada'],
      ['Pular quando',dados.pular.join(', ') || 'Sem exclusões adicionais'],['Mercado',dados.mercado],
      ['Modelo de trabalho',dados.modelos.join(', ')],['Inglês',dados.ingles]
    ];
    if (dados.mercado !== 'Nacional') linhas.push(['Pretensão em USD',dados.usd ? 'US$ '+dados.usd.toLocaleString('pt-BR')+'/mês' : 'A combinar']);
    dl.replaceChildren();
    for (const [nome,valor] of linhas) { elementos(dl,'dt',nome); elementos(dl,'dd',valor); }
  }
  const grupos = {Todas:'Todas',A:'A · Prioridade',B:'B · Revisar',C:'C · Baixa afinidade',Pulada:'Puladas'};
  function mostrarLista() {
    porId('filtros').replaceChildren();
    for (const [chave,rotulo] of Object.entries(grupos)) {
      const numero = chave === 'Todas' ? resultados.length : resultados.filter(v => v.grupo === chave).length;
      const botao = elementos(porId('filtros'),'button',rotulo+' ('+numero+')');
      botao.type = 'button'; botao.dataset.grupo = chave; botao.setAttribute('aria-pressed',String(filtro === chave));
    }
    const lista = porId('lista');
    lista.replaceChildren(); lista.scrollTop = 0;
    const visiveis = resultados.filter(v => filtro === 'Todas' || v.grupo === filtro);
    if (!visiveis.length) elementos(lista,'p','Nenhuma vaga de exemplo neste grupo.','vazio');
    for (const [indice,vaga] of visiveis.entries()) {
      const card = elementos(lista,'details',null,'vaga');
      card.open = indice === 0;
      const sum = elementos(card,'summary');
      const nota = elementos(sum,'span',vaga.grupo === 'Pulada' ? '↷' : vaga.grupo,'grupo grupo-'+vaga.grupo.toLowerCase());
      nota.setAttribute('aria-label',grupos[vaga.grupo]);
      const texto = elementos(sum,'span');
      elementos(texto,'strong',vaga.titulo);
      elementos(texto,'small',vaga.empresa+' · '+vaga.local);
      const motivos = elementos(card,'ul',null,'motivos');
      for (const motivo of vaga.motivos) elementos(motivos,'li',motivo);
    }
  }
  porId('filtros').addEventListener('click', ev => {
    const botao = ev.target.closest('[data-grupo]');
    if (!botao) return;
    filtro = botao.dataset.grupo; mostrarLista(); porId('filtros').querySelector('[aria-pressed=true]').focus();
  });
  function mostrar(indice,moverFoco = true) {
    ++leitura;
    etapa = indice; limparErro(); atualizarPassos();
    paginas.forEach((pagina,i) => pagina.hidden = i !== indice);
    porId('etapa-atual').textContent = indice === 4 ? 'Configuração concluída' : 'Etapa '+(indice+1)+' de 4';
    todos('.segmentos i').forEach((item,i) => item.classList.toggle('ativa',i<=indice));
    porId('voltar').hidden = indice === 0;
    porId('voltar').textContent = indice === 4 ? 'Editar perfil' : '← Voltar';
    porId('continuar').querySelector('span').textContent = ['Continuar','Continuar','Simular triagem','Concluir configuração','Ver minha triagem'][indice];
    if (indice === 3 || indice === 4) {
      const dados = perfil(); resultados = gerarPrevia(dados);
      if (indice === 3) {
        filtro = 'Todas'; preencherResumo(porId('dados-resumo'),dados); mostrarLista();
      } else {
        const n = grupo => resultados.filter(v => v.grupo === grupo).length;
        porId('resultado-final').textContent = 'Na simulação, '+n('A')+' de '+resultados.length+' vagas ficaram no grupo A e '+n('Pulada')+' foram puladas pelos seus filtros.';
        porId('dados-finais').replaceChildren(); preencherResumo(elementos(porId('dados-finais'),'dl'),dados);
      }
    }
    if (moverFoco) {
      window.scrollTo(0,0); paginas[indice].scrollTop = 0;
      paginas[indice].querySelector('h1').focus({preventScroll:true});
    }
  }
  function irPara(indice) {
    if (indice > etapa && !validar()) return;
    mostrar(indice);
  }
  porId('continuar').addEventListener('click', () => {
    if (etapa === 4) return mostrar(3);
    if (!validar()) return;
    liberada = Math.max(liberada,Math.min(3,etapa+1)); mostrar(etapa+1);
  });
  porId('voltar').addEventListener('click', () => mostrar(etapa === 4 ? 1 : Math.max(0,etapa-1)));
  passos.forEach((botao,i) => botao.addEventListener('click', () => irPara(i)));
  // Uma alteração invalida os passos seguintes até os novos valores serem conferidos.
  document.querySelector('.janela').addEventListener('input', ev => {
    if (!ev.target.matches('.campos input,.campos select')) return;
    limparErro(); liberada = Math.min(liberada,etapa); atualizarPassos();
    if (ev.target.name === 'mercado') porId('campo-usd').hidden = ev.target.value === 'Nacional';
  });
  porId('tema').addEventListener('click', () => {
    const claro = document.documentElement.dataset.tema !== 'claro';
    document.documentElement.dataset.tema = claro ? 'claro' : 'escuro';
    const rotulo = claro ? 'Ativar tema escuro' : 'Ativar tema claro';
    porId('tema').setAttribute('aria-label',rotulo); porId('tema').title = rotulo;
    porId('tema').querySelector('use').setAttribute('href',claro ? '#lua' : '#luz');
  });
  mostrarArquivo(); mostrar(0,false);
})();
