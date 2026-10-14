import assert from "node:assert/strict";
import crypto from "node:crypto";
import path from "node:path";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

const filename = fileURLToPath(import.meta.url);
const web = path.resolve(path.dirname(filename), "..");
const require = createRequire(import.meta.url);
// This is the affected type-dependency copy, not the already patched Axios copy.
const fetchRequire = createRequire(
  path.join(
    path.dirname(require.resolve("@types/node-fetch/package.json")),
    "package.json",
  ),
);
const FormData = fetchRequire("form-data");
const cases = [
  { kind: "boundary" },
  ...["\r", "\n", "\r\n", '"'].map((separator) => ({
    kind: "field",
    separator,
  })),
  ...[
    "filename-string",
    "filename-option",
    "filepath",
    "inferred-name",
    "inferred-path",
  ].map((api) => ({ kind: "filename", api })),
  { kind: "normal-body" },
  { kind: "explicit-boundary" },
];

function runCase(selected) {
  if (selected.kind === "boundary") {
    const savedCrypto = crypto.randomBytes;
    const savedMath = Math.random;
    let cryptoCalls = 0;
    let mathCalls = 0;
    crypto.randomBytes = (size) => {
      cryptoCalls += 1;
      assert.equal(size, 12);
      return Buffer.alloc(size, 0x7f);
    };
    Math.random = () => {
      mathCalls += 1;
      return 0;
    };
    try {
      const form = new FormData();
      const first = form.getBoundary();
      assert.equal(first, "--------------------------" + "7f".repeat(12));
      assert.equal(form.getBoundary(), first);
      assert.equal(cryptoCalls, 1);
      assert.equal(mathCalls, 0);
    } finally {
      crypto.randomBytes = savedCrypto;
      Math.random = savedMath;
    }
  } else if (selected.kind === "field") {
    const form = new FormData();
    form.append("onehub" + selected.separator + "label", "synthetic");
    const body = form.getBuffer().toString("utf8");
    const escaped = selected.separator
      .replace(/\r/g, "%0D")
      .replace(/\n/g, "%0A")
      .replace(/"/g, "%22");
    assert.ok(body.includes('name="onehub' + escaped + 'label"'));
  } else if (selected.kind === "filename") {
    const form = new FormData();
    const name = 'onehub\r\n"file.txt';
    const value = Buffer.from("synthetic");
    let options;
    if (selected.api === "filename-string") options = name;
    if (selected.api === "filename-option") options = { filename: name };
    if (selected.api === "filepath") options = { filepath: "relative/" + name };
    if (selected.api === "inferred-name") value.name = name;
    if (selected.api === "inferred-path") value.path = "relative/" + name;
    form.append("file", value, options);
    const body = form.getBuffer().toString("utf8");
    const expected =
      (selected.api === "filepath" ? "relative/" : "") +
      "onehub%0D%0A%22file.txt";
    assert.ok(body.includes('filename="' + expected + '"'));
    assert.equal(body.includes('\r\n"file.txt'), false);
  } else if (selected.kind === "normal-body") {
    const form = new FormData();
    form.append("title", "合成配置");
    form.append("count", 2);
    form.append("file", Buffer.from([0, 1, 255]), {
      filename: "onehub.txt",
      contentType: "application/octet-stream",
    });
    const body = form.getBuffer();
    assert.equal(body.length, form.getLengthSync());
    assert.ok(body.includes(Buffer.from("合成配置")));
    assert.ok(body.includes(Buffer.from([0, 1, 255])));
    assert.ok(body.toString("utf8").includes('filename="onehub.txt"'));
    assert.equal(
      form.getHeaders()["content-type"],
      "multipart/form-data; boundary=" + form.getBoundary(),
    );
  } else if (selected.kind === "explicit-boundary") {
    const form = new FormData();
    form.setBoundary("onehubSyntheticBoundary");
    form.append("title", "synthetic", {
      header: { "X-Onehub-Control": "synthetic" },
    });
    assert.equal(form.getBoundary(), "onehubSyntheticBoundary");
    assert.ok(
      form.getBuffer().toString("utf8").includes("X-Onehub-Control: synthetic"),
    );
  } else {
    throw new Error("unknown fixed synthetic case");
  }
}

if (process.env.ONEHUB_FORM_DATA_CASE !== undefined) {
  const selected = cases[Number(process.env.ONEHUB_FORM_DATA_CASE)];
  assert.ok(selected);
  runCase(selected);
  process.stdout.write(
    JSON.stringify({
      version: fetchRequire("form-data/package.json").version,
      passed: true,
    }),
  );
} else {
  test(
    "affected form-data copy: entropy, header parameters and legitimate multipart",
    { timeout: 60000 },
    async (t) => {
      t.diagnostic(
        "form-data via @types/node-fetch " +
          fetchRequire("form-data/package.json").version,
      );
      for (let index = 0; index < cases.length; index += 1) {
        const selected = cases[index];
        await t.test(
          selected.kind +
            " " +
            (selected.api || JSON.stringify(selected.separator) || ""),
          () => {
            const child = spawnSync(
              process.execPath,
              ["--max-old-space-size=192", filename],
              {
                env: {
                  PATH: process.env.PATH,
                  LANG: "C.UTF-8",
                  ONEHUB_FORM_DATA_CASE: String(index),
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
          },
        );
      }
    },
  );
}
