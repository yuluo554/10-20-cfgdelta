# plan/HANDOFF-M2 — 续接交接单（下一棒：M2 归一化）

> 用法（新对话第一句）：`/goal 读取 "<项目绝对路径>\plan\HANDOFF-M2.md" 继续完成任务`
> **动手前第一件事**：`node tools/check-sprint-layout.mjs`（红了先修布局再干活）。
> 写于 2026-10-20，M1 收尾时。

## 一、当前进度与证据

M1（数据先行）已完成。下表全部是**本轮实测输出**，不是计划口径。

| 项 | 状态 | 证据（命令 → 结果） |
|---|---|---|
| 三家基线留档 | ✅ | `testdata/baseline/{dyff,diffyml,confdiff}/` 各含 `VERSION.txt` / `COMMANDS.txt` / 原样 `*.out` / `exit_code.txt`，共 **40 个 `.out`** |
| 版本实测 | ✅ | `dyff version` → `1.12.0`；`diffyml --version` → `1.8.2 (0cf8001)`；`confdiff --version` → `0.17.2` |
| 基线守门 | ✅ | `tests/baseline_archive_test.go`：**7 组变异逐组变红**（删 `.out` / 少一行 rc / VERSION 置空 / VERSION 无版本号 / 多一条无命令 rc / 命令点名别的工具 / 删 C09 留档），还原后复绿 |
| 10 条默认值逐条核对 | ✅ | `data/profiles/k8s/REVIEW-1.29.md`：每条给到上游 `文件:行`，抓出 **1 条条件性默认值**（见下） |
| profile 修正 | ✅ | `v1.29.json` 加 `only_if_image_tag_not_latest`；`internal/profile` 执行条件；镜像已 `sync-profiles`（0 drift） |
| 条件默认值回归 | ✅ | `TestImagePullPolicyDependsOnImageTag` 6 子用例；`:latest` 显式 `IfNotPresent` 从"静默判等"变成 rc=1（手测） |
| 测试A 卖点 | ✅ | 无 profile rc=1（4 条）；`--profile k8s/v1.29` → `equivalent`，**rc=0**，`hits: 14` |
| 测试A 对照组 | ✅ | `testA-ctrl` → 仍报 1 条，**rc=1**（非默认值不被抵消） |
| golden `selftest` | ✅ | `27 passed, 0 failed, 2 skipped`（TOML/HCL 显式 skip，**未提前摘 pending**） |
| 生成物一致性 | ✅ | `sync-version` / `sync-profiles` / `gen-golden-manifest` 三个 `--check` 全 exit 0 |
| 布局门 | ✅ | 8 项全 PASS；翻 ✅ 后 `LAYOUT.GAPS` 曾红一次（`plan/06` 残留 M1 缺口行），已按门的要求删行 |
| CI | 🟡 | 仍未推送 = 未跑 CI。**不得写"CI 绿"** |

**本轮从"逐条核对"里抓出的真缺陷（都已修，有回归）**：

| # | 形态 | 症状 | 修法 / 回归 |
|---|---|---|---|
| ① | **`imagePullPolicy` 是条件默认值**，不是常量 | 原来无条件补 `IfNotPresent`。左 `:latest` 无 policy / 右 `:latest` 显式 `IfNotPresent`（**真实有差异**）会被静默判等（假阴性）；反向则假阳性 | 规则加 `only_if_image_tag_not_latest`；`Apply` 读同容器 `image`，tag 为 `latest`/缺失时**不补**。`TestImagePullPolicyDependsOnImageTag` |
| ② | 三条旧口径与实测不符 | ① dyff v1.12.0 **已能读 TOML**；② `1.50` vs `1.5` **dyff 仍报差异**（原记"三家都对"）；③ confdiff 测试B 用 `-l` 是 **6 条**不是 5 条 | 全部**追加带日期的复核记录**而非改写旧档；`data/README §1.2` / `plan/01` / `plan/06` 同步加注 |
| ③ | `serviceAccountName` 记错机制 | 原 `source` 写成"API Server defaulting"，实际是 **ServiceAccount admission 插件** | `source` 改准；行为值不变 |

**已知偏差（必须如实带到下一棒）**：
- **TOML 不再是差异化论据**。`plan/06` 的多格式缺口已收窄为 HCL / INI / .env；引用"dyff 读不了 TOML"会被复验者直接打脸（证据：`testdata/baseline/dyff/D-toml-dyff.out` = `returned no differences`）。
- 三家二进制**不入库**（体积/许可）。`testdata/baseline/README.md §五` 写了复验命令；守门只校验留档自洽，不要求本机装了三家。
- `data/profiles/k8s/v1.29.json` 本轮改了字节，属**首次冻结前修订**（冻结 = M6 打 tag）。之后同类纠错必须新增 `v1.29.x`。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`profile.Rule` 多了 `OnlyIfImageTagNotLatest`**。它不是为了好看：删掉它，`TestImagePullPolicyDependsOnImageTag` 立刻红，且会重新引入"`:latest` 假阴性"。
2. **"无 tag" 等价于 `latest`**。K8s 的 `parsers.ParseImageName` 会把空 tag 归一成 `latest`，所以 `image: repo/web` 的默认策略是 `Always`。带 digest 且无 tag 才是空 tag（→ `IfNotPresent`）。
3. **基线矩阵以 `testdata/baseline/README.md §三` 为准**：dyff 在 C01–C07 **七组全误报**；diffyml 6 组；confdiff 5 组。`data/README §1.2` 与 `plan/01` 已改成这个读数，**别再改回"6 组误报、三家 c07 都对"**。
4. **`confdiff` 的条数依赖开关**：`-l` 是 6 条，`-l --array-set` 是 2 条。引用条数必须带命令行。
5. **`exit_code.txt` 的格式是 `case=rc`**，`COMMANDS.txt` 是 `case=… cwd=repo-root cmd=<tool> <args>`；`argv[0]` 必须写**工具名**，**不得写本机绝对路径**（仓库纪律）。`version` 用例的输出落点是 `VERSION.txt`，**不要再加 `version.out`**（守门显式豁免）。
6. **`yaml.v3` 的 `Decode` 根是 `DocumentNode`**；新增解析入口必须走 `doc.ParseBytes` / `doc.ParseFile`（M0 踩过：自己 `yaml.Unmarshal` 会让整份 diff 变"0 差异"而 `selftest` 不比计数时不红）。
7. **默认族推断唯一实现处 = `internal/value/normalize.go:InferFamily`**；`500m` 与 `resources` 上下文的优先级是踩过两次的雷区。
8. **`quantity` 与 `decimal` 同属有理数代数可跨族比较；duration 不参与跨族比较**。`8000m` ≡ `8` 靠这条。
9. **`indeterminate` 是条目不是错误**：`compareScalar` 里必须记录并继续，不能 `return err`（否则连 summary 都打不出）。
10. **纯书写 = `diff.CosmeticKind`**，渲染层只计数不逐条打印。
11. **分类不确定 → `ConfigChange` + `class_guess: true`，禁止归 `Cosmetic`**；`isExposurePath` 显式排除 `/protocol`。
12. **`--strict` 只升级"string 兜底且两侧书写不同"那一类**（`TestStrictUpgradesOnlyUnknownFallback` 的第二三段会红）。
13. **`plan/05` 的排期表与状态表都以 `| M1 |` 起行**，但布局门只扫「表头含『里程碑』与『状态』」那张表——给别的表加同样表头会被误纳入状态比对。
14. **翻里程碑要四处动**：`plan/00` 里程碑表 + `plan/05` 状态表 + `README` 状态行 + **`plan/06 §四` 缺口表删行**（漏最后一条，`LAYOUT.GAPS` 必红——本轮实际红了）。
15. **路径参数写不带前导 `/` 的形态**（`--at limits/memory`）。MSYS 会把前导 `/` 改写成 `D:/program/...`；README 与 manifest 已统一，别"好心"改回。
16. **profile 只有一个可编辑源 = `data/profiles/`**；`internal/profile/profiles/` 是生成镜像，改源必须跑 `sync-profiles.mjs`。

## 三、下一棒（M2 归一化）待办

- [ ] **先核对 M2 的 DoD 是否已经事实上达成**（M0 骨架里 normalize 已实现，`selftest` 的 7 组已通过）。逐条重跑并留**本轮**证据，不要照抄 M0 的口径：
  `测试C 七组 100% 判等` / `1Gi vs 1000M 判不等` / `MB vs MiB → indeterminate + 理由码` / `浮点禁令静态拦截` / `双跑逐字节一致`。
- [ ] **补一个"非法输入 → exit 3"的 golden 用例**：当前 29 例里**没有** YAML 语法错误或缺文件的解析错误用例（`exit-missing-file` 是缺文件，不是坏语法）。§4.1 要求"非法输入 → `parse_error`（退出码 3），**不降级为字符串比较**"——这条现在只靠单测，不在 golden 计数对账里。
- [ ] **补文法边界用例**（§4.1/§4.2 明写但 golden 未覆盖）：`1e3Mi` 必须非法（指数与后缀不可同现）、`.5` 前导点、`+1` 符号、`µs`/`us` 两个拼写、`0`（duration 唯一允的无单位值）。
- [ ] **确认 `-0 ≡ 0` 与 `+1 ≡ 1`** 在规范串里真的折叠（§4.3），并确认 `family:value` 冻结串没被这次 profile 改动带偏（`TestCanonicalStringIsFrozen`）。
- [ ] **不要动测试D**：TOML/HCL 仍是 `SKIP`，等 M5 适配器。**别提前摘 `pending:`**。
- [ ] M2 收尾时：四处翻 ✅（含 `plan/06` 删缺口行），写 `plan/HANDOFF-M3.md`（七节），commit。

## 四、既定口径清单（K 编号）

| 编号 | 口径 | 落点 | 动它会打挂什么 |
|---|---|---|---|
| K-01 | **禁止浮点参与相等判定** | `internal/value`；`tests/TestNoFloatInComparisonPath` | 静态门立即变红 |
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

**K-11 本轮新增一条实战记录**：翻完 M1 后 `LAYOUT.GAPS` 红了，原因是 `plan/06 §四` 还留着 `| golden 夹具与基线留档 | 未关闭 | M1：… |`。**这不是门假，是状态副本漏翻一处**——判据读的正是"缺口列里有没有已 ✅ 的里程碑号"。翻里程碑时把这一处列进 checklist（§二 第 14 条）。

## 五、本机环境坑（只写实测过的）

| 事实（实测日期 2026-10-20） | 后果 / 处置 |
|---|---|
| `go` 不在 PATH；SDK 在 `C:\Users\<user>\go-sdk\go`（仓库外） | 每棒先 `export PATH="/c/<user>/go-sdk/go/bin:$PATH"` |
| `raw.githubusercontent.com` 直连 **SSL 失败（curl 35）**；`127.0.0.1:7897` 代理也不通（curl 7） | 取上游源码改走 **`api.github.com` contents API**：`Accept: application/vnd.github.raw`，或 base64 解码 `content` 字段 |
| `kubernetes/api` 仓库**没有** `v1.29.0` tag | 用 `kubernetes-1.29.0`；`kubernetes/kubernetes` 才是 `v1.29.0` |
| `api.github.com` contents API 对 >1 MB 文件返回空 `content` | 大文件（如 `core/v1/types.go`）必须加 `Accept: application/vnd.github.raw` |
| GitHub Releases 直下可用 | dyff `dyff_1.12.0_windows_amd64.tar.gz`、diffyml `diffyml_1.8.2_windows_amd64.zip` 均一次成功 |
| confdiff 是 npm 包，`npm pack` 解出来的 `dist/cli.js` **缺依赖跑不了** | 必须 `npm install confdiff@0.17.2` 到临时目录后用 `node_modules/.bin/confdiff` |
| `proxy.golang.org` 超时；`goproxy.cn` 200 | 必须 `GOPROXY=https://goproxy.cn,direct` |
| `node v24.15.0` 可用；三个 `tools/*.mjs` 零依赖 | 不需要 `npm install` |
| `py -0p` 有 3.8 / 3.12；`python` 是商店占位符 | 脚本一律 `py -X utf8`；读中文文件用 `encoding="utf-8"` |
| Git Bash 里 `grep`/`sed` 打印中文文件会乱码 | 判定"文件坏了"前先用解析器复读；`sed -i` 改写中文文档后务必回读校验 |
| `make` **不在 PATH**（`make: command not found`） | `make check` 跑不了；等价手动序列：`node tools/check-sprint-layout.mjs` → `go vet ./...` → `go test ./...` → 三个 `--check`。HANDOFF 与 CI 里别把 `make` 当唯一入口 |
| Windows 下 `go test` 输出里的路径是反斜杠 | 断言路径时用 `filepath.Join`，不要硬编码 `/` |

## 六、DoD checklist（M2）

- [ ] 测试C 七组 100% 判等（**本轮实测输出**，不是沿用 M0）
- [ ] `1Gi` vs `1000M` 判不等；`1MiB` vs `1MB` → `indeterminate` + `ambiguous_unit_family`
- [ ] 浮点禁令静态拦截（`tests/TestNoFloatInComparisonPath`）绿
- [ ] 双跑逐字节一致（`TestOutputIsDeterministic` + 手工 `diff <(run) <(run)`）
- [ ] **新增**非法输入 golden 用例（exit 3），并进 `manifest.json` 计数对账
- [ ] **新增**文法边界用例（`1e3Mi` 非法、`.5`、`+1`、`µs`/`us`、`0`）
- [ ] `node tools/gen-golden-manifest.mjs --check` exit 0
- [ ] `node tools/check-sprint-layout.mjs` exit 0
- [ ] `go vet ./...` && `go test ./...` && `cfgdelta selftest` 全绿
- [ ] `plan/00` 里程碑表 + `plan/05` 状态表 + `README.md` 状态行 **三处一致**翻 ✅，且 `plan/06 §四` 删掉 M2 行
- [ ] `plan/HANDOFF-M3.md` 已落盘（七节）
- [ ] commit（**未接远端，不得写"CI 绿"**）

## 七、关键命令速查

```bash
# 0. 环境（本机）
export PATH="/c/<user>/go-sdk/go/bin:$PATH"
export GOFLAGS=-mod=mod GOPROXY=https://goproxy.cn,direct GOTOOLCHAIN=local

# 1. 布局门（每棒第一件事）
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

# 6. 测试A / 对照组（README 首屏卖点）
./dist/cfgdelta.exe diff testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?                  # → 1
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?  # → 0
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA-ctrl/k1.yaml testdata/golden/testA-ctrl/k2.yaml; echo $?  # → 1

# 7. 条件默认值哨兵（`:latest` 不能静默判等）
printf 'kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: w\n          image: registry.example/x:latest\n' > /tmp/l1.yaml
printf 'kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: w\n          image: registry.example/x:latest\n          imagePullPolicy: IfNotPresent\n' > /tmp/l2.yaml
./dist/cfgdelta.exe diff --profile k8s/v1.29 /tmp/l1.yaml /tmp/l2.yaml; echo $?   # → 1

# 8. 基线守门（不需要本机装三家工具）
go test ./tests/ -run TestBaseline -v

# 9. 重抓基线（需要三家二进制；用环境变量传路径，仓库不放绝对路径）
DYFF=/path/to/dyff DIFFYML=/path/to/diffyml CONFDIFF=/path/to/confdiff \
  bash tools/capture-baseline.sh
go test ./tests/ -run TestBaseline -v

# 10. 一并检查（Makefile 的等价手动序列；本机无 make）
node tools/check-sprint-layout.mjs && go vet ./... && go test ./... \
  && node tools/sync-version.mjs --check \
  && node tools/sync-profiles.mjs --check \
  && node tools/gen-golden-manifest.mjs --check
```
