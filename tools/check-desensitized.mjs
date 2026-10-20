#!/usr/bin/env node
// 脱敏门：发布树里不得出现会话绝对路径、本机用户名、本地回环代理。
// 用法：node tools/check-desensitized.mjs
// 退出码：0 零命中 / 1 有命中 / 2 用法错误
//
// 为什么这是"门"而不是一次性 grep：
//   ① 发布前必须 0 命中，发布后任何一次改动都可能把本机路径带回来；
//   ② 判据必须可跑红——命中时逐条打印 `文件:行:内容`，不静默通过。
//
// 变异验证（改本文件前先看）：
//   在某份文档里写一行 `D:/ProgramData/...` 或 `C:\Users\<真实用户名>\...`
//   或 `127.0.0.1:7897` → 本门 exit 1 并点名该行；删掉该行 → 复绿。
//
// 扫描面：仓库内文本文件，跳过 `.git/` `.tmp/` `dist/` `node_modules/` 与二进制。
// 公开镜像域名（如 goproxy.cn）不是私有代理，不在判据内。
//
// 自豁免：本文件必须写出被判据禁止的字面量（规则本身），因此显式跳过自己。
// 这不是漏洞——其它任何文件都不豁免，变异验证就在别的文件上做。

import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SELF = "tools/check-desensitized.mjs";
const SKIP_DIRS = new Set([".git", ".tmp", "dist", "dist-release", "node_modules", ".cache"]);
const SKIP_EXTS = new Set([".png", ".jpg", ".jpeg", ".gif", ".zip", ".gz", ".exe"]);

const RULES = [
  {
    id: "absolute-session-path",
    // D:/… 或 D:\… 形式的盘符路径；`<repo>` / `<user>` 这类占位符放行。
    re: /\b[A-Za-z]:[\\/](?:ProgramData|Users|zcode|repos?)[\\/](?!<)/,
  },
  { id: "msys-user-path", re: /\/[a-z]\/Users\/[A-Za-z0-9._-]+\// },
  { id: "host-username", re: /\bASUS\b/ },
  { id: "loopback-proxy", re: /\b127\.0\.0\.1:\d{2,5}\b/ },
];

for (const a of process.argv.slice(2)) {
  process.stderr.write(`check-desensitized: unknown argument ${a}\n`);
  process.exit(2);
}

function* walk(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (SKIP_DIRS.has(entry.name)) continue;
      yield* walk(join(dir, entry.name));
      continue;
    }
    if (!entry.isFile()) continue;
    const name = entry.name;
    if (SKIP_EXTS.has(name.slice(name.lastIndexOf(".")))) continue;
    yield join(dir, name);
  }
}

const hits = [];
let scanned = 0;
for (const file of walk(ROOT)) {
  const rel = relative(ROOT, file).split(sep).join("/");
  if (rel === SELF) continue;
  let text;
  try {
    text = readFileSync(file, "utf8");
  } catch {
    continue; // 非 UTF-8（二进制）直接跳过
  }
  if (text.includes("\u0000")) continue;
  scanned++;
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    for (const rule of RULES) {
      if (rule.re.test(lines[i])) {
        hits.push(`${rel}:${i + 1}: [${rule.id}] ${lines[i].trim().slice(0, 160)}`);
      }
    }
  }
}

if (hits.length > 0) {
  process.stdout.write(`脱敏门：${hits.length} 处命中（扫了 ${scanned} 个文本文件）\n`);
  for (const h of hits) process.stdout.write(`FAIL  ${h}\n`);
  process.exit(1);
}
process.stdout.write(`脱敏门：扫了 ${scanned} 个文本文件，0 命中\n`);
