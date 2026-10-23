# plan/HANDOFF-M7 — 续接交接单（M6 收尾 → M7 缺陷修复与再发布）

> 用法（新对话第一句）：`/goal 读取 "<项目绝对路径>\plan\HANDOFF-M7.md" 继续完成任务`
> **动手前第一件事**：`node tools/check-sprint-layout.mjs`，然后 `node tools/verify-layout-mutations.mjs`
> （第二个不过说明门本身坏了，先修门再干活）。
> 写于 2026-10-23，M7 收尾时。**产品行为面从 v0.1.0 起零改动，M7 只动门、文档与发布表述。**

## 一、当前进度

M7（缺陷修复与再发布）已完成。下表全部是本轮**实测输出**。

| 项 | 状态 | 证据（命令 → 结果） |
|---|---|---|
| 布局门三态 | ✅ | `node tools/check-sprint-layout.mjs` → `结论: 全绿（PASS 7 / SKIP 1 / FAIL 0，共 8 条判据）`，exit 0；SKIP 的是 `LAYOUT.GAPS`（缺口表 0 数据行，合法） |
| 布局门变异实物 | ✅ | `node tools/verify-layout-mutations.mjs` → `结论：8/8 组变异如期变红 + 4/4 条回归断言成立`，exit 0 |
| `LAYOUT.GAPS` 方向 | ✅ | 变异脚本 V7：已 ✅ 的 M1 写进**缺口列（第 1 列）** → `FAIL LAYOUT.GAPS` + exit 1；回归 R2：合法未关闭缺口把目标写成 M6 → `PASS` + exit 0（旧实现会误报这里） |
| 空表是 SKIP 不是 PASS | ✅ | 回归 R1：缺口表裁成 0 数据行 → `SKIP LAYOUT.GAPS` + exit 0 |
| 覆盖度自洽 | ✅ | 回归 R3：门报 8 条判据 / 靶子 8 条，集合相等（防"门加了第 9 条、脚本还报 8/8"） |
| 脱敏门 | ✅ | `node tools/check-desensitized.mjs` → `扫了 233 个文本文件，0 命中`，exit 0 |
| 三件生成物 | ✅ | `sync-version --check` → `root=0.1.1 mirror=0.1.1`；`sync-profiles --check` → `0 drift`；`gen-golden-manifest --check` → `76 cases`；均 exit 0 |
| `go vet` / `go test` | ✅ | `go vet ./...` exit 0；`go test ./... -count=1` 7 个包 ok / 4 个无测试文件 / exit 0（104 个 `=== RUN`） |
| `selftest` | ✅ | `./dist/cfgdelta.exe selftest` → `76 passed, 0 failed, 0 skipped`，exit 0 |
| 版本 | ✅ | `VERSION` = `0.1.1`；`--version` → `cfgdelta 0.1.1` |

## 二、当前事实·状态不变量（下一棒动手前必须知道）

1. **`--root` 不是 CLI 旗标**。`Env.Root` 只是测试注入点；`selftest --root X` 得到
   `unknown flag --root` + exit 3。对外只说"在一份 checkout 里跑"。
2. **`selftest` 靠向上查找**，`testdata/golden` 没被 `go:embed` 进二进制。仓库外运行必 exit 3。
   这是既有设计，M7 的选择是**改口径不改产品**（DEC2-A）。
3. **布局门的发布里程碑 = 最高 ✅ 里程碑**（推导，不硬编码）。下一棒若加 M8，只要 M8 在
   三处状态副本都 ✅ 且 `plan/RELEASE-M8.md` 有「两栏 + 覆盖对账」，门就绿。
4. **`LAYOUT.GAPS` 的基线是 `SKIP`，不是 `PASS`**。看到 SKIP 不要以为门坏了；
   它是"缺口表 0 数据行，本判据无对象"。若缺口表**有**行，就应回到 PASS。
5. **门的判据数必须与变异脚本的靶子数相等**（R3 断言）。加判据时同步加一条变异，否则 R3 红。
6. **`update/` 已被 `.gitignore` 忽略、也被脱敏门跳过**（DEC3-A）。它是本地交接目录，
   **不能**用来藏任何受版本控制的内容。
7. **`v0.1.0` 的 tag 与 Release 原样保留**，不移动（`plan/RELEASE-M6.md §2.3` 的理由仍成立）。

## 三、下一棒待办（如有 M8）

- 若再发版：`VERSION` 升号 → 三处状态副本加 M8 行 → 写 `plan/RELEASE-M8.md`（**tag 前**落盘）→
  `plan/HANDOFF-M8.md` → commit → tag → 等 CI 绿 → 真下载复核。
- **不要**移动 `v0.1.0` / `v0.1.1`。
- 若改了布局门判据：同步在 `tools/verify-layout-mutations.mjs` 加一组变异（R3 会替你检查）。
- 若改了 `selftest` 的查找逻辑或加了 `--root` 旗标：**同时**改 README「Develop」节、
  `internal/cli/cli.go` 的 `--help`、以及 `ci.yml` 里 Release 正文的生成片段（三处必须一致）。

## 四、既定口径清单（K 编号）

K-01…K-10 见 `plan/05 §三`，M7 未改动任何一条。新增一条纪律（不是 K 编号，是门的自证）：

| 编号 | 口径 | 落点 |
|---|---|---|
| K-11 | 一条门里有几个判据，变异清单就覆盖几个；**变异没红先分三问**（靶子选错 / 假门 / 扫描面） | `tools/check-sprint-layout.mjs`、`tools/verify-layout-mutations.mjs` |

**K-11 本轮新增记录（第 6–8 条）**：⑥ `LAYOUT.GAPS` 的靶子选"已 ✅ 里程碑写进**缺口列**"，
不是"写进目标列"——旧实现扫的是目标列，用旧靶子会得出"门是好的"这个错误结论（靶子选错那一族）；
⑦ `LAYOUT.WRAPUP` 的靶子必须在**最新 ✅ 里程碑**上做（删 `RELEASE-M6.md`），
删一个过程里程碑的留档没有对象；⑧ 变异脚本自己也曾"假红"：把文档点名的新脚本写进 `plan/`
却没一起覆盖到导出树，`LAYOUT.DOC_TOOLS` 会误红并连带 7 条变异全废 —— 检查方式是"叠加文件清单"。

## 五、本机环境坑（只写实测过的）

1. **`go` 不在 PATH**。用 `%USERPROFILE%\go-sdk\go\bin\go.exe`（go1.24.5，与 `go.mod` 的 `go 1.24` 对齐），
   或 `GOTOOLCHAIN=go1.24.7`（已在 module 缓存，不用下载）。两者 `vet` / `test` 都绿。
2. **`| tail` 会吞退出码**。一律 `cmd; echo $?`，不要 `cmd | tail` 后报码。
3. **git-bash 的 `tar` 不接受 `-C <盘符路径>`**：会把 `C:\...` 当远程主机名报
   `Cannot open: No such file or directory`。给 `tar` 传 `cwd`（`spawnSync` 的 `cwd`），
   或先 `cd` 过去再 `tar -x`。
4. **`git archive HEAD | tar x` 导出的是 HEAD，不含工作树改动**。改门时要自验，
   得把工作树改动（含**未跟踪**新文件）覆盖进导出树——`verify-layout-mutations.mjs` 已内建这一步并打印清单。
5. **`semver`/版本号 grep 会误伤**：`README.md` 里 `v0.1.0` 出现在历史里程碑行，
   升号时只改 Status 行与 M7 行，别全局替换。

## 六、DoD checklist（M7，全部已勾）

- [x] 布局门每条判据有可复跑变异证据（8/8 + 4/4）
- [x] GAPS 方向正确、空表为 SKIP
- [x] 三处状态副本含 M7 且互相一致
- [x] `VERSION` / 镜像 / `--version` 三处一致 = 0.1.1
- [x] 五道门 + 变异脚本全绿
- [x] `go vet` / `go test` / `selftest` 全绿
- [x] `plan/RELEASE-M7.md` **在 tag 之前**落盘
- [x] `v0.1.0` 未移动
- [x] 产品行为面零改动（唯一改动是 `--help` 文案）

## 七、关键命令速查

```bash
cd <repo>

# 0. 环境（本机；go 不在 PATH）
export GO="$USERPROFILE/go-sdk/go/bin/go.exe"
"$GO" version                      # 期望 go1.24.5
# 或不装 SDK：GOTOOLCHAIN=go1.24.7 go version   → go1.24.7

# 1. 布局门 + 它的变异验证（每棒第一件事，两条都要）
node tools/check-sprint-layout.mjs;      echo "layout=$?"    # 期望 PASS 7 / SKIP 1 / FAIL 0
node tools/verify-layout-mutations.mjs;  echo "mut=$?"       # 期望 8/8 + 4/4

# 2. 生成物一致性
node tools/sync-version.mjs --check;     echo "ver=$?"
node tools/sync-profiles.mjs --check;    echo "prof=$?"
node tools/gen-golden-manifest.mjs --check; echo "manifest=$?"

# 3. 脱敏门
node tools/check-desensitized.mjs;       echo "privacy=$?"

# 4. 测试与构建
"$GO" vet ./...;                                   echo "vet=$?"
"$GO" test ./... -count=1;                         echo "test=$?"
CGO_ENABLED=0 "$GO" build -trimpath \
  -ldflags "-s -w -X main.version=$(cat VERSION)" -o dist/cfgdelta.exe ./cmd/cfgdelta
./dist/cfgdelta.exe --version;                     echo "ver=$?"
./dist/cfgdelta.exe selftest;                      echo "self=$?"   # 期望 76 passed

# 5. 发版后（人工，需要网络与凭据）
TMP=$(mktemp -d); cd "$TMP"
gh release download v0.1.1 -R yuluo554/10-20-cfgdelta
sha256sum -c SHA256SUMS; echo "exit=$?"            # 期望三行 OK
gh api repos/yuluo554/10-20-cfgdelta/releases --jq '.[0].assets[]|{name,size,digest}'
```

> 本机无 `make`；`make check` 的等价手动序列就是上面 1–4 段。
