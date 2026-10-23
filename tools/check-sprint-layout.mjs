#!/usr/bin/env node
// 布局门（方法论装订第②件）：把"项目还长在方法论上"变成可跑红的断言。
// 判据一律落**仓库内文件**（不读 git 元数据——浅克隆下 git tag/历史深度会翻转）。
// 退出码：0 全绿 / 1 有 FAIL / 2 用法错误。
//
// **三态**：PASS / SKIP / FAIL。SKIP 表示"本判据此刻没有判定对象"（例如缺口表为空、
// 发布里程碑还没翻 ✅），它**不是** PASS —— 汇总行会分别计数，避免把"没验"读成"验过了"。
//
// 变异验证清单（判据有几条就覆盖几条）：实物是 tools/verify-layout-mutations.mjs，
// 每条判据一组故障，全红后还原全绿。本文件只声明，实物在那边跑。

import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..");

const checks = [];
/**
 * status: true → PASS，false → FAIL，"SKIP" → SKIP（本次无判定对象）。
 * SKIP 不计入 FAIL，也**不**计入"验过了"——汇总行单独报数。
 */
const add = (id, status, detail) => {
  const state = status === "SKIP" ? "SKIP" : status ? "PASS" : "FAIL";
  checks.push({ id, state, detail });
};
const read = (rel) => (existsSync(join(ROOT, rel)) ? readFileSync(join(ROOT, rel), "utf8") : null);

const scanned = { docs: 0, handoffs: 0, copies: 0, docTools: 0 };

const REQUIRED_FILES = [
  "AGENTS.md",
  "README.md",
  "LICENSE",
  "data/README.md",
  "plan/00-README总览.md",
  "plan/01-题目文档.md",
  "plan/02-架构与技术选型.md",
  "plan/03-模块详设.md",
  "plan/04-数据计划.md",
  "plan/05-里程碑与验收.md",
  "plan/06-交付对标.md",
];

// 装订行：三个锚点必须同现于一行（前瞻，顺序无关；不用"或"关系）。
const BINDING_LINE = /^>(?=.*ai-tool-project-sprint)(?=.*check-sprint-layout)(?=.*20\d{2}-\d{2}-\d{2}).*$/m;

const REQUIRED_SECTIONS = {
  "plan/00-README总览.md": ["文档索引", "决策记录", "承诺-证据台账", "里程碑"],
  "plan/01-题目文档.md": ["任务要求", "评分标准", "边界与非目标"],
  "plan/02-架构与技术选型.md": ["分层", "构建"],
  "plan/03-模块详设.md": ["内部文档树", "CLI 契约", "分类"],
  "plan/04-数据计划.md": ["数据清单", "真值纪律", "数值口径"],
  "plan/05-里程碑与验收.md": ["DoD", "已知未验证项"],
  "plan/06-交付对标.md": ["评分对标", "风险登记"],
  "data/README.md": ["依据查证记录", "数值口径"],
};

const HANDOFF_SECTIONS = ["当前进度", "当前事实", "待办", "既定口径", "环境坑", "DoD", "命令速查"];

/** 项目已声明的豁免：M0 的收尾记录就是计划套件本身 + 本文件；不要求 HANDOFF-M0。 */
const EXEMPT_MILESTONES = [0];
const HANDOFF_FORMAT_FROM = 1;

/** 状态副本：同一里程碑在这几处的状态必须互相一致（真值不取自任何一个副本）。 */
const STATUS_COPIES = [
  { file: "plan/00-README总览.md", kind: "里程碑表" },
  { file: "plan/05-里程碑与验收.md", kind: "状态表" },
  { file: "README.md", kind: "状态行" },
];

/** 只认"状态行"：表格行且首格就是里程碑号（不扫自由文本——避免张冠李戴）。 */
const STATUS_LINE = /^\|\s*\*{0,2}M(\d+)\b/;
const PENDING_MARKERS = ["⬜", "🟡", "未开始", "未执行", "待执行", "待确认"];

function checkRequiredFiles() {
  const missing = REQUIRED_FILES.filter((rel) => !existsSync(join(ROOT, rel)));
  scanned.docs = REQUIRED_FILES.length - missing.length;
  add("LAYOUT.FILES", missing.length === 0, missing.length ? `缺: ${missing.join(",")}` : `${scanned.docs} 份齐备`);
}

function checkBindingHead() {
  const text = read("plan/00-README总览.md") ?? "";
  const ok = BINDING_LINE.test(text);
  add("LAYOUT.BINDING_HEAD", ok, ok ? "plan/00 顶部有装订行（三锚点齐）" : "plan/00 顶部缺装订行（需同现：方法论名 + 布局门命令 + 装订日期）");
}

function checkSections() {
  const bad = [];
  for (const [rel, needles] of Object.entries(REQUIRED_SECTIONS)) {
    const text = read(rel);
    if (text === null) continue;
    const missing = needles.filter((n) => !text.includes(n));
    if (missing.length) bad.push(`${rel}: ${missing.join("/")}`);
  }
  add("LAYOUT.SECTIONS", bad.length === 0, bad.length ? bad.join("；") : `${Object.keys(REQUIRED_SECTIONS).length} 份小节齐备`);
}

/**
 * 取"状态表"的行：按**表头**定位（表头行同时含「里程碑」与「状态」），
 * 从表头往下扫到第一张表结束为止。
 *
 * 为什么必须按表头定位：同一份文档里还有**排期表**（同样以 `| M1 |` 起行），
 * 全文件扫会把"每周可演示物"读成"里程碑状态"，于是 M1–M6 全部恒 pending
 * （与 plan/00 里程碑表冲突 → 假红）。这是"扫描面比你以为的长"那一族。
 */
function statusRows(text) {
  const lines = text.split("\n");
  const headerIdx = lines.findIndex(
    (l) => l.startsWith("|") && l.includes("里程碑") && l.includes("状态"),
  );
  if (headerIdx < 0) return null;
  const rows = [];
  for (let i = headerIdx + 2; i < lines.length; i++) {
    if (!lines[i].startsWith("|")) break;
    rows.push(lines[i]);
  }
  return rows;
}

/** 逐副本采集状态：Map<k, Set<done|pending>>；任一里程碑出现两种状态即 FAIL。 */
function collectStates() {
  const states = new Map();
  const note = (k, state) => {
    if (!states.has(k)) states.set(k, new Set());
    states.get(k).add(state);
  };
  for (const copy of STATUS_COPIES) {
    const text = read(copy.file);
    if (text === null) continue;
    scanned.copies += 1;
    const rows = statusRows(text);
    if (rows === null) {
      add(`LAYOUT.STATUS_HEADER(${copy.kind})`, false, `${copy.file} 找不到含「里程碑」「状态」的表头（判据表搬走了？）`);
      continue;
    }
    if (rows.length === 0) {
      add(`LAYOUT.STATUS_HEADER(${copy.kind})`, false, `${copy.file} 的状态表没有数据行`);
      continue;
    }
    for (const line of rows) {
      const m = STATUS_LINE.exec(line);
      if (!m) continue;
      note(Number(m[1]), PENDING_MARKERS.some((mark) => line.includes(mark)) ? "pending" : "done");
    }
  }
  return states;
}

function checkStatusAgreement(states) {
  const conflicts = [];
  for (const [k, set] of [...states.entries()].sort((a, b) => a[0] - b[0])) {
    if (set.size > 1) conflicts.push(`M${k} 在副本间状态不一致（${[...set].join(" vs ")}）`);
  }
  add(
    "LAYOUT.STATUS",
    conflicts.length === 0,
    conflicts.length ? conflicts.join("；") : `${scanned.copies} 处副本 × ${states.size} 个里程碑 状态全一致`,
  );
}

function checkHandoffs(done) {
  const missing = [];
  for (const k of done) {
    if (EXEMPT_MILESTONES.includes(k)) continue;
    const rel = `plan/HANDOFF-M${k}.md`;
    const text = read(rel);
    if (text === null) {
      missing.push(`${rel}(缺)`);
      continue;
    }
    scanned.handoffs += 1;
    if (k < HANDOFF_FORMAT_FROM) continue;
    const lack = HANDOFF_SECTIONS.filter((n) => !text.includes(n));
    if (lack.length) missing.push(`${rel}(缺节: ${lack.join("/")})`);
  }
  add("LAYOUT.HANDOFFS", missing.length === 0, missing.length ? missing.join("；") : `${scanned.handoffs} 份交接单齐（M0 已声明豁免）`);
}

/**
 * plan/06 的「缺口」列是状态的隐蔽副本。
 *
 * 判据：**表头首格必须是「缺口」**，且紧跟一行分隔行（`|---|`）——不能写成
 * "行里含『缺口』二字"：本项目实测风险登记表里有一条 `| R-05 | 跨平台二进制验证缺口 |`，
 * 它同样"含『缺口』且以 | 开头"，于是被判成表头，真正的缺口表从此**没人扫**
 * （变异回填 M0 后门仍绿）。表头判据要同时锚位置与结构。
 */
function findGapHeader(lines) {
  for (let i = 0; i < lines.length; i++) {
    const cells = lines[i].split("|");
    if (!lines[i].startsWith("|") || cells.length < 4) continue;
    if (cells[1].trim() !== "缺口") continue;
    const sep = lines[i + 1] ?? "";
    if (!/^\|[\s:-]+\|/.test(sep)) continue;
    return i;
  }
  return -1;
}

function checkStaleGaps(done) {
  const lines = (read("plan/06-交付对标.md") ?? "").split("\n");
  const headerIdx = findGapHeader(lines);
  if (headerIdx < 0) {
    add("LAYOUT.GAPS", false, "plan/06 找不到以「缺口」为首格且带分隔行的表头（判据表搬走了？）");
    return;
  }
  // 第 1 列（cells[1] 因为 split("|") 首尾各多一个空串）才是「缺口」列。
  // 取"最后一列"会扫到「目标里程碑」——那是**合法写 ✅ 里程碑**的地方（D2：方向反了）。
  const col = 1;
  const stale = [];
  let scannedRows = 0;
  // 扫描面：从表头到**下一张表头或文件末**——同一文件后面章节的表格不属于缺口表。
  for (let i = headerIdx + 2; i < lines.length; i++) {
    const line = lines[i];
    if (!line.startsWith("|")) break;
    scannedRows++;
    const cells = line.split("|");
    const cell = cells[col] ?? "";
    for (const k of done) {
      if (new RegExp(`(^|[^0-9A-Za-z])M${k}([^0-9]|$)`).test(cell)) {
        stale.push(`plan/06 缺口列仍提 M${k}: ${cell.trim()}`);
      }
    }
  }
  if (stale.length) {
    add("LAYOUT.GAPS", false, stale.join("；"));
    return;
  }
  // 空缺口表是**合法状态**（plan/06 只登记尚未关闭的缺口）。
  // 但"扫了 0 行"时"没有陈旧行"恒真 —— 此刻判据什么也没证明，不能报 PASS（D3）。
  if (scannedRows === 0) {
    add("LAYOUT.GAPS", "SKIP", "plan/06 缺口表 0 数据行（合法：无未关闭缺口），本判据本次无判定对象");
    return;
  }
  add("LAYOUT.GAPS", true, `plan/06 缺口列扫了 ${scannedRows} 行，无已 ✅ 里程碑`);
}

/**
 * 收尾记录：要求**最新的发布里程碑**有 `plan/RELEASE-M<k>.md` 或 `plan/WRAPUP-M<k>.md`。
 *
 * 为什么不按"最后一个 ✅ 里程碑"要求：M0–M5 是过程中里程碑，收尾记录在那个时点
 * 结构上不可能存在（发布留档必须写在发布前、tag 之前）。按"最后一个 ✅"要求会在
 * 每个中间里程碑制造假红。判据 = **"这个产物在当前时点本来就该存在吗？"**
 *
 * 「发布里程碑」= 最高 ✅ 里程碑中，**已有或应当有**收尾记录的那个。取法：所有 ✅ 里程碑里
 * 最大的那个（发布总是最后一棒）。这样 v0.1.1 起的再发布不必改本文件。
 *
 * 变异验证：把发布里程碑的 RELEASE/WRAPUP 文件删掉 → 本项必须红（见 verify-layout-mutations V8）。
 */
function releaseMilestone(done) {
  return done.length ? Math.max(...done) : 0;
}

/** plan/ 下所有 RELEASE-M*.md 与 WRAPUP-M*.md（每份都要满足同样的结构）。 */
function allWraupDocs() {
  const planDir = join(ROOT, "plan");
  if (!existsSync(planDir)) return [];
  return readdirSync(planDir)
    .filter((f) => /^(RELEASE|WRAPUP)-M\d+\.md$/.test(f))
    .sort()
    .map((f) => `plan/${f}`);
}

function checkWrapup(done) {
  const k = releaseMilestone(done);
  if (k === 0) {
    add("LAYOUT.WRAPUP", "SKIP", "没有已 ✅ 的里程碑，收尾记录尚不适用——本判据本次无判定对象");
    return;
  }
  const release = read(`plan/RELEASE-M${k}.md`);
  const wrapup = read(`plan/WRAPUP-M${k}.md`);
  if (!release && !wrapup) {
    add("LAYOUT.WRAPUP", false, `缺 plan/RELEASE-M${k}.md 或 plan/WRAPUP-M${k}.md（M${k} 是最新 ✅ 里程碑）`);
    return;
  }
  // 所有留档都必须"两栏 + 覆盖对账"——最新那份也不例外（防止只写 §一 就发版）。
  const bad = [];
  for (const rel of allWraupDocs()) {
    const text = read(rel);
    if (text === null) continue;
    if (rel.includes("/WRAPUP-")) {
      if (!(text.includes("DoD") && text.includes("偏差"))) bad.push(`${rel} 缺 DoD/偏差节`);
      continue;
    }
    const cols = /^##\s*一、tag 前事实/m.test(text) && /^##\s*二、tag 后追加/m.test(text);
    const heading = /^#{2,3}.*覆盖对账/m.test(text);
    if (!cols || !heading) bad.push(`${rel} 两栏:${cols} 覆盖对账:${heading}`);
  }
  const names = allWraupDocs().join(" / ") || "无";
  add("LAYOUT.WRAPUP", bad.length === 0, bad.length ? bad.join("；") : `${names} 结构齐（M${k} 为最新 ✅ 里程碑）`);
}

/** 文档点名的 tools/ 脚本必须有实物；非空跑自证（0 处引用 = 判据是空的）。 */
function checkDocumentedToolsExist() {
  const EXEMPT = new Set([]);
  const planDir = join(ROOT, "plan");
  const docs = [
    "AGENTS.md",
    ...(existsSync(planDir) ? readdirSync(planDir).filter((f) => f.endsWith(".md")).map((f) => `plan/${f}`) : []),
  ];
  const referenced = new Set();
  for (const rel of docs) {
    const text = read(rel);
    if (text === null) continue;
    for (const m of text.matchAll(/tools\/[\w.-]+\.(?:mjs|js|cjs|py|sh|ps1)/g)) referenced.add(m[0]);
  }
  if (referenced.size === 0) {
    add("LAYOUT.DOC_TOOLS", false, `扫了 ${docs.length} 份文档，0 处 tools/ 引用——判据是空的（阳性对照失败）`);
    return;
  }
  scanned.docTools = referenced.size;
  const missing = [...referenced].filter((rel) => !EXEMPT.has(rel) && !existsSync(join(ROOT, rel)));
  add(
    "LAYOUT.DOC_TOOLS",
    missing.length === 0,
    missing.length ? `文档点名但不存在的工具: ${missing.join(",")}` : `${referenced.size} 个文档点名的 tools/ 脚本全部在位`,
  );
}

function main() {
  for (const run of [checkRequiredFiles, checkBindingHead, checkSections, checkDocumentedToolsExist]) {
    try {
      run();
    } catch (error) {
      add(run.name, false, error.message);
    }
  }
  const states = collectStates();
  const done = [...states.entries()].filter(([, set]) => set.has("done")).map(([k]) => k);
  for (const run of [() => checkStatusAgreement(states), () => checkHandoffs(done), () => checkStaleGaps(done), () => checkWrapup(done)]) {
    try {
      run();
    } catch (error) {
      add(run.name, false, error.message);
    }
  }

  const failed = checks.filter((c) => c.state === "FAIL");
  const skipped = checks.filter((c) => c.state === "SKIP");
  const passed = checks.filter((c) => c.state === "PASS");
  process.stdout.write("布局门（方法论装订） · 判据只落仓库内文件\n");
  process.stdout.write("=".repeat(56) + "\n");
  for (const c of checks) process.stdout.write(`${c.state.padEnd(4)}  ${c.id}  ${c.detail}\n`);
  process.stdout.write("=".repeat(56) + "\n");
  process.stdout.write(`这轮扫到：文档 ${scanned.docs} / 交接单 ${scanned.handoffs} / 状态副本 ${scanned.copies} / 文档点名工具 ${scanned.docTools}\n`);
  process.stdout.write(
    `结论: ${failed.length === 0 ? "全绿" : `${failed.length} 项 FAIL`}` +
      `（PASS ${passed.length} / SKIP ${skipped.length} / FAIL ${failed.length}，共 ${checks.length} 条判据）\n`,
  );
  if (skipped.length) {
    process.stdout.write(`SKIP 的判据本次没有判定对象：${skipped.map((c) => c.id).join(", ")}\n`);
  }
  process.exitCode = failed.length === 0 ? 0 : 1;
}

main();
