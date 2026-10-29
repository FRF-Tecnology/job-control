// Busca vagas remotas na Gupy e envia as novas para a Central.
// Rodar numa aba de https://portal.gupy.io/ — gere com: node ferramentas/cli.mjs script busca-gupy [--dias 7]
window.__CENTRAL_RESULTADO = 'rodando';
(async () => {
  const CENTRAL = '{{central}}', CHAVE = '{{chave}}', PALAVRAS = {{palavras}}, DESDE = '{{desde}}';
  const vistas = new Map();
  for (const k of PALAVRAS) {
    for (let off = 0; off < 500; off += 100) {
      let r;
      for (let t = 0; t < 3; t++) {
        r = await fetch('/api/job-search/jobs?jobName=' + encodeURIComponent(k) + '&workplaceType=remote&limit=100&offset=' + off).then(x => x.json()).catch(() => null);
        if (r) break;
        await new Promise(s => setTimeout(s, 1500));
      }
      const d = r?.data ?? [];
      for (const j of d) if (j.publishedDate >= DESDE) vistas.set(j.id, j);
      if (d.length < 100) break;
    }
  }
  const FORA = /s[eê]nior|\bsr\b|especialista|staff|lead|l[ií]der|\.net|c#|python|php|ruby|qa\b|teste|dados|data|sap\b|estágio|est[aá]gi|trainee|pcd|afirmativa|banco de talentos/i;
  const DEV = /desenvolv|developer|engenheir|engineer|programador|back|front|full|stack|java|node|react|software/i;
  const vagas = [...vistas.values()].filter(j => DEV.test(j.name) && !FORA.test(j.name)).map(j => ({
    id: 'gupy-' + j.id, empresa: j.careerPageName, titulo: j.name.trim(), plataforma: 'gupy',
    link: String(j.jobUrl).split('?')[0], aderencia: /bil[ií]ngue|ingl[eê]s/i.test(j.name) ? 'B' : 'A', idioma: 'pt',
    motivoClaude: `Gupy, publicada ${String(j.publishedDate).slice(0, 10)}. Confira se pede RG ou graduação concluída antes de aplicar.`,
  }));
  window.__CENTRAL_VAGAS = vagas;
  window.__CENTRAL_PARTE = i => encodeURIComponent(JSON.stringify(vagas.slice(i, i + 1)));
  const ctl = new AbortController();
  const limite = setTimeout(() => ctl.abort(), 6000);
  let res;
  try {
    res = await (await fetch(CENTRAL + '/api/importar', { method: 'POST', signal: ctl.signal, headers: { 'content-type': 'application/json', 'x-central-chave': CHAVE }, body: JSON.stringify(vagas) })).json();
  } catch {
    return `PENDENTE: ${vagas.length} vagas guardadas em window.__CENTRAL_VAGAS (a Central não respondeu — o Chrome pode estar pedindo permissão de rede local). Leia window.__CENTRAL_PARTE(0), (1)… e grave cada parte com cli.mjs add --uri "<parte>".`;
  } finally { clearTimeout(limite); }
  return `vagas recentes ${vistas.size}, enviadas à Central ${vagas.length}: novas ${res.adicionadas.length}, já existiam ${res.duplicadas.length}, erros ${res.erros.length}`;
})().then(r => { window.__CENTRAL_RESULTADO = r; }, e => { window.__CENTRAL_RESULTADO = 'erro: ' + e.message; });
'iniciado — consulte window.__CENTRAL_RESULTADO em alguns segundos';
