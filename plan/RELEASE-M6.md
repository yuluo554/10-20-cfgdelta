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

（待 tag 后填写。）

## 三、覆盖对账

| 要求来源 | 要求 | 落点 | 覆盖状态 |
|---|---|---|---|
| `plan/00 §四` M6 DoD | ① git tag | `v0.1.0` → §二 | ⏳ 待 §二 |
| `plan/00 §四` M6 DoD | ② Release 附三平台二进制 + `SHA256SUMS`，本地留副本可复验 | `.github/workflows/ci.yml` `release-build`/`publish` + §1.2/§1.3 + §二 | ✅ 机制已验（workflow 形状 + 变异）；产物本身待 §二 |
| `plan/00 §四` M6 DoD | ③ 发布留档在 tag 前进 tag 树 | 本文件（§1.7） | ✅ |
| `plan/05 §二 M6` | 干净环境 clone 按 README 跑通 | §1.6（五段实际输出） | ✅ |
| `plan/05 §二 M6` | 脱敏扫描 0 命中 | §1.8 + `tools/check-desensitized.mjs`（进 CI `layout` job） | ✅ |
| `plan/06 §一` ① | 源码仓库公开、署名 `yuluo554` | `github.com/yuluo554/10-20-cfgdelta`（public）；`go.mod` module path 同源 | ✅ |
| `plan/06 §四` 缺口「CI 实跑」 | 首推实跑并确认绿 | §1.5（含首推失败与修正） | ✅ |
| `plan/06 §四` 缺口「三平台二进制」 | 三平台原生构建 + 原生冒烟 | `release-build` 三 runner 原生构建 + 各自跑 `selftest`；§1.2 | ✅ 机制 / ⏳ 产物（§二） |
| `plan/HANDOFF-M6 §三` | 代码面冻结：tag 后不改代码面 | §1.7 记录末态提交；tag 后只追加 §二 与状态副本 | ✅ |
| `plan/00 §五` 纪律 | 未实测的不写成"实测"；未跑通不写"CI 绿" | §1.5 如实记录首推失败；§二 之前不写"已发布" | ✅ |
| `plan/05 §四` 已知未验证项 | macOS/Linux 二进制 | 本机只有 windows/amd64；其余平台由 CI 原生构建（§1.2），本机不声称原生验证 | ✅ 口径一致 |
