import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import test from 'node:test';

const web = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const rootRequire = createRequire(path.join(web, 'package.json'));
const routes = [
  ['Sass', ['sass']],
  ['Vite/PostCSS', ['vite', 'postcss']],
  ['Vitest/Vite/PostCSS', ['vitest', 'vite', 'postcss']]
];
const flatMap = () => ({ version: 3, sources: ['original.scss'], sourcesContent: ['a {}'], names: [], mappings: 'AAAA' });
const indexed = (line = 0, column = 0, map = flatMap()) => ({ version: 3, sections: [{ offset: { line, column }, map }] });

// The optional root selects an isolated old package for the exact same assertions.
// Large offset cases only construct a consumer: never iterate or serialize them.
// All executed mapping work uses at most 64 lines and three nested sections.
for (const [label, parents] of routes) {
  let require = rootRequire;
  for (const parent of parents) require = createRequire(require.resolve(parent));
  const pkg = process.env.ONEHUB_SOURCE_MAP_JS_ROOT
    ? path.join(path.resolve(process.env.ONEHUB_SOURCE_MAP_JS_ROOT), 'package.json')
    : require.resolve('source-map-js/package.json');
  const metadata = JSON.parse(readFileSync(pkg, 'utf8'));
  assert.equal(metadata.name, 'source-map-js');
  const { SourceMapConsumer, SourceMapGenerator, SourceNode } = rootRequire(path.dirname(pkg));

  test(`${label}: source-map-js ${metadata.version} indexed map boundary`, { timeout: 5000 }, async (t) => {
    for (const field of ['line', 'column']) {
      await t.test(`rejects non-integer and non-finite ${field} before mapping work`, () => {
        for (const value of [-1, 1.5, Infinity, NaN, '1', null]) {
          const map = indexed();
          map.sections[0].offset[field] = value;
          assert.throws(() => new SourceMapConsumer(map), /non-negative integers/);
        }
      });
    }
    await t.test('rejects a line beyond the upstream bound without serializing it', () => {
      assert.throws(() => new SourceMapConsumer(indexed(10000001)), /must not exceed/);
    });
    await t.test('rejects accumulated nested offsets without iterating them', () => {
      assert.throws(() => new SourceMapConsumer(indexed(6000000, 0, indexed(4000001))), /including offsets of nested sections/);
    });
    await t.test('nested sources are read once at each level', () => {
      const consumer = new SourceMapConsumer(indexed(0, 0, indexed(0, 0, indexed())));
      const leaf = consumer._sections[0].consumer._sections[0].consumer._sections[0].consumer;
      const sources = leaf.sources;
      let reads = 0;
      Object.defineProperty(leaf, 'sources', { get() { reads += 1; return sources; } });
      assert.deepEqual(consumer.sources, ['original.scss']);
      assert.equal(reads, 1, 'tiny nested map must not multiply reads of the same source list');
    });
    await t.test('SourceNode preserves short code when a valid mapping lies past its end', () => {
      const code = 'a\nb\n';
      const node = SourceNode.fromStringWithSourceMap(code, new SourceMapConsumer(indexed(64)));
      assert.equal(node.toString(), code);
    });
    await t.test('flat and ordered nested maps retain mappings and source content', () => {
      for (const map of [flatMap(), indexed(2, 0, indexed(1))]) {
        const consumer = new SourceMapConsumer(map);
        assert.deepEqual(consumer.sources, ['original.scss']);
        assert.equal(consumer.sourceContentFor('original.scss'), 'a {}');
        const mappings = [];
        consumer.eachMapping((mapping) => mappings.push(mapping));
        assert.equal(mappings.length, 1);
        assert.equal(mappings[0].generatedLine, map.sections ? 4 : 1);
        if (!map.sections) {
          const output = SourceMapGenerator.fromSourceMap(consumer).toJSON();
          assert.equal(new SourceMapConsumer(output).sourceContentFor('original.scss'), 'a {}');
        }
      }
    });
  });
}

// Exercise PostCSS's actual input boundary independently of the library APIs.
const postcssRequire = createRequire(rootRequire.resolve('postcss'));
const Input = postcssRequire('./input');
test('PostCSS parses inline indexed maps at its real input boundary', { timeout: 5000 }, async (t) => {
  for (const encoding of ['base64', 'uri']) {
    const css = (map) => {
      const text = JSON.stringify(map);
      const url = encoding === 'base64'
        ? `data:application/json;base64,${Buffer.from(text).toString('base64')}`
        : `data:application/json,${encodeURIComponent(text)}`;
      return `a {}\n/*# sourceMappingURL=${url} */`;
    };
    await t.test(`${encoding}: invalid offset rejected before further map processing`, () => {
      assert.throws(() => new Input(css(indexed(1.5)), { from: '/synthetic/style.css' }), /non-negative integers/);
    });
    await t.test(`${encoding}: ordinary indexed source content preserved`, () => {
      const input = new Input(css(indexed()), { from: '/synthetic/style.css' });
      assert.equal(input.map.consumer().sourceContentFor('original.scss'), 'a {}');
    });
  }
});
