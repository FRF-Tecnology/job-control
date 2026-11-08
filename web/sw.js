// Service worker da Central (PWA). Rede primeiro: com a Central aberta, sempre a versão nova.
// Sem rede, abre a última interface guardada (os dados continuam vindo só da API, nunca do cache).
const CACHE = 'central-v1';

self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', ev => ev.waitUntil(
  caches.keys().then(nomes => Promise.all(nomes.filter(n => n !== CACHE).map(n => caches.delete(n)))).then(() => self.clients.claim()),
));

self.addEventListener('fetch', ev => {
  const url = new URL(ev.request.url);
  if (ev.request.method !== 'GET' || url.origin !== location.origin || url.pathname.startsWith('/api/')) return;
  ev.respondWith(
    fetch(ev.request)
      .then(r => { if (r.ok) { const copia = r.clone(); caches.open(CACHE).then(c => c.put(ev.request, copia)); } return r; })
      .catch(() => caches.match(ev.request).then(r => r ?? caches.match('/'))),
  );
});
