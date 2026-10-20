# plan/HANDOFF-M1 — 续接交接单（下一棒：M1 数据先行）

> 用法（新对话第一句）：`/goal 读取 "<项目绝对路径>\plan\HANDOFF-M1.md" 继续完成任务`
> **动手前第一件事**：`node tools/check-sprint-layout.mjs`（红了先修布局再干活）。
> 写于 2026-10-20，M0 收尾时。

## 一、当前进度与证据

M0（装订 + 骨架）已完成。全部证据均为**本轮实测输出**，不是计划口径。

| 项 | 状态 | 证据（命令 → 结果） |
|---|---|---|
| 第零步装订三件 | ✅ | `AGENTS.md` + `tools/check-sprint-layout.mjs` + `plan/00` 顶部装订行，门已跑绿 |
| 布局门变异验证 | ✅ | 9 组变异逐组变红，还原后复绿（详见 §4 K-11 的记录） |
| plan/00–06 七件 | ✅ | 各含必需小节；布局门 `LAYOUT.SECTIONS` 报 `8 份小节齐备` |
| Go 骨架 5 子命令 | ✅ | `normalize` / `diff` / `classify` / `gate` / `selftest` 均可跑 |
| 单静态二进制 | ✅ | `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w"` → `dist/cfgdelta.exe`（约 3.0 MB） |
| `go vet` + `go test ./...` | ✅ | 9 个包通过（`internal/{value,diff,classify,doc,profile,cli}` + `tests`）；另有 4 个包无测试文件 |
| golden `selftest` | ✅ | `27 passed, 0 failed, 2 skipped`（TOML/HCL 显式 skip） |
| 生成器三件 `--check` | ✅ | `sync-version` / `sync-profiles` / `gen-golden-manifest` 均 exit 0 |
| CI workflow | 🟡 | 本地 `yaml.safe_load` 解析通过（3 jobs / 18 steps），**未推送 = 未跑 CI** |

**干净 clone 复核抓出的三个缺陷（已修，全部有回归测试）**——这一族只有真 clone 才会暴露：

| # | 形态 | 症状 | 修法 / 回归 |
|---|---|---|---|
| ① | **Git Bash / MSYS2 静默改写参数** | README 写 `--at /limits/memory`，MSYS 把它转成 `D:/program/Git/limits/memory` → `matched no node`（工作树里也复现，但只有照 README 逐字敲才发现） | 前导 `/` 可省略；识别被改写形态并给出解释性错误（不静默失败）。`TestAtPathLeadingSlashOptional` / `TestShellRewrittenPathIsExplained` |
| ② | `--flag=value` 形态不被接受 | `--at=/limits/memory` 报 `unknown flag` | `normalize` 与 `diff`/`classify`/`gate` 全部支持两种形态。`TestEqualsFlagForm` |
| ③ | `--strict` 没生效 | `foobar` vs `100%` 在 `--strict` 下仍 rc=1（§4.7 要求 rc=2） | `InferFamilyDetailed` 返回 `fallback`，只升级"string 兜底 + 书写不同"那一类；显式字符串与百分比不升级。`TestStrictUpgradesOnlyUnknownFallback` |

**新增守门**：`tests/readme_commands_test.go` 把 README 每条 `cfgdelta` 命令做 **parser 反射对账** + 夹具存在性检查（占位符显式豁免）。变异验证：给 README 注入 `--show-kind=added` → 必红。

**已知偏差（必须如实带到下一棒）**：
- M1 只完成了「golden 夹具 + 10 条默认值 profile」这两半；**三家基线留档（`testdata/baseline/`）尚未落盘**——本机没有安装 dyff / diffyml / confdiff。M1 因此记 🟡 而非 ✅。
- 本机没有 `go` 在 PATH；SDK 解压在用户目录下的 `go-sdk/go`（**在仓库外**，不入库）。每棒开始需自行设置 PATH。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`yaml.v3` 的 `Decode` 根是 `DocumentNode`**，`doc.fromYAML` 已剥这一层。**任何新增解析入口都必须走 `doc.ParseBytes` / `doc.ParseFile`**，自己调 `yaml.Unmarshal` 会把整份文档读成空节点——M0 实测这个缺陷让整份 diff 变成"0 差异"，且 `selftest` 只比退出码时**不会变红**。
2. **默认族推断的唯一实现处 = `internal/value/normalize.go:InferFamily`**。M0 期间在这里踩过两个真缺陷：`500m` 被判成 duration（30000000000000ns），以及 `resources` 上下文优先级。改动前先读那段注释，改完必须跑 `testA` 与 `testC-c03`。
3. **`quantity` 与 `decimal` 同属有理数代数，允许跨族比较**（`canonical.go:Compare`）；**duration 不参与跨族比较**。`8000m`（quantity，=8）与 `8`（decimal）判等，靠的就是这条。
4. **纯书写 = `diff.CosmeticKind`**，渲染层**不逐条打印**，只在 `summary` 行计数。别在 `renderText` 里把它放出来——那正是本题要避免的 dyff 行为。
5. **`indeterminate` 是条目不是错误**：`compareScalar` 里必须把它记成条目并继续，不能 `return err`（否则整次 diff 中断、连 summary 都打不出，`selftest` 的计数对账会误报成"引擎没跑"）。
6. **分类不确定 → `ConfigChange` + `class_guess: true`，禁止归 `Cosmetic`**。`isExposurePath` 显式排除 `/protocol`——容器端口默认值不是暴露面变更。
7. **profile 只有一个可编辑源 = `data/profiles/`**；`internal/profile/profiles/` 是生成镜像。改源必须跑 `node tools/sync-profiles.mjs`，否则 `tests/TestProfilesMirrored` 变红。
8. **`VERSION` 与 `internal/version/VERSION` 必须同步**（`node tools/sync-version.mjs`）。
9. **CI 的 `test` job 用 `bash` + `set +e` + 显式断言退出码**：`selftest` 里刻意让某些用例非 0，`set -e` 会在检出问题处提前中断。
10. **`plan/05` 的排期表与状态表都以 `| M1 |` 起行**——布局门的 `LAYOUT.STATUS` 只扫「含『里程碑』与『状态』的表头」下方那张表。若给其它表加同样的表头，会被纳入状态比对。
11. **路径参数写成不带前导 `/` 的形态**（`--at limits/memory`、`--coerce-at "limits/*=quantity"`）。两种写法都支持，但 MSYS 改写会先于程序发生，README 与 golden manifest 已统一用无前导斜杠形态——**别"好心"把它改回带斜杠**，那会让 Git Bash 用户复现不了。
12. **`--strict` 只升级"走了 string 兜底且两侧书写不同"的路径**。改成"只要族是 string 就升级"会打挂 `TestStrictUpgradesOnlyUnknownFallback` 的第二、三段（引号字符串 / 百分比不该变吵）。

## 三、下一棒（M1 数据先行）待办

- [ ] **三家基线留档**：安装或下载 dyff v1.12.0 / diffyml v1.8.2 / confdiff 0.17.2，逐条跑 `data/README.md §1.2` 的命令，把 **原样 stdout/stderr** 落到 `testdata/baseline/<tool>/{VERSION.txt,COMMANDS.txt,*.out,exit_code.txt}`。**不得手工修饰**。跑不出来（上游已修）→ 追加带日期的复核记录，不删旧档。
- [ ] 新增守门：`testdata/baseline/` 每个工具目录四件齐全，且 `COMMANDS.txt` 里每条命令都能在当前版本回显版本号（非空自证）。
- [ ] 复核 `data/profiles/k8s/v1.29.json` 十条规则：每条 `source` 是否都能指到公开文档的具体行为（当前 10 条已填，但 M1 要求"逐条核对"而非"看起来合理"）。
- [ ] 把测试D 的 TOML/HCL 夹具保持为新格式适配器的回归锚（M5 用），**不要提前把 `pending:` 摘掉**。
- [ ] M1 收尾时把 `plan/00` 里程碑表、`plan/05` 状态表、`README.md` 状态行**三处同时**翻 ✅，再写 `plan/HANDOFF-M2.md`。

## 四、既定口径清单（K 编号）

| 编号 | 口径 | 落点 | 动它会打挂什么 |
|---|---|---|---|
| K-01 | **禁止浮点参与相等判定** | `internal/value`；`tests/TestNoFloatInComparisonPath` | 静态门立即变红 |
| K-02 | 输出确定性：键字节序、不读时钟/环境变量、双跑逐字节一致 | `internal/diff`、`internal/cli` | `TestOutputIsDeterministic` + CI determinism step |
| K-03 | 构建固定 `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.version"`；版本单一来源 `VERSION` | `Makefile` / CI / `internal/version` | `TestVersionSingleSource` |
| K-04 | 规范值字符串冻结为 `family:value`（如 `quantity:1073741824/1`） | `internal/value/canonical.go` | `TestCanonicalStringIsFrozen` + 所有 golden 期望 |
| K-05 | `keyed:` 重复键 = **exit 3 + 路径**，禁止取首 | `internal/diff/keyedIndex` | `TestKeyedDuplicateIsError` |
| K-06 | 默认族只推断 bool/quantity/duration/decimal；其余按 string | `internal/value/normalize.go` | `testC` 七组 + `testB` |
| K-07 | 用法错误统一 exit **3**（含未知子命令/缺参），不使用 2 | `internal/cli` | `TestExitCodeContract` |
| K-08 | golden 真值只存输入与期望，禁止 `observed_*`；`selftest` 除退出码外**还要对账计数** | `testdata/golden/manifest.json`、`internal/selftest` | 计数不符即 FAIL |
| K-09 | 默认值表外置版本化 JSON，`scope_note` 必写"非全量"，未知版本 exit 3 | `data/profiles/`、`internal/profile` | `TestUnknownProfileIsError` |
| K-10 | `indeterminate` 必带理由码；退出码优先级 `3 > 2 > 1 > 0` | `internal/exitcode`、`internal/value` | `TestExitCodePriority` |
| K-11 | 布局门「一条门里几个判据，变异清单就覆盖几个」；**变异没红先分三问**（靶子选错 / 假门 / 扫描面） | `tools/check-sprint-layout.mjs` | 见 §五 |

**K-11 的本轮实测记录**（下一棒改门时照抄这三问）：
- `LAYOUT.GAPS` 首版用 `l.includes("缺口")` 找表头 → 命中风险表的 `| R-05 | 跨平台二进制验证缺口 |`，真正的缺口表**从没被扫**（变异回填后门仍绿）。修法 = 表头首格必须**恰好**是「缺口」且下一行是分隔行。
- `LAYOUT.STATUS` 首版全文件扫 `| M1 |` 行 → 把 `plan/05` 的**排期表**读成状态表，M1–M6 全部假红。修法 = 按表头定位扫描面。
- 两次"没红"都不是门假，而是**变异靶子选错**（在装订行前面加残句、改风险表的行）。判别：先问"我改的是判据真正读的那个位置吗？"

## 五、本机环境坑（只写本轮实测过的）

| 事实（实测日期 2026-10-20） | 后果 / 处置 |
|---|---|
| `go` 不在 PATH；SDK 在用户目录下的 `go-sdk/go`（仓库外） | 每棒先 `export PATH="/c/<用户目录>/go-sdk/go/bin:$PATH"` |
| `crates.io` / `static.crates.io` 直连 **403**；`index.crates.io` 200 | 选 Go 而非 Rust 的次要理由之一；cargo 实际可 fetch（已实测），但 API 面被限 |
| `proxy.golang.org` **超时**；`goproxy.cn` **200** | 必须 `GOPROXY=https://goproxy.cn,direct`；`go mod tidy` 才能拉到 `gopkg.in/yaml.v3` |
| `dl.google.com` 可下载（87 MB，约 1 分钟内） | 换机时先装 Go 再谈其他 |
| `node v24.15.0` 可用 | 三个 `tools/*.mjs` 零依赖，不需要 npm install |
| `py -0p` 有 3.8 / 3.12；本机 `python` 是商店占位符 | 脚本一律 `py -X utf8`；读中文文件用 `encoding="utf-8"` |
| Git Bash 里 `grep`/`sed` 打印中文文件会乱码 | 判定"文件坏了"前先用解析器/`py -X utf8` 复读；`sed -i` 改写中文文档后务必回读校验 |

## 六、DoD checklist（M1）

- [ ] `testdata/baseline/{dyff,diffyml,confdiff}/` 各有 `VERSION.txt` / `COMMANDS.txt` / 原样 `*.out` / `exit_code.txt`
- [ ] 基线留档含调用命令与版本号，且**与 `data/README.md §1.2` 的表格逐条对应**
- [ ] `data/profiles/k8s/v1.29.json` 十条规则逐条核对来源（不是"看起来合理"）
- [ ] `node tools/gen-golden-manifest.mjs --check` exit 0
- [ ] `node tools/check-sprint-layout.mjs` exit 0
- [ ] `go vet ./...` && `go test ./...` && `cfgdelta selftest` 全绿
- [ ] `plan/00` 里程碑表 + `plan/05` 状态表 + `README.md` 状态行**三处一致**翻 ✅
- [ ] `plan/HANDOFF-M2.md` 已落盘（七节）
- [ ] commit（**未接远端，不得写"CI 绿"**）

## 七、关键命令速查

```bash
# 0. 环境（本机）
export PATH="/c/<用户目录>/go-sdk/go/bin:$PATH"   # 见 §五 第 1 行；本机把 <用户目录> 换成实际值
export GOFLAGS=-mod=mod GOPROXY=https://goproxy.cn,direct GOTOOLCHAIN=local

# 1. 布局门（每棒第一件事）
node tools/check-sprint-layout.mjs

# 2. 生成物一致性
node tools/sync-version.mjs --check
node tools/sync-profiles.mjs --check
node tools/gen-golden-manifest.mjs --check

# 3. 改过 profile 源 / VERSION 之后
node tools/sync-profiles.mjs
node tools/sync-version.mjs

# 4. 测试与构建
go vet ./...
go test ./...
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(cat VERSION)" -o dist/cfgdelta.exe ./cmd/cfgdelta

# 5. golden 套件（离线、不调外部命令）
./dist/cfgdelta.exe selftest
./dist/cfgdelta.exe selftest --verbose

# 6. 手工复现测试A（README 首屏卖点）
./dist/cfgdelta.exe diff testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?          # → 1
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?  # → 0

# 7. 门禁语义
./dist/cfgdelta.exe gate --fail-on 资源·收缩 testdata/golden/testGate/shrink-left.yaml testdata/golden/testGate/shrink-right.yaml; echo $?  # → 1
./dist/cfgdelta.exe gate --fail-on 接口·暴露 testdata/golden/testGate/shrink-left.yaml testdata/golden/testGate/shrink-right.yaml; echo $?  # → 0

# 8. 一并检查（Makefile）
make check      # gate + vet + test + 三个 --check
```
