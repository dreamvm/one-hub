import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(import.meta.url);
// Resolve from the top-level Vite, never Vitest's nested Vite installation.
const viteEntry = path.join(web, 'node_modules/vite/dist/node/index.js');
const viteRequire = createRequire(viteEntry);
const postcssEntry = viteRequire.resolve('postcss');

test('PostCSS file boundaries and legitimate CSS processing', { timeout: 60000 }, async (t) => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'onehub-postcss-')));
  const sourceDir = path.join(root, 'source');
  fs.mkdirSync(sourceDir);
  const sentinelMap = path.join(root, 'synthetic.map');
  const sentinelText = path.join(root, 'synthetic.txt');
  const originalSource = '.original { color: teal; }';
  const map = { version: 3, file: 'fixture.css', sources: ['original.css'], sourcesContent: [originalSource], names: [], mappings: 'AAAA' };
  fs.writeFileSync(sentinelMap, JSON.stringify(map));
  fs.writeFileSync(sentinelText, 'onehub_synthetic_non_json');
  fs.writeFileSync(path.join(sourceDir, 'own.map'), JSON.stringify(map));
  fs.writeFileSync(path.join(sourceDir, 'pixel.svg'), '<svg xmlns="http://www.w3.org/2000/svg"/>');
  fs.symlinkSync(path.join(web, 'node_modules'), path.join(sourceDir, 'node_modules'), 'dir');
  const reads = [];
  const originalRead = fs.readFileSync;
  fs.readFileSync = function (file, ...args) {
    if (file === sentinelMap || file === sentinelText) reads.push(file);
    return originalRead.call(this, file, ...args);
  };
  // Install the observer before loading PostCSS: its module captures readFileSync.
  const postcss = require(postcssEntry);
  const { createServer, build } = await import(pathToFileURL(viteEntry).href);
  const originalProcess = postcss.Processor.prototype.process;
  const versions = new Set();
  postcss.Processor.prototype.process = function (...args) {
    versions.add(this.version);
    return originalProcess.apply(this, args);
  };
  const css = '.normal { background: url("./pixel.svg"); color: teal; }';
  const annotated = (annotation) => `${css}\n/*# sourceMappingURL=${annotation} */`;
  const options = {
    configFile: false, envFile: false, root: sourceDir, logLevel: 'silent',
    css: { devSourcemap: false, preprocessorMaxWorkers: 0 },
    server: { middlewareMode: true, host: '127.0.0.1', hmr: false, watch: null, open: false, proxy: {} },
    build: { write: false, minify: false, cssMinify: false, assetsInlineLimit: 0, sourcemap: false, rollupOptions: { input: path.join(sourceDir, 'entry.js') } }
  };
  const checkCSS = (result) => assert.match(result, /color:\s*teal/);
  try {
    t.diagnostic(`top-level Vite uses PostCSS ${postcss().version}`);
    for (const [name, annotation, from] of [
      ['non-map annotation', '../synthetic.txt', path.join(sourceDir, 'fixture.css')],
      ['parent-directory map', '../synthetic.map', path.join(sourceDir, 'fixture.css')],
      ['map without from', sentinelMap, undefined]
    ]) {
      await t.test(`standalone rejects ${name}`, async () => {
        reads.length = 0;
        const result = await postcss([]).process(annotated(annotation), { from });
        checkCSS(result.css);
        assert.equal(reads.length, 0, 'must not read the synthetic file outside this CSS directory');
      });
    }
    for (const [name, annotation] of [
      ['external', 'own.map'],
      ['inline', `data:application/json;base64,${Buffer.from(JSON.stringify(map)).toString('base64')}`]
    ]) {
      await t.test(`standalone preserves legitimate ${name} map`, async () => {
        const result = await postcss([]).process(annotated(annotation), {
          from: path.join(sourceDir, 'fixture.css'), to: path.join(sourceDir, 'output.css'),
          map: { inline: false, annotation: false, sourcesContent: true }
        });
        checkCSS(result.css);
        assert.ok(result.map.toJSON().sourcesContent.includes(originalSource));
      });
    }
    for (const mode of ['build', 'transform']) {
      for (const scenario of mode === 'build' ? ['parent map', 'non-map', 'import map'] : ['import map']) {
        await t.test(`Vite ${mode} rejects PostCSS ${scenario} read`, async () => {
          reads.length = 0;
          versions.clear();
          const content = annotated(scenario === 'non-map' ? '../synthetic.txt' : '../synthetic.map');
          fs.writeFileSync(path.join(sourceDir, 'nested.css'), content);
          fs.writeFileSync(path.join(sourceDir, 'fixture.module.css'), scenario === 'import map' ? '@import "./nested.css"; .local { color: blue; }' : content);
          fs.writeFileSync(path.join(sourceDir, 'entry.js'), 'import styles from "./fixture.module.css"; console.log(styles.normal, styles.local);');
          let server;
          try {
            if (mode === 'build') {
              const result = await build(options);
              const output = result.output.filter((item) => item.type === 'asset' && item.fileName.endsWith('.css'));
              assert.equal(output.length, 1);
              checkCSS(output[0].source.toString());
            } else {
              server = await createServer(options);
              checkCSS((await server.transformRequest('/fixture.module.css?direct')).code);
            }
            assert.deepEqual([...versions], [postcss().version]);
            assert.equal(reads.length, 0, 'PostCSS must not read the synthetic parent file');
          } finally {
            await server?.close();
          }
        });
      }
    }
    // Normal controls use source maps explicitly and exercise the actual Vite compiler.
    const normalFiles = {
      'plain.css': '.ordinary { color: teal; }',
      'module.module.css': '.moduleBox { color: teal; }',
      'style.scss': '$tone: #123456; .scssBox { color: $tone; .child { padding: 2px; } }',
      'import.css': '@import "./plain.css"; .asset { background: url("./pixel.svg"); }',
      'external.module.css': '.externalBox { color: teal; }\n/*# sourceMappingURL=own.map */',
      'inline.module.css': `.inlineBox { color: teal; }\n/*# sourceMappingURL=data:application/json;base64,${Buffer.from(JSON.stringify(map)).toString('base64')} */`,
      'entry.js': 'import "./plain.css"; import classes from "./module.module.css"; import "./style.scss"; import "./import.css"; import external from "./external.module.css"; import inline from "./inline.module.css"; console.log(classes.moduleBox, external.externalBox, inline.inlineBox);'
    };
    for (const [name, content] of Object.entries(normalFiles)) fs.writeFileSync(path.join(sourceDir, name), content);
    const normalOptions = { ...options, css: { ...options.css, devSourcemap: true }, build: { ...options.build, sourcemap: true } };
    for (const [name, expression] of [
      ['plain.css', /\.ordinary/], ['module.module.css', /moduleBox/], ['style.scss', /\.scssBox\s+\.child/],
      ['import.css', /\.ordinary/], ['external.module.css', /externalBox/], ['inline.module.css', /inlineBox/]
    ]) {
      await t.test(`Vite transforms legitimate ${name}`, async () => {
        const server = await createServer(normalOptions);
        try {
          const result = await server.transformRequest(`/${name}?direct`);
          assert.match(result.code, expression);
          if (name === 'style.scss') assert.match(result.code, /color:\s*#123456/);
          if (name === 'import.css') assert.match(result.code, /url\(["']?\/pixel\.svg/);
          if (name.startsWith('external') || name.startsWith('inline')) assert.ok(result.map.sourcesContent.includes(originalSource));
        } finally {
          await server.close();
        }
      });
    }
    await t.test('Vite builds legitimate CSS, modules, SCSS and assets', async () => {
      const result = await build(normalOptions);
      const cssOutput = result.output.filter((item) => item.type === 'asset' && item.fileName.endsWith('.css')).map((item) => item.source.toString()).join('\n');
      for (const pattern of [/\.ordinary/, /moduleBox/, /\.scssBox\s+\.child/, /externalBox/, /inlineBox/, /url\([^)]*pixel[^)]*\.svg/]) assert.match(cssOutput, pattern);
      assert.ok(result.output.some((item) => item.type === 'asset' && item.fileName.endsWith('.svg')));
    });
  } finally {
    fs.readFileSync = originalRead;
    postcss.Processor.prototype.process = originalProcess;
    fs.rmSync(root, { recursive: true, force: true });
  }
});
