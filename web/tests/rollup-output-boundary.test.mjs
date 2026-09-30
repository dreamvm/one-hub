import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { createRequire } from 'node:module';
import test from 'node:test';

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const viteRequire = createRequire(path.join(web, 'node_modules/vite/dist/node/index.js'));
const pkg = path.dirname(viteRequire.resolve('rollup/package.json'));
const { rollup, VERSION } = await import(pathToFileURL(path.join(pkg, 'dist/es/rollup.js')).href);
test('Rollup final output names stay inside their output directory', { timeout: 30000 }, async (t) => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'onehub-rollup-draft-')));
  t.diagnostic(`top-level Vite uses Rollup ${VERSION}`);
  try {
    for (const api of ['generate', 'write']) {
      for (const kind of [
        'input',
        'manual-object',
        'manual-function',
        'asset',
        'entry-name',
        'bundle-key',
        'nested-input',
        'normalized-input',
        'nested-asset',
        'normal-chunk-map'
      ]) {
        await t.test(`${api}: ${kind}`, async () => {
          const outside = 'stage/../../synthetic-outside';
          const ordinary = ['nested-input', 'normalized-input', 'nested-asset', 'normal-chunk-map'].includes(kind);
          const fixture = path.join(root, api, kind);
          const output = path.join(fixture, 'output');
          const bundle = await rollup({
            input: {
              [kind === 'input'
                ? outside
                : kind === 'nested-input'
                  ? 'nested/inside'
                  : kind === 'normalized-input'
                    ? 'nested/../inside'
                    : 'main']: 'virtual-entry'
            },
            plugins: [
              {
                name: 'onehub-synthetic-fixture',
                resolveId(id) {
                  if (id.startsWith('virtual-')) return id;
                },
                load(id) {
                  if (id === 'virtual-entry')
                    return 'import { value } from "virtual-dep"; console.log(value); export const later = () => import("virtual-lazy");';
                  if (id === 'virtual-dep') return 'export const value = 42;';
                  if (id === 'virtual-lazy') return 'export const message = "synthetic fixture";';
                },
                generateBundle(_options, assets) {
                  if (kind === 'asset' || kind === 'nested-asset')
                    this.emitFile({
                      type: 'asset',
                      fileName: kind === 'asset' ? `${outside}.txt` : 'nested/inside.txt',
                      source: 'onehub synthetic asset'
                    });
                  const [key, entry] = Object.entries(assets).find(([, item]) => item.type === 'chunk' && item.isEntry);
                  if (kind === 'entry-name') entry.fileName = `${outside}.js`;
                  if (kind === 'bundle-key') {
                    delete assets[key];
                    assets[`${outside}.js`] = entry;
                  }
                }
              }
            ]
          });
          let error, result;
          try {
            result = await bundle[api]({
              dir: output,
              format: 'es',
              entryFileNames: '[name].js',
              chunkFileNames: '[name].js',
              sourcemap: true,
              ...(kind === 'manual-object'
                ? { manualChunks: { [outside]: ['virtual-dep'] } }
                : kind === 'manual-function'
                  ? { manualChunks: (id) => (id === 'virtual-dep' ? outside : undefined) }
                  : {})
            });
          } catch (e) {
            error = e;
          } finally {
            await bundle.close();
          }
          const escaped =
            fs.existsSync(path.join(fixture, 'synthetic-outside.js')) || fs.existsSync(path.join(fixture, 'synthetic-outside.txt'));
          if (ordinary) {
            assert.ifError(error);
            assert.ok(result.output.some((item) => item.type === 'chunk' && item.isEntry));
            assert.ok(result.output.some((item) => item.type === 'chunk' && item.code.includes('42')));
            assert.ok(result.output.some((item) => item.type === 'chunk' && item.code.includes('synthetic fixture')));
            const maps = result.output.filter((item) => item.type === 'asset' && item.fileName.endsWith('.map'));
            assert.ok(maps.length > 0);
            assert.ok(maps.some((item) => JSON.parse(item.source).sourcesContent.some((content) => content?.includes('42'))));
            if (kind === 'nested-asset')
              assert.equal(result.output.find((item) => item.fileName === 'nested/inside.txt')?.source, 'onehub synthetic asset');
            if (kind === 'nested-input' || kind === 'normalized-input') {
              const entry = result.output.find((item) => item.type === 'chunk' && item.isEntry);
              assert.equal(
                path.resolve(output, entry.fileName),
                path.resolve(output, kind === 'nested-input' ? 'nested/inside.js' : 'inside.js')
              );
            }
            if (api === 'write') {
              for (const item of result.output)
                assert.equal(fs.readFileSync(path.resolve(output, item.fileName), 'utf8'), item.type === 'chunk' ? item.code : item.source);
            }
            assert.equal(escaped, false);
          } else {
            assert.equal(error?.code, 'FILE_NAME_OUTSIDE_OUTPUT_DIRECTORY', 'final output validation must reject unsafe names');
            assert.equal(escaped, false, 'no synthetic file may be written outside the output directory');
          }
        });
      }
    }
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
});
