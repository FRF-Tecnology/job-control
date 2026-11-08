// Busca no LinkedIn e envia as vagas novas para a Central (etapa Descoberta).
// Rodar com javascript_tool numa aba logada de linkedin.com, — gere com: node ferramentas/cli.mjs script busca-linkedin [--dias 7]
// Roda em segundo plano: consulte window.__CENTRAL_RESULTADO. Resultado completo na tela Triagem.
window.__CENTRAL_RESULTADO = 'rodando';
(async () => {
  // Resposta HTTP com erro vira mensagem clara (ex.: 403 = sessão expirada ou chave trocada).
  const lerJson = async (r, onde) => { if (!r.ok) throw new Error(onde + ' respondeu HTTP ' + r.status + (r.status === 401 || r.status === 403 ? ' (faça login de novo ou gere o script outra vez)' : '')); return r.json(); };
  const CENTRAL = '{{central}}', CHAVE = '{{chave}}', PALAVRAS = {{palavras}}, DIAS = Number('{{dias}}') || 7, GEO = '{{geo}}' || '106057199', TFORA = '{{tituloFora}}', TDEV = '{{tituloDev}}', ACEITA_SENIOR = '{{aceitaSenior}}' === '1', INGLES_OK = '{{inglesOk}}' === '1';
  // Palavras com operadores booleanos: o restli exige ( ) , : ' escapados.
  const kwEnc = k => encodeURIComponent(k).replace(/[()',:!*]/g, c => '%' + c.charCodeAt(0).toString(16).toUpperCase());
  const csrf = document.cookie.match(/JSESSIONID="?([^";]+)/)?.[1];
  if (!csrf) return 'erro: não achei a sessão do LinkedIn. Abra linkedin.com logado e rode de novo.';
  const H = { 'csrf-token': csrf, 'x-restli-protocol-version': '2.0.0' };
  const cartoes = new Map();
  for (const kw of PALAVRAS) {
    for (let start = 0; start < 200; start += 100) {
      const u = '/voyager/api/voyagerJobsDashJobCards?decorationId=com.linkedin.voyager.dash.deco.jobs.search.JobSearchCardsCollection-220&count=100&q=jobSearch&query=(origin:JOB_SEARCH_PAGE_JOB_FILTER,keywords:' + kwEnc(kw) + ',locationUnion:(geoId:' + GEO + '),selectedFilters:(workplaceType:List(2),timePostedRange:List(r' + DIAS * 86400 + ')),spellCorrectionEnabled:true)&start=' + start;
      const j = await lerJson(await fetch(u, { headers: H }), 'LinkedIn (busca)');
      for (const el of j.elements ?? []) {
        const c = el.jobCardUnion?.jobPostingCard;
        if (!c) continue;
        const id = c.entityUrn.match(/\((\d+)/)?.[1];
        if (id && !cartoes.has(id)) cartoes.set(id, { titulo: c.title?.text ?? '', empresa: c.primaryDescription?.text ?? '' });
      }
      if ((j.paging?.total ?? 0) <= start + 100) break;
    }
  }
  // Filtro por título: fora do perfil (sênior, outras stacks, não-dev).
  const FORA = TFORA ? new RegExp(TFORA, 'i') : /s[eê]nior|\bsr\b|staff|principal|lead|l[ií]der|manager|gerente|coordena|head|architect|arquitet|especialista|specialist|expert|\.net|c#|c\+\+|python|php|ruby|golang|\bgo\b|data |dados|sap\b|salesforce|sfdc|qa\b|quality|test|devops|sre|mobile|ios|android|flutter|react native|design|ux|suporte|support|vendas|sales|recruit|intern|est[aá]gi|trainee|cobol|abap|delphi|rpa|power ?(bi|app)|scientist|machine|\bai\b|\bia\b|llm|marketing|tutor|annotat|freelance|mainframe|mulesoft|servicenow|pega|wordpress|shopify|game|security|seguran|dynamics|outsystems|oracle|totvs|protheus|rust|scala|elixir|blockchain|pcd|afirmativa/i;
  const DEV = TDEV ? new RegExp(TDEV, 'i') : /java|spring|back|full|stack|desenvolv|developer|engineer|engenheir|programador|react|node|kotlin|typescript|angular|software|web|quarkus/i;
  const candidatas = [...cartoes].filter(([, c]) => DEV.test(c.titulo) && !FORA.test(c.titulo));
  const vagas = [];
  for (const [id, c] of candidatas) {
    const r = await lerJson(await fetch('/voyager/api/jobs/jobPostings/' + id + '?decorationId=com.linkedin.voyager.deco.jobs.web.shared.WebFullJobPosting-65', { headers: { 'csrf-token': csrf } }), 'LinkedIn (vaga ' + id + ')');
    if (r.applyingInfo?.applied || r.jobState !== 'LISTED') continue;
    const d = r.description?.text ?? '';
    const anos = Math.max(0, ...(d.match(/(\d+)\s*\+?\s*(anos|years)/gi) ?? []).map(m => parseInt(m)).filter(n => n < 15));
    const inglesAv = /(ingl[eê]s|english)[^.\n]{0,60}(avan[cç]ad|fluent|fluente|advanced|c1|b2|upper)|(avan[cç]ad|fluent|fluente|advanced|c1|b2)[^.\n]{0,30}(ingl[eê]s|english)/i.test(d);
    const emIngles = /\b(the|and|with|you)\b/.test(d.slice(0, 600));
    const presencial = /presencial|h[ií]brid|hybrid|on-site|onsite/i.test(d);
    const diploma = /(gradua|superior|forma[çd])[^.\n]{0,50}(complet|conclu)/i.test(d);
    const senior = /s[eê]nior|senior/i.test(d.slice(0, 800));
    const stack = ['Java', 'Kotlin', 'Spring', 'Kafka', 'AWS', 'React', 'Node', 'Angular', 'TypeScript', '.NET', 'Python', 'LLM'].filter(s => new RegExp(s.replace('.', '\\.'), 'i').test(d));
    const alertas = [anos >= (ACEITA_SENIOR ? 9 : 5) && `pede ${anos}+ anos`, !INGLES_OK && inglesAv && 'inglês avançado', presencial && 'menciona presencial/híbrido', !ACEITA_SENIOR && diploma && 'pode exigir graduação concluída', !ACEITA_SENIOR && senior && 'texto fala em sênior', ACEITA_SENIOR && !/java|kotlin|spring|jvm/i.test(d) && 'sem Java/Kotlin na descrição', stack.includes('.NET') && 'tem .NET', stack.includes('Python') && 'tem Python'].filter(Boolean);
    const aderencia = (emIngles && !INGLES_OK) ? 'C' : alertas.length ? 'B' : 'A';
    vagas.push({
      id: 'li-' + id, empresa: c.empresa || r.companyDetails?.['com.linkedin.voyager.deco.jobs.web.shared.WebCompactJobPostingCompany']?.companyResolutionResult?.name || '?',
      titulo: c.titulo, plataforma: 'linkedin', link: 'https://www.linkedin.com/jobs/view/' + id + '/',
      aderencia, idioma: emIngles ? 'en' : 'pt', stack,
      motivoClaude: [r.applyMethod?.['com.linkedin.voyager.jobs.ComplexOnsiteApply'] || r.applyMethod?.['com.linkedin.voyager.jobs.SimpleOnsiteApply'] ? 'Candidatura simplificada' : 'Site da empresa', anos ? `${anos} anos pedidos` : '', ...alertas].filter(Boolean).join('; '),
    });
  }
  window.__CENTRAL_VAGAS = vagas;
  window.__CENTRAL_PARTE = i => encodeURIComponent(JSON.stringify(vagas.slice(i, i + 1)));
  const ctl = new AbortController();
  const limite = setTimeout(() => ctl.abort(), 6000);
  let res;
  try {
    const resp = await fetch(CENTRAL + '/api/v1/importar', { method: 'POST', signal: ctl.signal, headers: { 'content-type': 'application/json', 'x-central-chave': CHAVE }, body: JSON.stringify(vagas) });
    res = await resp.json().catch(() => ({}));
    if (!resp.ok) return 'erro: a Central recusou a importação (HTTP ' + resp.status + ': ' + (res.erro || '?') + '). Gere o script de novo com cli.mjs script busca-linkedin.';
  } catch {
    return `PENDENTE: ${vagas.length} vagas guardadas em window.__CENTRAL_VAGAS (a Central não respondeu — o Chrome pode estar pedindo permissão de rede local). Leia window.__CENTRAL_PARTE(0), (1)… e grave cada parte com cli.mjs add --uri "<parte>".`;
  } finally { clearTimeout(limite); }
  return `cartões ${cartoes.size}, candidatas ${candidatas.length}, enviadas à Central ${vagas.length}: novas ${res.adicionadas.length}, já existiam ${res.duplicadas.length}, erros ${res.erros.length}`;
})().then(r => { window.__CENTRAL_RESULTADO = r; }, e => { window.__CENTRAL_RESULTADO = 'erro: ' + e.message; });
'iniciado — consulte window.__CENTRAL_RESULTADO em alguns segundos';
