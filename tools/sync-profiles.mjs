#!/usr/bin/env node
// 把 data/profiles/ 逐字节镜像进 internal/profile/profiles/（go:embed 只能向下）。
//
// 单点可编辑源 = data/profiles/；本脚本只做复制与比对，不参与判定。
// 用法：
//   node tools/sync-profiles.mjs          写入镜像
//   node tools/sync-profiles.mjs --check  只比对（不一致 → exit 1）
// 退出码：0 一致/写入成功 · 1 不一致 · 2 用法错误

import { existsSync, mkdirSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SRC = join(ROOT, "data", "profiles");
const DST = join(ROOT, "internal", "profile", "profiles");
const checkOnly = process.argv.includes("--check");
for (const a of process.argv.slice(2)) {
  if (!["--check"].includes(a)) {
    process.stderr.write(`sync-profiles: unknown argument ${a}\n`);
    process.exit(2);
  }
}

function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) out.push(...walk(p));
    else if (name.endsWith(".json")) out.push(p);
  }
  return out;
}

if (!existsSync(SRC)) {
  process.stderr.write(`sync-profiles: source not found: ${SRC}\n`);
  process.exit(2);
}

const files = walk(SRC).sort();
if (files.length === 0) {
  process.stderr.write("sync-profiles: 0 profiles found — refusing to mirror an empty set\n");
  process.exit(1);
}

let drift = 0;
for (const src of files) {
  const rel = relative(SRC, src);
  const dst = join(DST, rel);
  const srcBytes = readFileSync(src);
  if (checkOnly) {
    if (!existsSync(dst)) {
      process.stdout.write(`MISSING  ${rel}\n`);
      drift++;
      continue;
    }
    if (!srcBytes.equals(readFileSync(dst))) {
      process.stdout.write(`DIFF     ${rel}\n`);
      drift++;
    }
    continue;
  }
  mkdirSync(dirname(dst), { recursive: true });
  writeFileSync(dst, srcBytes);
  process.stdout.write(`synced   ${rel}\n`);
}

if (checkOnly) {
  process.stdout.write(`sync-profiles --check: ${files.length} profile(s) scanned, ${drift} drift\n`);
  process.exit(drift === 0 ? 0 : 1);
}
process.stdout.write(`sync-profiles: ${files.length} profile(s) mirrored into internal/profile/profiles/\n`);
