# plan/HANDOFF-M4 — 续接交接单（下一棒：M4 默认值）

> 用法（新对话第一句）：`/goal 读取 "<项目绝对路径>\plan\HANDOFF-M4.md" 继续完成任务`
> **动手前第一件事**：`node tools/check-sprint-layout.mjs`（红了先修布局再干活）。
> 写于 2026-10-20，M3 收尾时。

## 一、当前进度与证据

M3（结构比较）已完成。下表全部是**本轮实测输出**，不是计划口径。

| 项 | 状态 | 证据（命令 → 结果） |
|---|---|---|
| 键重排零差异 | ✅ | golden `testM3-noise-keys` → `PASS exit 0`；`TestFourStructuralNoiseClasses` 全子例通过 |
| 缩进零差异 | ✅ | golden `testM3-noise-indent` → `PASS exit 0`（2 空格 vs 4 空格 + 序列缩进变化） |
| 引号风格零差异 | ✅ | golden `testM3-noise-quotes` → `PASS exit 0`（plain / 双引号 / 单引号同 Raw） |
| 注释零噪声（端到端） | ✅ | golden `testM3-noise-comments` → `PASS exit 0`，且 `contains ["comments: not compared"]` |
| `set` 乱序零差异 | ✅ | golden `testM3-set-reorder` → `PASS exit 0`（0/0/0） |
| `set` 重复元素 | ✅ | golden `testM3-set-duplicate` → `PASS exit 2`，输出 `? /items  indeterminate(duplicate_in_set)` |
| `set` 成员标识路径 | ✅ | golden `testM3-set-member-path` → `PASS exit 1`，`contains ['/items["string:charlie"]']` |
| `keyed:` 元素乱序零差异 | ✅ | golden `testM3-keyed-order` → `PASS exit 0` |
| `keyed:` 重复键 → exit 3 + 路径 | ✅ | golden `testM3-keyed-duplicate` → `PASS exit 3`，输出 `cfgdelta: keyed:name: duplicate key "alpha" at /items/1 (keys must be unique)`；`TestKeyedDuplicateIsError` 仍在 |
| RFC 6901 转义 | ✅ | golden `testM3-rfc6901-escape` → `PASS exit 1`，输出两行 `~ /a~1b`、`~ /c~0d`；`TestPathEscaping` |
| `contains` 判据本身会红 | ✅ | **本轮变异**：把 `setMemberPath` 改成裸拼 → `selftest: 42 passed, 1 failed`（`testM3-set-member-path … output does not contain`），单测同时红；还原后复绿 |
| golden 账 | ✅ | `selftest: 45 golden cases` → **43 passed, 0 failed, 2 skipped**（TOML/HCL 仍显式 skip） |
| 浮点禁令静态门 | ✅ | `go test ./tests/ -run TestNoFloatInComparisonPath -v` → `scanned 20 non-test files … 0 float identifiers` |
| 版本单一来源 | ✅ | `TestVersionSingleSource` PASS |
| 双跑字节一致 | ✅ | `TestOutputIsDeterministic` PASS；手测 `cmp` 三组（testA+profile / testB keyed / testM3 escape）逐字节相同 |
| 生成物一致性 | ✅ | `gen-golden-manifest --check`（45 例）/ `sync-version --check` / `sync-profiles --check` 全 exit 0 |
| `go vet` / `go test ./...` | ✅ | 全包 ok（`-count=1` 非缓存） |
| 布局门 | ✅ | 8 项全 PASS；M3 翻 ✅ 后 `LAYOUT.GAPS` 未红（缺口行已删） |
| CI | 🟡 | 仍未推送 = 未跑 CI。**不得写"CI 绿"** |

**M4 的 DoD 目前已"事实上"满足（本轮手测，下一棒必须逐条重跑后留新证据）**：

| M4 判据 | 本轮手测输出 |
|---|---|
| 测试A 4 → 0 | `diff --profile k8s/v1.29 testA/k1 testA/k2` → `equivalent`，`summary: total 2 · behavioral 0 · cosmetic 2`，rc=0，头部 `hits: 14` |
| 显式非默认对照组仍报 1 | `testA-ctrl` → `~ /spec/template/spec/containers/0/imagePullPolicy string:IfNotPresent -> string:Always`，rc=1 |
| 未知 profile 版本 exit 3 | `--profile k8s/v9.99` → `unknown profile "k8s/v9.99" (available: k8s/v1.29)`，rc=3 |
| 未命中降级标注 | ConfigMap 对照 → 头部尾部出现 `· degraded: structural-only`，`hits: 0`，rc=1 |
| `--dump-normalized` 双侧可见 | `TestDumpNormalizedShowsBothSides` PASS |

**已知偏差（必须如实带到下一棒）**：
- golden 从 **35 例 → 45 例**（新增 `testM3/` 10 条；15 个夹具）。
- `selftest` 的 `Case` 新增可选 `contains []string`（stdout+stderr 合并匹配）。K-08 仍成立：只存**期望**，不存观测输出。
- set 模式路径格式**改了**：由 `/items[string:charlie]`（裸拼）改为 `/items["string:charlie"]`（`strconv.Quote`）。理由与纪律见 §二.3，`plan/02`/`plan/03` 已同步。
- `plan/00 §三` 的三条结构比较承诺已翻 ✅；两条早已事实完成的台账（退出码、gate 选择性阻断）一并回写为 ✅，并在台账里把"模块 M3/M4"与"里程碑 M4/M5"的编号歧义标注清楚。
- M1/M2/M3 的 HANDOFF 是历史快照，本轮**未回改**；新事实只写在本文件。
- 三家二进制仍不入库；基线守门只校验留档自洽。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`selftest` 的 runner 把 stdout 与 stderr 拼成一个串**（`internal/cli/main_entry.go`）。`contains` 因此对错误路径（stderr）与正常输出（stdout）都有效。**不要**给 `Runner` 加第二返回值——会打挂所有用例的接线。
2. **`contains` 是大小写敏感的子串匹配，不是正则**。断言片段要短、且**不要**包含版本号/路径（`cfgdelta 0.1.0-dev`、绝对路径会随构建/机器漂移）。
3. **set 模式路径 = `<pointer>["<canonical-key>"]`，是成员标识，不是 JSON Pointer 段**。canonical key 由 `canonicalKey()` 生成（键按字节序、标量按族归一），整段经 `strconv.Quote`。该形态**不可**回喂 `--at`。落点：`internal/diff/diff.go:setMemberPath`、`plan/03 §三`、`TestSetMemberPathIsQuotedIdentity`、golden `testM3-set-member-path`。
4. **set 重复元素每条序列只产生 1 条 indeterminate**（每侧最多一条），Path 是序列本身的指针（如 `/items`），Reason 固定 `duplicate_in_set`；退出码 2。
5. **`keyed:` 重复键的错误文案**是 `keyed:<field>: duplicate key "<k>" at <p>/<i> (keys must be unique)`，走 stderr。golden 断的是 `duplicate key "alpha"` 与 `at /items/1` 两段。
6. **夹具值别用 `y` / `n` / `yes` / `no` / `on` / `off`**：YAML 1.1 布尔族会把它们归一成 bool（本轮实测 `name: y` → `bool:true`，制造了假差异）。夹具用 `alpha` / `beta` / `charlie` 这类安全串。
7. **golden 生成器有孤儿检查**：盘上任何夹具没被 case 引用 → `gen-golden-manifest` 直接 exit 1。加/删夹具必须同一次改 `tools/gen-golden-manifest.mjs`。
8. **`selftest` 的 `resolveArgs` 只改写"存在且含点"的参数**；夹具路径必须真实存在（`testM3/*.yaml` 均带 `.yaml`）。
9. **`testM3` 是新增目录**（15 个夹具 + 10 条 case）；`manifest.json` 现为 45 例。
10. **`plan/06 §四` 缺口表现有 6 行**（M3 行已删）；布局门从表头扫到下一张表结束。
11. **翻里程碑要四处动**：`plan/00` 里程碑表 + `plan/05` 状态表 + `README` 状态行 + `plan/06 §四` 缺口表删行（漏最后一条必红）。
12. **`plan/00 §三` 台账同时用了两套编号**：模块编号（M3=profile、M4=classify）与里程碑编号（M4=默认值、M5=分类）。本轮已在涉及行标注"模块 M3（里程碑 M4）"，改台账时**先看清是哪套**。
13. **`profile.Apply` 只补缺失键、绝不覆盖显式值**；`ApplyResult.Degraded` 在 CLI 里取 `la.Degraded && ra.Degraded`（**两侧都未命中**才算降级）——单侧命中意味着默认值确实参与了抵消，不能标降级。
14. **条件性默认值**：`imagePullPolicy` 仅当同容器 image 的 tag **非 latest**（且无 tag 时看 digest）才补 `IfNotPresent`。`:latest` / 无 tag / registry 端口形态都有单测钉死；改这条会打挂 `TestImagePullPolicyDependsOnImageTag`。
15. **`profile` 只有一个可编辑源 = `data/profiles/`**；`internal/profile/profiles/` 是生成镜像，改源必须跑 `sync-profiles.mjs`。
16. **不要再把 TOML 当差异化论据**；`testD` 的 TOML/HCL 仍是 `pending:`，等 M5。
17. **路径参数写不带前导 `/` 的形态**（`--at limits/memory`）。MSYS 会把前导 `/` 改写成 `D:/program/...`。

## 三、下一棒（M4 默认值）待办

- [ ] **先逐条重跑 §一"M4 事实上满足"表**，留本轮证据，不要照抄本文件。现有覆盖：`TestApplyCancelsOnBothSides`、`TestApplyDoesNotOverwriteExplicit`、`TestApplyDegradesWhenNothingMatches`、`TestImagePullPolicyDependsOnImageTag`、`TestImagePullPolicyConditionDoesNotOverwriteExplicit`、golden `testA-profile` / `testA-ctrl-profile` / `exit-unknown-profile`。
- [ ] **补 10 条规则的逐条覆盖**（plan/00 承诺"逐条 golden"）。当前只有 rule 1（imagePullPolicy）与 rule 2（ports/protocol）被 testA 真正走到；新增 `testdata/golden/testM4/`，每条规则至少一对夹具：默认值缺失 vs 显式写默认值 → 抵消（0 行为差异）；显式非默认 → 仍报 1 条。
- [ ] **降级标注的端到端 golden**：ConfigMap（未命中任何规则）→ 头部 `contains "degraded: structural-only"` 且 `hits: 0`。手测已通过，缺 golden。
- [ ] **施加顺序的专项断言**：外层先于内层、父先于子、同层按路径字节序（§4.11）。若现有单测没覆盖顺序，补一个可观测的顺序用例（不要只断言"最终值"）。
- [ ] **`--dump-normalized` 的默认值可见性**：现有测试断言两侧都出现 `imagePullPolicy`；M4 可再加"hits 数"与"缺失键被补齐"的断言。
- [ ] **不要动测试D**：TOML/HCL 仍是 `SKIP`，等 M5。别提前摘 `pending:`。
- [ ] M4 收尾时：四处翻 ✅（含 `plan/06` 删缺口行），写 `plan/HANDOFF-M5.md`（七节），commit。

## 四、既定口径清单（K 编号）

| 编号 | 口径 | 落点 | 动它会打挂什么 |
|---|---|---|---|
| K-01 | **禁止浮点参与相等判定** | `internal/value`；`tests/invariants_test.go:TestNoFloatInComparisonPath` | 静态门立即变红 |
| K-02 | 输出确定性：键字节序、不读时钟/环境变量、双跑逐字节一致 | `internal/diff`、`internal/cli` | `TestOutputIsDeterministic` + CI determinism step |
| K-03 | 构建固定 `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.version"`；版本单一来源 `VERSION` | `Makefile` / CI / `internal/version` | `TestVersionSingleSource` |
| K-04 | 规范值字符串冻结为 `family:value`；set 的 canonical key 也由它构成 | `internal/value/canonical.go`、`internal/diff/canonical_key.go` | `TestCanonicalStringIsFrozen` + 所有 golden |
| K-05 | `keyed:` 重复键 = **exit 3 + 路径**，禁止取首 | `internal/diff/keyedIndex` | `TestKeyedDuplicateIsError` + golden `testM3-keyed-duplicate` |
| K-06 | 默认族只推断 bool/quantity/duration/decimal；其余按 string | `internal/value/normalize.go` | `testC` 七组 + `testB` |
| K-07 | 用法错误统一 exit **3**（含未知子命令/缺参），不使用 2 | `internal/cli` | `TestExitCodeContract` |
| K-08 | golden 真值只存输入与期望，禁止 `observed_*`；`selftest` 对账**退出码 + summary 计数 + 可选 `contains`** | `testdata/golden/manifest.json`、`internal/selftest` | 任一项不符即 FAIL |
| K-09 | 默认值表外置版本化 JSON，`scope_note` 必写"非全量"，未知版本 exit 3 | `data/profiles/`、`internal/profile` | `TestUnknownProfileIsError` |
| K-10 | `indeterminate` 必带理由码；退出码优先级 `3 > 2 > 1 > 0` | `internal/exitcode`、`internal/value` | `TestExitCodePriority` |
| K-11 | 布局门/新判据「一条门里几个判据，变异清单就覆盖几个」；**变异没红先分三问**（靶子选错 / 假门 / 扫描面） | `tools/check-sprint-layout.mjs`、`internal/selftest` | 见 §五 |

**K-11 累计实战记录**：① M1 收尾时 `LAYOUT.GAPS` 红，是 `plan/06 §四` 漏删已 ✅ 行（不是门假）；② M0 时 `| R-05 | 跨平台二进制验证缺口 |` 曾被当成缺口表头，扫描面判据需锚位置+结构；③ **M3 新增 `contains` 判据后做了变异**：`setMemberPath` 裸拼 → selftest 精确点名 `testM3-set-member-path` 并失败，还原复绿。

## 五、本机环境坑（只写实测过的）

| 事实（实测日期 2026-10-20） | 后果 / 处置 |
|---|---|
| `go` 不在 PATH；SDK 在仓库外 `C:\Users\ASUS\go-sdk\go`（`.tmp/sdk/go` **不存在**） | **本轮实测 shell 里 `$USER` 为空**，`export PATH="/c/$USER/go-sdk/go/bin:$PATH"` 会失败；直接写 `/c/Users/ASUS/go-sdk/go/bin` |
| `raw.githubusercontent.com` 直连 SSL 失败（curl 35）；`127.0.0.1:7897` 代理也不通 | 取上游源码走 `api.github.com` contents API（`Accept: application/vnd.github.raw`） |
| `kubernetes/api` 仓库**没有** `v1.29.0` tag | 用 `kubernetes-1.29.0` |
| `api.github.com` 对 >1 MB 文件返回空 `content` | 大文件必须加 `Accept: application/vnd.github.raw` |
| `proxy.golang.org` 超时；`goproxy.cn` 200 | 必须 `GOPROXY=https://goproxy.cn,direct` |
| `make` 不在 PATH | `make check` 跑不了；用 §七 的等价手动序列 |
| `node v24.15.0` 可用；`tools/*.mjs` 零依赖 | 不需要 `npm install` |
| `py -0p` 有 3.8 / 3.12；`python` 是商店占位符 | 脚本一律 `py -X utf8` |
| `cd /d D:\...` 在 Git Bash 报 `cd: too many arguments` | 用 `cd "D:/path"`（本项目实测） |
| Git Bash 里 `grep`/`sed` 打印中文文件会乱码 | 判定"文件坏了"前先用解析器复读；`sed -i` 改写中文文档后务必回读 |
| Windows 下 `go test` 输出路径是反斜杠 | 断言路径用 `filepath.Join` |
| `cmp` 在 Git Bash 可用 | 双跑一致性可直接 `cmp a.txt b.txt` |
| `PIPESTATUS` 在 `cmd \| tail` 后取的是管道首元素退出码 | 要判断 cfgdelta 的 rc，别接管道；先重定向再 `echo $?` |

## 六、DoD checklist（M4）

- [ ] 测试A 从 4 条 → **0 条**（`testA-profile` exit 0，仅 cosmetic 计数）
- [ ] 显式非默认对照组 → **仍报 1 条**（`testA-ctrl-profile` exit 1）
- [ ] 10 条默认值规则**逐条 golden**（每条：抵消 + 显式非默认仍报）
- [ ] 未知 profile 版本 → exit 3（不回落）
- [ ] 未命中任何规则 → 头部 `degraded: structural-only`（端到端 golden，含 `contains`）
- [ ] `--dump-normalized` 可见双侧默认值
- [ ] 同输入双跑输出字节一致（`TestOutputIsDeterministic` + 手工 `cmp`）
- [ ] `node tools/gen-golden-manifest.mjs --check` exit 0（新增/删除夹具同一次改生成器）
- [ ] `node tools/check-sprint-layout.mjs` exit 0
- [ ] `go vet ./...` && `go test ./...` && `cfgdelta selftest` 全绿
- [ ] `plan/00` 里程碑表 + `plan/05` 状态表 + `README.md` 状态行 **三处一致**翻 ✅，且 `plan/06 §四` 删掉 M4 行
- [ ] `plan/00 §三` 台账的默认值三条承诺翻 ✅
- [ ] `plan/HANDOFF-M5.md` 已落盘（七节）
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
./dist/cfgdelta.exe selftest --verbose

# 6. M3 证据点（回归用）
./dist/cfgdelta.exe diff testdata/golden/testM3/noise-base-left.yaml testdata/golden/testM3/noise-comments-right.yaml; echo $?  # → 0
./dist/cfgdelta.exe diff --seq-mode set testdata/golden/testM3/set-reorder-left.yaml testdata/golden/testM3/set-dup-right.yaml; echo $?  # → 2
./dist/cfgdelta.exe diff --seq-mode keyed:name testdata/golden/testM3/keyed-dup-left.yaml testdata/golden/testM3/keyed-dup-right.yaml; echo $?  # → 3（stderr 带 /items/1）
./dist/cfgdelta.exe diff testdata/golden/testM3/escape-left.yaml testdata/golden/testM3/escape-right.yaml; echo $?  # → 1（/a~1b、/c~0d）

# 7. M4 证据点（下一棒先逐条重跑）
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?       # → 0，cosmetic 2
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA-ctrl/k1.yaml testdata/golden/testA-ctrl/k2.yaml; echo $?  # → 1
./dist/cfgdelta.exe diff --profile k8s/v9.99 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?       # → 3
./dist/cfgdelta.exe diff --profile k8s/v1.29 --dump-normalized testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml | head -20

# 8. 双跑一致性
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > /tmp/r1.txt 2>&1
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > /tmp/r2.txt 2>&1
cmp /tmp/r1.txt /tmp/r2.txt && echo BYTE_IDENTICAL

# 9. 基线守门（不需要本机装三家工具）
go test ./tests/ -run TestBaseline -v

# 10. 一并检查（Makefile 的等价手动序列；本机无 make）
node tools/check-sprint-layout.mjs && go vet ./... && go test -count=1 ./... \
  && node tools/sync-version.mjs --check \
  && node tools/sync-profiles.mjs --check \
  && node tools/gen-golden-manifest.mjs --check
```
