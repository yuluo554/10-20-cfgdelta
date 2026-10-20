# testdata/baseline — 竞品基线留档（原样输出，不得手工修饰）

本目录是 `data/README.md §1.2` 那张表的**机器留档**。每个工具一个目录，四件齐全：

| 文件 | 内容 |
|---|---|
| `VERSION.txt` | 该工具 `--version` / `version` 的**原样输出** |
| `COMMANDS.txt` | 逐字命令行（`cwd=repo-root`，`argv[0]` 写工具名；仓库内不放本机绝对路径） |
| `*.out` | 每条命令的**原样 stdout + stderr**（合并为一路） |
| `exit_code.txt` | 每条命令的退出码，`case=<id> rc=<n>` |

复核门：`tests/baseline_archive_test.go`（四件齐全 + `COMMANDS.txt` 的每条 case 都有 `.out` 与退出码 + `VERSION.txt` 回显的版本号与 fixture 断言一致）。

---

## 一、留档环境

| 项 | 值 |
|---|---|
| 留档日期 | **2026-10-20**（本轮实测，非沿用旧口径） |
| 平台 | Windows 10 amd64，Git Bash（MSYS2）；`node v24.15.0` |
| dyff | `v1.12.0` — GitHub release `dyff_1.12.0_windows_amd64.tar.gz`，实跑 `dyff.exe version` → `1.12.0` |
| diffyml | `v1.8.2` — GitHub release `diffyml_1.8.2_windows_amd64.zip`，`--version` → `1.8.2 (commit 0cf8001)` |
| confdiff | `0.17.2` — `npm install confdiff@0.17.2`（npm 分发，`node_modules/.bin/confdiff`） |

> 二进制**不入库**（体积与许可原因）。复验方式：按上表取到同版本，用 `COMMANDS.txt` 里的命令行在仓库根逐条重跑，与 `*.out` 比对。

---

## 二、与 `data/README.md §1.2` 的逐条对账

「预期」列抄自 §1.2 的「预期复现结果」；「本轮实测」列是本目录 `.out` / `exit_code.txt` 的真实读数。

| §1.2 行 | 命令 | 预期 | 本轮实测 | 结论 |
|---|---|---|---|---|
| 测试A（dyff） | `dyff between --set-exit-code k1 k2` | 4 条差异、rc=1 | 4 条差异、rc=1 | ✅ 一致（`A-dyff.out`） |
| 测试A（diffyml） | `diffyml k1 k2` | 同样 4 条 | 4 条差异；裸调用 **rc=0**，加 `--set-exit-code` 后 **rc=1** | ✅ 差异数一致；退出码需显式开关（`A-diffyml*.out`，两形态都留档） |
| 测试B（dyff） | `dyff between --set-exit-code va vb` | 5 条差异、rc=1 | 5 条差异、rc=1 | ✅ 一致（`B-dyff.out`） |
| 测试B（confdiff） | `confdiff va vb -l` | 仍 5 条 | **6 条**（`env` 按序对齐，两条各算 name+value 变更） | ⚠️ **数值不符，已如实留档**。加 `--array-set` 后降为 2 条（`B-confdiff-array-set.out`）：说明 6 条里 4 条来自数组按下标对齐，仍是"顺序敏感"的噪声 |
| 测试C | 逐组三工具 | 6 组误报；**仅 `1.50` vs `1.5` 三家正确** | dyff **9/9 全报**（含 c07）；diffyml **8/9 报**（c07 判等）；confdiff **7/9 报**（c06、c07 判等） | ⚠️ **"仅 c07 三家正确"不成立**：dyff 在 c07 上仍报 `value change`。逐组明细见下节 |
| 测试C 第 6 组 | `diffyml` 跑 `1e3` vs `1000` | `type change from float to int`，`-1000`/`+1000` | 输出为 `type change from float to int`，`-1000` / `+1000` | ✅ 形态一致（`C06-diffyml.out`；注意左值被渲染成 `1000`，与 `-1000` 的语义标签并存，属上游渲染瑕疵） |
| 测试D（TOML，dyff） | dyff 以 YAML 解析器读 TOML | `[service]` 被判"根级新增 + 段内删除" | **`returned no differences`、rc=0** | ❌ **上游已变**：v1.12.0 的原版 `dyff yaml a.toml` 输出 `service: {name, port}`，说明它已能直读 TOML 结构（依赖 `github.com/BurntSushi/toml`）。**该误报在当前版本不可复现** |
| 测试D（HCL，dyff） | dyff 读 HCL | 整块当字符串；属性重排报值变更 | 整块当字符串；`a.hcl`→`b.hcl` 属性重排报 `value change`、rc=1 | ✅ 一致（`D-hcl-dyff.out`） |
| 版本核对 | 三家 `--version` | 1.12.0 / 1.8.2 / 0.17.2 | 1.12.0 / 1.8.2 (0cf8001) / 0.17.2 | ✅ 一致 |

---

## 三、测试C 逐组明细（从 `.out` 机械判定，非手工记录）

判据：`.out` 里出现 `no differences` / `no semantic differences` 记为 0（判等），否则记为 1（报差异）。

| 组 | 语义 | dyff | diffyml | confdiff | 我方期望（manifest） |
|---|---|---|---|---|---|
| C01 | `1Gi` = `1024Mi` | 1 | 1 | 1 | cos 1 / exit 0 |
| C02 | `1000M` = `1G` | 1 | 1 | 1 | cos 1 / exit 0 |
| C03 | `8000m` = `8` | 1 | 1 | 1 | cos 1 / exit 0 |
| C04 | `90s` = `1m30s` | 1 | 1 | 1 | cos 1 / exit 0 |
| C05 | `1h` = `60m` | 1 | 1 | 1 | cos 1 / exit 0 |
| C06 | `1e3` = `1000` | 1 | 1 | **0** | cos 1 / exit 0 |
| C07 | `1.50` = `1.5` | **1** | 0 | 0 | cos 1 / exit 0 |
| C08 | `1Gi` ≠ `1000M`（反例，**必须报**） | 1 | 1 | 1 | behav 1 / exit 1 |
| C09 | `1MiB` vs `1MB`（歧义） | 1 | 1 | 1 | indeterminate / exit 2 |

**读法**：C01–C07 是"应当判等"的组，被报成差异即误报；C08/C09 是"必须报/必须 indeterminate"的组，报出来是正确行为。

- dyff：**C01–C07 七组全误报**（7/7），C08/C09 行为正确。
- diffyml：C01–C06 误报（6/7），C07 正确。
- confdiff：C01–C05 误报（5/7），C06/C07 正确。

> 上面这张表是对 `data/README.md §1.2`「6 组误报；仅 `1.50` vs `1.5` 三家正确」的**修正**：dyff 在 C07 上也误报，所以 dyff 是 7 组全中，不是 6 组。

---

## 四、需带进决策记录的实测偏差（不复写旧档，只追加）

按 §1.2 复现纪律：**上游修了不删旧档，追加带日期的复核记录**。以下三条是本轮（2026-10-20）新查明的，`data/README.md §1.2` 已同步加注。

1. **dyff v1.12.0 已能读 TOML（2026-10-20 复核）**
   `dyff between a.toml b.toml` 返回 rc=0（属性重排视为无差异），`dyff yaml a.toml` 直接打印 `service:` 映射。
   后果：测试D 的 **TOML 一半**不再是 dyff 的失分点；HCL 一半仍然成立。
   对项目的影响：`plan/06` 风险登记里"多格式"的卖点必须收窄为 **HCL / INI / .env 仍无结构语义**，TOML 不能再当差异化论据。

2. **`1.50` vs `1.5`：只有 diffyml + confdiff 判等，dyff 仍报差异（2026-10-20 复核）**
   `C07-dyff.out` 显示 `± value change - 1.50 + 1.5`，rc=1。§1.2 原写"仅 `1.50` vs `1.5` 三家正确"，与实测不符。
   影响：测试C 的"三家全错 6 组"应改述为 **dyff 9/9 全错**；这对我们是更有利的事实，但**必须按实测写**，不能沿用未复核的口径。

3. **confdiff 在测试B 上是 6 条而非 5 条（2026-10-20 复核）**
   差在 `env` 数组按下标对齐（`--array-set` 未加时）。加 `--array-set` 后 2 条。
   影响：`plan/06` 里"测试B 5 条"只对 dyff 成立；引用 confdiff 时必须写明所用开关，否则会被复验者判为口径造假。

---

## 五、复验方式

**重抓全套（推荐）**：取到三家同版本二进制后，在仓库根：

```bash
DYFF=/path/to/dyff DIFFYML=/path/to/diffyml CONFDIFF=/path/to/confdiff \
  bash tools/capture-baseline.sh   # 重写 VERSION.txt / COMMANDS.txt / *.out / exit_code.txt
go test ./tests/ -run TestBaseline -v
```

**逐条比对单条输出**（以 dyff 为例）：

```bash
DYFF=/path/to/dyff
while IFS= read -r line; do
  case_id="${line#case=}"; case_id="${case_id%% *}"
  args="${line#*cmd=}"; args="${args#* }"
  $DYFF $args > /tmp/recheck.out 2>&1
  diff -u "testdata/baseline/dyff/$case_id.out" /tmp/recheck.out && echo "OK $case_id"
done < <(grep -v '^case=version' testdata/baseline/dyff/COMMANDS.txt)
```

> 注：输出里含时间戳/提交号的行（diffyml `--version` 的 `built:`）会因重编而变；比对时以主版本行为准，`VERSION.txt` 只需满足"非空 + 含版本号"。
