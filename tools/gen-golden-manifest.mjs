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

function addCase(name, args, exit, behavioral, cosmetic, indeterminate = [], note = undefined, contains = []) {
  const c = { case: name, args, exit, behavioral, cosmetic, indeterminate };
  if (contains.length) c.contains = contains;
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

// ── M3 结构无关比较：四类噪声 / 序列三模式 / 路径形态 ────────────────────────
// 真值由文法推导：这四类差异只影响书写，不改变任何标量的 Raw 与 tag。
const noiseCases = [
  ["keys", "mapping key order is irrelevant: keys compare in UTF-8 byte order"],
  ["indent", "block indentation is presentation only"],
  [
    "quotes",
    "quote style is presentation only (plain / double / single all carry the same Raw and tag)",
  ],
  ["comments", "comments are dropped at parse time and never compared"],
];
for (const [suffix, note] of noiseCases) {
  // 注释用例额外断言头部声明，避免"没比注释"这件事只存在于文档里。
  const contains = suffix === "comments" ? ["comments: not compared"] : [];
  addCase(
    `testM3-noise-${suffix}`,
    ["diff", "testM3/noise-base-left.yaml", `testM3/noise-${suffix}-right.yaml`],
    0,
    0,
    0,
    [],
    note,
    contains,
  );
}

// set：顺序无关；重复元素是一等 indeterminate（不静默去重）。
addCase(
  "testM3-set-reorder",
  ["diff", "--seq-mode", "set", "testM3/set-reorder-left.yaml", "testM3/set-reorder-right.yaml"],
  0, 0, 0, [],
  "set membership ignores order: 0 entries",
);
addCase(
  "testM3-set-duplicate",
  ["diff", "--seq-mode", "set", "testM3/set-reorder-left.yaml", "testM3/set-dup-right.yaml"],
  2, 0, 0,
  ["duplicate_in_set"],
  "duplicate set elements are indeterminate, never silently deduped",
  ["indeterminate(duplicate_in_set)"],
);
// set 成员没有稳定下标，路径形态 = pointer[quoted canonical key]（成员标识，非 JSON Pointer）。
addCase(
  "testM3-set-member-path",
  ["diff", "--seq-mode", "set", "testM3/set-reorder-left.yaml", "testM3/set-add-right.yaml"],
  1, 1, 0, [],
  "set member identity is pointer[quoted canonical key], not a JSON Pointer segment",
  ['/items["string:charlie"]'],
);

// keyed: 按字段对齐（元素顺序无关）；重复键 = exit 3 且输出带定位路径。
addCase(
  "testM3-keyed-order",
  ["diff", "--seq-mode", "keyed:name", "testM3/keyed-order-left.yaml", "testM3/keyed-order-right.yaml"],
  0, 0, 0, [],
  "keyed:name aligns by field value: element order is irrelevant",
);
addCase(
  "testM3-keyed-duplicate",
  ["diff", "--seq-mode", "keyed:name", "testM3/keyed-dup-left.yaml", "testM3/keyed-dup-right.yaml"],
  3, 0, 0, [],
  "duplicate keyed field is exit 3 (never first-wins) and names the offending path",
  ['duplicate key "alpha"', "at /items/1"],
);

// RFC 6901：键内 `/` 与 `~` 必须转义成 `~1` / `~0`。
addCase(
  "testM3-rfc6901-escape",
  ["diff", "testM3/escape-left.yaml", "testM3/escape-right.yaml"],
  1, 2, 0, [],
  "keys containing / and ~ are escaped per RFC 6901",
  ["~ /a~1b", "~ /c~0d"],
);

// ── M4 默认值：10 条规则逐条 + 未命中降级 ──────────────────────────────────
// 真值由 §4.11 推导，不依赖本引擎：
//   ① 一侧缺键、另一侧**显式写默认值** → 双侧施加后相同 → 0 条（抵消）；
//   ② 另一侧**显式写非默认值** → 只有该键不等 → 恰好 1 条行为差异（证明没有过度静默）。
// ①与②必须成对：单看①，"引擎什么都不做 + 夹具写错"也能凑出 0 条；成对后两者
// 互为反证——不做默认值则①变红，覆盖显式值则②变红。
const ruleCases = [
  ["01", "/spec/template/spec/containers/0/imagePullPolicy", "string:IfNotPresent -> string:Always", "imagePullPolicy (conditional on a non-latest image tag)"],
  ["02", "/spec/template/spec/containers/0/ports/0/protocol", "string:TCP -> string:UDP", "container port protocol under a Deployment template"],
  ["03", "/spec/containers/0/ports/0/protocol", "string:TCP -> string:UDP", "container port protocol on a bare Pod"],
  ["04", "/spec/template/spec/terminationGracePeriodSeconds", "decimal:30/1 -> decimal:60/1", "terminationGracePeriodSeconds"],
  ["05", "/spec/template/spec/restartPolicy", "string:Always -> string:Never", "restartPolicy"],
  ["06", "/spec/template/spec/dnsPolicy", "string:ClusterFirst -> string:Default", "dnsPolicy"],
  ["07", "/spec/template/spec/serviceAccountName", "string:default -> string:build-bot", "serviceAccountName"],
  ["08", "/spec/revisionHistoryLimit", "decimal:10/1 -> decimal:3/1", "revisionHistoryLimit"],
  ["09", "/spec/strategy/type", "string:RollingUpdate -> string:Recreate", "strategy.type"],
  ["10", "/spec/progressDeadlineSeconds", "decimal:600/1 -> decimal:120/1", "progressDeadlineSeconds"],
];
for (const [id, path, transition, note] of ruleCases) {
  addCase(
    `testM4-rule${id}-cancel`,
    ["diff", "--profile", "k8s/v1.29", `testM4/rule${id}-left.yaml`, `testM4/rule${id}-default-right.yaml`],
    0, 0, 0, [],
    `rule ${id} (${note}): an omitted key cancels against an explicit default value`,
  );
  addCase(
    `testM4-rule${id}-nondefault`,
    ["diff", "--profile", "k8s/v1.29", `testM4/rule${id}-left.yaml`, `testM4/rule${id}-nondefault-right.yaml`],
    1, 1, 0, [],
    `rule ${id}: an explicit non-default value is still reported (no over-silencing)`,
    [path, transition],
  );
}
// 未命中任何规则 → 头部必须写明降级，且 hits 为 0；绝不静默假装已比较（K-09）。
addCase(
  "testM4-degraded-header",
  ["diff", "--profile", "k8s/v1.29", "testM4/degraded-left.yaml", "testM4/degraded-right.yaml"],
  1, 1, 0, [],
  "a ConfigMap matches no rule: header announces degraded: structural-only with hits: 0",
  ["degraded: structural-only", "hits: 0"],
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
  ["接口·暴露"],
);

// ── M5 分类门禁：六类各一例 + 镜像 semver 四例可见 ──────────────────────────
// 真值由 data/README.md §4.13 的分类表推导（资源·收缩 / 接口·暴露已有端到端用例）。
addCase(
  "classify-deploy-shrink",
  ["classify", "testGate/deploy-shrink-left.yaml", "testGate/deploy-shrink-right.yaml"],
  1, 1, 0, [],
  "minReadySeconds 0 -> 30 weakens the rollout: 部署·收缩",
  ["部署·收缩"],
);
addCase(
  "classify-config-remove",
  ["classify", "testGate/config-remove-left.yaml", "testGate/config-remove-right.yaml"],
  1, 1, 0, [],
  "removing a ConfigMap data key is 配置·新增·移除",
  ["配置·新增·移除"],
);
addCase(
  "classify-cosmetic",
  ["classify", "testGate/cosmetic-left.yaml", "testGate/cosmetic-right.yaml"],
  0, 0, 1, [],
  "1Gi vs 1024Mi is pure spelling: counted as cosmetic, never listed, exit 0",
  ["cosmetic 1"],
);
// semver 四例：major / minor / patch / non-semver 必须出现在 sub_class 输出面。
const semverCases = [
  ["major", "1.2.3", "2.0.0"],
  ["minor", "1.2.3", "1.3.0"],
  ["patch", "1.2.3", "1.2.4"],
  ["nonsemver", "latest", "1.2.4"],
];
for (const [id, l, r] of semverCases) {
  const sub = id === "nonsemver" ? "non-semver" : id;
  addCase(
    `classify-image-${id}`,
    ["classify", `testGate/image-${id}-left.yaml`, `testGate/image-${id}-right.yaml`],
    1, 1, 0, [],
    `image ${l} -> ${r} is 镜像·变更/${sub}`,
    ["镜像·变更/" + sub],
  );
}

// ── 测试D：非 YAML 格式（M5 适配器落地；按扩展名显式分派）───────────────────
addCase(
  "testD-toml",
  ["diff", "testD/a.toml", "testD/b.toml"],
  0, 0, 0, [],
  "TOML: [section] is a table, key order is irrelevant (0 entries)",
);
addCase(
  "testD-hcl",
  ["diff", "testD/a.hcl", "testD/b.hcl"],
  0, 0, 0, [],
  "HCL: attribute order is irrelevant inside a block (0 entries)",
);
addCase(
  "testD-hcl-unevaluated",
  ["diff", "testD/a-expr.hcl", "testD/b-expr.hcl"],
  2, 0, 0,
  ["unevaluated_expression"],
  "HCL expressions are never evaluated: differing expressions are indeterminate, with the literal text visible",
  ["indeterminate(unevaluated_expression)", "var.replicas -> var.nodes"],
);
addCase(
  "testD-ini",
  ["diff", "testD/a.ini", "testD/b.ini"],
  0, 0, 0, [],
  "INI: section key order is irrelevant; values stay strings (no type guessing)",
);
addCase(
  "testD-env",
  ["diff", "testD/a.env", "testD/b.env"],
  0, 0, 0, [],
  ".env: key order is irrelevant; quotes follow .env rules, not YAML's",
);

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
    "Covers the four red-teamed families (A/B/C/D), every one of the 10 defaulting rules in data/profiles/k8s/v1.29.json (each rule has a cancel case plus an explicit non-default control), the unmatched-profile degraded header, the §4.1/§4.2 grammar boundaries, the M3 structural-comparison cases (four noise classes / sequence modes / RFC 6901 paths), the M5 impact classes (six classes end-to-end + four semver sub-classes), and the M5 format adapters (TOML / HCL / INI / .env, HCL expressions as unevaluated). NOT full Kubernetes semantics.",
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
