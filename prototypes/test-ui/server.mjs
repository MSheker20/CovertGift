// Temporary, dependency-free development server. No production hosting.
import http from 'node:http';
import https from 'node:https';
import { readFile } from 'node:fs/promises';

const port = Number(process.env.UI_PORT || 5173);
const host = process.env.UI_HOST || '127.0.0.1';
const backend = new URL(process.env.API_TARGET || 'http://127.0.0.1:8080');
if (!['http:', 'https:'].includes(backend.protocol)) throw new Error('API_TARGET must use http(s)');
const files = new Map([
  ['/', ['index.html', 'text/html; charset=utf-8']],
  ['/app.js', ['app.js', 'text/javascript; charset=utf-8']],
  ['/style.css', ['style.css', 'text/css; charset=utf-8']],
]);
// Only the test panel's application endpoints are forwarded. In particular,
// never expose the unauthenticated /api/v1/dev/reset-db through a public tunnel.
const apiRoute = /^\/api\/v1\/(?:auth\/(?:register|login)|me|groups(?:\/[0-9a-f-]+(?:\/(?:members|interests|budget|ideas(?:\/(?:preview-link|generate-ai))?))?)?|ideas\/[0-9a-f-]+(?:\/(?:vote|reject))?)$/i;
const jsonError = (res, status, message) => {
  if (res.headersSent) return res.destroy();
  res.writeHead(status, { 'Content-Type': 'application/json; charset=utf-8' });
  res.end(JSON.stringify({ success: false, error: message }));
};

http.createServer(async (req, res) => {
  // Deliberately allowlisted static files: never serve .env or repository files.
  const path = (req.url || '/').split('?')[0];
  res.setHeader('Cache-Control', 'no-store');
  res.setHeader('X-Content-Type-Options', 'nosniff');
  if (path.startsWith('/api/v1/') && !apiRoute.test(path)) {
    return jsonError(res, 403, 'Bu uç nokta geçici test proxy’sinde erişime kapalıdır.');
  }
  if (apiRoute.test(path) || path === '/health' || path === '/swagger' || path.startsWith('/swagger/') || path === '/docs') {
    const headers = { ...req.headers, host: backend.host };
    delete headers.connection;
    const transport = backend.protocol === 'https:' ? https : http;
    const upstream = transport.request({
      hostname: backend.hostname, port: backend.port || undefined,
      protocol: backend.protocol, method: req.method, path: req.url, headers,
    }, (response) => {
      const outgoing = { ...response.headers, 'cache-control': 'no-store' };
      delete outgoing.connection;
      res.writeHead(response.statusCode || 502, outgoing);
      response.pipe(res);
    });
    upstream.setTimeout(180_000, () => upstream.destroy(new Error('timeout')));
    upstream.on('error', () => jsonError(res, 502, 'Backend erişilemiyor veya yanıt süresi doldu. API sunucusunu ve veritabanını kontrol edin.'));
    req.on('aborted', () => upstream.destroy());
    res.on('close', () => { if (!res.writableEnded) upstream.destroy(); });
    req.pipe(upstream);
    return;
  }
  const file = files.get(path);
  if (!file || !['GET', 'HEAD'].includes(req.method)) {
    return jsonError(res, 404, 'Dosya bulunamadı');
  }
  try {
    const content = await readFile(new URL(file[0], import.meta.url));
    res.writeHead(200, { 'Content-Type': file[1] });
    res.end(req.method === 'HEAD' ? undefined : content);
  } catch {
    jsonError(res, 500, 'Arayüz dosyası okunamadı');
  }
}).listen(port, host, () => {
  console.log(`CovertGift geçici test paneli: http://${host}:${port}`);
  console.log(`API hedefi: ${backend.origin}`);
}).on('error', (error) => {
  console.error(`Test paneli başlatılamadı: ${error.message}`);
  process.exitCode = 1;
});
