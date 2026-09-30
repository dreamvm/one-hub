import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import test from 'node:test';

const require = createRequire(import.meta.url);
// This is the direct development dependency, not plugin-react's possibly nested copy.
const babelEntry = require.resolve('@babel/core');
const pluginRequire = createRequire(require.resolve('@vitejs/plugin-react'));

test('Babel source map package boundaries and legitimate compilation', { timeout: 30000 }, async (t) => {
  const temp = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'onehub-babel-')));
  const project = path.join(temp, 'project');
  const sourceDir = path.join(project, 'src');
  const bare = path.join(temp, 'bare');
  const nested = path.join(project, 'packages', 'widget');
  for (const dir of [sourceDir, path.join(bare, 'src'), path.join(nested, 'src')]) fs.mkdirSync(dir, { recursive: true });
  for (const dir of [project, nested]) fs.writeFileSync(path.join(dir, 'package.json'), '{"private":true}');
  const source = 'export const value = 7;';
  const normalContent = 'export const original = 7;';
  const outsideContent = 'export const syntheticOutside = 11;';
  const makeMap = (content) => ({ version: 3, sources: ['original.js'], sourcesContent: [content], names: [], mappings: 'AAAA' });
  const normalMap = makeMap(normalContent);
  const outside = path.join(temp, 'outside.map');
  const nonMap = path.join(temp, 'outside.txt');
  const own = path.join(sourceDir, 'own.map');
  const parent = path.join(project, 'parent.map');
  for (const [file, content] of [[outside, JSON.stringify(makeMap(outsideContent))], [nonMap, 'synthetic non-JSON'], [own, JSON.stringify(normalMap)], [parent, JSON.stringify(normalMap)]]) fs.writeFileSync(file, content);
  const watched = new Set([outside, nonMap, own, parent]);
  const reads = [];
  const originalRead = fs.readFileSync;
  fs.readFileSync = function (file, ...args) {
    if (watched.has(file)) reads.push(file);
    return originalRead.call(this, file, ...args);
  };
  const babel = require(babelEntry);
  const pluginBabel = pluginRequire('@babel/core');
  const filename = path.join(sourceDir, 'fixture.js');
  const options = { cwd: project, root: project, filename, configFile: false, babelrc: false, sourceMaps: true };
  const annotated = (annotation, block = false) => source + (block ? '\n/*# sourceMappingURL=' + annotation + ' */' : '\n//# sourceMappingURL=' + annotation);
  const relative = (file, from = sourceDir) => path.relative(from, file).split(path.sep).join('/');
  const run = async (api, code, opts = options) => {
    if (api.startsWith('transformFromAst')) return babel[api](babel.parseSync(code, opts), code, opts);
    if (api.startsWith('transformFile')) {
      fs.writeFileSync(opts.filename, code);
      const { filename: input, ...fileOptions } = opts;
      return babel[api](input, fileOptions);
    }
    return babel[api](code, opts);
  };
  const checkOutput = (result) => assert.match(result.code, /export const value = 7/);
  const checkDenied = (result, target) => {
    checkOutput(result);
    assert.equal(reads.includes(target), false, 'must not read the synthetic file outside the allowed package/root');
    assert.equal(result.map?.sourcesContent?.includes(outsideContent) ?? false, false);
  };
  const checkNormalMap = (result) => {
    checkOutput(result);
    assert.ok(result.map.sourcesContent.includes(normalContent), 'legitimate original source must survive map composition');
  };
  try {
    t.diagnostic('direct Babel ' + babel.version + '; plugin-react Babel ' + pluginBabel.version);
    const apis = ['transformSync', 'transformAsync', 'transformFromAstSync', 'transformFromAstAsync', 'transformFileSync', 'transformFileAsync'];
    for (const api of apis) {
      await t.test(api + ' rejects a map outside the package', async () => {
        reads.length = 0;
        const result = await run(api, annotated(relative(outside)));
        t.diagnostic(api + ': outside read=' + reads.includes(outside) + ', map content=' + result.map?.sourcesContent?.includes(outsideContent));
        checkDenied(result, outside);
      });
      await t.test(api + ' preserves an ordinary same-directory map', async () => {
        reads.length = 0;
        checkNormalMap(await run(api, annotated('own.map')));
        assert.ok(reads.includes(own));
      });
    }
    for (const [name, target, annotation, block] of [
      ['absolute map', outside, outside, false],
      ['normalized path and block comment', outside, 'unused/../' + relative(outside), true],
      ['non-JSON file', nonMap, relative(nonMap), false]
    ]) {
      await t.test('rejects ' + name + ' outside the package', async () => {
        reads.length = 0;
        checkDenied(await run('transformSync', annotated(annotation, block)), target);
      });
    }
    await t.test('uses nearest package boundary inside the configured root', async () => {
      reads.length = 0;
      const nestedDir = path.join(nested, 'src');
      checkDenied(await run('transformSync', annotated(relative(parent, nestedDir)), { ...options, filename: path.join(nestedDir, 'fixture.js') }), parent);
    });
    await t.test('uses configured root when there is no package manifest', async () => {
      reads.length = 0;
      const bareDir = path.join(bare, 'src');
      checkDenied(await run('transformSync', annotated(relative(outside, bareDir)), { ...options, cwd: bare, root: bare, filename: path.join(bareDir, 'fixture.js') }), outside);
    });
    await t.test('does not read outside maps when output maps are disabled', async () => {
      reads.length = 0;
      checkDenied(await run('transformSync', annotated(relative(outside)), { ...options, sourceMaps: false }), outside);
    });
    await t.test('preserves parent-directory maps inside the same package', async () => {
      reads.length = 0;
      checkNormalMap(await run('transformSync', annotated(relative(parent))));
      assert.ok(reads.includes(parent));
    });
    await t.test('preserves inline maps', async () => {
      reads.length = 0;
      checkNormalMap(await run('transformSync', annotated('data:application/json;base64,' + Buffer.from(JSON.stringify(normalMap)).toString('base64'))));
      assert.deepEqual(reads, []);
    });
    await t.test('explicit input map takes precedence over the external comment', async () => {
      reads.length = 0;
      checkNormalMap(await run('transformSync', annotated(relative(outside)), { ...options, inputSourceMap: normalMap }));
      assert.deepEqual(reads, []);
    });
    await t.test('inputSourceMap false keeps normal compilation without reading a map', async () => {
      reads.length = 0;
      checkDenied(await run('transformSync', annotated(relative(outside)), { ...options, inputSourceMap: false }), outside);
    });
    await t.test('no filename keeps normal compilation without reading a map', async () => {
      reads.length = 0;
      const { filename: unused, ...noFilename } = options;
      checkDenied(await run('transformSync', annotated(outside), noFilename), outside);
    });
    await t.test('missing and malformed local maps keep the documented fallback', async () => {
      fs.writeFileSync(path.join(sourceDir, 'invalid.map'), 'synthetic invalid map');
      for (const name of ['missing.map', 'invalid.map']) checkOutput(await run('transformSync', annotated(name)));
    });
    await t.test('Babel parseSync parses JSX without loading external maps', () => {
      reads.length = 0;
      const result = babel.parseSync('const element = <span>normal</span>;\n//# sourceMappingURL=' + relative(outside), { ...options, parserOpts: { plugins: ['jsx'] } });
      assert.equal(result.program.body[0].declarations[0].init.type, 'JSXElement');
      assert.deepEqual(reads, []);
    });
    await t.test('ESLint Babel parser preserves JSX and does not load external maps', () => {
      reads.length = 0;
      const parser = require('@babel/eslint-parser');
      const result = parser.parseForESLint('const element = <span>normal</span>;\n//# sourceMappingURL=' + relative(outside), {
        filePath: filename, requireConfigFile: false, sourceType: 'module',
        babelOptions: { cwd: project, root: project, configFile: false, babelrc: false, parserOpts: { plugins: ['jsx'] } }
      });
      assert.equal(result.ast.body[0].declarations[0].init.type, 'JSXElement');
      assert.deepEqual(reads, []);
    });
    await t.test('plugin-react resolved Babel rejects the same outside map', async () => {
      reads.length = 0;
      checkDenied(await pluginBabel.transformAsync(annotated(relative(outside)), options), outside);
    });
    await t.test('plugin-react resolved Babel preserves a legitimate map', async () => {
      reads.length = 0;
      checkNormalMap(await pluginBabel.transformAsync(annotated('own.map'), options));
      assert.ok(reads.includes(own));
    });
  } finally {
    fs.readFileSync = originalRead;
    fs.rmSync(temp, { recursive: true, force: true });
  }
});
