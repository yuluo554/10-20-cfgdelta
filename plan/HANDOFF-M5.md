# plan/HANDOFF-M5 — 续接交接单（下一棒：M5 分类门禁 + 多格式）

> 用法（新对话第一句）：`/goal 读取 "<项目绝对路径>\plan\HANDOFF-M5.md" 继续完成任务`
> **动手前第一件事**：`node tools/check-sprint-layout.mjs`（红了先修布局再干活）。
> 写于 2026-10-20，M4 收尾时。

## 一、当前进度与证据

M4（默认值：双侧施加 + 抵消 + 未命中降级）已完成。下表全部是**本轮实测输出**，不是计划口径。

| 项 | 状态 | 证据（命令 → 结果） |
|---|---|---|
| 测试A 4 → 0 | ✅ | `diff --profile k8s/v1.29 testA/k1.yaml testA/k2.yaml` → `equivalent (no behavioural differences)`，`summary: total 2 · behavioral 0 · cosmetic 2`，rc=0，头部 `hits: 14` |
| 显式非默认对照组仍报 1 | ✅ | `testA-ctrl` → `~ /spec/template/spec/containers/0/imagePullPolicy string:IfNotPresent -> string:Always`，rc=1，`hits: 15` |
| 未知 profile 版本 exit 3 | ✅ | `--profile k8s/v9.99` → `unknown profile "k8s/v9.99" (available: k8s/v1.29)`，rc=3；golden `exit-unknown-profile` |
| 未命中降级标注 | ✅ | ConfigMap 对照 → 头部 `… · degraded: structural-only`，`hits: 0`，rc=1；golden `testM4-degraded-header`（`contains` 两项） |
| 10 条规则逐条 golden | ✅ | 新增 `testdata/golden/testM4/` 32 个夹具 / 21 条 case：`testM4-rule01..10-cancel`（0 条目）+ `testM4-rule01..10-nondefault`（1 条 + 路径 + 归一值断言） |
| 施加顺序可观测 | ✅ | `TestApplyOrderIsOuterFirstThenBytewise`：深度升序 + 同层字节序 + 通配展开逐条断言；`Apply` 改为按路径排序，不再用 profile 文件顺序 |
| `--dump-normalized` 可见 | ✅ | `TestDumpNormalizedShowsBothSides`：头部 `(hits: 14)`；左树含 8 个补齐键（含 `progressDeadlineSeconds: 600`、`terminationGracePeriodSeconds: 30`） |
| 变异验证（K-11，4 组） | ✅ | ①去掉规则排序 → 顺序用例红；②禁用施加 → `selftest: 43 passed, 21 failed`（testA-profile + 全部 20 条 M4 case）；③删 `degraded:` 标注 → `63 passed, 1 failed`（`testM4-degraded-header` 单例）；④覆盖显式值 → `53 passed, 11 failed`（testA-ctrl + 全部 10 条 nondefault）。四次均还原复绿 |
| golden 账 | ✅ | `cfgdelta selftest` → **66 例：64 passed, 0 failed, 2 skipped**（TOML/HCL 仍显式 skip） |
| 生成物一致性 | ✅ | `gen-golden-manifest --check`（66 例）/ `sync-version --check` / `sync-profiles --check` 全 exit 0 |
| `go vet` / `go test ./...` | ✅ | 全包 ok（`-count=1` 非缓存） |
| 双跑字节一致 | ✅ | `TestOutputIsDeterministic` PASS；手工 `cmp` 三组（testA / testM4-rule02 / testM4-rule09）逐字节相同 |
| 布局门 | ✅ | 8 项全 PASS；M4 翻 ✅ 后 `LAYOUT.STATUS` 3 处一致、`LAYOUT.GAPS` 由 6 行降到 5 行 |
| CI | 🟡 | 仍未推送 = 未跑 CI。**不得写"CI 绿"** |

**已知偏差（必须如实带到下一棒）**：
- golden 从 **45 例 → 66 例**（新增 `testM4/` 21 条；32 个夹具）。
- `ApplyResult` 新增 `Applied []string`；`Hits` 语义细化为**具体补齐路径条数**（`containers/*` 会展开），不再是"命中规则的条数"。测试A 因此是 14（左 8 + 右 6）。
- `Apply` 的规则顺序改为**按路径推导**（段数升序 → 同层字节序）。行为面不变（规则之间无依赖），但 `--dump-normalized` 的键插入顺序变了，属预期。
- M1/M2/M3/M4 的 HANDOFF 是历史快照，本轮**未回改**；新事实只写在本文件。
- 三家二进制仍不入库；基线守门只校验留档自洽。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`ApplyResult.Applied` 是顺序的唯一可断言面**。它记录具体路径（通配符已展开），`Hits == len(Applied)`。加规则会改变 `testA` 的 `hits` 数值——`TestDumpNormalizedShowsBothSides` 的 `(hits: 14)` 与交接单里的数字要重新推导，**不许照抄**。
2. **规则顺序由路径推导**：`orderedRules` 先按 `len(splitPointer(path))` 升序，再按 path 字节序。不要改回 profile JSON 书写顺序（`TestApplyOrderIsOuterFirstThenBytewise` 会红）。
3. **`kind: ""` 的规则对所有 kind 生效**（规则 2 = Deployment 模板里的 `ports/*/protocol`）；`kind: "Pod"` 的是规则 3。规则 2 与 3 是两条独立记录，不要合并成一条"通用规则"。
4. **只在最后一个段补键**：中间层缺失不猜结构。因此 `strategy` 整体缺失时 `/spec/strategy/type` 不补；`strategy: {}` 或 `strategy: {rollingUpdate: …}` 存在时才补 `type`。这条是 `insertDefault` 的显式分支，不是 bug。
5. **`imagePullPolicy` 是条件默认值**：image tag 为 `latest` 或无 tag（且无 digest）→ 不补（API Server 给的是 `Always`）。改这条会打挂 `TestImagePullPolicyDependsOnImageTag`。
6. **降级判据**：`ApplyResult.Degraded = (Hits == 0)`（单侧）；CLI 合并为 `la.Degraded && ra.Degraded`（**两侧都未命中**才算降级）。单侧命中意味着默认值确实参与了抵消，不能标降级。
7. **cancel / nondefault 夹具成对存在，互为反证**：删掉任意一半，"什么都不做"或"覆盖显式值"都能骗过剩下的那一半。新增规则必须同时补两条 case。
8. **golden 生成器有孤儿检查**：盘上任何夹具没被 case 引用 → `gen-golden-manifest` exit 1。加/删夹具必须同一次改 `tools/gen-golden-manifest.mjs`。
9. **`contains` 是大小写敏感的字面子串**（stdout+stderr 合并）。断言片段要短，且**不要**包含版本号/绝对路径。
10. **`selftest` 的 `resolveArgs` 只改写"存在且含点"的参数**；夹具路径必须真实存在。
11. **TOML/HCL 当前的真实行为（本轮实测，M5 要修）**：
    - `testD/a.toml` 以 `[service]` 开头 → `DetectFormat` 的首字节 `[` 判定把它路由到 **JSON** → `invalid character 's' looking for beginning of value`，rc=3。**报错是诚实的，但原因是错的**。
    - `testD/a.hcl` → 落到 **YAML**，整份文件被解析成**一个标量**；两侧属性重排变成 `~  string:resource "thing" … -> string:resource "thing" …`，rc=1。**这是假差异，必须在 M5 用按扩展名分派的适配器消除**。
    - 两条 golden 仍是 `pending:` skip，**M5 落地适配器前不许摘掉**。
12. **`testD` 夹具本身是正确的**（TOML 两份只差键序；HCL 两份只差属性顺序）；不要为了让引擎通过而改夹具。
13. **M5 的"测试B 5 条 → 0"口径存疑**：`plan/05` 写"测试B 5 条 exit 1 → 0 行为差异 exit 0"，但现有 golden `testB-ordered` 的实测真值是 **4 behavioral + 2 cosmetic, exit 1**（env 顺序成对错位），`--seq-mode set` / `keyed:name` 才是 0/0 exit 0。**先回查 `plan/01` 的测试B 原文**再定，禁止为了凑"5 → 0"改夹具或改期望。
14. **分类引擎已存在**：`internal/classify` 有 6 类枚举 + 固定排序 + semver 四例单测。golden 里目前只有 `classify-resource-shrink` / `classify-exposure` 两个类的端到端例子；`部署·收缩` / `镜像·变更` / `配置·新增·移除` 还缺 golden。
15. **翻里程碑要四处动**：`plan/00` 里程碑表 + `plan/05` 状态表 + `README` 状态行 + `plan/06 §四` 缺口表删行（漏最后一条必红）。
16. **`plan/00 §三` 台账同时用了两套编号**：模块编号（M4=classify、M5=多格式）与里程碑编号（M5=分类门禁、M6=发布）。改台账时先看清是哪套；本轮涉及行已标"模块 M3（里程碑 M4）"之类。
17. **夹具值别用 `y` / `n` / `yes` / `no` / `on` / `off`**：YAML 1.1 布尔族会把它们归一成 bool。用 `alpha` / `beta` / `build-bot` 这类安全串。
18. **路径参数写不带前导 `/` 的形态**（`--at limits/memory`）。MSYS 会把前导 `/` 改写成 `D:/program/...`。
19. **退出码 0/1/2/3 单点定义**（`internal/exitcode`），优先级 `3 > 2 > 1 > 0`；用法错误一律 3。
20. **`indeterminate` 是一等结果**，必须带理由码。HCL 的 `unevaluated` 若要新增理由码，走 `internal/value` 的既有理由码机制，**不要**另造一套旁路。

## 三、下一棒（M5 分类门禁 + 多格式）待办

- [ ] **先查清测试B 口径**（见 §二.13）：读 `plan/01` 的测试B 原文，确认"5 条"指哪种 seq-mode / 哪组输入；结论写进 `plan/05` M5 DoD，**不改 golden 去凑数**。
- [ ] **五类各一例 golden**：现有 `classify-resource-shrink` / `classify-exposure`；补 `部署·收缩`、`镜像·变更`、`配置·新增·移除` 三例（后两类可复用 testB / testM3 的输入）。
- [ ] **镜像 semver 四例**：`TestImageSemverClassification` 已有 major/minor/patch/non-semver 单测；若 DoD 要求端到端，补一条 golden 断言 `subclass` 出现在输出里。
- [ ] **多格式适配器**（`internal/doc`）：按扩展名显式分派 `.toml` / `.ini` / `.env` / `.hcl`，并让未知格式**明确报错**而不是回落 YAML。最低验收：
  - `testD-toml` 零噪声（键序无关，rc=0）——摘掉 `pending:` 并确认 0/0/0；
  - `testD-hcl` 属性重排零差异（rc=0）；
  - HCL 表达式（如 `count = var.n`）标 `unevaluated` 且**可见**（理由码 + 输出面，不要静默当字符串比）。
- [ ] **`docs/testA-before-after.png`**：用已生效的 M4 输出做 4 → 0 对照图，落盘并在 README 引用。
- [ ] **不要动已 ✅ 的 M1–M4 口径**：归一化文法、序列三模式、RFC 6901 转义、默认值顺序与 `Applied` 语义都已被 golden 钉死。
- [ ] M5 收尾时：四处翻 ✅（含 `plan/06` 删缺口行）、写 `plan/HANDOFF-M6.md`（七节）、commit。

## 四、既定口径清单（K 编号）

| 编号 | 口径 | 落点 | 动它会打挂什么 |
|---|---|---|---|
| K-01 | **禁止浮点参与相等判定** | `internal/value`；`tests/invariants_test.go:TestNoFloatInComparisonPath` | 静态门立即变红 |
| K-02 | 输出确定性：键字节序、不读时钟/环境变量、双跑逐字节一致 | `internal/diff`、`internal/cli` | `TestOutputIsDeterministic` + CI determinism step |
| K-03 | 构建固定 `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.version"`；版本单一来源 `VERSION` | `Makefile` / CI / `internal/version` | `TestVersionSingleSource` |
| K-04 | 规范值字符串冻结为 `family:value`（整数是 `decimal:30/1`，不是 `decimal:30`）；set 的 canonical key 也由它构成 | `internal/value/canonical.go`、`internal/diff/canonical_key.go` | `TestCanonicalStringIsFrozen` + 所有 golden |
| K-05 | `keyed:` 重复键 = **exit 3 + 路径**，禁止取首 | `internal/diff/keyedIndex` | `TestKeyedDuplicateIsError` + golden `testM3-keyed-duplicate` |
| K-06 | 默认族只推断 bool/quantity/duration/decimal；其余按 string | `internal/value/normalize.go` | `testC` 七组 + `testB` |
| K-07 | 用法错误统一 exit **3**（含未知子命令/缺参），不使用 2 | `internal/cli` | `TestExitCodeContract` |
| K-08 | golden 真值只存输入与期望，禁止 `observed_*`；`selftest` 对账**退出码 + summary 计数 + 可选 `contains`** | `testdata/golden/manifest.json`、`internal/selftest` | 任一项不符即 FAIL |
| K-09 | 默认值表外置版本化 JSON，`scope_note` 必写"非全量"，未知版本 exit 3；未命中 → `degraded: structural-only` | `data/profiles/`、`internal/profile` | `TestUnknownProfileIsError`、golden `testM4-degraded-header` |
| K-10 | `indeterminate` 必带理由码；退出码优先级 `3 > 2 > 1 > 0` | `internal/exitcode`、`internal/value` | `TestExitCodePriority` |
| K-11 | 布局门/新判据「一条门里几个判据，变异清单就覆盖几个」；**变异没红先分三问**（靶子选错 / 假门 / 扫描面） | `tools/check-sprint-layout.mjs`、`internal/selftest` | 见 §五 |

**K-11 累计实战记录**：① M1 收尾时 `LAYOUT.GAPS` 红，是 `plan/06 §四` 漏删已 ✅ 行（不是门假）；② M0 时 `| R-05 | 跨平台二进制验证缺口 |` 曾被当成缺口表头，扫描面判据需锚位置+结构；③ M3 新增 `contains` 判据后做了变异：`setMemberPath` 裸拼 → selftest 精确点名 `testM3-set-member-path` 并失败；④ **M4 新增「规则顺序 / 施加 / 降级标注 / 不覆盖显式值」四类判据后逐类变异**：分别红在顺序单测、13 条 cancel+testA、降级单例、11 条 nondefault+testA-ctrl，还原后全绿。

## 五、本机环境坑（只写实测过的）

| 事实（实测日期 2026-10-20） | 后果 / 处置 |
|---|---|
| `go` 不在 PATH；SDK 在仓库外 `C:\Users\ASUS\go-sdk\go`（`.tmp/sdk/go` **不存在**） | shell 里 `$USER` 为空；直接写 `/c/Users/ASUS/go-sdk/go/bin` |
| **`python` 是商店占位符**（本轮实测：`python - <<EOF` 静默失败、退出码 1） | 脚本一律 `py -X utf8`；改文件优先用编辑工具 |
| `raw.githubusercontent.com` 直连 SSL 失败（curl 35）；`127.0.0.1:7897` 代理也不通 | 取上游源码走 `api.github.com` contents API（`Accept: application/vnd.github.raw`） |
| `kubernetes/api` 仓库**没有** `v1.29.0` tag | 用 `kubernetes-1.29.0` |
| `api.github.com` 对 >1 MB 文件返回空 `content` | 大文件必须加 `Accept: application/vnd.github.raw` |
| `proxy.golang.org` 超时；`goproxy.cn` 200 | 必须 `GOPROXY=https://goproxy.cn,direct` |
| `make` 不在 PATH | `make check` 跑不了；用 §七 的等价手动序列 |
| `node v24.15.0` 可用；`tools/*.mjs` 零依赖 | 不需要 `npm install` |
| `py -0p` 有 3.8 / 3.12 | 脚本一律 `py -X utf8` |
| `cd /d D:\...` 在 Git Bash 报 `cd: too many arguments` | 用 `cd "D:/path"`（本项目实测） |
| Git Bash 里 `grep`/`sed` 打印中文文件会乱码 | 判定"文件坏了"前先用解析器复读；`sed -i` 改写中文文档后务必回读 |
| Windows 下 `go test` 输出路径是反斜杠 | 断言路径用 `filepath.Join` |
| `cmp` 在 Git Bash 可用 | 双跑一致性可直接 `cmp a.txt b.txt` |
| `PIPESTATUS` 在 `cmd \| tail` 后取的是管道首元素退出码 | 要判断 cfgdelta 的 rc，别接管道；先重定向再 `echo $?` |

## 六、DoD checklist（M5）

- [ ] 测试B 口径查清并按 `plan/01` 原文达标（**不改 golden 凑数**）
- [ ] 六类各一例端到端 golden（含纯书写只计数）；semver 四例可见
- [ ] 退出码 0/1/2/3 逐位参数化断言（现有 `TestExitCodeContract` + `exit-*` golden，复核即可）
- [ ] TOML 零噪声：`testD-toml` 摘掉 `pending:` 后 exit 0 / 0 条目
- [ ] HCL 属性重排零差异：`testD-hcl` exit 0
- [ ] HCL 表达式 → `unevaluated` 可见（理由码 + 输出面）
- [ ] INI / .env 适配器（plan/06 缺口表口径），未知格式明确报错
- [ ] `docs/testA-before-after.png` 落盘且内容与 M4 实测一致
- [ ] `node tools/gen-golden-manifest.mjs --check` exit 0（夹具增删同一次改生成器）
- [ ] `node tools/check-sprint-layout.mjs` exit 0
- [ ] `go vet ./...` && `go test ./...` && `cfgdelta selftest` 全绿
- [ ] `plan/00` 里程碑表 + `plan/05` 状态表 + `README.md` 状态行 **三处一致**翻 ✅，且 `plan/06 §四` 删掉 M5 行
- [ ] `plan/00 §三` 台账的分类 / semver / 多格式三条承诺翻 ✅
- [ ] 新判据逐条变异（K-11），记录"哪条红在哪"
- [ ] `plan/HANDOFF-M6.md` 已落盘（七节）
- [ ] commit（**未接远端，不得写"CI 绿"**）

## 七、关键命令速查

```bash
# 0. 环境（本机；$USER 为空，写死用户名）
export PATH="/c/Users/ASUS/go-sdk/go/bin:$PATH"
export GOFLAGS=-mod=mod GOPROXY=https://goproxy.cn,direct GOTOOLCHAIN=local

# 1. 布局门（每棒第一件事）
cd "D:/ProgramData/zcode/10-20-cfgdelta"
node tools/check-sprint-layout.mjs

# 2. 生成物一致性
node tools/sync-version.mjs --check
node tools/sync-profiles.mjs --check
node tools/gen-golden-manifest.mjs --check

# 3. 改过 profile 源 / VERSION 之后
node tools/sync-profiles.mjs && node tools/sync-version.mjs

# 4. 测试与构建
go vet ./...
go test -count=1 ./...
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(cat VERSION)" -o dist/cfgdelta.exe ./cmd/cfgdelta

# 5. golden 套件（离线、不调外部命令）
./dist/cfgdelta.exe selftest
./dist/cfgdelta.exe selftest --verbose | grep testM4

# 6. M4 证据点（回归用）
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?       # → 0（hits: 14，cosmetic 2）
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA-ctrl/k1.yaml testdata/golden/testA-ctrl/k2.yaml; echo $?  # → 1
./dist/cfgdelta.exe diff --profile k8s/v9.99 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?       # → 3
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testM4/degraded-left.yaml testdata/golden/testM4/degraded-right.yaml; echo $?  # → 1，头部含 degraded + hits: 0
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testM4/rule04-left.yaml testdata/golden/testM4/rule04-default-right.yaml; echo $?     # → 0（抵消）
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testM4/rule04-left.yaml testdata/golden/testM4/rule04-nondefault-right.yaml; echo $?  # → 1
./dist/cfgdelta.exe diff --profile k8s/v1.29 --dump-normalized testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml | head -20

# 7. M5 待修点（当前真实行为）
./dist/cfgdelta.exe diff testdata/golden/testD/a.toml testdata/golden/testD/b.toml; echo $?  # 现在 → 3（被误判成 JSON）
./dist/cfgdelta.exe diff testdata/golden/testD/a.hcl testdata/golden/testD/b.hcl; echo $?    # 现在 → 1（整份文件被当 YAML 单标量）

# 8. 双跑一致性
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > .tmp/r1.txt 2>&1
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > .tmp/r2.txt 2>&1
cmp .tmp/r1.txt .tmp/r2.txt && echo BYTE_IDENTICAL

# 9. 即席夹具（M4 手测降级用；`.tmp/` 已被 .gitignore 排除）
mkdir -p .tmp/m4

# 10. 一并检查（Makefile 的等价手动序列；本机无 make）
node tools/check-sprint-layout.mjs && go vet ./... && go test -count=1 ./... \
  && node tools/sync-version.mjs --check \
  && node tools/sync-profiles.mjs --check \
  && node tools/gen-golden-manifest.mjs --check
```
