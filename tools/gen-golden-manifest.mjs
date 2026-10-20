#!/usr/bin/env node
// 从 testdata/golden/ 重建 manifest.json（真值只存输入与期望，K-08）。
//
// 用法：
//   node tools/gen-golden-manifest.mjs           写入
//   node tools/gen-golden-manifest.mjs --check   只比对（不一致 → exit 1）
// 退出码：0 一致/写入成功 · 1 不一致或空集 · 2 用法错误
//
// 纪律：本脚本**不调用 cfgdelta**，期望值是文法推导的结果（不是观测），
// 因此"引擎全坏"不会让它变绿。夹具增删必须同时改这里，防"各有件"被读成全穷举。

import { existsSync, readFileSync, readdirSync, statSync, writeFileSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const GOLDEN = join(ROOT, "testdata", "golden");
const MANIFEST = join(GOLDEN, "manifest.json");
const checkOnly = process.argv.includes("--check");
for (const a of process.argv.slice(2)) {
  if (a !== "--check") {
    process.stderr.write(`gen-golden-manifest: unknown argument ${a}\n`);
    process.exit(2);
  }
}


const cases = [];

function rel(p) {
  return relative(GOLDEN, p).split("\\").join("/");
}

function addCase(name, args, exit, behavioral, cosmetic, indeterminate = [], note = undefined) {
  const c = { case: name, args, exit, behavioral, cosmetic, indeterminate };
  if (note) c.note = note;
  cases.push(c);
}

// ── 测试A：K8s 默认值 + 数量单位（4 条误报 → 加 profile 后 0 条）────────────────
// 真值说明（由文法推导，不依赖本引擎）：
//   k2 相对 k1 的四处书写差异 = imagePullPolicy(默认值) + protocol(默认值)
//   + memory(1Gi vs 1024Mi) + cpu(500m vs 0.5)。
//   前两处是无中生有（added），后两处是归一后相等（cosmetic）。
//   无 profile 时：4 条发现里有 2 条被误判成行为差异（默认值那两条），4 条全部是书写层面。
addCase(
  "testA-no-profile",
  ["diff", "testA/k1.yaml", "testA/k2.yaml"],
  1,
  2,
  2,
  [],
  "without the defaults table the two defaulted keys are reported as behavioural findings",
);
addCase(
  "testA-profile",
  ["diff", "--profile", "k8s/v1.29", "testA/k1.yaml", "testA/k2.yaml"],
  0,
  0,
  2,
  [],
  "headline case: defaults cancel on both sides; only the two unit spellings remain (cosmetic)",
);

// ── 测试A 对照组：显式写了非默认值，必须仍报 1 条 ─────────────────────────────
addCase(
  "testA-ctrl-profile",
  ["diff", "--profile", "k8s/v1.29", "testA-ctrl/k1.yaml", "testA-ctrl/k2.yaml"],
  1,
  1,
  0,
  [],
  "anti-over-silencing control: explicit imagePullPolicy: Always is still reported",
);

// ── 测试B：Helm values 跨环境（ordered 下 env 顺序会成对错位；set/keyed 消除）──
addCase(
  "testB-ordered",
  ["diff", "testB/va.yaml", "testB/vb.yaml"],
  1,
  4,
  2,
  [],
  "ordered mode mismatches the two reordered env entries — expected, that is why set/keyed exist",
);
addCase(
  "testB-seq-set",
  ["diff", "--seq-mode", "set", "testB/va.yaml", "testB/vb.yaml"],
  0,
  0,
  2,
  [],
  "env ordering is irrelevant in set mode: 0 behaviour differences",
);
addCase(
  "testB-seq-keyed",
  ["diff", "--seq-mode", "keyed:name", "testB/va.yaml", "testB/vb.yaml"],
  0,
  0,
  2,
  [],
  "keyed:name aligns env entries by name: 0 behaviour differences",
);

// ── 测试C：单位族 7 组判等 + 反例 + 歧义 ────────────────────────────────────
// 七组的族：c01..c03 quantity，c04..c05 duration，c06..c07 decimal。
const cCases = [
  ["c01", "quantity", 0, 0, 1, [], "1Gi == 1024Mi (1024^3)"],
  ["c02", "quantity", 0, 0, 1, [], "1000M == 1G (10^9)"],
  ["c03", "quantity", 0, 0, 1, [], "8000m == 8"],
  ["c04", "duration", 0, 0, 1, [], "90s == 1m30s"],
  ["c05", "duration", 0, 0, 1, [], "1h == 60m"],
  ["c06", "decimal", 0, 0, 1, [], "1e3 == 1000 (exact rational, no float)"],
  ["c07", "decimal", 0, 0, 1, [], "1.50 == 1.5"],
];
for (const [id, fam, exit, behavioral, cosmetic, indeterminate, note] of cCases) {
  addCase(`testC-${id}`, ["diff", "--coerce-at", `limits/*=${fam}`, `testC/${id}-left.yaml`, `testC/${id}-right.yaml`], exit, behavioral, cosmetic, indeterminate, note);
}
addCase(
  "testC-c08-counterexample",
  ["diff", "--coerce-at", "limits/*=quantity", "testC/c08-left.yaml", "testC/c08-right.yaml"],
  1,
  1,
  0,
  [],
  "counterexample: 1Gi != 1000M (1024^3 != 1000^3) — must be reported",
);
addCase(
  "testC-c09-ambiguous",
  ["diff", "--coerce-at", "limits/*=quantity", "testC/c09-left.yaml", "testC/c09-right.yaml"],
  2,
  0,
  0,
  ["ambiguous_unit_family"],
  "MB vs MiB is indeterminate, never silently equal",
);

// ── 文法边界（§4.1 / §4.2 明写的形态；真值同样由文法推导）──────────────────
// 指数与后缀不可同现：`1e3Mi` 必须 exit 3（parse_error），不许降级成字符串比较。
addCase(
  "grammar-e3mi-invalid",
  ["diff", "--coerce-at", "limits/*=quantity", "testGrammar/e3mi-left.yaml", "testGrammar/e3mi-right.yaml"],
  3,
  0,
  0,
  [],
  "1e3Mi is illegal (exponent and suffix are mutually exclusive) -> parse error 3",
);
// 前导点：`.5` ≡ `0.5`（§4.1 number 允许 ". digits"）。
addCase(
  "grammar-leading-dot",
  ["diff", "--coerce-at", "limits/*=quantity", "testGrammar/frac-left.yaml", "testGrammar/frac-right.yaml"],
  0,
  0,
  1,
  [],
  ".5 == 0.5 (leading dot is part of the quantity grammar)",
);
// 显式正号：`+1` ≡ `1`（§4.3 保留符号位，+1 与 1 折叠）。
addCase(
  "grammar-plus-sign",
  ["diff", "--coerce-at", "limits/*=quantity", "testGrammar/plus-left.yaml", "testGrammar/plus-right.yaml"],
  0,
  0,
  1,
  [],
  "+1 == 1 (sign folds into the rational value)",
);
// 微秒两种拼写：`µs`（U+00B5）与 `us` 等值（§4.2 unit 表）。
addCase(
  "grammar-duration-micro",
  ["diff", "--coerce-at", "limits/*=duration", "testGrammar/micro-left.yaml", "testGrammar/micro-right.yaml"],
  0,
  0,
  1,
  [],
  "1µs == 1us (both spellings are in the duration unit table)",
);
// duration 唯一允许的无单位值：`0` ≡ `0s`（§4.2 特例）。
addCase(
  "grammar-duration-zero",
  ["diff", "--coerce-at", "limits/*=duration", "testGrammar/zero-left.yaml", "testGrammar/zero-right.yaml"],
  0,
  0,
  1,
  [],
  "0 == 0s (bare zero is the only unit-less duration)",
);

// ── 分类与门禁（五类各一例 + 选择性阻断）────────────────────────────────────
addCase(
  "classify-resource-shrink",
  ["classify", "testGate/shrink-left.yaml", "testGate/shrink-right.yaml"],
  1,
  2,
  0,
  [],
  "replicas 3->2 and memory 1Gi->512Mi are both 资源·收缩",
);
addCase(
  "gate-fail-on-shrink",
  ["gate", "--fail-on", "资源·收缩", "testGate/shrink-left.yaml", "testGate/shrink-right.yaml"],
  1,
  2,
  0,
  [],
  "gate blocks because the listed class is present",
);
addCase(
  "gate-ignore-shrink",
  ["gate", "--fail-on", "接口·暴露", "testGate/shrink-left.yaml", "testGate/shrink-right.yaml"],
  0,
  2,
  0,
  [],
  "gate does NOT block: unlisted behavioural differences are printed but do not fail the build",
);
addCase(
  "classify-exposure",
  ["classify", "testGate/exposure-left.yaml", "testGate/exposure-right.yaml"],
  1,
  2,
  0,
  [],
  "service type + port change are 接口·暴露",
);

// ── 测试D：非 YAML 格式（M5 适配器未落地 → 显式 skip）────────────────────────
addCase("testD-toml", ["diff", "testD/a.toml", "testD/b.toml"], 0, 0, 0, [], "pending: TOML adapter lands in M5");
addCase("testD-hcl", ["diff", "testD/a.hcl", "testD/b.hcl"], 0, 0, 0, [], "pending: HCL adapter lands in M5 (unevaluated expressions)");

// ── 退出码契约（参数化，逐位）────────────────────────────────────────────
addCase("exit-unknown-subcommand", ["nosuchcommand"], 3, 0, 0, [], "usage error is 3, never 2");
addCase("exit-missing-args", ["diff", "testA/k1.yaml"], 3, 0, 0, [], "missing positional is a usage error (3)");
addCase("exit-missing-file", ["diff", "testA/does-not-exist.yaml", "testA/k2.yaml"], 3, 0, 0, [], "missing input file is 3");
addCase("exit-invalid-yaml", ["diff", "testInvalid/bad-yaml.yaml", "testA/k2.yaml"], 3, 0, 0, [], "YAML syntax error is parse_error 3, never downgraded to a string comparison");
addCase("exit-unknown-profile", ["diff", "--profile", "k8s/v9.99", "testA/k1.yaml", "testA/k2.yaml"], 3, 0, 0, [], "unknown profile version is 3, no fallback");
addCase("exit-unknown-class", ["gate", "--fail-on", "not-a-class", "testA/k1.yaml", "testA/k2.yaml"], 3, 0, 0, [], "unknown impact class is 3");
addCase("exit-coerce-miss", ["diff", "--coerce-at", "nope/*=quantity", "testA/k1.yaml", "testA/k2.yaml"], 3, 0, 0, [], "unmatched --coerce-at glob is a usage error, never silent");
addCase("exit-gate-empty", ["gate", "testA/k1.yaml", "testA/k2.yaml"], 3, 0, 0, [], "gate without --fail-on is a usage error");
addCase("exit-version", ["--version"], 0, 0, 0, [], "version always exits 0");

// 夹具自证（双向，非空跑）：
//   ① manifest 引用的每个**夹具路径**必须真在盘上（排除开关值、glob、故意缺失的负例）；
//   ② 盘上每个夹具文件必须至少被一条用例引用（防"加了夹具却没人跑"）。
const INTENTIONALLY_MISSING = new Set(["testA/does-not-exist.yaml"]);
const isFixturePath = (a) =>
  !a.startsWith("-") &&
  !a.startsWith("/") &&
  !a.includes("=") &&
  /\.(ya?ml|json|toml|hcl|ini|env)$/.test(a);

const referenced = new Set();
const missing = [];
for (const c of cases) {
  for (const a of c.args) {
    if (!isFixturePath(a)) continue;
    referenced.add(a);
    if (INTENTIONALLY_MISSING.has(a)) continue;
    if (!existsSync(join(GOLDEN, a))) missing.push(`${c.case}: ${a}`);
  }
}
if (missing.length > 0) {
  process.stderr.write(`gen-golden-manifest: ${missing.length} referenced fixture(s) missing:\n  ${missing.join("\n  ")}\n`);
  process.exit(1);
}

const onDisk = [];
(function walk(dir) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p);
    else if (name !== "manifest.json") onDisk.push(rel(p));
  }
})(GOLDEN);

const orphans = onDisk.filter((f) => !referenced.has(f));
if (orphans.length > 0) {
  process.stderr.write(`gen-golden-manifest: ${orphans.length} fixture(s) on disk are never exercised:\n  ${orphans.join("\n  ")}\n`);
  process.exit(1);
}

const manifest = {
  generated_by: "tools/gen-golden-manifest.mjs",
  scope_note:
    "Covers the four red-teamed families (A/B/C/D), the 10 defaulting rules in data/profiles/k8s/v1.29.json, and the §4.1/§4.2 grammar boundaries. NOT full Kubernetes semantics.",
  cases: cases.sort((a, b) => (a.case < b.case ? -1 : a.case > b.case ? 1 : 0)),
};

const text = JSON.stringify(manifest, null, 2) + "\n";
if (checkOnly) {
  const current = existsSync(MANIFEST) ? readFileSync(MANIFEST, "utf8") : "";
  if (current !== text) {
    process.stdout.write("DIFF  testdata/golden/manifest.json is stale — re-run without --check\n");
    process.exit(1);
  }
  process.stdout.write(`gen-golden-manifest --check: ${cases.length} cases, manifest is up to date\n`);
  process.exit(0);
}
writeFileSync(MANIFEST, text);
process.stdout.write(`gen-golden-manifest: wrote ${cases.length} cases to testdata/golden/manifest.json\n`);
