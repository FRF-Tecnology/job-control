(function art(){
const paths={
radar:'<circle cx="12" cy="12" r="8.7"/><circle cx="12" cy="12" r="4.8"/><path d="m12 12 7-7"/><circle cx="12" cy="12" r=".8"/>',
list:'<rect x="4" y="4" width="16" height="16" rx="3"/><path d="m7 10 1 1 2-2m-3 6 1 1 2-2m3-4h4m-4 5h4"/>',
board:'<rect x="3" y="4.5" width="18" height="15" rx="3"/><path d="M9 5v14m6-14v14"/>',
profile:'<circle cx="12" cy="8" r="3.4"/><path d="M5 20a7 7 0 0 1 14 0"/>',
sliders:'<path d="M4 7h8m4 0h4M4 17h3m4 0h9"/><circle cx="14" cy="7" r="2"/><circle cx="9" cy="17" r="2"/>',
area:'<rect x="3" y="4" width="18" height="16" rx="3"/><path d="M9 4v16"/>',
search:'<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>',
pin:'<path d="M19 10c0 5-7 11-7 11S5 15 5 10a7 7 0 1 1 14 0Z"/><circle cx="12" cy="10" r="2.2"/>',
briefcase:'<rect x="3" y="7" width="18" height="13" rx="2.5"/><path d="M8 7V5a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2M3 12c6 3 12 3 18 0"/>',
wallet:'<rect x="3" y="6" width="18" height="13" rx="2"/><path d="M3 8V6l13-3v3m5 5h-6v4h6"/>',
sparkles:'<path d="m13 3 1.8 5.2L20 10l-5.2 1.8L13 17l-1.8-5.2L6 10l5.2-1.8ZM5 15l.9 2.1L8 18l-2.1.9L5 21l-.9-2.1L2 18l2.1-.9Z"/>',
plus:'<path d="M12 5v14M5 12h14"/>',check:'<path d="m4.5 12 5 5 10-11"/>',
'chevron-right':'<path d="m9 5 7 7-7 7"/>','chevron-left':'<path d="m15 5-7 7 7 7"/>','chevron-down':'<path d="m6 9 6 6 6-6"/>',
'arrow-right':'<path d="M4 12h16m-6-6 6 6-6 6"/>','arrow-left':'<path d="M20 12H4m6-6-6 6 6 6"/>',
refresh:'<path d="M20 8a8 8 0 1 0 .2 8M20 3v5h-5"/>',file:'<path d="M6 3h8l5 5v13H6zM14 3v6h5M9 13h7m-7 4h6"/>',
send:'<path d="m3 10 18-7-6 18-4-7-8-4Zm8 4L21 3"/>',
clock:'<circle cx="12" cy="12" r="9"/><path d="M12 7v6l4 2"/>',edit:'<path d="m4 20 5-1 11-11-4-4L5 15l-1 5Zm10-14 4 4"/>',
compass:'<circle cx="12" cy="12" r="9"/><path d="m16 8-2.5 5.5L8 16l2.5-5.5L16 8Z"/>',
flask:'<path d="M9 3h6m-5 0v6l-5.5 9a2 2 0 0 0 2 3h11a2 2 0 0 0 2-3L14 9V3M7.5 14h9"/>',
close:'<path d="m6 6 12 12M18 6 6 18"/>',menu:'<path d="M4 6h16M4 12h16M4 18h16"/>',shield:'<path d="m12 2 8 3v6c0 6-8 10-8 10S4 17 4 11V5l8-3Z"/><path d="m8.5 11 2.5 2.5 4.5-5"/>',
monitor:'<rect x="2.5" y="4" width="19" height="13" rx="2"/><path d="M12 17v4m-5 0h10"/>',globe:'<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18"/>',help:'<circle cx="12" cy="12" r="9"/><path d="M9 9a3 3 0 1 1 4.5 2.6c-1.2.8-1.5 1-1.5 2.4m0 2.8h.01"/>',undo:'<path d="M8 4 3 9l5 5M3 9h11a6 6 0 0 1 0 12"/>',star:'<path d="m12 2 2.8 6.2 6.7.9-5 4.8 1.2 6.7L12 17.3l-5.7 3.3 1.2-6.7-5-4.8 6.7-.9Z"/>',login:'<path d="M14 4h5v16h-5M3 12h12m-4-5 5 5-5 5"/>'};
const aliases={'map-pin':'pin',location:'pin',filter:'sliders',user:'profile',settings:'sliders',arrow:'arrow-right',back:'arrow-left',x:'close',document:'file',building:'briefcase',loader:'refresh',layers:'board',info:'help',sparkle:'sparkles',redo:'refresh',target:'radar',chevron:'chevron-right'};
window.uiIcon=(key,size=20)=>`<svg aria-hidden="true" focusable="false" class="ui-icon" width="${Number(size)||20}" height="${Number(size)||20}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round">${paths[key]||paths[aliases[key]]||paths.compass}</svg>`;
const marks={
forma:`<g fill="currentColor"><path d="M8 4h8v8H8zM16 12h8v8h-8z" transform="rotate(-22 16 16)"/><path d="M8 12h8v8H8zM16 20h8v8h-8z" transform="rotate(-22 16 16)"/></g>`,
orbe:'<g fill="none" stroke="currentColor" stroke-width="3"><ellipse cx="16" cy="16" rx="11" ry="6" transform="rotate(-35 16 16)"/><ellipse cx="16" cy="16" rx="11" ry="6" transform="rotate(75 16 16)"/></g>',
nimbo:'<g fill="none" stroke="currentColor" stroke-width="3.4" stroke-linecap="round"><path d="M5 18q4-12 10-5t12-1M5 23q5-10 11-5t11-2"/></g>',
vertice:'<path fill="currentColor" d="m3 23 9-17 5 8-5 9Zm13 0 6-12 8 12Z"/>',
luma:'<g fill="none" stroke="currentColor" stroke-width="3.5"><path d="M16 3v26M3 16h26M7 7l18 18M7 25 25 7"/></g>',
onda:'<g fill="none" stroke="currentColor" stroke-width="3"><path d="M2 11c8-10 19 10 28 0M2 17c8-10 19 10 28 0M2 23c8-10 19 10 28 0"/></g>',
junco:'<path fill="currentColor" d="M6 24 15 5l3 10 7-5-6 14Z"/>',atlas:'<path fill="currentColor" d="m16 3 14 25h-6l-8-15-8 15H2Z"/>',prisma:'<path fill="currentColor" d="m16 2 13 21-13 6L3 23Zm0 6L9 21l7 3 7-3Z"/>'};
function companyInitials(name) {
  const words = String(name || '').normalize('NFC').match(/[\p{L}\p{N}][\p{L}\p{M}\p{N}]*/gu) || [];
  if (!words.length) return '·';
  const first = [...words[0]][0];
  const last = words.length > 1 ? [...words[words.length - 1]][0] : '';
  return (first + last).toLocaleUpperCase('pt-BR');
}

function companyLogoSource(value) {
  if (typeof value !== 'string' || !value.trim()) return '';
  // The demo bundles Hugging Face's SVG from https://huggingface.co/brand.
  const embedded = window.designAssets?.companyLogos;
  if (embedded && Object.hasOwn(embedded, value)) return embedded[value];
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && !url.username && !url.password ? url.href : '';
  } catch {
    return '';
  }
}

window.jobMark = (job, size = 40) => {
  const pixels = Number.isFinite(size) && size > 0 ? size : 40;
  const logo = job.companyLogo || job.companyLogoUrl;
  const src = companyLogoSource(logo);
  const demoBrand = !logo && Object.hasOwn(marks, job.brand) ? job.brand : '';
  const kind = demoBrand ? 'mk-' + demoBrand : 'mk-company';
  let content;
  if (demoBrand) {
    content = `<svg viewBox="0 0 32 32" width="62%" height="62%">${marks[demoBrand]}</svg>`;
  } else {
    content = `<span class="company-monogram">${companyInitials(job.company)}</span>`;
    if (src) {
      const safeSrc = src.replaceAll('&', '&amp;').replaceAll('"', '&quot;').replaceAll('<', '&lt;');
      content = `<img class="company-logo" alt="" referrerpolicy="no-referrer" src="${safeSrc}">${content}`;
    }
  }
  return `<span aria-hidden="true" class="company-mark ${kind}" style="width:${pixels}px;height:${pixels}px;font-size:${Math.round(pixels * .38)}px">${content}</span>`;
};

// Image events do not bubble. Capture them once for all views and re-renders.
document.addEventListener('load', event => {
  if (event.target.matches?.('img.company-logo')) event.target.classList.add('loaded');
}, true);
document.addEventListener('error', event => {
  if (event.target.matches?.('img.company-logo')) event.target.remove();
}, true);
})();
