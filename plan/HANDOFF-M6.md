# plan/HANDOFF-M6 — 续接交接单（下一棒：M6 发布）

> 用法（新对话第一句）：`/goal 读取 "<项目绝对路径>\plan\HANDOFF-M6.md" 继续完成任务`
> **动手前第一件事**：`node tools/check-sprint-layout.mjs`（红了先修布局再干活）。
> 写于 2026-10-20，M5 收尾时。**代码面已冻结口径，M6 只做发布，不改行为。**

## 一、当前进度与证据

M5（分类门禁 + 多格式）已完成。下表全部是**本轮实测输出**，不是计划口径。

| 项 | 状态 | 证据（命令 → 结果） |
|---|---|---|
| 测试B 口径查清 | ✅ | `plan/01 §测试B` 五行语义全等；env 的差异是元素顺序。默认 `ordered` 下按下标对齐必然 4 behavioral + 2 cosmetic（`testB-ordered`，exit 1）；`--seq-mode keyed:name` / `set` 均 `0 behavioral / 2 cosmetic`，exit 0（`testB-seq-keyed` / `testB-seq-set`）。**夹具与期望未改** |
| 六类各一例端到端 | ✅ | `classify-resource-shrink` / `classify-deploy-shrink` / `classify-image-*` / `classify-exposure` / `classify-config-remove` / `classify-cosmetic`；后两条新增（`classify-cosmetic` exit 0 / cosmetic 1，只计数不逐条列） |
| 镜像 semver 四例可见 | ✅ | 四条 golden 的 `contains` 断言 `镜像·变更/major`、`/minor`、`/patch`、`/non-semver` 出现在输出面；`TestImageSemverClassification` 保留四条单测 |
| TOML 适配器 | ✅ | `testD-toml` → 0 条目 exit 0；`[service]` 解析为表（`TestTOMLSectionIsTable`）；0x 整数转十进制原文、浮点保留原文、日期按字符串 |
| HCL 适配器 | ✅ | `testD-hcl`（属性重排）→ 0 条目 exit 0；块形态 `/resource/thing/b/...` |
| HCL 表达式 ≈ unevaluated | ✅ | `testD-hcl-unevaluated` → exit 2，输出含 `indeterminate(unevaluated_expression)` 与 `var.replicas -> var.nodes`；`TestHCLExpressionIsUnevaluated`、`TestUnevaluatedExpressions`、`TestUnevaluatedOneSidedIsIndeterminate` |
| INI / .env 适配器 | ✅ | `testD-ini` / `testD-env` → 0 条目 exit 0；值一律字符串；重复键 → exit 3 |
| 未知格式明确报错 | ✅ | `DetectFormat` 只按扩展名；无扩展名/未知扩展名 → `unsupported input format "…" (supported: …)` exit 3；`TestUnknownExtensionIsExplicitError` + `TestUnknownFormatIsUsageError` |
| golden 账 | ✅ | `cfgdelta selftest` → **76 例：76 passed, 0 failed, 0 skipped**（测试D 不再 skip） |
| 生成物一致性 | ✅ | `gen-golden-manifest --check`（76 例）/ `sync-version --check` / `sync-profiles --check` 全 exit 0 |
| `go vet` / `go test ./...` | ✅ | 全包 ok（`-count=1` 非缓存） |
| 双跑字节一致 | ✅ | 手工 `cmp` 三组（testA+profile / testD 表达式 / classify semver）逐字节相同；`TestOutputIsDeterministic` PASS |
| 首屏截图 | ✅ | `docs/testA-before-after.png`（1280×760）落盘并进 README 首屏；左 `total 4 · behavioral 2 · cosmetic 2` exit 1，右 `total 2 · behavioral 0 · cosmetic 2` exit 0 |
| 变异验证（K-11，4 组） | ✅ | ①`.toml` 派发改 JSON → `75 passed, 1 failed`（`testD-toml` exit 3）；②删 compareScalar 的 unevaluated 短路 → `75 passed, 1 failed`（`testD-hcl-unevaluated` 缺 `var.replicas -> var.nodes`）+ 两条单测红；③`semverBump` 恒返 `patch` → `73 passed, 3 failed`（major/minor/nonsemver）；④纯书写改判 ConfigChange → 大面积红（`classify-cosmetic` exit 1 + testA/testB/testC 计数）。四次均还原复绿 |
| 布局门 | ✅ | 8 项全 PASS；M5 翻 ✅ 后 `LAYOUT.STATUS` 3 处一致、`LAYOUT.GAPS` 由 5 行降到 2 行（只剩 CI 实跑 / 三平台二进制） |
| CI | 🟡 | 仍未推送 = 未跑 CI。**不得写"CI 绿"** |

**已知偏差（必须如实带到下一棒）**：
- golden 从 **66 例 → 76 例**（新增 6 条分类/纯书写 + 4 条 semver + 3 条测试D 新增 + 1 条 HCL 表达式；`testD-toml`/`testD-hcl` 由 skip 转为真跑）。
- `doc.DetectFormat` **签名变了**：`DetectFormat(path string) Format`（不再接收 `data`），因为内容嗅探正是 M4 的缺陷来源。任何新调用方必须按路径传参。
- `diff.Entry` 新增 `Detail string`；indeterminate 条目在文本输出里追加原文证据（`? path  indeterminate(reason)  detail`），JSON 增加 `detail` 字段。
- TOML 解析器是**手写子集**（表 / 数组表 / 点号键 / 基本与字面字符串 / 整数 / 浮点 / 布尔 / 数组 / 内联表 / 日期时间按字符串）。不支持的高级语法一律 exit 3，不静默降级。
- INI / .env 的**所有值都是字符串**（落树时带 quoted 样式）；不要"顺手"改成数字推断。
- HCL 块落树为 `type → label → … → body`；同名块第二次出现转成序列。表达式按**折叠空白后的原文**比较，不做任何求值。
- M1–M4 的 HANDOFF 是历史快照，本轮**未回改**；新事实只写在本文件。

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **格式分派只看扩展名**（`.yaml/.yml/.json/.toml/.ini/.env/.hcl/.tf`）。未知 = exit 3。**禁止**加回内容嗅探：`[service]` 开头的 TOML 会被送进 JSON 解析器（M4 实测缺陷）。
2. **HCL 表达式不参与求值判定**：两侧原文（`collapseHCLSpace` 折叠空白后）相同 → 不产生条目；不同 → `indeterminate(unevaluated_expression)` 且 `Detail` 带 `左 -> 右`；只有一侧存在 → 同样是 indeterminate（不是 added/removed）。这条由 `value.Normalize` 与 `diff.compareScalar` **两处**共同保证，删任一处都会打挂不同的测试。
3. **INI/.env 一律字符串**：`port = 8080` 与 `port = "8080"` 在 INI 里是同一个值；`true` 不是 bool。别为了"看起来更好"改族推断。
4. **TOML 数值不经 float64**：整数（含 0x/0o/0b）落成十进制原文，浮点保留原文；`1_000` 去下划线。日期/时间与 inf/nan 按字符串。
5. **HCL 块路径含类型**：`resource "thing" "b"` → `/resource/thing/b`。不要退回"只用 label"（`resource` 与 `data` 会互相覆盖）。
6. **纯书写夹具必须"值等、写法不同"**：只差引号（`3` vs `"3"`）在 yaml.v3 里 `Raw` 相同，不会产生 cosmetic 条目；用 `1Gi` vs `1024Mi` 这类同族异写。
7. **测试B 的口径是"键序无关的序列模式下 0 行为差异"**：`ordered` 下的 4+2 是显式模式的诚实结果，不是 bug。禁止为了凑 0 改夹具或改期望。
8. **golden 生成器有孤儿检查**：盘上任何夹具没被 case 引用 → `gen-golden-manifest` exit 1。加/删夹具必须同一次改 `tools/gen-golden-manifest.mjs`；`pending:` 前缀是 skip 标记，测试D 已全部摘掉。
9. **`contains` 是大小写敏感的字面子串**（stdout+stderr 合并），断言片段要短，且不要含版本号/绝对路径。
10. **`selftest` 的 `resolveArgs` 只改写"存在且含点"的参数**；夹具路径必须真实存在。
11. **分类排序固定**（影响类顺序 → 路径字节序）；汇总计数从明细算得。`indeterminate` 计入 indeterminate，不计 behavioral/cosmetic。
12. **`semverBump` 的边界**：三段纯数字才算 semver；`latest`、含 digest、非三段一律 `non-semver`。改它先看 `classify-image-*` 四条 golden。
13. **首屏截图是实测输出的排版**，不是终端截图。改输出格式（header / summary / 条目形态）后必须重做 `docs/testA-before-after.png`，否则 README 与真值脱节。
14. **M5 已 ✅**：`plan/06 §四` 只剩 CI 实跑与三平台二进制两条缺口。再删/加缺口行要同时过布局门。
15. **发布三件套尚未发生**：没有 tag、没有 Release、没有 SHA256SUMS。M6 之前任何文档不得出现"已发布/CI 绿"。

## 三、下一棒（M6 发布）待办

- [ ] **发布留档先落盘**：写 `plan/RELEASE-M6.md`（DoD / 偏差记录 / 三平台产物清单 / SHA256 复验命令 / 干净 clone 复跑记录），**必须在 tag 之前进 tag 树**。
- [ ] **代码面冻结**：tag 之后不改代码面；任何修补都要回到 M5 语义并重跑全门。
- [ ] 三平台构建（CI 原生）：`linux/amd64`、`darwin/arm64`、`windows/amd64`；产物 + `SHA256SUMS` 上传 Release。
- [ ] 干净环境 clone 按 README 跑通（`go build` → `selftest` → 30 秒三连命令），把实际输出贴进 `plan/RELEASE-M6.md`。
- [ ] 首推 CI 并确认绿；**未跑通前不得写"CI 绿"**（本地只有 workflow 解析）。
- [ ] 脱敏扫描：库内文档不得含会话绝对路径、机器名、私有代理地址。
- [ ] 发布后在 `plan/00` 里程碑表 + `plan/05` 状态表 + `README` 状态行 **三处一致**翻 M6 ✅，`plan/06 §一/§四` 同步关闭 CI 与三平台缺口（布局门会逐处断言）。
- [ ] 若题目要求署名：GitHub 公开仓库署名 `yuluo554`；README 只写"版本 + 能力"，发布事实放 Releases 与 `plan/`。
- [ ] 不要动已 ✅ 的 M1–M5 口径：归一化文法、序列三模式、RFC 6901、默认值顺序与 `Applied` 语义、分类表与 `unevaluated` 语义都已被 golden 钉死。

## 四、既定口径清单（K 编号）

| 编号 | 口径 | 落点 | 动它会打挂什么 |
|---|---|---|---|
| K-01 | **禁止浮点参与相等判定** | `internal/value`；`tests/invariants_test.go:TestNoFloatInComparisonPath` | 静态门立即变红 |
| K-02 | 输出确定性：键字节序、不读时钟/环境变量、双跑逐字节一致 | `internal/diff`、`internal/cli` | `TestOutputIsDeterministic` + 手工 `cmp` |
| K-03 | 构建固定 `CGO_ENABLED=0 -trimpath -ldflags "-s -w -X main.version"`；版本单一来源 `VERSION` | `Makefile` / CI / `internal/version` | `TestVersionSingleSource` |
| K-04 | 规范值字符串冻结为 `family:value`；set 的 canonical key 也由它构成 | `internal/value/canonical.go`、`internal/diff/canonical_key.go` | `TestCanonicalStringIsFrozen` + 所有 golden |
| K-05 | `keyed:` 重复键 = **exit 3 + 路径**，禁止取首 | `internal/diff/keyedIndex` | `TestKeyedDuplicateIsError` + golden `testM3-keyed-duplicate` |
| K-06 | 默认族只推断 bool/quantity/duration/decimal；其余按 string | `internal/value/normalize.go` | `testC` 七组 + `testB` |
| K-07 | 用法错误统一 exit **3**（含未知子命令/缺参/未知格式），不使用 2 | `internal/cli` | `TestExitCodeContract` + `TestUnknownFormatIsUsageError` |
| K-08 | golden 真值只存输入与期望，禁止 `observed_*`；`selftest` 对账**退出码 + summary 计数 + 可选 `contains`** | `testdata/golden/manifest.json`、`internal/selftest` | 任一项不符即 FAIL |
| K-09 | 默认值表外置版本化 JSON，未知版本 exit 3；未命中 → `degraded: structural-only` | `data/profiles/`、`internal/profile` | `TestUnknownProfileIsError`、golden `testM4-degraded-header` |
| K-10 | `indeterminate` 必带理由码（可带 `Detail` 原文）；退出码优先级 `3 > 2 > 1 > 0` | `internal/exitcode`、`internal/value`、`internal/diff` | `TestExitCodePriority`、`testD-hcl-unevaluated` |
| K-11 | 新判据「一条门里几个判据，变异清单就覆盖几个」；**变异没红先分三问**（靶子选错 / 假门 / 扫描面） | `tools/check-sprint-layout.mjs`、`internal/selftest` | 见下 |

**K-11 累计实战记录**：① M1 收尾时 `LAYOUT.GAPS` 红，是 `plan/06 §四` 漏删已 ✅ 行（不是门假）；② M0 时 `| R-05 | …` 曾被当成缺口表头，扫描面判据需锚位置+结构；③ M3 `setMemberPath` 裸拼 → selftest 精确点名失败；④ M4 四类判据逐类变异（顺序 / 施加 / 降级 / 不覆盖显式值）；⑤ **M5 四组变异**：`.toml`→JSON 派发改错 → `testD-toml` exit 3；删 compareScalar 的 unevaluated 短路 → `testD-hcl-unevaluated` 缺少 `左 -> 右` 证据；`semverBump` 恒返 `patch` → 三条 semver 用例红（patch 仍绿，符合预期）；纯书写改判 ConfigChange → 大面积红。**另有一次"靶子选错"**：只删 `value.Normalize` 的 unevaluated 分支时 selftest 仍全绿（compareScalar 短路先兜住了）——据此补了 `TestUnevaluatedOneSidedIsIndeterminate`，让该分支有直接靶子。

## 五、本机环境坑（只写实测过的）

| 事实（实测日期 2026-10-20） | 后果 / 处置 |
|---|---|
| `go` 不在 PATH；SDK 在仓库外 `C:\Users\ASUS\go-sdk\go` | shell 里 `$USER` 为空；直接写 `/c/Users/ASUS/go-sdk/go/bin` |
| **`python` 是商店占位符** | 脚本一律 `py -X utf8` |
| `py -X utf8` 有 Pillow 10.4.0；`C:\Windows\Fonts\msyh.ttc` / `consola.ttf` 可用 | `docs/testA-before-after.png` 由 `tools/make-testa-shot.py` 生成（Pillow + 系统字体；改动输出格式后必须重跑） |
| `raw.githubusercontent.com` 直连 SSL 失败；`127.0.0.1:7897` 也不通 | 取上游源码走 `api.github.com` contents API（`Accept: application/vnd.github.raw`） |
| `kubernetes/api` 仓库**没有** `v1.29.0` tag | 用 `kubernetes-1.29.0` |
| `api.github.com` 对 >1 MB 文件返回空 `content` | 大文件必须加 `Accept: application/vnd.github.raw` |
| `proxy.golang.org` 超时；`goproxy.cn` 200 | 必须 `GOPROXY=https://goproxy.cn,direct` |
| `make` 不在 PATH | `make check` 跑不了；用 §七 的等价手动序列 |
| `node v24.15.0` 可用；`tools/*.mjs` 零依赖 | 不需要 `npm install` |
| `cd /d D:\...` 在 Git Bash 报 `cd: too many arguments` | 用 `cd "D:/path"` |
| Git Bash 里 `grep`/`sed` 打印中文文件会乱码 | 判定"文件坏了"前先用解析器复读；改中文文档优先用编辑工具 |
| Windows 下 `go test` 输出路径是反斜杠 | 断言路径用 `filepath.Join` |
| `cmp` 在 Git Bash 可用 | 双跑一致性直接 `cmp a.txt b.txt` |
| `PIPESTATUS` 在 `cmd \| tail` 后取的是管道首元素退出码 | 判断 cfgdelta 的 rc 时别接管道；先重定向再 `echo $?` |
| MSYS 会把以 `/` 开头的参数改写成 `D:/program/...` | 路径参数写不带前导 `/` 的形态（`--at limits/memory`） |

## 六、DoD checklist（M6）

- [ ] `plan/RELEASE-M6.md` 在 tag **之前**落盘，含 DoD / 偏差 / 产物清单 / SHA256 / 干净 clone 复跑
- [ ] GitHub 公开仓库署名 `yuluo554`，`git log` 保留 M0–M6 完整里程碑提交
- [ ] git tag（建议 `v0.1.0`，与 `VERSION` 一致；版本单一来源 K-03）
- [ ] GitHub Release：三平台二进制（linux/amd64、darwin/arm64、windows/amd64）+ `SHA256SUMS`；本地留副本可复验
- [ ] 干净环境 clone 按 README 跑通：`go build` → `selftest`（76 passed）→ 30 秒三连命令
- [ ] CI 首推实跑并确认绿（未跑 = 不得写"CI 绿"）
- [ ] 脱敏扫描 0 命中（绝对路径 / 机器名 / 私有代理）
- [ ] `plan/00` 里程碑表 + `plan/05` 状态表 + `README` 状态行 **三处一致**翻 M6 ✅
- [ ] `plan/06 §一/§四` 关闭 CI 与三平台缺口（缺口表只留真实未关闭项）
- [ ] `node tools/check-sprint-layout.mjs` / `go vet ./...` / `go test -count=1 ./...` / `cfgdelta selftest` 全绿
- [ ] tag 之后未改代码面

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

# 3. 测试与构建
go vet ./...
go test -count=1 ./...
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(cat VERSION)" -o dist/cfgdelta.exe ./cmd/cfgdelta

# 4. golden 套件（离线、不调外部命令）
./dist/cfgdelta.exe selftest
./dist/cfgdelta.exe selftest --verbose | grep -E "testD|classify-image"

# 5. M5 证据点（回归用）
./dist/cfgdelta.exe diff testdata/golden/testD/a.toml testdata/golden/testD/b.toml; echo $?          # → 0
./dist/cfgdelta.exe diff testdata/golden/testD/a.hcl testdata/golden/testD/b.hcl; echo $?            # → 0
./dist/cfgdelta.exe diff testdata/golden/testD/a.ini testdata/golden/testD/b.ini; echo $?            # → 0
./dist/cfgdelta.exe diff testdata/golden/testD/a.env testdata/golden/testD/b.env; echo $?            # → 0
./dist/cfgdelta.exe diff testdata/golden/testD/a-expr.hcl testdata/golden/testD/b-expr.hcl; echo $?  # → 2（unevaluated_expression）
./dist/cfgdelta.exe classify testdata/golden/testGate/deploy-shrink-left.yaml testdata/golden/testGate/deploy-shrink-right.yaml; echo $?  # → 1（部署·收缩）
./dist/cfgdelta.exe classify testdata/golden/testGate/image-minor-left.yaml testdata/golden/testGate/image-minor-right.yaml; echo $?    # → 1（镜像·变更/minor）
./dist/cfgdelta.exe classify testdata/golden/testGate/cosmetic-left.yaml testdata/golden/testGate/cosmetic-right.yaml; echo $?          # → 0（cosmetic 1）

# 6. M4 回归（口径未动）
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml; echo $?       # → 0（hits: 14，cosmetic 2）
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA-ctrl/k1.yaml testdata/golden/testA-ctrl/k2.yaml; echo $?  # → 1（hits: 15）

# 7. 双跑一致性
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > .tmp/r1.txt 2>&1
./dist/cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > .tmp/r2.txt 2>&1
cmp .tmp/r1.txt .tmp/r2.txt && echo BYTE_IDENTICAL

# 8. 重新生成首屏图（改了输出格式才需要；.tmp/ 已 gitignore）
py -X utf8 tools/make-testa-shot.py

# 9. 一并检查（Makefile 的等价手动序列；本机无 make）
node tools/check-sprint-layout.mjs && go vet ./... && go test -count=1 ./... \
  && node tools/sync-version.mjs --check \
  && node tools/sync-profiles.mjs --check \
  && node tools/gen-golden-manifest.mjs --check
```
