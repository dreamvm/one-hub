import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const filename = fileURLToPath(import.meta.url);
const require = createRequire(import.meta.url);
const globRequire = createRequire(require.resolve("glob"));
const minimatch9Require = createRequire(globRequire.resolve("minimatch"));
const entries = [
  {
    major: 1,
    expand: require("brace-expansion"),
    version: require("brace-expansion/package.json").version,
  },
  {
    major: 2,
    expand: minimatch9Require("brace-expansion"),
    version: minimatch9Require("brace-expansion/package.json").version,
  },
];
const cases = entries.flatMap(({ major }) =>
  [
    "count-budget",
    "length-budget",
    "depth-budget",
    "rewrite-budget",
    "normal-grammar",
    "actual-consumer",
  ].map((kind) => ({ major, kind })),
);

function runCase(selected) {
  const { expand } = entries.find((entry) => entry.major === selected.major);
  if (selected.kind === "count-budget") {
    const result = expand("{one,two,three}", { max: 2 });
    assert.deepEqual(result, ["one", "two"]);
  } else if (selected.kind === "length-budget") {
    const result = expand("{one,two}{three,four}", { maxLength: 10 });
    assert.ok(
      result.length > 0,
      "legitimate results within the budget must remain",
    );
    assert.ok(result.reduce((sum, value) => sum + value.length, 0) <= 10);
  } else if (selected.kind === "depth-budget") {
    assert.deepEqual(expand("x{a,{b,c}}", { maxDepth: 0 }), ["xa", "x{b,c}"]);
  } else if (selected.kind === "rewrite-budget") {
    assert.deepEqual(expand("{a},}", { maxRewrites: 0 }), ["{a},}"]);
  } else if (selected.kind === "normal-grammar") {
    for (const [input, expected] of [
      [
        "src/*.{js,jsx,ts,tsx}",
        ["src/*.js", "src/*.jsx", "src/*.ts", "src/*.tsx"],
      ],
      ["{01..03}", ["01", "02", "03"]],
      ["{3..1}", ["3", "2", "1"]],
      ["{a..c}", ["a", "b", "c"]],
      ["{one,{two,three}}", ["one", "two", "three"]],
      ["{a,b}{c,d}", ["ac", "ad", "bc", "bd"]],
      ["x\\{one,two\\}", ["x{one,two}"]],
      ["src/{one", ["src/{one"]],
      ["{one}", ["{one}"]],
      ["", []],
    ])
      assert.deepEqual(expand(input), expected, input);
  } else if (selected.kind === "actual-consumer") {
    const pattern = "src/**/*.{js,jsx,ts,tsx}";
    if (selected.major === 1) {
      const matcher = require("minimatch");
      const matcherRequire = createRequire(require.resolve("minimatch"));
      assert.equal(
        matcherRequire.resolve("brace-expansion"),
        require.resolve("brace-expansion"),
      );
      for (const ext of ["js", "jsx", "ts", "tsx"])
        assert.equal(matcher("src/one." + ext, pattern), true);
      assert.equal(matcher("src/one.css", pattern), false);
      assert.equal(matcher("src/.hidden.js", pattern), false);
    } else {
      const glob = require("glob");
      const cwd = fs.mkdtempSync(path.join(os.tmpdir(), "onehub-brace-glob-"));
      try {
        fs.mkdirSync(path.join(cwd, "src"));
        for (const ext of ["js", "jsx", "ts", "tsx", "css"])
          fs.writeFileSync(path.join(cwd, "src", "one." + ext), "synthetic");
        fs.writeFileSync(path.join(cwd, "src", ".hidden.js"), "synthetic");
        assert.deepEqual(glob.sync(pattern, { cwd }).sort(), [
          "src/one.js",
          "src/one.jsx",
          "src/one.ts",
          "src/one.tsx",
        ]);
      } finally {
        fs.rmSync(cwd, { recursive: true, force: true });
      }
    }
  } else {
    throw new Error("unknown fixed synthetic case");
  }
}

if (process.env.ONEHUB_BRACE_CASE !== undefined) {
  const selected = cases[Number(process.env.ONEHUB_BRACE_CASE)];
  assert.ok(selected);
  runCase(selected);
  process.stdout.write(JSON.stringify({ passed: true }));
} else {
  test(
    "legacy brace branches: bounded expansion and maintained file patterns",
    { timeout: 70000 },
    async (t) => {
      t.diagnostic(entries.map(({ version }) => version).join(", "));
      for (let index = 0; index < cases.length; index += 1) {
        const selected = cases[index];
        await t.test("v" + selected.major + " " + selected.kind, () => {
          const child = spawnSync(
            process.execPath,
            ["--max-old-space-size=192", filename],
            {
              env: {
                PATH: process.env.PATH,
                LANG: "C.UTF-8",
                ONEHUB_BRACE_CASE: String(index),
              },
              timeout: 5000,
              maxBuffer: 32768,
              encoding: "utf8",
            },
          );
          assert.ifError(child.error);
          assert.equal(child.signal, null);
          assert.equal(child.status, 0, child.stderr || child.stdout);
          assert.equal(JSON.parse(child.stdout).passed, true);
        });
      }
    },
  );
}
