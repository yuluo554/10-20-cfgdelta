#!/usr/bin/env node
// 布局门的变异验证（判据有几条就覆盖几条）。
//
// 为什么需要有实物：`plan/00` 与 `plan/05` 曾写「9 组变异验证全红」，但仓库里
// 没有任何可复跑的脚本 —— 那句话是**声明**，不是证据。本文件把那句话变成可跑的东西。
//
// 做法：把 HEAD 的树导出到系统临时目录（不在工作树上动手），逐条注入故障、
// 跑布局门、断言两件事 ——**门 exit ≠ 0** 且 **报出的 FAIL 判据 id 就是预期那条**
// （"随便红了"不算过，误伤别的判据也不算过），然后还原，最后重跑基线要求全绿。
//
// 工作树里若有**已跟踪但未提交**的改动，会覆盖到导出树上并打印出来，
// 这样改门的同时就能自验；CI 上工作树等于 HEAD，不会有覆盖。
//
// 用法：node tools/verify-layout-mutations.mjs
// 退出码：0 全部如期 / 1 有断言不成立 / 2 用法错误

import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const GATE = "tools/check-sprint-layout.mjs";

for (const a of process.argv.slice(2)) {
  process.stderr.write(`verify-layout-mutations: unknown argument ${a}\n`);
  process.exit(2);
}

const results = [];
const say = (line) => process.stdout.write(`${line}\n`);

function git(args, cwd = ROOT) {
  const r = spawnSync("git", args, { cwd, encoding: "buffer", maxBuffer: 256 * 1024 * 1024 });
  if (r.status !== 0) {
    process.stderr.write(`git ${args.join(" ")} 失败: ${r.stderr?.toString("utf8") ?? ""}\n`);
    process.exit(1);
  }
  return r.stdout;
}

// ── 1. 导出干净树 ────────────────────────────────────────────────────────────
const tmpRoot = mkdtempSync(join(tmpdir(), "cfgdelta-layout-mut-"));
const tree = join(tmpRoot, "tree");
mkdirSync(tree, { recursive: true });

{
  const archive = git(["archive", "HEAD"]);
  // 用 cwd 而不是 `tar -C <path>`：Windows 上 git-bash 的 tar 会把盘符路径误当远程主机名。
  const r = spawnSync("tar", ["-x"], { input: archive, cwd: tree });
  if (r.status !== 0) {
    process.stderr.write(`tar 解包失败: ${r.stderr?.toString("utf8") ?? ""}\n`);
    process.exit(1);
  }
}

// 工作树的未提交改动（含**未跟踪**的新增文件）覆盖上去 —— 改门的同时可自验。
// 未跟踪文件必须一起带，否则"文档点名了新脚本、树里没有"会让 LAYOUT.DOC_TOOLS 假红。
const overlaid = [];
{
  const tracked = git(["diff", "--name-only", "--diff-filter=AMD", "HEAD"]).toString("utf8").split("\n");
  const untracked = git(["ls-files", "--others", "--exclude-standard"]).toString("utf8").split("\n");
  for (const rel of [...tracked, ...untracked].filter(Boolean)) {
    const from = join(ROOT, rel);
    const to = join(tree, rel);
    if (!existsSync(from)) continue;
    mkdirSync(dirname(to), { recursive: true });
    copyFileSync(from, to);
    overlaid.push(rel);
  }
}

const head = git(["rev-parse", "--short", "HEAD"]).toString("utf8").trim();
const dirty = git(["status", "--porcelain"]).toString("utf8").trim();
say(`变异验证 · 布局门（${GATE}）`);
say(`导出树：HEAD ${head}${dirty ? `（工作树有未提交改动，已覆盖 ${overlaid.length} 个文件）` : "（干净）"}`);
if (overlaid.length) for (const rel of overlaid) say(`  overlay ${rel}`);
say("=".repeat(72));

// ── 2. 跑门 ─────────────────────────────────────────────────────────────────
/** @returns {{code:number, states:Map<string,string>, detail:Map<string,string>, raw:string}} */
function runGate() {
  const r = spawnSync(process.execPath, [join(tree, GATE)], { cwd: tree, encoding: "utf8" });
  const raw = `${r.stdout ?? ""}${r.stderr ?? ""}`;
  const states = new Map();
  const detail = new Map();
  for (const line of raw.split("\n")) {
    const m = /^(PASS|SKIP|FAIL)\s+(LAYOUT\.[A-Z_]+(?:\([^)]*\))?)\s+(.*)$/.exec(line.trimEnd());
    if (m) {
      states.set(m[2], m[1]);
      detail.set(m[2], m[3]);
    }
  }
  return { code: r.status ?? 1, states, detail, raw };
}

const BINDING_OK = /^>(?=.*ai-tool-project-sprint)(?=.*check-sprint-layout)(?=.*20\d{2}-\d{2}-\d{2}).*$/m;

// ── 3. 变异工具：改 → 跑 → 还原 ──────────────────────────────────────────────
const read = (rel) => readFileSync(join(tree, rel), "utf8");
const write = (rel, text) => writeFileSync(join(tree, rel), text);
const pathOf = (rel) => join(tree, rel);

/** 改一个文件 → 返回还原函数。所有"注入"都必须回报一个 undo。 */
function edit(rel, fn) {
  const original = read(rel);
  write(rel, fn(original));
  return () => write(rel, original);
}

/** 删一个文件 → 返回还原函数。 */
function drop(rel) {
  const p = pathOf(rel);
  const original = readFileSync(p);
  rmSync(p);
  return () => writeFileSync(p, original);
}

/**
 * 注入一条故障，断言门 exit ≠ 0 且 FAIL 集合里**恰好**有预期判据
 * （"随便红了"不算过，误伤别的判据也不算过），最后还原并复绿。
 * @param {{id:string, desc:string, expect:string, apply:() => () => void}} m
 */
function mutation({ id, desc, expect, apply }) {
  const undo = apply();
  const run = runGate();
  undo();
  const after = runGate(); // 还原后必须复绿，否则"变红"无从归因
  const fails = [...run.states.entries()].filter(([, s]) => s === "FAIL").map(([k]) => k);
  const hitExpected = fails.includes(expect);
  const extra = fails.filter((k) => k !== expect);
  const restored = after.code === 0;
  const ok = run.code !== 0 && hitExpected && extra.length === 0 && restored;
  const why = [
    run.code !== 0 ? null : "门仍 exit 0（没变红）",
    hitExpected ? null : `FAIL 里没有 ${expect}（实际: ${fails.join(", ") || "无"}）`,
    extra.length ? `误伤额外判据: ${extra.join(", ")}` : null,
    restored ? null : `还原后门未复绿（exit ${after.code}）`,
  ].filter(Boolean);
  results.push({ id, ok, desc, expect });
  say(`${ok ? "PASS" : "FAIL"}  ${id} ${desc}`);
  say(
    `      → ${expect} ${ok ? "如期变红" : `未如期变红：${why.join("；")}`}` +
      `${run.code !== 0 ? ` [exit ${run.code}]` : ""}`,
  );
}

/** 在 `## 四、缺口表` 的分隔行后插一行数据行。 */
function insertGapRow(text, row) {
  const i = text.indexOf("## 四、缺口表");
  if (i < 0) throw new Error("plan/06 找不到「## 四、缺口表」");
  const lines = text.slice(i).split("\n");
  const out = [];
  let inserted = false;
  for (const l of lines) {
    out.push(l);
    if (!inserted && l.startsWith("|---")) {
      out.push(row);
      inserted = true;
    }
  }
  if (!inserted) throw new Error("plan/06 缺口表缺分隔行");
  return text.slice(0, i) + out.join("\n");
}

/** 把缺口表裁成「表头 + 分隔行」，即 0 数据行（合法状态）。 */
function blankGapTable(text) {
  const i = text.indexOf("## 四、缺口表");
  if (i < 0) throw new Error("plan/06 找不到「## 四、缺口表」");
  const out = [];
  let seenSep = false;
  for (const l of text.slice(i).split("\n")) {
    if (!seenSep) {
      out.push(l);
      if (l.startsWith("|---")) seenSep = true;
      continue;
    }
    if (l.startsWith("|")) continue; // 丢掉全部数据行
    out.push(l);
  }
  if (!seenSep) throw new Error("plan/06 缺口表缺分隔行");
  return text.slice(0, i) + out.join("\n");
}

/** 门当前要求存在的那份收尾记录：最高 ✅ 里程碑的 RELEASE-/WRAPUP-。 */
function latestReleaseDoc() {
  const planDir = join(tree, "plan");
  const nums = readdirSync(planDir)
    .map((f) => /^RELEASE-M(\d+)\.md$/.exec(f))
    .filter(Boolean)
    .map((m) => Number(m[1]));
  if (!nums.length) throw new Error("plan/ 下没有 RELEASE-M*.md，V8 没有靶子");
  const k = Math.max(...nums);
  const release = `plan/RELEASE-M${k}.md`;
  if (existsSync(join(tree, release))) return release;
  return `plan/WRAPUP-M${k}.md`;
}

// ── 4. 八条变异：一条判据一条 ────────────────────────────────────────────────
mutation({
  id: "V1",
  desc: "删 AGENTS.md",
  expect: "LAYOUT.FILES",
  apply: () => drop("AGENTS.md"),
});

mutation({
  id: "V2",
  desc: "删 plan/02 的必需小节关键词「构建」",
  expect: "LAYOUT.SECTIONS",
  apply: () =>
    edit("plan/02-架构与技术选型.md", (t) => {
      const out = t.replace(/构建/g, "建置");
      if (out === t) throw new Error("plan/02 里没有「构建」二字（判据表已变？）");
      return out;
    }),
});

mutation({
  id: "V3",
  desc: "删 plan/00 顶部装订行",
  expect: "LAYOUT.BINDING_HEAD",
  apply: () =>
    edit("plan/00-README总览.md", (t) => t.split("\n").filter((l) => !BINDING_OK.test(l)).join("\n")),
});

mutation({
  id: "V4",
  desc: "文档点名一个不存在的 tools/ 脚本",
  expect: "LAYOUT.DOC_TOOLS",
  apply: () => edit("plan/03-模块详设.md", (t) => `${t}\n见 \`tools/no-such-tool.mjs\`（变异注入）。\n`),
});

mutation({
  id: "V5",
  desc: "某副本把 ✅ 里程碑翻成 ⬜",
  expect: "LAYOUT.STATUS",
  apply: () =>
    edit("README.md", (t) => {
      // 只翻**一处副本**的 M5 行 → 副本间状态不一致即红
      let done = false;
      return t
        .split("\n")
        .map((l) => {
          if (done || !/^\|\s*\*{0,2}M5\b/.test(l)) return l;
          done = true;
          return l.replace(/\|([^|]*✅[^|]*)\|/, "| ⬜ 未开始（变异注入） |");
        })
        .join("\n");
    }),
});

mutation({
  id: "V6",
  desc: "删一份 HANDOFF-M<k>（k 已 ✅）",
  expect: "LAYOUT.HANDOFFS",
  apply: () => drop("plan/HANDOFF-M2.md"),
});

mutation({
  id: "V7",
  desc: "把已 ✅ 里程碑写回 plan/06 的**缺口列（第 1 列）**",
  expect: "LAYOUT.GAPS",
  apply: () => edit("plan/06-交付对标.md", (t) => insertGapRow(t, "| M1 应该在缺口里（变异注入） | 未关闭 | — |")),
});

mutation({
  id: "V8",
  desc: "删**最新 ✅ 里程碑**的收尾记录（「哪一份必须存在」由最高 ✅ 里程碑决定）",
  expect: "LAYOUT.WRAPUP",
  apply: () => drop(latestReleaseDoc()),
});

// ── 5. 两条反向回归（不是变异：要求"该绿就绿 / 该 SKIP 就 SKIP"）───────────
function regression({ id, desc, expectState, expectId = "LAYOUT.GAPS", apply }) {
  const undo = apply();
  const run = runGate();
  undo();
  const state = run.states.get(expectId);
  const ok = run.code === 0 && state === expectState;
  results.push({ id, ok, desc, expect: `${expectId}=${expectState}` });
  say(`${ok ? "PASS" : "FAIL"}  ${id} ${desc}`);
  say(
    `      → ${expectId} ${ok ? `如期为 ${expectState}` : `期望 ${expectState}、实得 ${state ?? "（未报）"}，门 exit ${run.code}`}`,
  );
}

regression({
  id: "R1",
  desc: "缺口表 0 数据行必须 SKIP（不是 PASS —— 空转门）",
  expectState: "SKIP",
  apply: () => edit("plan/06-交付对标.md", blankGapTable),
});

regression({
  id: "R2",
  desc: "合法的未关闭缺口、目标列写当前最高 ✅ 里程碑 → PASS（不误报）",
  expectState: "PASS",
  apply: () => edit("plan/06-交付对标.md", (t) => insertGapRow(t, "| 一个合法的未关闭缺口（回归注入） | 未关闭 | M6 补齐 |")),
});

// ── 6. 覆盖度：门的判据集合必须与本文的靶子集合逐条对上 ─────────────────────
// 这一步防的是"门加了第 9 条判据、脚本还报 8/8"——数字会漂，关系不会。
{
  const baselineIds = [...runGate().states.keys()].filter((id) => !id.includes("(")).sort();
  const targeted = [...new Set(results.filter((r) => /^V\d+$/.test(r.id)).map((r) => r.expect))].sort();
  const uncovered = baselineIds.filter((id) => !targeted.includes(id));
  const phantom = targeted.filter((id) => !baselineIds.includes(id));
  const ok = uncovered.length === 0 && phantom.length === 0 && baselineIds.length === targeted.length;
  results.push({ id: "R3", ok, desc: "每条判据恰好一组变异（覆盖度）", expect: "集合相等" });
  say("=".repeat(72));
  say(`${ok ? "PASS" : "FAIL"}  R3 每条判据恰好一组变异（覆盖度）`);
  say(
    `      → 门报 ${baselineIds.length} 条判据 / 本文靶 ${targeted.length} 条` +
      (ok ? "，集合相等" : `；未覆盖: ${uncovered.join(", ") || "无"}；多出: ${phantom.join(", ") || "无"}`),
  );
}

// ── 7. 基线：还原后整棵树必须全绿 ───────────────────────────────────────────
const baseline = runGate();
const baselineOk = baseline.code === 0;
results.push({ id: "R0", ok: baselineOk, desc: "还原全部变异后基线全绿", expect: "exit 0" });
say("=".repeat(72));
say(`${baselineOk ? "PASS" : "FAIL"}  R0 还原全部变异后基线全绿 → ${baselineOk ? "exit 0" : `exit ${baseline.code}`}`);
if (!baselineOk) say(baseline.raw.trimEnd());

// ── 8. 结论 ─────────────────────────────────────────────────────────────────
const mutations = () => results.filter((r) => /^V\d+$/.test(r.id));
const regs = () => results.filter((r) => /^R\d+$/.test(r.id));
const failed = results.filter((r) => !r.ok);
const mutRed = mutations().filter((r) => r.ok).length;
const criteria = [...baseline.states.keys()].filter((id) => !id.includes("("));
say("=".repeat(72));
say(
  `结论：${mutRed}/${mutations().length} 组变异如期变红 + ${regs().filter((r) => r.ok).length}/${regs().length} 条回归断言成立` +
    `（布局门判据 ${criteria.length} 条：${criteria.join(" / ")}）`,
);
if (failed.length) {
  say(`不成立：${failed.map((r) => r.id).join(", ")}`);
  process.exitCode = 1;
}

rmSync(tmpRoot, { recursive: true, force: true });
