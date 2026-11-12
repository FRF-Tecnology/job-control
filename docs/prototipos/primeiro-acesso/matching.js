(() => {
  'use strict';

  // Vagas e empresas fictícias. Os valores em dólares são mensais.
  const vagas = [
    { titulo: 'Desenvolvimento backend Java', empresa: 'Nimbo do Vale', area: 'backend', nivel: 'Pleno', modelo: 'Remoto', local: 'Brasil', mercado: 'Nacional', requisitos: ['Java', 'Spring Boot'], ingles: 'Sem exigência', moeda: 'BRL', termos: ['produto digital'] },
    { titulo: 'Software Engineer · APIs Java', empresa: 'Harbor Finch Studio', area: 'backend', nivel: 'Pleno', modelo: 'Remoto', local: 'América Latina', mercado: 'Internacional', requisitos: ['Java', 'Spring Boot'], ingles: 'Avançado', moeda: 'USD', mensal: 4200, termos: ['produto digital'] },
    { titulo: 'Desenvolvimento full stack Java e React', empresa: 'Trama de Cedro', area: 'fullstack', nivel: 'Júnior', modelo: 'Remoto', local: 'Brasil', mercado: 'Nacional', requisitos: ['Java', 'React'], ingles: 'Sem exigência', moeda: 'BRL', termos: ['consultoria'] },
    { titulo: 'Backend Engineer · plataforma', empresa: 'Cedar Kite Lab', area: 'backend', nivel: 'Pleno', modelo: 'Remoto', local: 'América Latina', mercado: 'Internacional', requisitos: ['Java', 'Spring Boot'], ingles: 'Intermediário', moeda: 'USD', mensal: null, termos: ['plantão obrigatório', 'produto digital'] },
    { titulo: 'Desenvolvimento frontend React', empresa: 'Ateliê Vento Norte', area: 'frontend', nivel: 'Júnior', modelo: 'Remoto', local: 'Brasil', mercado: 'Nacional', requisitos: ['React', 'TypeScript'], ingles: 'Básico', moeda: 'BRL', termos: ['agência'] },
    { titulo: 'Software Engineer · serviços em C#', empresa: 'Ember Canal Works', area: 'backend', nivel: 'Pleno', modelo: 'Remoto', local: 'América Latina', mercado: 'Internacional', requisitos: ['C#', '.NET'], ingles: 'Fluente', moeda: 'USD', mensal: 2900, termos: ['consultoria'] },
    { titulo: 'Desenvolvimento backend sênior', empresa: 'Estúdio Sabiá Azul', area: 'backend', nivel: 'Sênior', modelo: 'Remoto', local: 'Brasil', mercado: 'Nacional', requisitos: ['Java', 'Spring Boot'], ingles: 'Avançado', moeda: 'BRL', termos: ['liderança técnica'] },
    { titulo: 'Análise de dados', empresa: 'Oficina Maré Alta', area: 'dados', nivel: 'Pleno', modelo: 'Híbrido', local: 'Recife', mercado: 'Nacional', requisitos: ['SQL', 'Power BI'], ingles: 'Intermediário', moeda: 'BRL', termos: ['consultoria'] },
    { titulo: 'Design de produto digital', empresa: 'Papel de Aurora', area: 'design', nivel: 'Pleno', modelo: 'Remoto', local: 'Brasil', mercado: 'Nacional', requisitos: ['Figma', 'Pesquisa com usuários'], ingles: 'Sem exigência', moeda: 'BRL', termos: ['produto digital'] },
    { titulo: 'Análise de marketing digital', empresa: 'Coral de Tinta', area: 'marketing', nivel: 'Júnior', modelo: 'Presencial', local: 'Curitiba', mercado: 'Nacional', requisitos: ['Google Analytics', 'SEO'], ingles: 'Sem exigência', moeda: 'BRL', termos: ['agência'] },
    { titulo: 'Análise financeira', empresa: 'Serra de Oliva', area: 'financas', nivel: 'Pleno', modelo: 'Remoto', local: 'Brasil', mercado: 'Nacional', requisitos: ['Excel', 'Planejamento financeiro'], ingles: 'Sem exigência', moeda: 'BRL', termos: ['controladoria'] },
  ];

  const ordem = ['A', 'B', 'C', 'Pulada'];
  const niveisDeIngles = ['nao falo ingles', 'basico', 'intermediario', 'avancado', 'fluente'];
  const normalizar = valor => String(valor ?? '').normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase().trim().replace(/\s+/g, ' ');
  const lista = valor => Array.isArray(valor) ? valor.map(item => String(item).trim()).filter(Boolean) : [];
  const aliases = {
    'springboot': 'spring boot', 'react.js': 'react', 'reactjs': 'react', 'csharp': 'c#', 'c sharp': 'c#',
    'dotnet': '.net', 'powerbi': 'power bi', 'js': 'javascript', 'ts': 'typescript',
    'jr': 'junior', 'jr.': 'junior', 'sr': 'senior', 'sr.': 'senior', 'mid-level': 'pleno',
    'remota': 'remoto', 'home office': 'remoto', 'hibrida': 'hibrido',
  };
  const chave = valor => aliases[normalizar(valor)] || normalizar(valor);

  const termosDeCargo = {
    backend: /\b(back[ -]?end|desenvolvimento de apis|java developer|desenvolvedora? (?:java|c#|python))(?=\s|$|[-,(])/,
    frontend: /\b(front[ -]?end|react developer|desenvolvedor[a]? react)\b/,
    fullstack: /\bfull[ -]?stack\b/,
    dados: /\b(dados|data analyst|business intelligence)\b/,
    design: /\b(design de produto|product design(?:er)?|designer de produto|ux|ui)\b/,
    marketing: /\bmarketing\b/,
    financas: /\b(financas|financeir[oa]|financial analyst|controladoria)\b/,
  };
  const desenvolvimento = /^(?:(?:desenvolvimento|desenvolvedor|desenvolvedora|programador|programadora|programacao)(?: (?:de )?(?:software|sistemas|aplicacoes|web))?|engenh(?:aria|eiro|eira) de software|software (?:engineering|engineer|developer)|developer)$/;
  function areasDoCargo(cargo) {
    const especificas = Object.keys(termosDeCargo).filter(area => termosDeCargo[area].test(cargo));
    if (especificas.length) return especificas;
    return desenvolvimento.test(cargo) ? ['backend', 'frontend', 'fullstack'] : [];
  }

  function palavrasDaRegra(texto) {
    return normalizar(texto)
      .replace(/^(?:(?:eu )?nao (?:quero|aceito|busco)|(?:quero |prefiro )?(?:pular|evitar|excluir|ignorar))\s+/g, '')
      .replace(/\bplantoes\b/g, 'plantao')
      .replace(/\b(seniores|juniores)\b/g, palavra => palavra.startsWith('sen') ? 'senior' : 'junior')
      .replace(/\b(remotas?|remotos)\b/g, 'remoto')
      .replace(/\b(presenciais)\b/g, 'presencial')
      .replace(/\b(hibridas?|hibridos)\b/g, 'hibrido')
      .split(/[^a-z0-9+#.]+/)
      .map(palavra => chave(palavra.replace(/\.$/, '')))
      .filter(palavra => palavra && !/^(a|as|o|os|em|de|da|das|do|dos|e|com|que|vaga[s]?|cargo[s]?|empresa[s]?|nivel|regime|trabalho)$/.test(palavra))
      .map(palavra => /^(consultorias|agencias)$/.test(palavra) ? palavra.slice(0, -1) : palavra);
  }

  function criarRegra(texto) {
    const alternativas = texto.split(/[,;]|\bou\b/i).map(palavrasDaRegra).filter(palavras => palavras.length);
    return { texto, corresponde: vaga => alternativas.some(palavras => palavras.every(palavra => vaga.palavras.has(palavra))) };
  }
  for (const vaga of vagas) {
    const descricao = [vaga.titulo, vaga.empresa, vaga.nivel, vaga.modelo, vaga.local, vaga.mercado, ...vaga.requisitos, ...vaga.termos];
    if (vaga.ingles !== 'Sem exigência') descricao.push('inglês ' + vaga.ingles);
    if (vaga.moeda === 'USD' && !vaga.mensal) descricao.push('sem salário divulgado', 'sem faixa salarial');
    vaga.palavras = new Set(palavrasDaRegra(descricao.join(' ')));
  }
  const dolares = valor => 'US$ ' + valor.toLocaleString('pt-BR');

  globalThis.gerarPrevia = function gerarPrevia(perfil = {}) {
    const cargo = normalizar(perfil.cargo);
    const areas = areasDoCargo(cargo);
    const niveis = lista(perfil.niveis).map(chave);
    const modelos = lista(perfil.modelos).map(chave);
    const principal = lista(perfil.principal).map(chave);
    const secundaria = lista(perfil.secundaria).map(chave);
    const mercado = chave(perfil.mercado);
    const ingles = niveisDeIngles.indexOf(chave(perfil.ingles));
    const piso = Number.isFinite(perfil.usd) && perfil.usd > 0 ? perfil.usd : null;
    const regras = lista(perfil.pular).map(criarRegra);
    const regrasDesconhecidas = regras.filter(regra => !vagas.some(regra.corresponde));

    return vagas.map(vaga => {
      const pulos = regras.filter(regra => regra.corresponde(vaga)).map(regra => 'Sua regra de exclusão: “' + regra.texto + '”.');
      const resultado = (grupo, motivos) => ({ titulo: vaga.titulo, empresa: vaga.empresa, grupo, motivos, local: vaga.modelo + ' · ' + vaga.local });
      if (niveis.length && !niveis.includes(chave(vaga.nivel))) pulos.push('Nível ' + vaga.nivel + ' fora dos selecionados.');
      if (modelos.length && !modelos.includes(chave(vaga.modelo))) pulos.push('Modelo ' + vaga.modelo.toLowerCase() + ' fora dos selecionados.');
      if (mercado && mercado !== 'os dois' && mercado !== chave(vaga.mercado)) pulos.push('Mercado ' + vaga.mercado.toLowerCase() + ' fora do foco.');
      if (pulos.length) return resultado('Pulada', pulos);

      const divergencias = [], pendencias = [], afinidades = [];
      if (!cargo) pendencias.push('Informe o cargo desejado para conferir a área.');
      else if (!areas.includes(vaga.area) && cargo !== normalizar(vaga.titulo)) divergencias.push('A função difere do cargo desejado.');
      else afinidades.push('Função alinhada ao cargo desejado.');

      const centrais = vaga.requisitos.filter(r => principal.includes(chave(r)));
      const deApoio = vaga.requisitos.filter(r => !principal.includes(chave(r)) && secundaria.includes(chave(r)));
      const ausentes = vaga.requisitos.filter(r => !centrais.includes(r) && !deApoio.includes(r));
      if (!principal.length && !secundaria.length) pendencias.push('Confira os requisitos: ' + vaga.requisitos.join(' e ') + '.');
      else {
        if (centrais.length) afinidades.push('Na sua lista principal: ' + centrais.join(' e ') + '.');
        if (deApoio.length) pendencias.push('Na sua lista secundária: ' + deApoio.join(' e ') + '.');
        if (ausentes.length) {
          const destino = ausentes.length === vaga.requisitos.length ? divergencias : pendencias;
          destino.push('Fora das suas listas: ' + ausentes.join(' e ') + '.');
        }
      }
      const exigido = niveisDeIngles.indexOf(chave(vaga.ingles));
      if (exigido >= 0 && ingles < 0) pendencias.push('Confira o inglês ' + vaga.ingles.toLowerCase() + ' exigido.');
      else if (exigido > ingles) divergencias.push('Pede inglês ' + vaga.ingles.toLowerCase() + ', acima do nível informado.');
      if (vaga.moeda === 'USD') {
        if (piso && !vaga.mensal) pendencias.push('Valor mensal em USD a confirmar.');
        else if (piso && vaga.mensal < piso) divergencias.push(dolares(vaga.mensal) + '/mês fica abaixo da sua pretensão de ' + dolares(piso) + '.');
        else if (piso) afinidades.push(dolares(vaga.mensal) + '/mês atende à sua pretensão.');
        else if (vaga.mensal) afinidades.push('Oferta de ' + dolares(vaga.mensal) + '/mês.');
      }
      if (!niveis.length) pendencias.push('Confira se o nível ' + vaga.nivel + ' serve para você.');
      if (!modelos.length) pendencias.push('Confira o modelo ' + vaga.modelo.toLowerCase() + '.');
      if (!mercado) pendencias.push('Confira o mercado ' + vaga.mercado.toLowerCase() + '.');
      for (const regra of regrasDesconhecidas) pendencias.push('O catálogo não confirma sua regra “' + regra.texto + '”.');
      if (divergencias.length) return resultado('C', divergencias.concat(pendencias));
      if (pendencias.length) return resultado('B', pendencias.concat(afinidades));
      return resultado('A', afinidades);
    }).sort((a,b) => ordem.indexOf(a.grupo) - ordem.indexOf(b.grupo));
  };
})();
