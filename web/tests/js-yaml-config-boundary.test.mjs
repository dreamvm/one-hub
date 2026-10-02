import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';

const filename = fileURLToPath(import.meta.url);
const web = path.resolve(path.dirname(filename), '..');
const require = createRequire(import.meta.url);
const yaml = require('js-yaml');
const yamlRoot = path.dirname(require.resolve('js-yaml/package.json'));
const { ConfigArrayFactory } = require('@eslint/eslintrc').Legacy;
const extensions = ['', '.yaml', '.yml'];
const cases = [
  ...extensions.flatMap((extension) => ['plain', 'quoted'].map((key) => ({ kind: 'prototype', extension, key }))),
  ...extensions.map((extension) => ({ kind: 'sequence-limit', extension })),
  ...extensions.map((extension) => ({ kind: 'normal-config', extension })),
  { kind: 'repository-config' },
  ...['load', 'loadAll-array', 'loadAll-iterator'].map((api) => ({ kind: 'merge-budget', api })),
  { kind: 'esm-budget' },
  { kind: 'omap' },
  { kind: 'tap' },
  { kind: 'shared-callers' }
];

function budgetCheck(api, parser) {
  const normal = 'base: &base {a: 1}\nresult: {<<: *base, b: 2}\n';
  const emptySources = 'base: &base [{}, {}]\nresult: {<<: *base}\n';
  const options = { maxTotalMergeKeys: 1 };
  const run = (input, selectedOptions) => {
    if (api === 'load') return parser.load(input, selectedOptions);
    if (api === 'loadAll-array') return parser.loadAll(input, selectedOptions);
    const documents = [];
    parser.loadAll(input, (document) => documents.push(document), selectedOptions);
    return documents;
  };
  const result = run(normal, { maxTotalMergeKeys: 3 });
  assert.deepEqual(api === 'load' ? result.result : result[0].result, { a: 1, b: 2 });
  assert.throws(() => run(emptySources, options), parser.YAMLException, 'empty merge mappings must consume the configured work budget');
  const multipleDocuments = emptySources + '---\n' + emptySources;
  if (api !== 'load') {
    assert.throws(() => run(multipleDocuments, { maxTotalMergeKeys: 3 }), parser.YAMLException, 'loadAll must share the merge work budget across documents');
  }
}

async function runCase(selected) {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'onehub-yaml-')));
  const configFile = path.join(root, '.eslintrc' + (selected.extension ?? ''));
  const factory = new ConfigArrayFactory({ cwd: root, resolvePluginsRelativeTo: web });
  const loadConfig = (content) => {
    assert.ok(Buffer.byteLength(content) < 4096, 'synthetic config must stay below 4KiB');
    fs.writeFileSync(configFile, content);
    return factory.loadFile(configFile);
  };
  try {
    if (selected.kind === 'prototype') {
      const key = selected.key === 'quoted' ? '"__proto__"' : '__proto__';
      const array = loadConfig([
        'root: true',
        'settings:',
        '  syntheticDefaults: &syntheticDefaults',
        '    ' + key + ': {onehubSyntheticMarker: true}',
        '  syntheticResult:',
        '    <<: *syntheticDefaults',
        ''
      ].join('\n'));
      const result = array.find((config) => config.settings?.syntheticResult).settings.syntheticResult;
      assert.equal(Object.getPrototypeOf(result), Object.prototype, 'merge must preserve the result object prototype');
      assert.equal(result.onehubSyntheticMarker, undefined, 'synthetic marker must not become an inherited configuration property');
      assert.ok(Object.hasOwn(result, '__proto__'), 'the ordinary data key must be retained');
      assert.equal(Object.prototype.onehubSyntheticMarker, undefined, 'no global prototype mutation');
    } else if (selected.kind === 'sequence-limit') {
      const content = 'root: true\nsettings:\n  base: &base {}\n  result:\n    <<: [' + Array(101).fill('*base').join(', ') + ']\n';
      assert.throws(() => loadConfig(content), (error) => error instanceof yaml.YAMLException && /Cannot read config file/.test(error.message), 'actual ESLint loader must reject a merge sequence above the default hard limit');
    } else if (selected.kind === 'normal-config') {
      const content = 'root: true\nsettings:\n  base: &base {onehubTitle: "合成配置", count: 2}\n  result:\n    <<: [' + Array(100).fill('*base').join(', ') + ']\n    count: 3\n';
      const array = loadConfig(content);
      const settings = array.find((config) => config.settings?.result).settings;
      assert.deepEqual(settings.result, { onehubTitle: '合成配置', count: 3 });
      assert.equal(Object.getPrototypeOf(settings.result), Object.prototype);
    } else if (selected.kind === 'repository-config') {
      const { ESLint } = require('eslint');
      const config = await new ESLint({ cwd: web }).calculateConfigForFile(path.join(web, 'src/index.jsx'));
      assert.equal(config.settings.react.version, 'detect');
      assert.equal(config.parser, require.resolve('@babel/eslint-parser'));
      assert.equal(config.parserOptions.requireConfigFile, false);
      assert.deepEqual(config.parserOptions.babelOptions.parserOpts.plugins, ['jsx']);
      assert.ok([0, 'off'].includes(config.rules['react/react-in-jsx-scope'][0]));
      assert.ok([1, 'warn'].includes(config.rules['prettier/prettier'][0]));
      assert.equal(config.env.browser, true);
      assert.ok(config.plugins.includes('react') && config.plugins.includes('react-hooks'));
    } else if (selected.kind === 'merge-budget') {
      budgetCheck(selected.api, yaml);
    } else if (selected.kind === 'esm-budget') {
      throw new Error('esm-budget must use the async dispatch');
    } else if (selected.kind === 'omap') {
      assert.deepEqual(yaml.load('!!omap\n- onehubFirst: 1\n- onehubSecond: 2\n'), [{ onehubFirst: 1 }, { onehubSecond: 2 }]);
      assert.throws(() => yaml.load('!!omap\n- onehubFirst: 1\n- onehubFirst: 2\n'), yaml.YAMLException);
    } else if (selected.kind === 'tap') {
      const formatter = require(path.join(path.dirname(require.resolve('eslint/package.json')), 'lib/cli-engine/formatters/tap.js'));
      const output = formatter([{ filePath: path.join(root, 'fixture.js'), messages: [{ severity: 1, ruleId: 'fixture', message: 'onehub synthetic message', line: 1, column: 1 }] }]);
      assert.match(output, /^TAP version 13/);
      assert.match(output, /onehub synthetic message/);
    } else if (selected.kind === 'shared-callers') {
      for (const name of ['eslint', '@eslint/eslintrc', 'cosmiconfig', 'langchain']) {
        const caller = createRequire(path.join(web, 'node_modules', name, 'package.json'));
        assert.equal(caller.resolve('js-yaml'), require.resolve('js-yaml'), name + ' must use the shared v4 entry');
      }
      const matter = require('gray-matter');
      const matterRequire = createRequire(require.resolve('gray-matter'));
      assert.notEqual(matterRequire.resolve('js-yaml'), require.resolve('js-yaml'), 'v3 safeLoad consumer must retain its separate major version');
      assert.deepEqual(matter('---\nonehubTitle: synthetic\n---\nfixture body').data, { onehubTitle: 'synthetic' });
    } else {
      throw new Error('unknown fixed regression case');
    }
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
}

if (process.env.ONEHUB_YAML_BOUNDARY_CASE !== undefined) {
  const selected = cases[Number(process.env.ONEHUB_YAML_BOUNDARY_CASE)];
  assert.ok(selected, 'only fixed synthetic cases are supported');
  if (selected.kind === 'esm-budget') {
    const esm = await import(pathToFileURL(path.join(yamlRoot, 'dist/js-yaml.mjs')).href);
    for (const api of ['load', 'loadAll-array', 'loadAll-iterator']) budgetCheck(api, esm);
  } else {
    await runCase(selected);
  }
  process.stdout.write(JSON.stringify({ yaml: require('js-yaml/package.json').version, case: selected.kind, passed: true }));
} else {
  test('shared js-yaml configuration boundary and legitimate tooling', { timeout: 90000 }, async (t) => {
    t.diagnostic('shared js-yaml ' + require('js-yaml/package.json').version);
    for (let index = 0; index < cases.length; index += 1) {
      const selected = cases[index];
      await t.test([selected.kind, selected.extension === '' ? 'extensionless' : selected.extension, selected.key, selected.api].filter(Boolean).join(' '), () => {
        const child = spawnSync(process.execPath, ['--max-old-space-size=192', filename], {
          cwd: web,
          env: { PATH: process.env.PATH, LANG: 'C.UTF-8', ONEHUB_YAML_BOUNDARY_CASE: String(index) },
          timeout: 8000,
          maxBuffer: 32768,
          encoding: 'utf8'
        });
        assert.ifError(child.error);
        assert.equal(child.signal, null, 'bounded worker must finish normally');
        assert.equal(child.status, 0, child.stderr || child.stdout);
        assert.equal(JSON.parse(child.stdout).passed, true);
      });
      if (t.signal.aborted) break;
    }
  });
}
