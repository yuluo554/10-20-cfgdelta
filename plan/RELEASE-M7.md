# plan/RELEASE-M7 — 发布留档（v0.1.1，缺陷修复与再发布）

> 本文件**在 tag 之前**落盘并进 tag 树（同 M6 的纪律）。tag 之后才产生的事实
> （附件 SHA256、真下载复核、Release URL、CI run id）**只**追加到 §二，§一 不回改。
> 口径：`plan/00 §四` 的发布三件套 + `plan/05 §二 M7`。
>
> **与 M6 的关系**：`v0.1.0` 的 tag 与 Release **原样保留、不移动**（`plan/RELEASE-M6.md §2.3`
> 已明确反对移动已发布的 tag —— 那会让「tag = 产出二进制的源码」这条保证失效）。
> v0.1.1 是一次**追加**发布：修的是「门本身」与「对外表述」，产品行为面零改动。

## 一、tag 前事实

### 1.1 这一版修了什么（全部来自二次迭代的只读诊断）

诊断交接物在 `update/`（不入库，见 `.gitignore`）。编号沿用诊断报告的 D1–D11。

| 缺陷 | 级别 | 处置 | 落点 |
|---|---|---|---|
| **D1** 「布局门 9 组变异验证全红」是声明、没有实物 | P1 | 新建 `tools/verify-layout-mutations.mjs`；8 条判据各一条故障 + 3 条反向回归 + 1 条覆盖度断言；接入 `make check` 与 CI `layout` job | `tools/verify-layout-mutations.mjs`、`Makefile`、`.github/workflows/ci.yml` |
| **D2** `LAYOUT.GAPS` 扫的是最后一列（目标里程碑），方向与「缺口格」语义相反 | P1 | 改扫第 1 列（`cells[1]`），保留「表头首格必须是『缺口』」锚点 | `tools/check-sprint-layout.mjs` |
| **D3** 缺口表为空时扫 0 行仍判 PASS（空转门） | P1 | 引入三态，0 数据行降级为 `SKIP` 并显式说明 | 同上 |
| **D7** `LAYOUT.WRAPUP` 未适用时给 PASS + SKIP 文案 | P2 | 同上，未适用时用 `SKIP` 状态词 | 同上 |
| **D10** 「9 组变异」与实际 8 条判据不符（M7/M8 打同一条判据） | P3 | 四处文档统一成实际条数，并改为指向实物脚本 | `plan/00`、`plan/04`、`plan/05`、`tools/check-sprint-layout.mjs` 文件头 |
| **D4** 发布版二进制的 `selftest` 不能在仓库外运行，而 Release 正文当卖点 | P1 | **改口径（DEC2-A）**：Release 正文（CI 生成器）+ `README` + `--help` 全部注明「必须在一份 checkout 里跑」；不动产品、不重打包 | `.github/workflows/ci.yml`、`README.md`、`internal/cli/cli.go` |
| **D8** `AGENTS.md` 的占位符让「照做」不可执行 | P2 | 换成可执行表述，并写明「代理地址不入库」的原因 | `AGENTS.md` |
| **D9** 脱敏门会扫 `update/`（三仓里唯一如此） | P2 | **DEC3-A**：`SKIP_DIRS` 加 `update`，`.gitignore` 加 `update/` | `tools/check-desensitized.mjs`、`.gitignore` |
| **D5** tag 树自述「M6 ⬜ 未开始」，而它就是 v0.1.0 的发布物 | P2 | HEAD 上 M6 早已 ✅；本版把 M7 一并落进三处状态副本，新 tag 树自洽 | `README.md`、`plan/00`、`plan/05` |
| **D6** `SHA256SUMS` 曾在本机被覆盖，靠 CI 字节复原，无自动防线 | P2 | 本版**未做自动化**（`gh api` 需要网络，不适合放 CI）；改为在 §1.5 写死手工对撞清单 | 本文件 §1.5 |

**产品面（`cmd/ internal/ data/ testdata/ go.mod go.sum`）相对 `v0.1.0` 的唯一改动**
是 `internal/cli/cli.go` 里的 `--help` 文案（「bundled golden suite」→「shipped in
testdata/golden/，未嵌入二进制」）。**没有任何比较/归一化/退出码行为被改动。**

### 1.2 DoD 对账（tag 前可验部分）

| DoD 项 | 状态 | 证据（本轮实测） |
|---|---|---|
| `plan/RELEASE-M7.md` 在 tag 前落盘 | ✅ | 本文件 |
| 版本单一来源 = `VERSION`（K-03） | ✅ | `VERSION` = `0.1.1`；`internal/version/VERSION` 逐字节镜像；`node tools/sync-version.mjs --check` → `root=0.1.1 mirror=0.1.1`，exit 0 |
| 布局门全绿 | ✅ | `node tools/check-sprint-layout.mjs` → `结论: 全绿（PASS 7 / SKIP 1 / FAIL 0，共 8 条判据）`，exit 0。唯一的 SKIP 是 `LAYOUT.GAPS`（缺口表 0 数据行，合法状态） |
| 布局门变异验证有实物且全过 | ✅ | `node tools/verify-layout-mutations.mjs` → `结论：8/8 组变异如期变红 + 4/4 条回归断言成立`，exit 0 |
| 脱敏门 0 命中 | ✅ | `node tools/check-desensitized.mjs` → `脱敏门：扫了 233 个文本文件，0 命中`，exit 0 |
| 三件生成物一致 | ✅ | `sync-profiles --check` → `1 profile(s) scanned, 0 drift`；`gen-golden-manifest --check` → `76 cases, manifest is up to date`，均 exit 0 |
| `go vet` 干净 | ✅ | `go vet ./...` exit 0（go1.24.5） |
| `go test ./...` 全绿 | ✅ | 7 个有测试的包全 `ok`，4 个包无测试文件，exit 0（104 个 `=== RUN`） |
| `selftest` | ✅ | `selftest: 76 passed, 0 failed, 0 skipped`，exit 0 |
| 产品红线未动 | ✅ | `tests/invariants_test.go`（禁浮点静态拦截）绿；退出码 0/1/2/3 与 `3 > 2 > 1 > 0` 优先级未改；`data/README.md §四` 数值口径未改；`manifest.json` 76 例期望值未改 |
| git tag `v0.1.1` | ⏳ tag 后 | §2.1 |
| Release 附三平台产物 + `SHA256SUMS` | ⏳ tag 后 | §2.2 |
| 真下载 `sha256sum -c` 复验 | ⏳ tag 后 | §2.4 |

### 1.3 发布产物清单（预期；实际附件以 §二 为准）

| 附件 | 构建 runner | 构建方式 |
|---|---|---|
| `cfgdelta-linux-amd64` | `ubuntu-latest` | 原生 `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=0.1.1"` |
| `cfgdelta-darwin-arm64` | `macos-latest` | 同上，原生 arm64 |
| `cfgdelta-windows-amd64.exe` | `windows-latest` | 同上 |
| `SHA256SUMS` | `ubuntu-latest` | `sha256sum cfgdelta-*`（无路径前缀，`sha256sum -c` 可直接用） |

`release-build` 的门：`VERSION`（去空白）必须等于 tag 去 `v` 后的值，不等即失败。

**`selftest` 的 cwd 前提（D4 的口径修正，本版重点）**：`release-build` 的
"Native smoke" 步骤在**仓库根**跑 `selftest`，所以能过；但下载者把单文件放到任意目录后
运行会 exit 3。**这不是新发现的行为**（`locateGolden` 的查找顺序未改），只是本版把这件事
写进了 Release 正文与 README：

```text
# 会失败（二进制在仓库外，且 CWD 也不是仓库）
$ /tmp/cfgdelta-linux-amd64 selftest
cfgdelta: selftest: testdata/golden not found (searched 14 locations from CWD / executable)
exit 3

# 会成功（在一份 checkout 里跑）
$ cd <checkout of v0.1.1> && ./cfgdelta-linux-amd64 selftest
selftest: 76 passed, 0 failed, 0 skipped
exit 0
```

**注意**：`--root` **不是** CLI 旗标。`Env.Root` 只是测试注入点；命令行传 `--root` 会得到
`cfgdelta: selftest: unknown flag --root` + exit 3（本轮实测）。所以口径只能写"在 checkout 里跑"。

### 1.4 偏差记录（如实，不裁剪）

1. **诊断报告建议的 `--root <repo>` 口径不成立**：诊断（D4）与迭代路线（批 3）都写了
   「加 `--root <repo>` 说明」，但实测 `--root` 不是 CLI 旗标。本版按实测改写为
   "在 checkout 里跑"，没有为了让文档好看而去实现一个新旗标（那属于改产品，DEC2 选的是 A）。
2. **`RELEASE_MILESTONE` 曾被硬编码为 6**：本版把它改成「最高 ✅ 里程碑」推导，
   否则新增 M7 会让 `LAYOUT.WRAPUP` 永远检查 M6。改动本身由变异脚本 V8 + 覆盖度断言 R3 看住。
3. **`verify-layout-mutations.mjs` 在 Windows 上不能用 `tar -C <盘符路径>`**：git-bash 的
   `tar` 会把 `C:\...` 当远程主机名（`Cannot open: No such file or directory`）。改为给
   `tar` 传 `cwd` 而非 `-C`。这条只在 Windows 复现，CI 的 ubuntu 上两种写法都行。
4. **变异脚本必须把工作树的未跟踪文件也覆盖进导出树**：否则「文档点名了新脚本、树里没有」
   会让 `LAYOUT.DOC_TOOLS` 假红，8 条变异全被误伤。已改为 tracked + `git ls-files --others`。
5. **`plan/05:23` 与 `plan/00:99` 的「9 组变异」是错误声明，不是笔误**：文件头列了 M1–M9，
   其中 M7 与 M8 打的是同一条判据（WRAPUP），所以"9 组"在原理上就不可能等于"一条判据一组"。
   本版把数字改成 8 并附实物；**没有**为了让旧声明成立去补第 9 条变异。
6. **`plan/RELEASE-M6.md` 与 `plan/HANDOFF-M1.md` 里的历史数字保留**：前者是 tag 树里的
   历史留档，后者是当时的交接快照，只在 HANDOFF-M1 那一行追加了更正标注；不重写历史。

### 1.5 附件与本地副本的手工对撞清单（D6 的处置：不做自动化，做清单）

`gh api` 需要网络与凭据，放进 CI 会让门依赖外部服务（本仓 `check` 的定位是"无网络"）。
所以这一条留成发版后人工执行的清单，**每次发版都要走一遍**：

```bash
# 1) 本地副本 vs GitHub digest（逐字符）
gh api repos/yuluo554/10-20-cfgdelta/releases --jq '.[0].assets[] | "\(.name)  \(.size)  \(.digest)"'
sha256sum dist-release/*

# 2) 真下载到全新空目录 + 标准校验
TMP=$(mktemp -d); cd "$TMP"
gh release download v0.1.1 -R yuluo554/10-20-cfgdelta
sha256sum -c SHA256SUMS; echo "exit=$?"     # 期望三行 OK / exit 0

# 3) 单文件 sha256 与 SHA256SUMS 那一行逐字符相同（不依赖 SHA256SUMS 的形态）
sha256sum -c SHA256SUMS --ignore-missing
```

### 1.6 被 tag 的提交

- tag 对象 `7fbb327`（annotated）→ 指向提交 `69e3529e3a3000b19b4fcefc701ca0f3e6d20882`
- tag 前的 main 两个提交：
  - `06cad17` M7：修布局门本身 + 补变异验证实物 + 对齐 selftest 对外表述（v0.1.0 → v0.1.1）
  - `69e3529` verify-layout-mutations：在没有 .git 的源码包/导出树里也能跑
- tag 树**包含本节以外的全部 §一 内容**（含 `plan/RELEASE-M7.md` 自身、`plan/HANDOFF-M7.md`、`tools/verify-layout-mutations.mjs`）

## 二、tag 后追加

### 2.1 tag

- 本地：`v0.1.1`（annotated，对象 `7fbb327` → 提交 `69e3529`）
- 远端与本地一致；`v0.1.0`（对象 `4834c47` → `de5599eb`）**未动**
- push 通道：HTTPS 直连与本地代理均失败（`Recv failure: Connection was reset`），
  改走 `ssh://git@ssh.github.com:443/...` 成功；main 推送 `4dc84d1..69e3529`

### 2.2 Release

- `cfgdelta 0.1.1` · **非 draft、非 prerelease** · published `2026-10-23T04:28:13Z`
- 附件 4 个：

| 附件 | 字节数 | digest（GitHub） |
|---|---|---|
| `cfgdelta-darwin-arm64` | 2,981,586 | `66f298788a13c31566d4bb7c1973a0b0a4446856b5004a4ae46dea58ebf62461` |
| `cfgdelta-linux-amd64` | 3,023,032 | `fd68b6e652fab51bc014808adc4017cb0eb1d6014dee010104ab2b0460c0bc90` |
| `cfgdelta-windows-amd64.exe` | 3,172,864 | `afdfe383eee945e215afa220eaa60733a284564b0e135f34dc51e1951dfd3dfb` |
| `SHA256SUMS` | 268 | `5d6e604ea0ada4c60c896a97f2d7f6e4daa6b9373e34a438f87fefaa94ca6f35` |

> `SHA256SUMS` 的三行与上表三个二进制 **逐字符相同**，且**无路径前缀**（`sha256sum -c` 可直接用）。

- **正文已带 D4 的 cwd 说明**（这是本版要修的东西之一）：明写 `selftest` 必须在一份 checkout 里跑、
  仓库外 exit 3、以及「没有 `--root` 旗标」。

### 2.3 tag run 实跑结果

- run `39900427662`（`push`, tag `v0.1.1`）→ **success**，9 个 job 全 success：
  `Sprint layout gate`（9 steps，**含新增的变异验证步骤**）/ `Generated artefacts in sync` /
  `test (ubuntu|macos|windows-latest)` / `release build (三个 asset)` / **`publish release`**
- 对照 `v0.1.0`：那次同一条 `publish release` job 是**失败**的（`open RELEASE_NOTES.md: no such file or directory`，
  见 `plan/RELEASE-M6.md §2.3`）。本版该 job 首次实跑成功，说明 M6 的修正（把 notes 写到
  `$GITHUB_WORKSPACE`）是有效的。—— 这是**顺带被验证**的，不是本版的目标。
- 另一条 push run `39900249742`（提交 `69e3529`）→ success，5 个 job 全 success
  （`release-build` / `publish` 按 `if: startsWith(github.ref, 'refs/tags/v')` 正确跳过）

### 2.4 真下载复核（在全新空目录）

```text
$ TMP=$(mktemp -d); cd "$TMP"
$ gh release download v0.1.1 -R yuluo554/10-20-cfgdelta
$ ls -l
268       SHA256SUMS
2981586   cfgdelta-darwin-arm64
3023032   cfgdelta-linux-amd64
3172864   cfgdelta-windows-amd64.exe

$ sha256sum -c SHA256SUMS
cfgdelta-darwin-arm64: OK
cfgdelta-linux-amd64: OK
cfgdelta-windows-amd64.exe: OK
exit 0
```

本地 `dist-release/` 已按新附件刷新，`cd dist-release && sha256sum -c SHA256SUMS` → 三行 OK / exit 0；
四个文件的 sha256 与 GitHub digest **逐字符一致**（四方对撞：SHA256SUMS 行 / GitHub digest / 真下载 / 本地副本）。

### 2.5 D4 行为实测（下载到的二进制，两个方向）

```text
# ① 仓库外（空目录）—— 与 README / Release 正文一致
$ cd /tmp/<下载目录> && ./cfgdelta-windows-amd64.exe selftest
cfgdelta: selftest: testdata/golden not found (searched 14 locations from CWD / executable)
exit 3

$ ./cfgdelta-windows-amd64.exe --version
cfgdelta 0.1.1
exit 0

# ② 一份 checkout 里（同一个二进制，只是换了 cwd）
$ cd <repo> && /tmp/<下载目录>/cfgdelta-windows-amd64.exe selftest
selftest: 76 passed, 0 failed, 0 skipped
exit 0
```

**结论**：产品行为没变（`locateGolden` 一字未改），变的是对外表述现在与事实一致。

### 2.6 干净环境复验（两棵无 `.git` 的导出树 + 两个 Go 版本）

| 检查 | go1.24.5 | go1.24.7（`GOTOOLCHAIN`） |
|---|---|---|
| `go vet ./...` | exit 0 | exit 0 |
| `go test ./... -count=1` | exit 0（7 包 ok / 4 包无测试） | exit 0（7 包 ok） |

在同一棵无 `.git` 的导出树上：
`check-sprint-layout` exit 0（PASS 7 / SKIP 1）／`verify-layout-mutations` **8/8 + 4/4** exit 0
（脚本会打印"这里没有 .git，按源码包形态自验"）／`check-desensitized` 235 文件 0 命中 exit 0／
`sync-version` exit 0／`sync-profiles` exit 0／`gen-golden-manifest` 76 cases exit 0；
`CGO_ENABLED=0 go build` → `--version` = `cfgdelta 0.1.1`、`selftest` = 76 passed exit 0。

## 三、覆盖对账

| 要求来源 | 要求 | 落点 | 覆盖状态 |
|---|---|---|---|
| `plan/00 §四` M7 DoD | ① git tag | `v0.1.1` → `69e3529`（§2.1） | ✅ |
| `plan/00 §四` M7 DoD | ② Release 附三平台二进制 + `SHA256SUMS`，本地留副本可复验 | 附件四件（§2.2）+ 真下载与本地双重复核（§2.4） | ✅ |
| `plan/00 §四` M7 DoD | ③ 发布留档在 tag 前进 tag 树 | 本文件（§1.6；tag 树含本文件） | ✅ |
| `plan/05 §二 M7` | D1/D2/D3/D7/D10 全部修复且各有可复跑证据 | §1.1 + §1.2 + §2.6（变异脚本 8/8 + 4/4，两棵干净树 + 双 Go 版本） | ✅ |
| `plan/05 §二 M7` | D4 对外表述与事实一致 | §1.3 + §2.2（Release 正文带 cwd 说明）+ §2.5（两个方向实测）+ `README.md`「Develop」+ `--help` | ✅ |
| `plan/05 §二 M7` | 产品行为面零改动 | §1.1 末段（唯一产品面改动是 `--help` 文案）；§2.5 证实 `locateGolden` 行为未变 | ✅ |
| `plan/00 §五` 纪律 | 未实测的不写成"实测"；未跑通不写"CI 绿" | §1.2 / §2.3 每行都带 run id、命令与退出码；§二 之前不写"已发布" | ✅ |
| `plan/06 §四` 缺口「布局门无可复跑变异脚本」 | 补实物 | `tools/verify-layout-mutations.mjs`，接入 `make check` 与 CI（§2.3 的 layout job 9 steps 含它） | ✅ |
| `plan/05 §四` 已知未验证项 | macOS/Linux 二进制 | 本机只有 windows/amd64；其余由 CI 原生构建并各自跑 `selftest`（§2.3）；本机不声称原生验证 | ✅ 口径一致 |
