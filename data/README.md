# data/README 数据与口径说明

本文件是**数据依据与数值口径的唯一权威来源**。plan/01、plan/03、代码与 golden 夹具均引用本文件，**不得另写第二份口径**。

**查证基准日：2026-10-20**　**状态标记**：✅ 实测可取 / ⬜ 待核对或软论据 / 扩展项。

---

## 一、依据查证记录（三态 + 编号 + 可复现实测命令）

### 1.1 红队实测环境

| 项 | 值 | 说明 |
|---|---|---|
| 基准日 | 2026-10-20 | 所有版本号与输出均为该日实测 |
| dyff | **v1.12.0** | ✅ 已实测 |
| diffyml | **v1.8.2** | ✅ 已实测 |
| confdiff | **0.17.2** | ✅ 已实测（npm 分发） |
| 夹具 | 见 §二 合成数据纪律 | 最小复现 YAML / TOML / HCL 文件，不含任何真实集群配置 |

### 1.2 可复现实测命令

> 下列为基准日复现四组误报所用的调用方式。**参数名以复现时各工具的 `--help` 为准**：本表记录的是"跑出该结果的那一次"所用形态，不是对工具 CLI 的规范描述。M1 落 golden 时必须把**实际跑通的完整命令行逐字粘进 `testdata/baseline/<tool>/COMMANDS.txt`**，并附 `--version` 输出。

| 用途 | 命令形态 | 预期复现结果 | 状态 |
|---|---|---|---|
| 测试A（K8s 默认值 + 数量） | `dyff between --set-exit-code k1.yaml k2.yaml` | 4 条差异；退出码 `1` | ✅ 实测 |
| 测试A（同谱系对照） | `diffyml k1.yaml k2.yaml` | 同样 4 条 | ✅ 实测 |
| 测试B（Helm values 跨环境） | `dyff between --set-exit-code va.yaml vb.yaml` | 5 条差异；退出码 `1` | ✅ 实测 |
| 测试B（confdiff 对照） | `confdiff va.yaml vb.yaml -l` | 仍 5 条 | ✅ 实测 |
| 测试C（单位族 7 组） | 逐组两文件跑 dyff / diffyml / confdiff | 6 组误报；仅 `1.50` vs `1.5` 三家正确 | ✅ 实测 |
| 测试C 第 6 组细节 | `diffyml` 跑 `1e3` vs `1000` | 输出 `type change from float to int`，`-1000` / `+1000` | ✅ 实测 |
| 测试D（TOML） | dyff 直接以 YAML 解析器读 TOML 文件 | `[service]` 段被判"根级新增 + 段内删除" | ✅ 实测 |
| 测试D（HCL） | dyff 读 HCL 文件 | 整个 `resource "..." "b" { }` 块被当字符串；属性重排报值变更 | ✅ 实测 |
| 版本核对 | `dyff version` / `diffyml --version` / `confdiff --version` | 三家须分别回显 v1.12.0 / v1.8.2 / 0.17.2 | ✅ 实测 |

**复现纪律**：`testdata/baseline/` 下每个工具一个目录，含 `COMMANDS.txt`（逐字命令行）、`VERSION.txt`（版本输出）、`*.out`（原始 stdout/stderr）、`exit_code.txt`。**留档必须是原样输出，不得手工修饰**；若某条在复现时不再误报（上游修了），**不删旧档，追加一条带日期的复核记录**——上游修复本身就是决策记录里风险①的触发信号。

### 1.3 需求证据（✅ 原话，逐条附编号）

| 日期 | 编号 | 原话要点 | 状态 |
|---|---|---|---|
| 2018-03-12 | kubernetes/kubernetes#61032（closed，**由 stale bot 关闭，非解决**） | "kubectl return that deployment configured - i.e. changed, even when deployment are same."；`cpu:1000m` 被转成 `1`、`memory:1024Mi` → `1Gi` | ✅ issue 可取 |
| 2019-07-20 | pulumi/pulumi-kubernetes#644 | "A deployment marked with resources 2048Mi is being normalized to 2Gi on the initial update... trying to change it back to 2048Mi"；hausdorff："**the API server normalizes the memory values, it registers as changes to us.**" | ✅ issue 可取 |
| — | kubectl-neat-diff（117★）README | "Newer Kubernetes versions (1.18+) add a jungle of keys to any YAML / JSON output" | ✅ 仓库可取 |
| — | HN 26283698（kmarc） | "I understand that sometimes you just want to diff two k8s objects, kubectl diff is not a tool for that." | ✅ 页面可取 |
| — | HN 26283522（rmetzler） | "The problem with diffing JSON and yaml is, that these formats aren't line based" | ✅ 页面可取 |
| — | HN 40861457（radiator） | "a value in the base and deployment chart might be named slightly differently... you must always check the templates." | ✅ 页面可取 |
| 2023-02-21 | homeport/dyff#280（**open**） | "provides little to no information on the actual change" | ✅ issue 可取 |
| 2022-08-23 | homeport/dyff#242（**open**） | "`--exclude` only works for specifying the key name. Would it be possible to exclude on the value?" | ✅ issue 可取 |
| 2023-05-30 | kubernetes/kubernetes#118325（**open**） | "`kubectl diff --server-side --force-conflicts` ... fails… field is immutable" | ✅ issue 可取 |

⚠️ **口径纪律**：`#61032` 的 closed 状态**不能**读作"已修复"——关闭者是 stale bot。引用时必须保留该注记，否则会变成对上游的错误陈述。

### 1.4 覆盖矩阵数据（基准日 2026-10-20）

| 工具 | 星数 | 键序 | 注释 | 值语义 | 默认值 | 需集群 | 状态 |
|---|---|---|---|---|---|---|---|
| dyff | 1898★ | ✅ | ✅ | ❌ | ❌ | 否 | ✅ 实测 |
| diffyml | 127★ | ✅ | ✅ | ❌ | ❌ | 否 | ✅ 实测 |
| confdiff | 41★ | ✅ | ✅ | ❌ | ❌ | 否 | ✅ 实测 |
| diffx | 89★ | ✅ | ✅ | 未实现 | 未实现 | 否 | ✅ 实测（未实现项为读文档结论） |
| kubectl diff | 内核 | — | — | — | — | **是** | ✅（k8s#61032 / #118325） |
| kustomize | 12186★ | — | — | 构建器 | — | 否 | ✅（非 differ） |
| helm-diff | 3507★ | — | — | — | — | **是**（+helm） | ✅ |
| json-diff | 1203★ | — | — | 位置比较 | — | 否 | ✅ |
| yq | 16142★ | — | — | 查询器 | — | 否 | ✅ |
| dasel | 8049★ | — | — | 查询器 | — | 否 | ✅ |
| hcl2json | 480★ | — | — | 仅转 JSON | — | 否 | ✅ |
| graphtage | 2491★ | — | — | 多格式，无 K8s 语义 | — | 否 | ✅ |

**星数是基准日快照**，会随时间漂移，引用时必须带基准日；不得作为"市场份额"论证。

### 1.5 开源空白检索（`gh api search/repositories`，基准日 2026-10-20）

| 检索式 | total_count | 头部 | 状态 |
|---|---|---|---|
| `semantic diff yaml` | 37 | graphtage 2491★ | ✅ 实测 |
| `yaml semantic diff cli` | 15 | diffx 89★ | ✅ 实测 |
| `toml diff semantic` | 21 | — | ✅ 实测 |
| `dotenv semantic diff` | 3 | — | ✅ 实测 |
| `config diff kubernetes` | 16 | 最高 2★（其余无关噪声） | ✅ 实测 |
| `kubernetes manifest diff tool` | 7 | argocd-diff-preview 766★ | ✅ 实测 |
| `kubectl diff` | 68 | diffyml 127★ | ✅ 实测 |
| `dyff` | 62 | dyff 1898★ | ✅ 实测 |
| `helm values diff tool` | 3 | 最高 1★ | ✅ 实测 |
| `config semantic diff multi format` | 1 | — | ✅ 实测 |
| `config equivalence checker` | **0** | — | ⬜ 软论据 |
| `config equivalence oracle` | **0** | — | ⬜ 软论据 |
| `kubernetes quantity diff` | **0** | — | ⬜ 软论据 |
| `kubernetes default values normalization diff` | **0** | — | ⬜ 软论据 |
| `normalize yaml kubernetes defaults` | **0** | — | ⬜ 软论据 |
| `k8s defaulting spurious diff` | **0** | — | ⬜ 软论据 |
| `semantic equal config file` | **0** | — | ⬜ 软论据 |
| `values.yaml diff environments` | **0** | — | ⬜ 软论据 |
| `env var diff dotenv` | **0** | — | ⬜ 软论据 |
| `quantity normalization kubernetes` | 1 | — | ✅ 实测 |
| `resource quantity parser` | 2 | — | ✅ 实测 |

**0 结果的读法（硬纪律）**：0 只说明"该检索式下没有同名项目"，**不得**用作市场规模、需求强度或"没人打算做"的证据。本题的硬证据是 §1.1–1.3 的实测输出与需求原话。

---

## 二、数据源可获取性记录

本题**无外部数据依赖**：运行期不联网、不读集群、不读远端 API。

| 数据 | 来源 | 获取方式 | 状态 |
|---|---|---|---|
| K8s Quantity 文法 | Kubernetes apimachinery 公开文档 / 源码文法 | 人工核对后写入 §四（**文法本身是常量事实，不是抓取数据**） | ✅ 可直连 |
| Go Duration 文法 | Go 官方 `time.ParseDuration` 文档 | 人工核对后写入 §四 | ✅ 可直连 |
| YAML 1.1 布尔族 | YAML 1.1 规范 | 人工核对后写入 §四 | ✅ 可直连 |
| K8s 默认值 | 公开 manifest 语义（API 参考文档 + 行为观察） | 写入版本化 JSON profile，逐条标注来源 | ⬜ 逐条核对中（首期 10 条，M1 落盘时补全来源列） |
| golden 夹具 | 本地合成 | 见 §三 | ✅ 可自制 |
| 三家工具基线留档 | 本地运行 dyff / diffyml / confdiff | 见 §1.2 | ✅ 已实测 |

**无阻断项**：本题不存在"外部接口挂了题目就没素材"的结构性风险——全部素材要么是常量文法，要么是本地合成的夹具。这是本题相对数据类题目的形态优势，也是选它的原因之一。

---

## 三、合成数据纪律

### 3.1 夹具原则

- **最小复现**：每个夹具只保留触发该误报所需的最小字段，不复制任何真实集群配置、不包含真实镜像仓库地址、不包含真实内网域名或 IP、不包含任何凭据。
- **镜像地址统一使用不可解析的文档保留域名占位**：`registry.example/` 前缀 + 明确标注"合成夹具"。**保留域名只出现在夹具与文档里，不得作为提交身份**（commit author / tagger 邮箱一律 `yuluo554` + GitHub noreply）。
- **命名空间 / 对象名**：`synth-a` / `synth-b` 或 `fixture-*`，不使用任何真实业务名称。
- **数值**：全部来自 §四 的文法示例（`1Gi` / `1024Mi` / `500m` / `0.5` / `90s` / `1m30s` / `1e3` / `1000`）。这些是**文法常量**，不是业务数据。
- **三个"假"白名单**（进夹具前必须自检）：
  1. 镜像仓库前缀 → `registry.example/`（RFC 2606 保留域名）；
  2. 邮箱 / 身份 → 只出现在 git 配置，不出现在夹具；
  3. IP / 域名 → 一律 `192.0.2.0/24`（TEST-NET-1）或 `example` 域。

### 3.2 真值来源（两条独立通道）

| 通道 | 内容 | 为什么独立 |
|---|---|---|
| A 文法推导 | 由 §四 的文法机械推导真值（如 `1Gi = 1024³`） | 不依赖任何工具实现，可手算复核 |
| B 竞品实测 | 三家在最小夹具上的原始输出留档 | 证明"误报真实存在"，且不依赖我们的实现 |

**两条通道冲突时的处置**：以 A 为准，并把冲突记为夹具缺陷（通常是夹具本身不最小或存在额外差异）；**不得为了让 B 好看而改夹具**。

### 3.3 覆盖度口径

以 `testdata/golden/` 的 manifest 为准，逐族登记 `family → cases → expected`，覆盖范围显式写为"**四组实测族（A/B/C/D）+ 10 条高频默认值**"，**不声称覆盖全部 K8s 语义**。新增族必须同时加夹具与 manifest 行。

---

## 四、数值口径纪律（唯一来源）

> 本节的每一条在代码里**只能被引用的常量/函数实现一次**；plan/01、plan/03、README 中出现相关描述时均为引用。

### 4.1 K8s Quantity 文法

```
quantity  := sign? number suffix?
number    := digits ( "." digits )? | "." digits      ; 也可为指数式 digits ( "e" | "E" ) sign? digits
sign      := "+" | "-"
suffix    := binary | decimal | ""
binary    := "Ki" | "Mi" | "Gi" | "Ti" | "Pi" | "Ei"   ; 1024^1 .. 1024^6
decimal   := "n" | "u" | "m" | "" | "k" | "M" | "G" | "T" | "P" | "E"
                                                      ; 10^-9 -6 -3 0 3 6 9 12 15 18
```

**规范化**：解析为**精确有理数**（`{numerator: bigint, denominator: bigint}`，最小化约分），**全程不经过浮点**。

| 输入 | 规范值（有理数） | 判等结论 |
|---|---|---|
| `1Gi` | 1024³ / 1 | 与 `1024Mi` 相等 |
| `1024Mi` | 1024³ / 1 | — |
| `1000M` | 1000³ / 1 | 与 `1G` 相等；**与 `1Gi` 不等** |
| `1G` | 1000³ / 1 | — |
| `8000m` | 8 / 1 | 与 `8` 相等 |
| `500m` | 1 / 2 | 与 `0.5` 相等 |
| `1e3` | 1000 / 1 | 与 `1000` 相等 |
| `1.50` | 3 / 2 | 与 `1.5` 相等 |

**边界**：指数式与后缀不可同时出现（`1e3Mi` 非法）；小数与指数混用按十进制精确展开；非法输入 → `parse_error`（退出码 `3`），**不降级为字符串比较**。

### 4.2 Go Duration 文法

```
duration := ( number unit )+        ; 至少一组；无单位视为非法（除 "0"）
unit     := "ns" | "us" | "µs" | "ms" | "s" | "m" | "h"
number   := digits ( "." digits )?
```

**规范化**：换算为**纳秒整数**（`1h = 3600e9`，`1m = 60e9`，`1s = 1e9`）。

| 输入 | 纳秒 | 判等 |
|---|---|---|
| `90s` | 90 000 000 000 | 与 `1m30s` 相等 |
| `1m30s` | 90 000 000 000 | — |
| `1h` | 3 600 000 000 000 | 与 `60m` 相等 |
| `60m` | 3 600 000 000 000 | — |

### 4.3 十进制数值

- 十进制精确规范化：去前导零、去尾随零；保留符号位（`+1` ≡ `1`，`-0` ≡ `0`）。
- **整值浮点折叠**：`1.0` → `1`，`1e3` → `1000`（规范形式为有理数整数）。
- `1.50` ≡ `1.5` ≡ `3/2`。
- **不同值族之间不做隐式强转**：`500m` 只有在被声明为 quantity 族时才与 `0.5` 相等；作为裸字符串时二者不等（见 §4.8 `--coerce-at`）。

### 4.4 YAML 布尔族

按 **YAML 1.1** 全族归一（`y / Y / yes / Yes / YES / n / N / no / No / NO / true / True / TRUE / false / False / FALSE / on / On / ON / off / Off / OFF`）→ 规范 `true` / `false`。

**字符串不自动强转**：只有 YAML 双引号/单引号包裹的 `"true"` 是字符串，与布尔 `true` **不等**；要判等必须 `--coerce-at <path>=bool` 显式声明。

> 注：YAML 1.2 只认 `true/false`。本题按 **1.1 全族**处理并在输出中标注所用规范版本——因为 K8s 工具链里 1.1 风格的值仍大量存在。此选择在 `plan/03` 中必须与代码常量一致。

### 4.5 null 三态（严格区分，不折叠）

| 态 | 表示 | 与另两态的关系 |
|---|---|---|
| 缺失 | key 不存在 | ≠ 显式 null；≠ 空串 |
| 显式 null | `null` / `~` / 空值 | ≠ 缺失；≠ 空串 |
| 空字符串 | `""` | ≠ 缺失；≠ null |

三者在输出中各有独立标记；`--treat-absent-as null` 这类折叠开关**首期不提供**（会掩盖真实差异）。

### 4.6 百分比

**默认按字符串比较**（`50%` ≠ `0.5`）。仅当 profile 或 `--coerce-at <path>=ratio` 显式声明该位置为 ratio 时，才把 `50%` 归一为 `1/2`。

### 4.7 indeterminate（一等结果）

| 理由码 | 触发条件 |
|---|---|
| `ambiguous_unit_family` | 非 K8s 字节单位（`MB` vs `MiB`，十进/二进歧义） |
| `unknown_family` | `--strict` 下遇到未声明值族且两侧书写不同 |
| `unevaluated_expression` | HCL 插值 / 变量引用等未求值节点 |

**默认行为**：未知族**按原样比较**（不产生 indeterminate）；`--strict` 才升级为 `indeterminate`。任何 `indeterminate` 必须携带理由码，**禁止无理由 `unknown`**。

### 4.8 `--coerce-at` 语义

`--coerce-at <path-glob>=<family>`：把**指定路径**的值按 `<family>` 解释。`family ∈ {quantity, duration, decimal, bool, ratio}`。

- 只作用于显式命中的路径；**不做全局推断**；
- 路径匹配不到任何节点 → 退出码 `3` + 明确提示（静默无效是最难排查的坑）；
- 多条规则命中同一路径 → 退出码 `3` + 冲突提示。

### 4.9 浮点禁令

- **任何路径上出现浮点数参与相等判定即视为 bug**。
- 拦截方式：① 归一化模块的公共接口只接受有理数 / 整数纳秒 / 枚举，不暴露浮点类型；② 静态扫描（禁止在比较路径出现浮点类型的比较运算）；③ golden 里放 `1e3` vs `1000`、`1.50` vs `1.5` 两条哨兵用例。
- 文本输出需要十进制展示时，从有理数**精确格式化**（必要时给出分数形式），不得先转浮点再格式化。

### 4.10 序列三模式

| 模式 | 语义 | 重复键 / 歧义处置 |
|---|---|---|
| `ordered` | 按索引逐项比较 | 不适用 |
| `set` | 元素按规范化后字节序排序再比对；顺序差异不产生条目 | 重复元素保留计数并报 `indeterminate`（`duplicate_in_set`） |
| `keyed:<field>` | 按指定字段对齐 | **键必须唯一；重复键直接报错（退出码 `3`），禁止取第一个** |

**不做模糊匹配、不做相似度、不做启发式对齐**：无法对齐的元素一律成对列出或成单列出，不猜配对。

### 4.11 默认值施加顺序

1. **双侧同时施加**：左侧与右侧都先施加 profile 默认值，再比较（单侧施加会把"一侧写了默认值、一侧没写"误判为差异）；
2. **外层先于内层，父先于子**；
3. **同层按路径的 UTF-8 字节序**；
4. `profile` 未命中任何规则 → **降级为纯结构 diff，并在输出头部标注**（`profile_hit: 0`）；
5. **默认值不覆盖显式值**：显式写了非默认值时，归一后与默认值不等 → 正常报差异；
6. 默认值表**必须带版本**（`k8s/v1.29`）；未知版本 → 退出码 `3`，**不得**回落到"最近的版本"。

### 4.12 退出码（一处定义）

| 码 | 含义 | 触发条件 |
|---|---|---|
| `0` | 等价 | 无行为差异（纯书写差异不算；被 `--fail-on` 排除的行为差异也不算） |
| `1` | 有行为差异 | 存在命中 `--fail-on` 列表的行为差异；未指定 `--fail-on` 时 = 存在任何行为差异 |
| `2` | 存在 `indeterminate` | 任一 `indeterminate`（除非 `--allow-indeterminate`） |
| `3` | 解析错误 / 用法错误 | 语法错误、未知 profile 版本、`keyed:` 重复键、`--coerce-at` 未命中、参数互斥 |

优先级：`3` > `2` > `1` > `0`（多个条件同时成立时取最高优先级）。

### 4.13 影响分类表（6 类）

| 类 | 判定线索 | 是否行为类 |
|---|---|---|
| 资源·收缩 | limit/request 归一后变小；副本数减少 | 是 |
| 部署·收缩 | 可用副本、maxUnavailable 等使可用容量下降 | 是 |
| 镜像·变更 | image 引用变化；按 semver 细分 `major` / `minor` / `patch`；非 semver 单列 `non-semver` | 是 |
| 接口·暴露 | Service 端口 / 类型 / selector / Ingress 路由变化 | 是 |
| 配置·新增·移除 | env、ConfigMap key、Secret key 的增删 | 是 |
| 纯书写 | 归一后语义相同，或只影响呈现 | **否（只计数，不逐条列）** |

**分类不确定时**：归入"配置·新增·移除"并在条目上标 `class_guess: true`，**不允许**归入"纯书写"——把不确定判成"无害"是最危险的静默。
