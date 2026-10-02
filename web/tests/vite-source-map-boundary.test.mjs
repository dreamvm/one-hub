import assert from 'node:assert/strict';
import fs from 'node:fs';
import fsp from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import test from 'node:test';

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
test('Vite dependency map reads stay within their package', { timeout: 30000 }, async (t) => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'onehub-vite-map-')));
  const project = path.join(root, 'project');
  const textFile = path.join(root, 'synthetic.txt');
  const mapFile = path.join(root, 'synthetic.map');
  const marker = 'ONEHUB_SELF_CREATED_MAP_MARKER';
  fs.writeFileSync(textFile, marker);
  fs.writeFileSync(
    mapFile,
    JSON.stringify({ version: 3, sources: ['original.js'], sourcesContent: [marker], names: [], mappings: 'AAAA' })
  );
  const reads = [];
  const asyncRead = fsp.readFile,
    syncRead = fs.readFileSync;
  const observe = (file) => {
    if (file === textFile || file === mapFile) reads.push(file);
  };
  fsp.readFile = function (file, ...args) {
    observe(file);
    return asyncRead.call(this, file, ...args);
  };
  fs.readFileSync = function (file, ...args) {
    observe(file);
    return syncRead.call(this, file, ...args);
  };
  const { createServer } = await import(pathToFileURL(path.join(web, 'node_modules/vite/dist/node/index.js')).href);
  const scenarios = [
    { name: 'JS external map', kind: 'annotation', ext: 'js' },
    { name: 'CSS external map default', kind: 'annotation', ext: 'css' },
    { name: 'CSS external map enabled', kind: 'annotation', ext: 'css', cssMap: true },
    { name: 'inline JS missing source', kind: 'inline', ext: 'js' },
    { name: 'external JS missing source', kind: 'source', ext: 'js' },
    { name: 'inline JS sourceRoot', kind: 'sourceRoot', ext: 'js' },
    { name: 'scoped package annotation', kind: 'annotation', ext: 'js', scoped: true },
    { name: 'scoped package source', kind: 'inline', ext: 'js', scoped: true },
    { name: 'CSS injected map enabled', kind: 'annotation', ext: 'css', cssMap: true, injected: true },
    { name: 'normal package parent map', kind: 'normalParentMap', ext: 'js', normal: true },
    { name: 'normal first-party parent map', kind: 'normalFirstParty', ext: 'js', normal: true, firstParty: true },
    { name: 'normal JS external map', kind: 'normalMap', ext: 'js', normal: true },
    { name: 'normal JS inline map', kind: 'normalInline', ext: 'js', normal: true },
    { name: 'normal JS source content', kind: 'normalSource', ext: 'js', normal: true },
    { name: 'normal CSS external map', kind: 'normalMap', ext: 'css', cssMap: true, normal: true }
  ];
  try {
    fs.mkdirSync(project, { recursive: true });
    for (const [index, s] of scenarios.entries()) {
      await t.test(s.name, async () => {
        const pkg = s.firstParty
          ? path.join(project, 'src')
          : path.join(project, 'node_modules', s.scoped ? '@onehub-qa/maps' : 'onehub-qa-maps');
        const dir = path.join(pkg, 'dist');
        fs.mkdirSync(dir, { recursive: true });
        fs.writeFileSync(
          path.join(pkg, 'package.json'),
          JSON.stringify({ name: s.scoped ? '@onehub-qa/maps' : 'onehub-qa-maps', version: '0.0.0', type: 'module' })
        );
        const filename = path.join(dir, `case-${index}.${s.ext}`);
        const normal = 'ONEHUB_NORMAL_SOURCE';
        const map = { version: 3, sources: ['original.js'], sourcesContent: [normal], names: [], mappings: 'AAAA' };
        let annotation;
        if (s.kind === 'annotation') annotation = path.relative(dir, mapFile);
        else {
          if (['inline', 'source'].includes(s.kind)) {
            map.sources = [path.relative(dir, textFile)];
            delete map.sourcesContent;
          }
          if (s.kind === 'sourceRoot') {
            map.sourceRoot = path.relative(dir, root);
            map.sources = ['synthetic.txt'];
            delete map.sourcesContent;
          }
          if (s.kind === 'normalSource') {
            fs.mkdirSync(path.join(pkg, 'src'), { recursive: true });
            fs.writeFileSync(path.join(pkg, 'src/normal.js'), normal);
            map.sources = ['../src/normal.js'];
            delete map.sourcesContent;
          }
          if (['inline', 'sourceRoot', 'normalInline'].includes(s.kind))
            annotation = 'data:application/json;base64,' + Buffer.from(JSON.stringify(map)).toString('base64');
          else {
            annotation = `case-${index}.map`;
            if (s.kind === 'normalParentMap') {
              fs.mkdirSync(path.join(pkg, 'src'), { recursive: true });
              annotation = '../src/normal.map';
            }
            if (s.kind === 'normalFirstParty') annotation = '../../../trusted.map';
            fs.writeFileSync(path.resolve(dir, annotation), JSON.stringify(map));
          }
        }
        const css = s.ext === 'css';
        fs.writeFileSync(
          filename,
          (css ? '.normal { color: teal; }' : 'export const normal = 1;') +
            '\n' +
            (css ? `/*# sourceMappingURL=${annotation} */` : `//# sourceMappingURL=${annotation}`)
        );
        const server = await createServer({
          configFile: false,
          envFile: false,
          root: project,
          logLevel: 'silent',
          cacheDir: path.join(root, 'cache-' + index),
          css: { devSourcemap: !!s.cssMap },
          optimizeDeps: { noDiscovery: true, include: [] },
          server: { middlewareMode: false, port: 0, hmr: false, watch: null, open: false, host: '127.0.0.1', proxy: {} }
        });
        try {
          reads.length = 0;
          const result = await server.transformRequest(
            '/' + path.relative(project, filename).split(path.sep).join('/') + (css && !s.injected ? '?direct' : '')
          );
          assert.match(result.code, css ? /color:\s*teal/ : /export const normal = 1/);
          await server.listen();
          const address = server.httpServer.address();
          const response = await fetch(
            'http://127.0.0.1:' +
              address.port +
              '/' +
              path.relative(project, filename).split(path.sep).join('/') +
              (css && !s.injected ? '?direct' : ''),
            { signal: AbortSignal.timeout(3000) }
          );
          const body = await response.text();
          assert.equal(response.status, 200);
          assert.ok(body.length < 500000);
          const maps = [
            ...body.matchAll(new RegExp('sourceMappingURL=data:application/json(?:;charset=utf-8)?;base64,([A-Za-z0-9+/=]+)', 'g'))
          ]
            .map((m) => Buffer.from(m[1], 'base64').toString('utf8'))
            .join('');
          const observed = {
            name: s.name,
            httpStatus: response.status,
            httpContainsMarker: maps.includes(marker),
            httpContainsNormal: maps.includes(normal),
            syntheticReads: reads.length,
            syntheticContentReturned: JSON.stringify(result.map).includes(marker),
            normalContentReturned: JSON.stringify(result.map).includes(normal)
          };
          t.diagnostic(JSON.stringify(observed));
          if (s.normal) {
            assert.equal(observed.normalContentReturned, true);
            assert.equal(observed.httpContainsNormal, true);
          } else {
            assert.equal(observed.syntheticReads, 0, 'must not read this test-owned file outside package');
            assert.equal(observed.syntheticContentReturned, false);
            assert.equal(observed.httpContainsMarker, false);
          }
        } finally {
          await server.close();
        }
      });
    }
  } finally {
    fsp.readFile = asyncRead;
    fs.readFileSync = syncRead;
    fs.rmSync(root, { recursive: true, force: true });
  }
});
