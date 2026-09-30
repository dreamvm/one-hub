import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import http from 'node:http';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { JSDOM } from 'jsdom';

// Real browser distribution and jsdom XMLHttpRequest against synthetic loopback
// endpoints. This is transport coverage, not a native browser or UI end-to-end.
const webRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const axiosRoot = path.resolve(process.env.ONEHUB_AXIOS_ROOT || path.join(webRoot, 'node_modules/axios'));

test('browser Axios XMLHttpRequest compatibility', async (t) => {
  const requests = [];
  const handler = async (req, res) => {
    let body = '';
    for await (const chunk of req) body += chunk;
    requests.push({ method: req.method, url: req.url, body, contentType: req.headers['content-type'] });
    res.setHeader('Access-Control-Allow-Origin', '*');
    if (req.url === '/blob') {
      res.setHeader('Content-Type', 'text/plain');
      res.end('onehub_synthetic_export');
    } else if (req.url === '/unauthorized') {
      res.writeHead(401, { 'Content-Type': 'application/json' });
      res.end('{"message":"synthetic unauthorized"}');
    } else {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify({ success: true, method: req.method, url: req.url, body }));
    }
  };
  const server = http.createServer(handler);
  const priceServer = http.createServer(handler);
  // Register cleanup before setup so startup or import failures still close it.
  t.after(async () => {
    await Promise.all([server, priceServer].map((entry) => {
      entry.closeAllConnections();
      return new Promise((resolve, reject) => entry.close((error) => error && error.code !== 'ERR_SERVER_NOT_RUNNING' ? reject(error) : resolve()));
    }));
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  await new Promise((resolve, reject) => {
    priceServer.once('error', reject);
    priceServer.listen(0, '127.0.0.1', resolve);
  });
  const origin = `http://127.0.0.1:${server.address().port}`;
  const priceOrigin = `http://127.0.0.1:${priceServer.address().port}`;
  const dom = new JSDOM('', { url: `${origin}/`, runScripts: 'outside-only' });
  t.after(() => dom.window.close());
  const pkg = JSON.parse(await readFile(path.join(axiosRoot, 'package.json'), 'utf8'));
  dom.window.eval(await readFile(path.join(axiosRoot, 'dist/axios.js'), 'utf8'));
  const axios = dom.window.axios;
  assert.equal(axios.VERSION, pkg.version);
  const api = axios.create({ baseURL: origin, timeout: 3000 });

  await t.test('default browser adapter uses XHR for normal JSON GET', async () => {
    const res = await api.get('/api/user/self');
    assert.ok(res.request instanceof dom.window.XMLHttpRequest);
    assert.equal(res.data.success, true);
    assert.equal(res.data.url, '/api/user/self');
  });
  await t.test('fixed query keys preserve Chinese, punctuation and NUL', async () => {
    const keyword = '中文 &=?/#\u0000';
    const res = await api.get('/api/channel/', { params: { keyword, page: 2 } });
    const url = new URL(res.data.url, origin);
    assert.equal(url.searchParams.get('keyword'), keyword);
    assert.equal(url.searchParams.get('page'), '2');
    assert.deepEqual([...url.searchParams.keys()], ['keyword', 'page']);
  });
  await t.test('JSON POST data remains data with its content type', async () => {
    const data = { username: 'synthetic-user', name: '测试', baseURL: 'payload-only' };
    const res = await api.post('/api/user/login', data);
    assert.equal(res.data.method, 'POST');
    assert.deepEqual(JSON.parse(res.data.body), data);
    assert.equal(requests.at(-1).contentType, 'application/json');
  });
  await t.test('explicit batch DELETE body reaches the synthetic endpoint', async () => {
    const data = { value: 'batch_delete', ids: [1, 2] };
    const res = await api.delete('/api/channel/batch', { data });
    assert.equal(res.data.method, 'DELETE');
    assert.deepEqual(JSON.parse(res.data.body), data);
  });
  await t.test('legitimate absolute external price URL with CORS remains supported', async () => {
    assert.notEqual(priceOrigin, origin);
    const res = await api.get(`${priceOrigin}/prices.json`);
    assert.equal(res.data.url, '/prices.json');
  });
  await t.test('blob export preserves exact synthetic bytes', async () => {
    const res = await api.get('/blob', { responseType: 'blob' });
    assert.ok(res.data instanceof dom.window.Blob);
    const bytes = await new Promise((resolve, reject) => {
      const reader = new dom.window.FileReader();
      reader.onload = () => resolve(Buffer.from(reader.result));
      reader.onerror = () => reject(reader.error);
      reader.readAsArrayBuffer(res.data);
    });
    assert.equal(bytes.toString('utf8'), 'onehub_synthetic_export');
  });
  await t.test('401 rejects with response JSON preserved', async () => {
    await assert.rejects(api.get('/unauthorized'), (error) => {
      assert.equal(error.response.status, 401);
      assert.equal(error.response.data.message, 'synthetic unauthorized');
      return true;
    });
  });
  await t.test('pre-cancelled request does not reach the endpoint', async () => {
    const before = requests.length;
    const controller = new dom.window.AbortController();
    controller.abort();
    await assert.rejects(api.get('/cancelled', { signal: controller.signal }), (error) => axios.isCancel(error));
    assert.equal(requests.length, before);
  });
});
