const http = require('http');

const PORT = process.env.PORT || 3000;

const server = http.createServer((req, res) => {
  if (req.url === '/healthz') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', service: 'cms-placeholder' }));
    return;
  }

  res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
  res.end(`
    <!DOCTYPE html>
    <html lang="en">
    <head>
      <meta charset="utf-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1" />
      <title>LiveLaw CMS (Placeholder)</title>
      <style>
        body { font-family: system-ui, -apple-system, sans-serif; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; background: #0f172a; color: #f8fafc; }
        .card { background: #1e293b; padding: 2.5rem; border-radius: 12px; box-shadow: 0 10px 25px -5px rgba(0,0,0,0.5); max-width: 540px; text-align: center; border: 1px solid #334155; }
        h1 { margin-top: 0; font-size: 1.75rem; color: #38bdf8; }
        p { color: #94a3b8; line-height: 1.6; }
        .badge { display: inline-block; padding: 0.4rem 1rem; background: #0369a1; color: #e0f2fe; border-radius: 9999px; font-weight: 600; font-size: 0.875rem; margin-top: 1rem; }
      </style>
    </head>
    <body>
      <div class="card">
        <h1>LiveLaw CMS</h1>
        <p>Phase 1 Placeholder Container is running.</p>
        <p>Next.js App Router, TipTap editor, and generated OpenAPI TypeScript client will be wired in Phase 7 &amp; 8.</p>
        <div class="badge">CMS Container Active: Port 3000</div>
      </div>
    </body>
    </html>
  `);
});

server.listen(PORT, '0.0.0.0', () => {
  console.log(`CMS placeholder running on port ${PORT}`);
});
