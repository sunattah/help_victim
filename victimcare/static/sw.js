/* Victim Care service worker.
 *
 * It caches the static shell (styles, scripts, artwork, icons and the first
 * aid guides) so the app opens fast and keeps working when the connection is
 * weak. Nothing that belongs to a signed in person is cached.
 */

const CACHE = 'victimcare-v1';

const PRECACHE = [
  '/static/css/app.css',
  '/static/js/app.js',
  '/static/manifest.webmanifest',
  '/static/img/favicon.svg',
  '/static/img/icon-192.png',
  '/static/img/icon-512.png',
  '/static/img/slides/care.svg',
  '/static/img/slides/firstaid.svg',
  '/static/img/slides/support.svg',
  '/static/img/slides/response.svg',
  '/static/img/slides/hope.svg',
  '/api/v1/guides'
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(PRECACHE)).then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(
      keys.filter((key) => key !== CACHE).map((key) => caches.delete(key))
    )).then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (event) => {
  const request = event.request;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;

  // Static files are served from the cache first.
  if (url.pathname.startsWith('/static/') || url.pathname === '/api/v1/guides') {
    event.respondWith(
      caches.match(request).then((cached) => cached || fetch(request).then((response) => {
        const copy = response.clone();
        caches.open(CACHE).then((cache) => cache.put(request, copy));
        return response;
      }).catch(() => cached))
    );
    return;
  }

  // Pages and personal data always come from the network.
  event.respondWith(fetch(request).catch(() => caches.match('/static/offline.html')));
});
