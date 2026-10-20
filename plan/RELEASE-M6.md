# plan/RELEASE-M6 — 发布留档（v0.1.0）

> 本文件**在 tag 之前**落盘并进 tag 树（M6 DoD 第三件）。tag 之后才产生的事实
> （附件 SHA256、真下载复核、Release URL）**只**追加到 §二，§一 不回改。
> 口径：`plan/00 §四` 的 M6 三件套 + `plan/05 §二 M6` + `plan/HANDOFF-M6.md §六`。

## 一、tag 前事实

### 1.1 DoD 对账（tag 前可验部分）

| DoD 项 | 状态 | 证据（本轮实测） |
|---|---|---|
| `plan/RELEASE-M6.md` 在 tag 前落盘 | ✅ | 本文件；tag 指向的提交包含本文件（§1.7 记录被 tag 的 commit） |
| 版本单一来源 = `VERSION`（K-03） | ✅ | `VERSION` = `0.1.0`；`internal/version/VERSION` 逐字节镜像；`node tools/sync-version.mjs --check` → `root=0.1.0 mirror=0.1.0` exit 0 |
| CI 首推实跑并确认绿 | ✅ | 三平台 `test` + `layout` + `consistency` 全 success（§1.5，含两次真实失败的处置） |
| 发布 workflow 具备三平台原生构建 + `SHA256SUMS` + 附件上传 | ✅ | `.github/workflows/ci.yml` 的 `release-build` / `publish`；`TestWorkflowParses` 钉住产物名 / `sha256sum` / `contents: write`，3 组变异各自变红后还原 |
| 干净环境 clone 按 README 跑通 | ✅ | §1.6（`go build` → `selftest` 76 passed → 30 秒三连命令，全部贴实际输出） |
| 脱敏扫描 0 命中 | ✅ | `node tools/check-desensitized.mjs` → 扫 231 个文本文件 0 命中；注入 3 处真命中后 exit 1 并逐行点名，删除后复绿 |
| 本地全门绿 | ✅ | 布局门全绿 / `go vet ./...` 干净 / `go test -count=1 ./...` 全包 ok / `selftest` 76 passed 0 failed 0 skipped / 三件生成物 `--check` 全 exit 0 |
| git tag `v0.1.0` | ⏳ tag 后 | §二 |
| Release 附三平台产物 + `SHA256SUMS` | ⏳ tag 后 | §二 |
| 本地留副本 + SHA256 可复验 | ⏳ tag 后 | §二 |

### 1.2 发布产物清单（预期；实际附件以 §二 为准）

| 附件 | 构建 runner | 构建方式 |
|---|---|---|
| `cfgdelta-linux-amd64` | `ubuntu-latest` | 原生 `CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=0.1.0"` |
| `cfgdelta-darwin-arm64` | `macos-latest`（macos-26-arm64） | 同上，原生 arm64 |
| `cfgdelta-windows-amd64.exe` | `windows-latest` | 同上 |
| `SHA256SUMS` | `ubuntu-latest` | `sha256sum cfgdelta-*` |

三条产物在各自平台上**先跑 `--version` 与 `selftest` 再上传**（`release-build` 的
"Native smoke" 步骤），不是"构建完就发"。`release-build` 另有一条门：
`VERSION` 去掉空白后必须等于 tag 去掉 `v` 后的值，不等直接失败（K-03 的发布侧落实）。

### 1.3 SHA256 复验命令（拿到附件后照着敲）

```bash
# 下载四个附件到同一目录，然后：
sha256sum -c SHA256SUMS

# 单文件对照（不依赖 SHA256SUMS 的形态）：
sha256sum cfgdelta-linux-amd64 cfgdelta-darwin-arm64 cfgdelta-windows-amd64.exe
```

macOS 上没有 `sha256sum` 时用 `shasum -a 256 -c SHA256SUMS`。
Windows 上用 `certutil -hashfile cfgdelta-windows-amd64.exe SHA256` 取单文件值再比对。

### 1.4 偏差记录（如实，不裁剪）

| # | 偏差 | 处置 |
|---|---|---|
| 1 | `VERSION` 由 `0.1.0-dev` 改为 `0.1.0` | 发布版本号必须与 tag 一致；`sync-version.mjs` 同步镜像，`TestVersionSingleSource` 仍绿 |
| 2 | CI `test` 矩阵**新增** `macos-latest`（原为 ubuntu + windows） | 发布产物含 `darwin/arm64`；只在 tag 上才发现 macOS 构建失败会逼着重新打 tag。代价是每次 push 多一个 job |
| 3 | **首推 CI 三平台同一处红**：`Determinism` 步骤 | 根因：`classify testA`（无 profile）按契约返回 **1**，而该 step 用了 `set -euo pipefail`，第一条命令就中止，`cmp` 根本没跑。**这是本地没验过的面**——本地只跑了二进制，没跑过这段 shell。修正：分别捕获两次退出码 → 断言 `rc1 == rc2` → 再 `cmp`。修正提交 `1fa0724`，复跑三平台全绿 |
| 4 | **macOS job 第一次复跑失败于基础设施**，非代码 | 标注原文：`The job was not acquired by Runner of type hosted even after multiple attempts` + `Due to capacity constraints, jobs targeting macOS arm64 runners may experience longer queue times`。`gh run rerun --failed` 后 success（36s）。**不计为代码缺陷，也不隐藏** |
| 5 | 脱敏门首次跑出 7 处命中 | 全是历史 HANDOFF 里的本机路径 / 用户名 / 回环代理。按 `<repo>` / `<user>` / `<local-proxy>` 占位符改写；判据本身也调整过一次（`C:\Users\<user>` 这类占位符必须放行，否则门会把"已脱敏的写法"判成违规） |
| 6 | `.gitignore` 新增 `/dist-release/` | Release 附件本地留副本的落点；`*.zip` 等压缩包仍不入库 |
| 7 | `plan/HANDOFF-M2…M5` 被改动 | **只改脱敏字样**，不改任何事实陈述；M1–M5 的历史结论未被回改 |
| 8 | golden 仍是 **76 例**（M5 末态） | M6 不改行为面，不新增用例；`gen-golden-manifest --check` 报 `76 cases, manifest is up to date` |

### 1.5 CI 实跑记录（首推，2026-10-20）

| run | commit | 结果 | 说明 |
|---|---|---|---|
| `39435342194` | `5430c75` | ❌ failure | `test` 三平台全红于 `Determinism`（偏差 #3）；`layout` / `consistency` 绿 |
| `39436267047` | `1fa0724` | ✅ success | `Sprint layout gate` / `Generated artefacts in sync` / `test (ubuntu-latest)` / `test (windows-latest)` / `test (macos-latest)` 全 success；`release-build` / `publish release` 按 `if: startsWith(github.ref, 'refs/tags/v')` **skipped**（预期） |

被 tag 的提交与上面第二行同源（§1.7）。**"CI 绿"指的是 `1fa0724` 这次 push 的实跑结论**，
不是 workflow 解析结论；`release-build` / `publish` 两个 job 的首次真实执行发生在 tag 上，
其结论记在 §二。

### 1.6 干净 clone 复跑记录（按 README 逐条）

clone 自公开远端（`https://github.com/yuluo554/10-20-cfgdelta`），落在仓库外的临时目录
（路径以 `<clone>` 代称，`.tmp/` 已被 `.gitignore` 排除）。

**① `go build`（README §Install 的命令原样）**

```
$ go version
go version go1.24.5 windows/amd64

$ CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o cfgdelta.exe ./cmd/cfgdelta
$ ./cfgdelta.exe --version
cfgdelta 0.1.0
```

产物 3,171,840 字节（Windows/amd64，`-s -w` 后）。

**② `selftest`（README 承诺 76 例）**

```
$ ./cfgdelta.exe selftest
selftest: 76 golden cases from <clone>/testdata/golden
scope: Covers the four red-teamed families (A/B/C/D), every one of the 10 defaulting rules in data/profiles/k8s/v1.29.json (each rule has a cancel case plus an explicit non-default control), the unmatched-profile degraded header, the §4.1/§4.2 grammar boundaries, the M3 structural-comparison cases (four noise classes / sequence modes / RFC 6901 paths), the M5 impact classes (six classes end-to-end + four semver sub-classes), and the M5 format adapters (TOML / HCL / INI / .env, HCL expressions as unevaluated). NOT full Kubernetes semantics.
selftest: 76 passed, 0 failed, 0 skipped
$ echo $?
0
```

（首行原本打印 clone 的绝对路径，此处按脱敏纪律替换为 `<clone>`；其余逐字未改。）

**③ 30 秒三连命令（README §30-second tour 原样）**

```
$ ./cfgdelta.exe normalize --at limits/memory --family quantity testdata/golden/testC/c01-left.yaml
quantity:1073741824/1
$ echo $?
0

$ ./cfgdelta.exe diff testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml
cfgdelta 0.1.0 · profile none (hits: n/a) · seq-mode ordered · comments: not compared
- /spec/template/spec/containers/0/imagePullPolicy  string:IfNotPresent  [配置·新增·移除 (class_guess)]
- /spec/template/spec/containers/0/ports/0/protocol  string:TCP  [配置·新增·移除 (class_guess)]
summary: total 4 · behavioral 2 · cosmetic 2 · indeterminate 0
$ echo $?
1

$ ./cfgdelta.exe diff --profile k8s/v1.29 testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml
cfgdelta 0.1.0 · profile k8s/v1.29 (hits: 14) · seq-mode ordered · comments: not compared
equivalent (no behavioural differences)
summary: total 2 · behavioral 0 · cosmetic 2 · indeterminate 0
$ echo $?
0
```

**④ `gate` / `classify`（第 4 条命令的实体版 + 分类可见性）**

```
$ ./cfgdelta.exe gate --fail-on 资源·收缩,接口·暴露 testdata/golden/testGate/deploy-shrink-left.yaml testdata/golden/testGate/deploy-shrink-right.yaml
cfgdelta 0.1.0 · profile none (hits: n/a) · seq-mode ordered · comments: not compared
~ /spec/minReadySeconds  decimal:0/1 -> decimal:30/1  [部署·收缩]
summary: total 1 · behavioral 1 · cosmetic 0 · indeterminate 0
$ echo $?
0
```

`--fail-on` 只列了「资源·收缩」「接口·暴露」，本例命中的是「部署·收缩」——**输出照常、不阻断**，
退出码 0。这是 `gate` 选择性阻断语义在干净 clone 里的直接复现。

```
$ ./cfgdelta.exe classify testdata/golden/testGate/image-minor-left.yaml testdata/golden/testGate/image-minor-right.yaml
cfgdelta 0.1.0 · profile none (hits: n/a) · seq-mode ordered · comments: not compared
~ /spec/template/spec/containers/0/image  string:registry.example/synth/web:1.2.3 -> string:registry.example/synth/web:1.3.0  [镜像·变更/minor]
summary: total 1 · behavioral 1 · cosmetic 0 · indeterminate 0
$ echo $?
1
```

**⑤ 双跑字节一致（干净 clone 内）**

```
$ ./cfgdelta.exe classify testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > r1.txt 2>&1
$ ./cfgdelta.exe classify testdata/golden/testA/k1.yaml testdata/golden/testA/k2.yaml > r2.txt 2>&1
$ cmp r1.txt r2.txt && echo BYTE_IDENTICAL
BYTE_IDENTICAL
```

### 1.7 被 tag 的提交

tag `v0.1.0` 指向 `main` 上包含本文件的提交；该提交的 SHA 记在 §二（tag 后回填，
因为"包含本文件的提交"在写本文件时还不存在）。tag 前的代码面末态：

| 项 | 值 |
|---|---|
| 代码面末态提交（tag 前最后一笔代码提交） | `1fa0724` |
| 该提交的 CI 实跑 | ✅ 三平台全绿（§1.5） |
| `VERSION` | `0.1.0` |
| golden 例数 | 76（76 pass / 0 skip） |
| 第三方依赖 | `gopkg.in/yaml.v3 v3.0.1`（唯一一个） |

### 1.8 脱敏扫描（发布前）

```
$ node tools/check-desensitized.mjs
脱敏门：扫了 231 个文本文件，0 命中
$ echo $?
0
```

判据四条：盘符绝对路径（`<repo>` / `<user>` 占位符放行）、MSYS 用户路径、本机用户名、
回环代理地址。**公开镜像域名（`goproxy.cn`）不在判据内**——它是公开镜像，不是私有代理。
门自身豁免自己（它必须写出被判据禁止的字面量），变异验证在别的文件上做：注入 3 处真命中
→ exit 1 并逐行点名 → 删除后复绿。

## 二、tag 后追加

> 本栏**只追加**，不回改 §一。内容为 tag / Release / 附件 / 真下载复核 / 状态翻转。

### 2.1 tag

| 项 | 值 |
|---|---|
| tag | `v0.1.0`（annotated） |
| tag 指向的提交 | `de5599eb761f39cc0be32c7e893b4b87b80403c2` |
| tag 提交的 CI run | `39439782328`（见 2.3） |
| `VERSION` @ tag | `0.1.0`（与 tag 去 `v` 后一致，`release-build` 的版本门实测通过） |

### 2.2 Release

| 项 | 值 |
|---|---|
| URL | https://github.com/yuluo554/10-20-cfgdelta/releases/tag/v0.1.0 |
| 状态 | 已发布（`draft: false` / `prerelease: false`），2026-10-20T14:33:52Z |
| 附件数 | 4（三平台二进制 + `SHA256SUMS`） |

附件与 GitHub 侧计算的摘要（`gh release view --json assets` 实测）：

| 附件 | 字节 | sha256 |
|---|---|---|
| `cfgdelta-darwin-arm64` | 2,981,586 | `cf517390eacb5b1021d983850eee423e47a9e8ade4f8ce0d6ea4684d9584b093` |
| `cfgdelta-linux-amd64` | 3,023,032 | `fec4408c33305e52ac3cf8207227c260af7d7c7e13e9bbf7c90d6a1902fceaff` |
| `cfgdelta-windows-amd64.exe` | 3,172,352 | `0b3fe14d98bf468343004da8bac1f47946e2b8f7dfc99194dfe23b6f5c59c670` |
| `SHA256SUMS` | 268 | `da5487f4f541873ae8273da9152600e179a2406785ca1917cf207ad34feaecf4` |

三个二进制的 sha256 与 tag run `39439782328` 的 `Checksums + manifest` 步骤打印值**逐字相同**
（该步骤日志里的三行就是上表前三行）——即"上传的就是那三个 runner 上原生构建并跑过
`selftest` 的产物"，中间没有重新打包。

### 2.3 tag run 实跑结果与处置（如实记录）

`39439782328`（`v0.1.0`，`de5599eb`）：

| job | 结果 |
|---|---|
| `Sprint layout gate` | ✅ |
| `Generated artefacts in sync` | ✅ |
| `test (ubuntu-latest)` / `test (windows-latest)` / `test (macos-latest)` | ✅ 全绿（macOS 24s，本次未再遇容量问题） |
| `release build (cfgdelta-linux-amd64)` | ✅ 原生构建 + `--version` + `selftest` |
| `release build (cfgdelta-darwin-arm64)` | ✅ 同上 |
| `release build (cfgdelta-windows-amd64.exe)` | ✅ 同上 |
| `publish release` | ❌ **失败** |

失败原文（`gh run view --job 118686580376 --log`）：

```
open RELEASE_NOTES.md: no such file or directory
##[error]Process completed with exit code 1.
```

**根因**：`Checksums + manifest` 步骤先 `cd dist` 再写 `RELEASE_NOTES.md`，文件落在
`dist/`；下一步 `Create GitHub Release` 从仓库根执行，读不到该文件。产物、校验和、
三平台构建与冒烟**全部正常**，坏的只有"把说明文件喂给 `gh`"这一段。

**处置（两步，都留证据）**：

1. **Release 用那次 run 的真实产物手工创建**，不重新构建：
   `gh run download 39439782328` → 三个附件落到本地 → `sha256sum` 与 run 日志逐字一致
   → `gh release create v0.1.0 --verify-tag` 附四件。所以 Release 里的字节 = CI 产物字节。
2. **修 workflow 并补判据**（提交 `M6 修正`，见 2.4）：说明文件改写到
   `$GITHUB_WORKSPACE/RELEASE_NOTES.md`；`gh release create` 改为**幂等**
   （已存在则 `gh release upload --clobber`），并回显附件清单与 digest。
   `TestWorkflowParses` 新增两条判据（说明文件必须锚 `$GITHUB_WORKSPACE`、publish 里
   不得出现裸相对重定向；必须有 `gh release upload` + `--clobber`），**两条各自变异变红后还原**。

**已知偏差（不裁剪）**：`v0.1.0` 的 tag 树里仍是**旧版** workflow（带路径缺陷）。
修正提交在 tag 之后，按纪律**不回改 tag、不移动 tag**（移动已发布的 tag 会让
"tag = 产出二进制的源码"这条保证失效）。因此 v0.1.0 的 run 页面上 `publish release`
显示红色，而 Release 本身已正确发布且附件可复验。修正只影响"下一次 tag 能否全自动发布"，
不影响任何产物字节。

### 2.4 本地留副本与真下载复核

**本地留副本**（`dist-release/`，已 gitignore，不入库）：

```
$ cd dist-release && sha256sum -c SHA256SUMS
cfgdelta-darwin-arm64: OK
cfgdelta-linux-amd64: OK
cfgdelta-windows-amd64.exe: OK
```

**真下载复核**（`gh release download v0.1.0` 到空目录后复验）：

```
$ sha256sum -c SHA256SUMS
cfgdelta-darwin-arm64: OK
cfgdelta-linux-amd64: OK
cfgdelta-windows-amd64.exe: OK
$ echo $?
0

$ ./cfgdelta-windows-amd64.exe --version
cfgdelta 0.1.0
$ ./cfgdelta-windows-amd64.exe selftest
selftest: 76 passed, 0 failed, 0 skipped
```

即"下载 → 校验 → 真跑"三段都在本机实测过，且用的是 Release 上的附件而不是本地构建。

### 2.5 状态翻转（三处一致）

| 副本 | 翻转结果 |
|---|---|
| `plan/00` 里程碑表 | M6 → ✅ 完成（2026-10-20，tag `v0.1.0` + Release 四件 + 发布留档） |
| `plan/05` 状态表 | 同上 |
| `README.md` 状态行 | 同上 |

`plan/06 §四` 缺口表：关闭「三平台二进制」后**清空为 0 行**（布局门 `LAYOUT.GAPS` 实测
`扫了 0 行，无已 ✅ 里程碑`）。`plan/00` 承诺-证据台账里「单静态二进制」「发布可用性」
两行同步翻 ✅。


## 三、覆盖对账

| 要求来源 | 要求 | 落点 | 覆盖状态 |
|---|---|---|---|
| `plan/00 §四` M6 DoD | ① git tag | `v0.1.0` → `de5599eb`（§2.1） | ✅ |
| `plan/00 §四` M6 DoD | ② Release 附三平台二进制 + `SHA256SUMS`，本地留副本可复验 | Release 四件（§2.2）+ `dist-release/` 与真下载双重复验（§2.4） | ✅ |
| `plan/00 §四` M6 DoD | ③ 发布留档在 tag 前进 tag 树 | 本文件（§1.7 记录 `de5599eb`；该提交含本文件） | ✅ |
| `plan/05 §二 M6` | 干净环境 clone 按 README 跑通 | §1.6（五段实际输出） | ✅ |
| `plan/05 §二 M6` | 脱敏扫描 0 命中 | §1.8 + `tools/check-desensitized.mjs`（进 CI `layout` job） | ✅ |
| `plan/06 §一` ① | 源码仓库公开、署名 `yuluo554` | `github.com/yuluo554/10-20-cfgdelta`（public）；`go.mod` module path 同源 | ✅ |
| `plan/06 §四` 缺口「CI 实跑」 | 首推实跑并确认绿 | §1.5（含首推失败与修正） | ✅ |
| `plan/06 §四` 缺口「三平台二进制」 | 三平台原生构建 + 原生冒烟 | `release-build` 三 runner 原生构建 + 各自跑 `selftest`；§2.2 产物摘要 | ✅ |
| `plan/HANDOFF-M6 §三` | 代码面冻结：tag 后不改代码面 | §1.7 记录末态提交；tag 后只追加 §二 与状态副本 | ✅ |
| `plan/00 §五` 纪律 | 未实测的不写成"实测"；未跑通不写"CI 绿" | §1.5 如实记录首推失败；§二 之前不写"已发布" | ✅ |
| `plan/05 §四` 已知未验证项 | macOS/Linux 二进制 | 本机只有 windows/amd64；其余平台由 CI 原生构建（§1.2），本机不声称原生验证 | ✅ 口径一致 |
