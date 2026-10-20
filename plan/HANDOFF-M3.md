# plan/HANDOFF-M3 — 续接交接单（下一棒：M3 结构比较）

> 用法（新对话第一句）：`/goal 读取 "<项目绝对路径>\plan\HANDOFF-M3.md" 继续完成任务`
> **动手前第一件事**：`node tools/check-sprint-layout.mjs`（红了先修布局再干活）。
> 写于 2026-10-20，M2 收尾时。

## 一、当前进度与证据

M2（归一化）已完成。下表全部是**本轮实测输出**，不是计划口径。

| 项 | 状态 | 证据（命令 → 结果） |
|---|---|---|
| 测试C 七组 100% 判等 | ✅ | `./dist/cfgdelta.exe selftest --verbose` → `testC-c01`…`c07` 全 `PASS  exit 0` |
| `1Gi` vs `1000M` 反例 | ✅ | `testC-c08-counterexample` → `PASS  exit 1`（未被误判成 cosmetic） |
| `MB` vs `MiB` 歧义 | ✅ | `testC-c09-ambiguous` → `PASS  exit 2`；手测输出行 `? /limits/storage  indeterminate(ambiguous_unit_family)` |
| 浮点禁令静态门 | ✅ | `go test ./tests/ -run TestNoFloatInComparisonPath -v` → `scanned 20 non-test files … 0 float identifiers`；**本轮变异实测**：临时插 `var _ float64` 到 `internal/value/zz_mutation_tmp.go` → FAIL 并点名文件:行，删除后复绿 |
| 双跑字节一致 | ✅ | `TestOutputIsDeterministic` PASS；手测 `cmp`：testA+profile、testB keyed 两组输出逐字节相同 |
| 非法输入 → exit 3 | ✅ | 新增 golden `exit-invalid-yaml`：`diff testInvalid/bad-yaml.yaml …` → `yaml: did not find expected ',' or ']'`，rc=3，**无 summary 行**（未降级为字符串比较） |
| 文法边界 5 例 | ✅ | 新增 golden `grammar-e3mi-invalid`（rc=3）、`grammar-leading-dot`、`grammar-plus-sign`、`grammar-duration-micro`、`grammar-duration-zero`（后四条 rc=0 · cosmetic 1） |
| 符号折叠 `+1≡1` / `-0≡0` | ✅ | `TestDecimalSignFolding`（规范串逐字节相同）+ `TestCanonicalStringIsFrozen` 仍绿 |
| golden 账 | ✅ | `selftest: 35 golden cases` → **33 passed, 0 failed, 2 skipped**（TOML/HCL 仍显式 skip，未摘 pending） |
| 生成物一致性 | ✅ | `gen-golden-manifest --check` / `sync-version --check` / `sync-profiles --check` 全 exit 0 |
| `go vet` / `go test ./...` | ✅ | 全包 ok，无 skip 掩盖 |
| 布局门 | ✅ | 8 项全 PASS；翻 ✅ 后 `LAYOUT.GAPS` 未红（缺口行已删） |
| CI | 🟡 | 仍未推送 = 未跑 CI。**不得写"CI 绿"** |

**本轮从文法边界核对里抓出的真缺陷（已修，有回归）**：

| # | 形态 | 症状 | 修法 / 回归 |
|---|---|---|---|
| ① | **`ParseDecimal` 不接受符号** | §4.3 明写 `+1 ≡ 1`、`-0 ≡ 0`，实际 `normalize --family decimal` 对 `+1`/`-0` 回 `not a decimal literal` 并 **exit 3**（合法书写被判成坏输入） | `ParseDecimal` 先剥离 `+`/`-` 再解析、最后取负；`parseDecimalRat` 保持"只收无符号数"（quantity/duration 的符号已在各自入口处理）。回归：`TestDecimalSignFolding` + golden `grammar-plus-sign` |
| ② | 文档点错浮点门文件名 | `AGENTS.md` 与 `plan/05` 写 `tests/float_ban_test.go`，仓库里**没有这个文件** | 改为 `tests/invariants_test.go:TestNoFloatInComparisonPath`；`internal/value/value_test.go` 注释同步 |
| ③ | AGENTS 里的 Go 路径与实测不符 | 写 `.tmp/sdk/go`，本机该目录不存在（`/.tmp/` 还被 `.gitignore` 排除） | 按实测改为仓库外 `C:\Users\<user>\go-sdk\go`，并注明 `.tmp/sdk/go` 不存在 |

**已知偏差（必须如实带到下一棒）**：
- golden 从 **29 例 → 35 例**（新增 5 条文法边界 + 1 条非法 YAML）。`scope_note` 同步加了一句 "§4.1/§4.2 grammar boundaries"。
- `plan/00 §三` 的 7 条 normalize 承诺已随 M2 翻 ✅（密度：每条给了测试/golden 落点，不是"整体完成"一句）。
- M1/M2 的 HANDOFF 是历史快照，本轮**未回改**；新事实只写在本文件。
- 三家二进制仍不入库；基线守门只校验留档自洽。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`ParseDecimal` 接受符号，`parseDecimalRat` 不接受**。`ParseQuantity`/`ParseDuration` 在各自入口剥离符号后调用它；把符号处理再下沉会打乱 duration 的"整串取负 + 拒绝负总数"语义。
2. **`1e3Mi` 由 `splitQuantity` 的指数校验拒绝**（消息是 `bad exponent`）。`ParseQuantity` 里"指数与后缀互斥"那条分支对 `1e3Mi` 实际走不到；改文案前先跑 golden `grammar-e3mi-invalid`（只断言 rc=3，不断言文案）。
3. **golden 新增两个目录**：`testdata/golden/testGrammar/`（10 个夹具）、`testdata/golden/testInvalid/bad-yaml.yaml`。manifest 生成器有**孤儿检查**：盘上任何夹具没被 case 引用 → `gen-golden-manifest` 直接 exit 1；因此加/删夹具必须同一次改 `tools/gen-golden-manifest.mjs`。
4. **`selftest` 的 `resolveArgs` 只改写"存在且含点"的参数**。坏 YAML 夹具必须**真实存在**才会被改写成绝对路径；写成不存在的路径会变成另一种用法错误。
5. **`exit-invalid-yaml` 的 stdout 为空且无 `summary:` 行**。`checkSummary` 的规则是"manifest 期望计数全 0 + 无 summary → 通过"；若以后给错误路径加 summary，这条用例会开始 FAIL。
6. **`indeterminate` 的理由码在条目行里**（`? /path  indeterminate(reason)`），不在 stderr。stderr 只用于"整份文档无法解析"或用法错误。
7. **YAML 的 `+1` 原文保留 `+`**；折叠发生在归一化（`ParseDecimal`），不在解析层。`Raw != Raw` 才会产生 cosmetic 条目。
8. **`cosmetic` 只计数不逐条打印**；canonical 相等但原文不同 → `CosmeticKind`（`TestNormalizedEqualIsCosmetic`）。
9. **`indeterminate` 不会中断比较**：`compareScalar` 记条目并继续；任何 `return err` 都会让 summary 打不出来。
10. **M2 DoD 的浮点门实际文件名是 `tests/invariants_test.go`**（不是 `float_ban_test.go`）；K-01 的落点按本文件读。
11. **`plan/05` 的排期表与状态表都以 `| M1 |` 起行**，布局门只扫「表头含『里程碑』与『状态』」那张表。
12. **翻里程碑要四处动**：`plan/00` 里程碑表 + `plan/05` 状态表 + `README` 状态行 + `plan/06 §四` 缺口表删行（漏最后一条必红）。
13. **路径参数写不带前导 `/` 的形态**（`--at limits/memory`）。MSYS 会把前导 `/` 改写成 `D:/program/...`。
14. **profile 只有一个可编辑源 = `data/profiles/`**；`internal/profile/profiles/` 是生成镜像，改源必须跑 `sync-profiles.mjs`。
15. **不要再把 TOML 当差异化论据**；`testD` 的 TOML/HCL 仍是 `pending:`，等 M5。

## 三、下一棒（M3 结构比较）待办

- [ ] **先核对 M3 的 DoD 是否已经事实上达成**，逐条重跑并留**本轮**证据，不要照抄旧口径。现有单测：`TestFourStructuralNoiseClasses`（键重排 / 缩进 / 引号 / 注释）、`TestSetMode`、`TestKeyedDuplicateIsError`、`TestOrderedLengthDifference`、`TestPathEscaping`。
- [ ] **补 golden 覆盖 M3 的三条判据**（现在只在单测里，`selftest` 计数对账看不到）：
  - 四类结构噪声（键重排 / 缩进 / 引号 / 注释）→ 零差异条目；
  - `keyed:` 重复键 → **exit 3 且输出带路径**（不是单测断言，而是端到端）；
  - RFC 6901 转义（键含 `/` 与 `~`）→ 路径形如 `/a~1b`、`/c~0d`。
- [ ] **确认 set 模式的路径形态**：`compareSet` 产出 `p+"["+k+"]"`（k 是元素规范键，可能含 `/`），与 `value.JoinPointer` 的转义规则**不一致**。要么说明这是"集合成员标识"不是 JSON Pointer 段，要么统一——写清楚再用 golden 钉死。
- [ ] **注释零噪声的端到端证据**：解析层丢弃注释，头部写 `comments: not compared`；golden 里应有一条"只加注释"用例。
- [ ] **不要动测试D**：TOML/HCL 仍是 `SKIP`，等 M5。别提前摘 `pending:`。
- [ ] M3 收尾时：四处翻 ✅（含 `plan/06` 删缺口行），写 `plan/HANDOFF-M4.md`（七节），commit。

## 四、既定口径清单（K 编号）

| 编号 | 口径 | 落点 | 动它会打挂什么 |
|---|---|---|---|
| K-01 | **禁止浮点参与相等判定** | `internal/value`；`tests/invariants_test.go:TestNoFloatInComparisonPath` | 静态门立即变红 |
| K-02 | 输出确定性：键字节序、不读时钟/环境变量、双跑逐字节一致 | `internal/diff`、`internal/cli` | `TestOutputIsDeterministic` + CI determinism step |
| K-03 | 构建固定 `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.version"`；版本单一来源 `VERSION` | `Makefile` / CI / `internal/version` | `TestVersionSingleSource` |
| K-04 | 规范值字符串冻结为 `family:value` | `internal/value/canonical.go` | `TestCanonicalStringIsFrozen` + 所有 golden |
| K-05 | `keyed:` 重复键 = **exit 3 + 路径**，禁止取首 | `internal/diff/keyedIndex` | `TestKeyedDuplicateIsError` |
| K-06 | 默认族只推断 bool/quantity/duration/decimal；其余按 string | `internal/value/normalize.go` | `testC` 七组 + `testB` |
| K-07 | 用法错误统一 exit **3**（含未知子命令/缺参），不使用 2 | `internal/cli` | `TestExitCodeContract` |
| K-08 | golden 真值只存输入与期望，禁止 `observed_*`；`selftest` 除退出码外**还要对账计数** | `testdata/golden/manifest.json`、`internal/selftest` | 计数不符即 FAIL |
| K-09 | 默认值表外置版本化 JSON，`scope_note` 必写"非全量"，未知版本 exit 3 | `data/profiles/`、`internal/profile` | `TestUnknownProfileIsError` |
| K-10 | `indeterminate` 必带理由码；退出码优先级 `3 > 2 > 1 > 0` | `internal/exitcode`、`internal/value` | `TestExitCodePriority` |
| K-11 | 布局门「一条门里几个判据，变异清单就覆盖几个」；**变异没红先分三问**（靶子选错 / 假门 / 扫描面） | `tools/check-sprint-layout.mjs` | 见 §五 |

**K-11 累计实战记录**：① M1 收尾时 `LAYOUT.GAPS` 红，是 `plan/06 §四` 漏删已 ✅ 行（不是门假）；② M0 时"`| R-05 | 跨平台二进制验证缺口 |`" 曾被当成缺口表头，扫描面判据需锚位置+结构。

## 五、本机环境坑（只写实测过的）

| 事实（实测日期 2026-10-20） | 后果 / 处置 |
|---|---|
| `go` 不在 PATH；SDK 在仓库外 `C:\Users\<user>\go-sdk\go`（`.tmp/sdk/go` **不存在**） | 每棒先 `export PATH="/c/$USER/go-sdk/go/bin:$PATH"`；`go version` → `go1.24.5 windows/amd64` |
| `raw.githubusercontent.com` 直连 SSL 失败（curl 35）；`<local-proxy>` 代理也不通 | 取上游源码走 `api.github.com` contents API（`Accept: application/vnd.github.raw`） |
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

## 六、DoD checklist（M3）

- [ ] 键重排 / 缩进 / 引号 / 注释四类输入 → **零差异条目**（单测 + golden 端到端）
- [ ] `set` 乱序零差异；重复元素 → `indeterminate(duplicate_in_set)`
- [ ] `keyed:` 重复键 → **exit 3 且给路径**（端到端 golden，不只单测）
- [ ] 路径转义（RFC 6901）有专项用例，且 set 模式的 `[key]` 形态已写清口径
- [ ] 同输入双跑输出字节一致（`TestOutputIsDeterministic` + 手工 `cmp`）
- [ ] `node tools/gen-golden-manifest.mjs --check` exit 0（新增/删除夹具同一次改生成器）
- [ ] `node tools/check-sprint-layout.mjs` exit 0
- [ ] `go vet ./...` && `go test ./...` && `cfgdelta selftest` 全绿
- [ ] `plan/00` 里程碑表 + `plan/05` 状态表 + `README.md` 状态行 **三处一致**翻 ✅，且 `plan/06 §四` 删掉 M3 行
- [ ] `plan/HANDOFF-M4.md` 已落盘（七节）
- [ ] commit（**未接远端，不得写"CI 绿"**）

## 七、关键命令速查

```bash
# 0. 环境（本机）
export PATH="/c/$USER/go-sdk/go/bin:$PATH"
export GOFLAGS=-mod=mod GOPROXY=https://goproxy.cn,direct GOTOOLCHAIN=local

# 1. 布局门（每棒第一件事）
cd "<repo>"
node tools/check-sprint-layout.mjs

# 2. 生成物一致性
node tools/sync-version.mjs --check
node tools/sync-profiles.mjs --check
node tools/gen-golden-manifest.mjs --check

# 3. 改过 profile 源 / VERSION 之后
node tools/sync-profiles.mjs && node tools/sync-version.mjs

# 4. 测试与构建
go vet ./...
go test ./...
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(cat VERSION)" -o dist/cfgdelta.exe ./cmd/cfgdelta

# 5. golden 套件（离线、不调外部命令）
./dist/cfgdelta.exe selftest
./dist/cfgdelta.exe selftest --verbose

# 6. M2 证据点（回归用）
./dist/cfgdelta.exe diff --coerce-at 'limits/*=quantity' testdata/golden/testGrammar/e3mi-left.yaml testdata/golden/testGrammar/e3mi-right.yaml; echo $?  # → 3
./dist/cfgdelta.exe diff --coerce-at 'limits/*=quantity' testdata/golden/testC/c08-left.yaml testdata/golden/testC/c08-right.yaml; echo $?       # → 1
./dist/cfgdelta.exe diff --coerce-at 'limits/*=quantity' testdata/golden/testC/c09-left.yaml testdata/golden/testC/c09-right.yaml; echo $?       # → 2

# 7. 双跑一致性
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > /tmp/r1.txt 2>&1
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > /tmp/r2.txt 2>&1
cmp /tmp/r1.txt /tmp/r2.txt && echo BYTE_IDENTICAL

# 8. 基线守门（不需要本机装三家工具）
go test ./tests/ -run TestBaseline -v

# 9. 一并检查（Makefile 的等价手动序列；本机无 make）
node tools/check-sprint-layout.mjs && go vet ./... && go test ./... \
  && node tools/sync-version.mjs --check \
  && node tools/sync-profiles.mjs --check \
  && node tools/gen-golden-manifest.mjs --check
```
