#!/usr/bin/env node
// 版本号单一来源检查：仓库根 VERSION 与 internal/version/VERSION 必须逐字节一致。
// 用法：node tools/sync-version.mjs [--check]
// 退出码：0 一致/写入成功 · 1 不一致 · 2 用法错误

import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SRC = join(ROOT, "VERSION");
const DST = join(ROOT, "internal", "version", "VERSION");
const checkOnly = process.argv.includes("--check");
for (const a of process.argv.slice(2)) {
  if (a !== "--check") {
    process.stderr.write(`sync-version: unknown argument ${a}\n`);
    process.exit(2);
  }
}

if (!existsSync(SRC)) {
  process.stderr.write(`sync-version: VERSION not found at repository root\n`);
  process.exit(2);
}

const src = readFileSync(SRC, "utf8").trim();
if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(src)) {
  process.stderr.write(`sync-version: VERSION is not semver: ${JSON.stringify(src)}\n`);
  process.exit(1);
}

if (checkOnly) {
  if (!existsSync(DST)) {
    process.stdout.write(`MISSING  internal/version/VERSION\n`);
    process.exit(1);
  }
  const dst = readFileSync(DST, "utf8").trim();
  process.stdout.write(`sync-version --check: root=${src} mirror=${dst}\n`);
  process.exit(dst === src ? 0 : 1);
}

writeFileSync(DST, src + "\n");
process.stdout.write(`sync-version: ${src} → internal/version/VERSION\n`);
