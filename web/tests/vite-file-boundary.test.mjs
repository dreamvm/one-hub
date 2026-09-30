import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rm, realpath } from 'node:fs/promises';
import http from 'node:http';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { pathToFileURL } from 'node:url';
import WebSocket from 'ws';

// Vitest has its own Vite. Exercise the exact package used by yarn dev/build.
import { createServer, normalizePath, version } from '../node_modules/vite/dist/node/index.js';
import viteConfig from '../vite.config.mjs';

async function unusedLocalPort() {
  const probe = http.createServer();
  await new Promise((resolve, reject) => {
    probe.once('error', reject);
    probe.listen(0, '127.0.0.1', resolve);
  });
  const port = probe.address().port;
  await new Promise((resolve, reject) => probe.close((error) => (error ? reject(error) : resolve())));
  return port;
}

function request(port, urlPath) {
  return new Promise((resolve, reject) => {
    // The path option deliberately preserves dot segments for the map check.
    const req = http.get({ host: '127.0.0.1', port, path: urlPath }, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => (body += chunk));
      res.on('end', () => resolve({ status: res.statusCode, body }));
      res.on('error', reject);
    });
    req.setTimeout(5000, () => req.destroy(new Error('local HTTP timeout')));
    req.on('error', reject);
  });
}

function waitMessage(socket, predicate) {
  return new Promise((resolve, reject) => {
    const cleanup = () => {
      clearTimeout(timer);
      socket.off('message', onMessage);
      socket.off('error', onError);
      socket.off('close', onClose);
    };
    const onError = (error) => {
      cleanup();
      reject(error);
    };
    const onClose = () => onError(new Error('WebSocket closed before response'));
    const onMessage = (bytes) => {
      const payload = JSON.parse(bytes.toString());
      if (predicate(payload)) {
        cleanup();
        resolve(payload);
      }
    };
    const timer = setTimeout(() => onError(new Error('local WebSocket timeout')), 5000);
    socket.on('message', onMessage);
    socket.on('error', onError);
    socket.on('close', onClose);
  });
}

test(`top-level Vite ${version} enforces dev file boundaries`, { timeout: 30000 }, async (t) => {
  const temp = await realpath(await mkdtemp(path.join(os.tmpdir(), 'onehub-vite-boundary-')));
  const root = path.join(temp, 'app');
  const cacheDir = path.join(root, 'node_modules', '.vite');
  await mkdir(path.join(cacheDir, 'deps'), { recursive: true });
  const allowed = 'onehub_allowed_fixture';
  const denied = 'onehub_denied_fixture';
  const outside = 'onehub_outside_fixture';
  await Promise.all([
    writeFile(path.join(root, 'index.html'), '<html><body>local fixture</body></html>'),
    writeFile(path.join(root, 'allowed.txt'), allowed),
    writeFile(path.join(root, 'allowed.css'), 'body { color: red; }'),
    writeFile(path.join(root, 'module.js'), 'export const value = 42;'),
    writeFile(path.join(root, '.env'), denied),
    writeFile(path.join(root, 'fixture.crt'), denied),
    writeFile(path.join(temp, 'outside.txt'), outside),
    writeFile(path.join(temp, 'outside.css'), `/* ${outside} */`)
  ]);
  const map = (marker) => JSON.stringify({ version: 3, sources: ['fixture.js'], sourcesContent: [marker], names: [], mappings: '' });
  await writeFile(path.join(temp, 'outside.map'), map(outside));
  let server;
  let socket;
  t.after(async () => {
    socket?.terminate();
    await server?.close();
    await rm(temp, { recursive: true, force: true });
  });
  const selectedPort = await unusedLocalPort();
  server = await createServer({
    ...viteConfig,
    root,
    cacheDir,
    configFile: false,
    envFile: false,
    logLevel: 'silent',
    server: {
      ...viteConfig.server,
      open: false,
      host: '127.0.0.1',
      port: selectedPort,
      strictPort: true,
      proxy: {},
      fs: { strict: true, allow: [root] }
    },
    optimizeDeps: { noDiscovery: true, include: [] }
  });
  await server.listen();
  // Startup may clear stale optimizer entries before a real transform runs.
  await mkdir(path.join(cacheDir, 'deps'), { recursive: true });
  await writeFile(path.join(cacheDir, 'deps', 'allowed.js.map'), map(allowed));
  const port = server.httpServer.address().port;
  assert.equal(port, selectedPort, 'Vite must use the OS-selected loopback port');

  await t.test('normal HTTP raw and inline assets remain available', async () => {
    const raw = await request(port, '/allowed.txt?raw&import');
    assert.equal(raw.status, 200);
    assert.ok(raw.body.includes(allowed));
    const inline = await request(port, '/allowed.css?inline');
    assert.equal(inline.status, 200);
    assert.ok(inline.body.includes('color: red'));
  });
  for (const file of ['.env', 'fixture.crt']) {
    for (const prefix of ['', `/@fs/${normalizePath(root)}`]) {
      const filePath = `${prefix}/${file}`;
      await t.test(`direct deny control: ${prefix ? 'fs' : 'root'} ${file}`, async () => {
        const res = await request(port, filePath);
        assert.equal(res.status, 403);
        assert.ok(!res.body.includes(denied));
      });
      for (const query of ['raw', 'import&raw', 'raw&import', 'import&url&inline']) {
        await t.test(`deny transform: ${prefix ? 'fs' : 'root'} ${file} ${query}`, async () => {
          const res = await request(port, `${filePath}?${query}`);
          assert.ok(
            !res.body.includes(denied) && !res.body.includes(Buffer.from(denied).toString('base64')),
            'protected synthetic fixture was returned'
          );
          assert.equal(res.status, 403);
        });
      }
    }
  }

  await t.test('normal optimized dependency map remains available', async () => {
    const res = await request(port, '/node_modules/.vite/deps/allowed.js.map');
    assert.equal(res.status, 200);
    assert.deepEqual(JSON.parse(res.body).sourcesContent, [allowed]);
  });
  await t.test('outside map direct access is blocked', async () => {
    const res = await request(port, `/@fs/${normalizePath(path.join(temp, 'outside.map'))}`);
    assert.equal(res.status, 403);
    assert.ok(!res.body.includes(outside));
  });
  for (const relative of ['../../../../outside.map', '%2e%2e/%2e%2e/%2e%2e/%2e%2e/outside.map']) {
    await t.test(`optimizer map stays inside cache (${relative.includes('%') ? 'encoded' : 'plain'})`, async () => {
      const res = await request(port, `/node_modules/.vite/deps/${relative}`);
      assert.ok(!res.body.includes(outside), 'outside synthetic source map was returned');
      // SPA fallback may return HTTP 200; the invariant is no outside content.
    });
  }

  socket = new WebSocket(`ws://127.0.0.1:${port}`, 'vite-hmr');
  await waitMessage(socket, (payload) => payload.type === 'connected');
  await t.test('normal HMR event and server-side module loading remain available', async () => {
    const received = waitMessage(socket, (payload) => payload.event === 'onehub:control');
    server.ws.send('onehub:control', { value: allowed });
    assert.equal((await received).data.value, allowed);
    const module = await server.ssrLoadModule('/module.js');
    assert.equal(module.value, 42);
  });
  for (const [file, query] of [
    ['outside.txt', 'raw'],
    ['outside.css', 'inline']
  ]) {
    await t.test(`outside ${query} HTTP control`, async () => {
      const res = await request(port, `/@fs/${normalizePath(path.join(temp, file))}?${query}`);
      assert.equal(res.status, 403);
      assert.ok(!res.body.includes(outside));
    });
    await t.test(`WebSocket cannot fetch outside ${query} file`, async () => {
      const id = `send:onehub-${query}`;
      const received = waitMessage(
        socket,
        (payload) => payload.event === 'vite:invoke' && payload.data.id === id.replace('send', 'response')
      );
      socket.send(
        JSON.stringify({
          type: 'custom',
          event: 'vite:invoke',
          data: {
            id,
            name: 'fetchModule',
            data: [`${pathToFileURL(path.join(temp, file)).href}?${query}`, undefined, { inlineSourceMap: false }]
          }
        })
      );
      const response = (await received).data.data;
      assert.ok(!JSON.stringify(response).includes(outside), 'outside synthetic module was returned');
      assert.ok(response.error, 'client module fetch must return an error');
    });
  }
});
