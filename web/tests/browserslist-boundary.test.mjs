import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const filename = fileURLToPath(import.meta.url);
const require = createRequire(import.meta.url);
const browserslist = require('browserslist');
const targetsRequire = createRequire(require.resolve('@babel/helper-compilation-targets'));
const getTargets = require('@babel/helper-compilation-targets').default;
const reserved = ['constructor', 'toString', 'valueOf', 'hasOwnProperty', 'isPrototypeOf', '__proto__'];
const cases = [
  ...reserved.map((name) => 'stats:' + name),
  'actual-babel-stats-file',
  'bounded-result-cache',
  'normal-queries',
  'actual-babel-targets'
];

function statsWithUnknownAgent(name) {
  return Object.defineProperty({ chrome: { 90: 1 } }, name, {
    value: { 90: 1 },
    enumerable: true
  });
}

function runCase(selected) {
  if (selected.startsWith('stats:')) {
    const stats = statsWithUnknownAgent(selected.slice(6));
    assert.deepEqual(browserslist('chrome 90', { stats }), ['chrome 90']);
    assert.equal(Object.getPrototypeOf(stats), Object.prototype);
  } else if (selected === 'actual-babel-stats-file') {
    const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'onehub-browser-targets-'));
    const originalCwd = process.cwd();
    try {
      fs.writeFileSync(path.join(cwd, 'browserslist-stats.json'), JSON.stringify(statsWithUnknownAgent('constructor')));
      process.chdir(cwd);
      assert.equal(targetsRequire.resolve('browserslist'), require.resolve('browserslist'));
      assert.deepEqual(getTargets({ browsers: 'chrome 90' }, { configPath: cwd }), { chrome: '90.0.0' });
    } finally {
      process.chdir(originalCwd);
      fs.rmSync(cwd, { recursive: true, force: true });
    }
  } else if (selected === 'bounded-result-cache') {
    const query = 'chrome 90';
    const resolve = browserslist;
    const first = resolve(query);
    // A fixed small set of valid queries observes eviction without measuring heap or exhausting it.
    for (let day = 1; day <= 502; day += 1) resolve('since 1900-01-' + day);
    const after = resolve(query);
    assert.notStrictEqual(after, first, 'the oldest entry must leave the bounded cache');
    assert.deepEqual(after, first, 'eviction must preserve query meaning');
  } else if (selected === 'normal-queries') {
    assert.deepEqual(browserslist('chrome 90'), ['chrome 90']);
    assert.deepEqual(browserslist(['chrome 90', 'firefox 90']), ['chrome 90', 'firefox 90']);
    assert.deepEqual(browserslist('chrome 90', { stats: { chrome: { 90: 1 } } }), ['chrome 90']);
    assert.deepEqual(browserslist('> 0.5% in my stats', { stats: { chrome: { 90: 1 } } }), ['chrome 90']);
    assert.throws(() => browserslist('syntheticbrowser 90'), (error) => error.browserslist === true);
  } else if (selected === 'actual-babel-targets') {
    assert.equal(targetsRequire.resolve('browserslist'), require.resolve('browserslist'));
    assert.deepEqual(getTargets({ browsers: ['chrome 90', 'firefox 90'] }), { chrome: '90.0.0', firefox: '90.0.0' });
    assert.deepEqual(getTargets({ browsers: 'ie 11' }), { ie: '11.0.0' });
  } else {
    throw new Error('unknown fixed synthetic case');
  }
}

if (process.env.ONEHUB_BROWSERSLIST_CASE !== undefined) {
  const selected = cases[Number(process.env.ONEHUB_BROWSERSLIST_CASE)];
  assert.ok(selected);
  runCase(selected);
  process.stdout.write(JSON.stringify({ passed: true }));
} else {
  test('Babel browser targets: own stats and bounded caches', { timeout: 70000 }, async (t) => {
    t.diagnostic(require('browserslist/package.json').version);
    for (let index = 0; index < cases.length; index += 1) {
      await t.test(cases[index], () => {
        const child = spawnSync(process.execPath, ['--max-old-space-size=192', filename], {
          env: { PATH: process.env.PATH, LANG: 'C.UTF-8', BROWSERSLIST_IGNORE_OLD_DATA: 'true', ONEHUB_BROWSERSLIST_CASE: String(index) },
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
