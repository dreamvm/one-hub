import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

// Explicitly target web/node_modules/axios, not a test runner's dependency.
// A frozen baseline may use ONEHUB_AXIOS_ROOT to select its top-level package.
// These synthetic pollution gadgets exercise shared browser Axios behavior.
// They do not establish an application source of Object.prototype pollution.
// No HTTP service, actual credentials, or real user data is used.
const thisFile = fileURLToPath(import.meta.url);
const axiosRoot = path.resolve(process.env.ONEHUB_AXIOS_ROOT || path.join(path.dirname(thisFile), '../node_modules/axios'));

async function loadBrowserAxios(root) {
  const pkg = JSON.parse(await readFile(path.join(root, 'package.json'), 'utf8'));
  assert.equal(pkg.name, 'axios', 'the target must be the explicit top-level Axios package');
  const require = createRequire(import.meta.url);
  const axios = require(path.join(root, 'dist/browser/axios.cjs'));
  assert.equal(axios.VERSION, pkg.version, 'browser distribution must match package metadata');
  return { axios, version: pkg.version };
}

function response(config, data = '{"success":true,"value":7}') {
  return Promise.resolve({ config, data, status: 200, statusText: 'OK', headers: { 'content-type': 'application/json' } });
}

async function worker(scenario, root) {
  // Import first. Pollute only this disposable process, after loading the library.
  const { axios } = await loadBrowserAxios(root);
  let captured;
  let reviverCalls = 0;
  const client = axios.create({ adapter: (config) => {
    captured = config;
    return response(config);
  } });
  let result;
  try {
    if (scenario.startsWith('inherited-data:')) {
      const method = scenario.split(':')[1];
      assert.ok(['delete', 'get', 'head', 'options'].includes(method));
      Object.defineProperty(Object.prototype, 'data', {
        value: 'onehub_synthetic_inherited_body', writable: true, configurable: true
      });
      try {
        await client[method]('/api/redemption/1');
        result = { error: null, body: captured.data === undefined ? null : captured.data };
      } catch (error) {
        // In 1.12.2 a string-valued inherited data option is promoted by the
        // alias, then its inherited spelling-validator slot causes TypeError.
        // Preserve that actual observation; do not claim a body was sent.
        result = { error: { name: error.name, message: error.message }, adapterReached: captured !== undefined };
      }
    } else if (scenario === 'inherited-reviver') {
      Object.defineProperty(Object.prototype, 'parseReviver', {
        value: (key, value) => {
          reviverCalls += 1;
          return key === 'value' ? 'onehub_synthetic_inherited_reviver' : value;
        }, writable: true, configurable: true
      });
      result = { parsed: (await client.get('/api/user/self')).data, reviverCalls };
    } else {
      throw new Error(`unknown child scenario: ${scenario}`);
    }
  } finally {
    delete Object.prototype.data;
    delete Object.prototype.parseReviver;
  }
  process.stdout.write(JSON.stringify(result));
}

function runIsolated(scenario) {
  const result = spawnSync(process.execPath, [thisFile, '--worker', scenario, axiosRoot], {
    encoding: 'utf8', timeout: 10000, maxBuffer: 1024 * 1024
  });
  assert.equal(result.error, undefined, `child process failed: ${result.error?.message}`);
  assert.equal(result.status, 0, `child process exited unsuccessfully: ${result.stderr}`);
  return JSON.parse(result.stdout);
}

if (process.argv[2] === '--worker') {
  await worker(process.argv[3], process.argv[4]);
} else {
  const { axios, version } = await loadBrowserAxios(axiosRoot);
  test(`explicit top-level browser Axios ${version}: Node HTTP adapter is absent`, () => {
    assert.throws(() => axios.getAdapter('http'), (error) => error.code === 'ERR_NOT_SUPPORT');
  });
  for (const method of ['delete', 'get', 'head', 'options']) {
    test(`Axios ${version}: bodyless ${method.toUpperCase()} ignores inherited data`, () => {
      const result = runIsolated(`inherited-data:${method}`);
      assert.equal(result.error, null, JSON.stringify(result));
      assert.equal(result.body, null);
      assert.equal(Object.prototype.hasOwnProperty.call(Object.prototype, 'data'), false);
    });
  }
  test(`Axios ${version}: JSON transformation ignores inherited parseReviver`, () => {
    const result = runIsolated('inherited-reviver');
    assert.deepEqual(result.parsed, { success: true, value: 7 });
    assert.equal(result.reviverCalls, 0);
    assert.equal(Object.prototype.hasOwnProperty.call(Object.prototype, 'parseReviver'), false);
  });
  test(`Axios ${version}: explicit channel batch DELETE data is preserved`, async () => {
    let captured;
    const client = axios.create({ adapter: (config) => {
      captured = config;
      return response(config);
    } });
    const data = { value: 'batch_delete', ids: [1, 2] };
    const res = await client.delete('/api/channel/batch', { data });
    assert.deepEqual(JSON.parse(captured.data), data);
    assert.equal(captured.headers.getContentType(), 'application/json');
    assert.equal(res.data.success, true);
  });
  test(`Axios ${version}: normal JSON and explicit parseReviver are preserved`, async () => {
    const client = axios.create({ adapter: (config) => response(config) });
    assert.deepEqual((await client.get('/api/user/self')).data, { success: true, value: 7 });
    const configured = await client.get('/api/user/self', {
      parseReviver: (key, value) => key === 'value' ? value * 2 : value
    });
    assert.deepEqual(configured.data, { success: true, value: 14 });
  });
  test(`Axios ${version}: API base URL, fixed query keys and absolute price URL work`, () => {
    const client = axios.create({ baseURL: '/' });
    const keyword = '中文 &=?/#\u0000';
    const uri = client.getUri({ url: '/api/user/', params: { keyword, page: 1 } });
    const parsed = new URL(uri, 'http://onehub.invalid');
    assert.equal(parsed.pathname, '/api/user/');
    assert.equal(parsed.searchParams.get('keyword'), keyword);
    assert.equal(parsed.searchParams.get('page'), '1');
    assert.deepEqual([...parsed.searchParams.keys()], ['keyword', 'page']);
    assert.equal(client.getUri({ url: 'https://prices.invalid/models.json' }), 'https://prices.invalid/models.json');
    const configured = axios.create({ baseURL: 'https://api.invalid' });
    assert.equal(configured.getUri({ url: '/api/user/self' }), 'https://api.invalid/api/user/self');
  });
  test(`Axios ${version}: JSON POST remains request data, not request configuration`, async () => {
    let captured;
    const client = axios.create({ adapter: (config) => {
      captured = config;
      return response(config);
    } });
    const data = { username: 'synthetic-user', password: 'synthetic-fixture', baseURL: 'payload-only', headers: { title: 'payload-only' } };
    await client.post('/api/user/login', data);
    assert.equal(captured.url, '/api/user/login');
    assert.equal(captured.baseURL, undefined);
    assert.deepEqual(JSON.parse(captured.data), data);
    assert.equal(captured.headers.has('title'), false);
  });
  test(`Axios ${version}: blob response remains a Blob`, async () => {
    const fixture = new Blob(['onehub_synthetic_download'], { type: 'text/plain' });
    const client = axios.create({ adapter: (config) => response(config, fixture) });
    const res = await client.get('/api/analytics/multi_user_stats/export', { responseType: 'blob' });
    assert.equal(res.data, fixture);
    assert.equal(await res.data.text(), 'onehub_synthetic_download');
  });
  test(`Axios ${version}: cancellation occurs before the synthetic adapter`, async () => {
    const controller = new AbortController();
    controller.abort();
    let adapterCalls = 0;
    const client = axios.create({ adapter: (config) => {
      adapterCalls += 1;
      return response(config);
    } });
    await assert.rejects(client.get('/api/user/self', { signal: controller.signal }), (error) => axios.isCancel(error));
    assert.equal(adapterCalls, 0);
  });
}
