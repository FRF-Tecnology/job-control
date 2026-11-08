// Varre as empresas da InHire e envia as vagas remotas novas para a Central.
// Rodar numa aba de qualquer *.inhire.app (CORS) — gere com: node ferramentas/cli.mjs script busca-inhire [--dias 7]
window.__CENTRAL_RESULTADO = 'rodando';
(async () => {
  // Resposta HTTP com erro vira mensagem clara (ex.: 403 = sessão expirada ou chave trocada).
  const lerJson = async (r, onde) => { if (!r.ok) throw new Error(onde + ' respondeu HTTP ' + r.status + (r.status === 401 || r.status === 403 ? ' (faça login de novo ou gere o script outra vez)' : '')); return r.json(); };
  const CENTRAL = '{{central}}', CHAVE = '{{chave}}', DESDE = '{{desde}}', EMPRESAS = {{empresas}};
  const DEV = /desenvolv|developer|engineer|engenheir|programador|back|front|full|stack|java|node|react|software/i;
  const FORA = /s[eê]nior|\bsr\b|lead|especialista|staff|\.net|python|php|dados|data|gis|qa\b|test|sap\b|salesforce|devops|sre|ios|android|mobile|\bia\b|\bai\b|golang|c#|ruby|arquitet|architect|coorden|gerente|design/i;
  const achadas = [];
  for (const s of EMPRESAS) {
    try {
      const r = await fetch('https://api.inhire.app/job-posts/public/pages', { headers: { 'X-Tenant': s } }).then(x => lerJson(x, 'InHire'));
      for (const j of r.jobsPage ?? []) if (j.status === 'published' && DEV.test(j.displayName) && !FORA.test(j.displayName) && !/On-site|Hybrid/i.test(j.workplaceType)) achadas.push({ s, ...j });
    } catch {}
  }
  const vagas = [];
  let i = 0;
  const trabalhar = async () => {
    while (i < achadas.length) {
      const j = achadas[i++];
      try {
        const d = await fetch('https://api.inhire.app/job-posts/public/pages/' + j.jobId, { headers: { 'X-Tenant': j.s } }).then(x => lerJson(x, 'InHire'));
        const quando = d.lastPublishedAt || d.publishedAt || d.createdAt || '';
        if (quando.slice(0, 10) < DESDE) continue;
        vagas.push({
          id: 'inhire-' + j.s + '-' + j.jobId.slice(0, 8), empresa: d.tenantName || j.s, titulo: j.displayName.trim(), plataforma: 'inhire',
          link: 'https://' + j.s + '.inhire.app/vagas/' + j.jobId + '/vaga', contrato: Array.isArray(d.contractType) ? d.contractType.join('/') : d.contractType,
          aderencia: 'A', idioma: 'pt', motivoClaude: `InHire (${j.s}), publicada ${quando.slice(0, 10)}. Alguns formulários pedem CPF.`,
        });
      } catch {}
    }
  };
  await Promise.all(Array.from({ length: 8 }, trabalhar));
  window.__CENTRAL_VAGAS = vagas;
  window.__CENTRAL_PARTE = i => encodeURIComponent(JSON.stringify(vagas.slice(i, i + 1)));
  const ctl = new AbortController();
  const limite = setTimeout(() => ctl.abort(), 6000);
  let res;
  try {
    const resp = await fetch(CENTRAL + '/api/v1/importar', { method: 'POST', signal: ctl.signal, headers: { 'content-type': 'application/json', 'x-central-chave': CHAVE }, body: JSON.stringify(vagas) });
    res = await resp.json().catch(() => ({}));
    if (!resp.ok) return 'erro: a Central recusou a importação (HTTP ' + resp.status + ': ' + (res.erro || '?') + '). Gere o script de novo com cli.mjs script busca-inhire.';
  } catch {
    return `PENDENTE: ${vagas.length} vagas guardadas em window.__CENTRAL_VAGAS (a Central não respondeu — o Chrome pode estar pedindo permissão de rede local). Leia window.__CENTRAL_PARTE(0), (1)… e grave cada parte com cli.mjs add --uri "<parte>".`;
  } finally { clearTimeout(limite); }
  return `empresas ${EMPRESAS.length}, vagas dev remotas ${achadas.length}, publicadas desde ${DESDE}: ${vagas.length}; novas ${res.adicionadas.length}, já existiam ${res.duplicadas.length}, erros ${res.erros.length}`;
})().then(r => { window.__CENTRAL_RESULTADO = r; }, e => { window.__CENTRAL_RESULTADO = 'erro: ' + e.message; });
'iniciado — consulte window.__CENTRAL_RESULTADO em alguns segundos';
