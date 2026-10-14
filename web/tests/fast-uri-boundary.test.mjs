import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';

const filename = fileURLToPath(import.meta.url);
const require = createRequire(import.meta.url);
const uri = require('fast-uri');
const malformed = [
  'https://onehub.invalid\\@other.invalid/x',
  'https:\\\\onehub.invalid/x',
  'https:/\\onehub.invalid/x',
  'https:\\/onehub.invalid/x',
  '/\t/onehub.invalid/x',
  '/\n\\onehub.invalid/x',
  'https://[::1]extra/x',
  'https://[:::1]/x',
  'https://onehub.invalid:65536/x',
  'x://onehub.invalid/a%',
  'x://onehub.invalid/a%2',
  'x://onehub.invalid/a%GG',
  'x://exa%mple.invalid/x',
  'x://us%er@onehub.invalid/x',
  'x://onehub.invalid/x?q=%G0',
  'x://onehub.invalid/x#frag%1',
  'foo%3Abar:value',
  'foo%2Fbar:value',
  '1http://onehub.invalid/x',
  'https://\u200D.invalid/x'
];
const cases = [
  ...malformed.map((input) => ({ kind: 'malformed', input })),
  { kind: 'encoded-authority' },
  { kind: 'encoded-path' },
  { kind: 'component-port' },
  { kind: 'host-case' },
  { kind: 'normal-uri' },
  { kind: 'ajv-consumer' },
  { kind: 'conf-consumer' }
];

async function runCase(selected) {
  if (selected.kind === 'malformed') {
    assert.ok(uri.parse(selected.input).error, 'parse must report malformed input');
    assert.equal(uri.normalize(selected.input), selected.input, 'normalize must preserve the failing input');
    assert.equal(uri.equal(selected.input, selected.input), false, 'malformed URIs must not compare equal');
    assert.throws(() => uri.resolve('https://onehub.invalid/base/', selected.input));
    assert.throws(() => uri.resolve(selected.input, 'child'));
  } else if (selected.kind === 'encoded-authority') {
    for (const escape of ['%40', '%3A', '%2F', '%23', '%3F', '%5C', '%2540']) {
      const input = 'https://onehub.invalid' + escape + 'other.invalid/x';
      assert.equal(uri.normalize(input), input);
      assert.notEqual(uri.parse(input).host, 'other.invalid');
    }
    assert.equal(uri.equal('x://onehub.invalid%40other.invalid/', 'x://onehub.invalid@other.invalid/'), false);
    const serialized = uri.serialize({ scheme: 'x', host: 'onehub.invalid@other.invalid', path: '/x' });
    assert.notEqual(uri.parse(serialized).host, 'other.invalid');
  } else if (selected.kind === 'encoded-path') {
    for (const [encoded, literal] of [
      ['/public/%2e%2e/private', '/private'],
      ['/one%2Ftwo', '/one/two']
    ]) {
      const input = 'https://onehub.invalid' + encoded;
      assert.equal(uri.normalize(input), input.replace(/%2e/g, '%2E'));
      assert.equal(uri.equal(input, 'https://onehub.invalid' + literal), false);
    }
    assert.equal(uri.serialize(uri.parse('https://onehub.invalid/a%3Ab')), 'https://onehub.invalid/a%3Ab');
  } else if (selected.kind === 'component-port') {
    for (const port of ['80@other.invalid', '443/path', '443?query', -1, 1.5]) {
      assert.throws(() => uri.serialize({ scheme: 'https', host: 'onehub.invalid', port, path: '/x' }));
    }
    assert.equal(uri.serialize({ scheme: 'https', host: 'onehub.invalid', port: 8443, path: '/x' }), 'https://onehub.invalid:8443/x');
    assert.ok(
      uri.parse(uri.serialize({ scheme: 'https', host: 'onehub.invalid', port: '65536', path: '/x' })).error,
      'out-of-range numeric port is rejected when parsed, not when serialized'
    );
  } else if (selected.kind === 'host-case') {
    assert.equal(uri.parse('//%41.onehub.invalid/x').host, 'a.onehub.invalid');
    assert.equal(uri.normalize('//%41.onehub.invalid/x'), '//a.onehub.invalid/x');
    assert.equal(uri.equal('//%41.onehub.invalid/x', '//a.onehub.invalid/x'), true);
    assert.equal(uri.equal('//User@%41.onehub.invalid/Path', '//user@a.onehub.invalid/path'), false);
    assert.equal(uri.normalize('HTTPS://ONEHUB.INVALID:443/a'), 'https://onehub.invalid/a');
    assert.equal(uri.parse('https://\uFF45xample.invalid/').host, 'example.invalid');
  } else if (selected.kind === 'normal-uri') {
    assert.equal(uri.resolve('https://onehub.invalid/a/b', '../c#part'), 'https://onehub.invalid/c#part');
    assert.equal(uri.normalize('https://ONEHUB.INVALID:443/a'), 'https://onehub.invalid/a');
    assert.equal(uri.normalize('http://[2001:db8::7]:80/x'), 'http://[2001:db8::7]/x');
    assert.equal(uri.parse('x://user:pass@onehub.invalid/a;b?q=One#Two').error, undefined);
  } else if (selected.kind === 'ajv-consumer') {
    const confRequire = createRequire(require.resolve('conf'));
    const Ajv = confRequire('ajv');
    const ajvRequire = createRequire(confRequire.resolve('ajv/package.json'));
    assert.equal(ajvRequire.resolve('fast-uri'), require.resolve('fast-uri'), 'AJV8 must load the updated shared entry');
    const ajv = new Ajv({ uriResolver: uri });
    ajv.addSchema({
      $id: 'https://onehub.invalid/schema/person',
      type: 'object',
      properties: { name: { type: 'string' } },
      required: ['name']
    });
    const validate = ajv.compile({ $id: 'https://onehub.invalid/schema/root', $ref: './person' });
    assert.equal(validate({ name: 'synthetic' }), true);
    assert.equal(validate({ name: 2 }), false);
    const local = ajv.compile({ $ref: '#/definitions/Record%3Cstring%3E', definitions: { 'Record<string>': { type: 'string' } } });
    assert.equal(local('synthetic'), true);
    assert.equal(local(2), false);
  } else if (selected.kind === 'conf-consumer') {
    const { default: Conf } = await import(pathToFileURL(require.resolve('conf')).href);
    const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'onehub-uri-conf-'));
    try {
      const store = new Conf({
        cwd,
        projectName: 'onehub-synthetic',
        schema: { apiBaseUrl: { type: 'string', default: 'https://onehub.invalid' }, openaiToken: { type: 'string', default: '' } }
      });
      assert.equal(store.get('apiBaseUrl'), 'https://onehub.invalid');
      assert.equal(store.get('openaiToken'), '');
      store.set('apiBaseUrl', 'https://other.invalid');
      assert.equal(store.get('apiBaseUrl'), 'https://other.invalid');
      assert.throws(() => store.set('apiBaseUrl', 2));
      assert.equal(store.get('apiBaseUrl'), 'https://other.invalid');
    } finally {
      fs.rmSync(cwd, { recursive: true, force: true });
    }
  } else {
    throw new Error('unknown fixed synthetic case');
  }
}

if (process.env.ONEHUB_FAST_URI_CASE !== undefined) {
  const selected = cases[Number(process.env.ONEHUB_FAST_URI_CASE)];
  assert.ok(selected);
  await runCase(selected);
  process.stdout.write(JSON.stringify({ version: require('fast-uri/package.json').version, passed: true }));
} else {
  test('shared fast-uri boundary and legitimate AJV references', { timeout: 90000 }, async (t) => {
    t.diagnostic('fast-uri ' + require('fast-uri/package.json').version);
    for (let index = 0; index < cases.length; index += 1) {
      const selected = cases[index];
      await t.test(selected.kind + (selected.input ? ' ' + JSON.stringify(selected.input) : ''), () => {
        const child = spawnSync(process.execPath, ['--max-old-space-size=192', filename], {
          cwd: path.dirname(path.dirname(filename)),
          env: { PATH: process.env.PATH, LANG: 'C.UTF-8', ONEHUB_FAST_URI_CASE: String(index) },
          timeout: 5000,
          maxBuffer: 32768,
          encoding: 'utf8'
        });
        assert.ifError(child.error);
        assert.equal(child.signal, null);
        assert.equal(child.status, 0, child.stderr || child.stdout);
        assert.equal(JSON.parse(child.stdout).passed, true);
      });
    }
  });
}
