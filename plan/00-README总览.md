# plan/00-README 总览

> 方法论：`ai-tool-project-sprint`（装订 2026-10-20）｜布局门：`node tools/check-sprint-layout.mjs`｜续接对话第一件事 = 跑它

仓库 `10-20-cfgdelta`（行为级配置 diff：归一化层 + 影响分类层）总览。v2（2026-10-20，M0 骨架落盘）。

**项目**：把两份配置文件按"值等价代数"归一 → 结构无关比较 → K8s 默认值双侧抵消 → 行为影响分类 → CI 门禁退出码。只报会改变运行时行为的那几条。
**方法论**：证据先行（四组实测误报先落盘）→ 语法与口径冻结 → golden 先于实现 → 退出码一处定义 → 单静态二进制交付。
**交付形态**：**单静态二进制 CLI**（零运行时依赖、不调外部命令、无集群交互、不需要 helm / kubectl）。
**命名约定**：本地目录与 GitHub 仓库名均为 `10-20-cfgdelta`；命令名 `cfgdelta`；署名 `yuluo554`。
**定位宣言**：结构等价层已经很挤（dyff 1898★ / graphtage 2491★ / diffyml 127★ / diffx 89★ / confdiff 41★），但**值等价层是空的**。本工具做的是 **dyff 的前置归一化层 + 影响分类层**：先归一（数量 / 时长 / 类型 / YAML 布尔族），再分行为 vs 纯书写，最后只报会改变行为的那几条。**不是更好的 YAML diff，是归一化层 + 影响分类层。**
**品牌隔离**：本系列已有若干本地 CLI 工具。本题与它们零代码复制，文档不提名任何其他项目，只写"配置文件语义比较"。

## 一、文档索引表

| 文档 | 内容 | 状态 |
|---|---|---|
| `AGENTS.md` | 方法论锚点 / 强制布局 / 门命令 / 口径红线 / 环境事实 | ✅ v1（装订 2026-10-20） |
| `plan/00-README总览.md` | 索引 / 决策记录 / 弃选项 / 承诺-证据台账 / 里程碑 / 纪律 | ✅ v2（M0 回写） |
| `plan/01-题目文档.md` | 十段式题目定稿（头部 + 介绍 + 任务要求 + 提交材料 + 评分 + 锚点 + 边界） | ✅ v1（首版定稿 2026-10-20） |
| `plan/02-架构与技术选型.md` | Go 选型 / 分层 / 内部文档树 / 值族代数 / 确定性设计 / 退出码 | ✅ v1（M0 落盘） |
| `plan/03-模块详设.md` | 内部文档树契约 / M1–M5 详设 / 分类表 / CLI 契约总表 / 失败态矩阵 | ✅ v1（M0 落盘） |
| `plan/04-数据计划.md` | 数据清单 / profile 形状 / golden 分层 / 真值纪律 / 生成器与守门 | ✅ v1（M0 落盘） |
| `plan/05-里程碑与验收.md` | M0–M6 DoD / **K 编号清单** / 已知未验证项 / 排期 | ✅ v1（M0 落盘） |
| `plan/06-交付对标.md` | 提交材料落点 / 评分对标 / 风险登记 / 缺口表 | ✅ v1（M0 落盘） |
| `plan/HANDOFF-M<next>.md` | 每里程碑续接快照（七节） | ✅ `HANDOFF-M1`–`M6` 已落盘；M6 为末棒（无 M7），收尾事实见 `plan/RELEASE-M6.md §二` |
| `data/README.md` | 依据查证（三态 + issue/HN 编号 + 基准日）/ 可复现实测命令 / 数据源记录 / 合成数据纪律 / 数值口径纪律 | ✅ v1（出题阶段 4，2026-10-20） |
| `tools/check-sprint-layout.mjs` | 布局门（方法论装订第②件） | ✅ v1（9 组变异验证） |
| `.gitignore` | Go 编译产物 / 二进制 / 覆盖率 | ✅ v1 |
| `README.md` / `LICENSE` / `docs/testA-before-after.png` | 对外说明 / 许可 / 首屏对照截图 | ✅ 三件齐（截图 M5 落盘并进 README 首屏） |

## 二、决策记录表

> 本节同时是**弃选项记录**。新增决策追加到表尾，不改历史行。

| 日期 | 决策 | 结论与理由 |
|---|---|---|
| 2026-10-20 | 选题：行为级配置 diff（不参赛，发 GitHub 公开仓库） | 出题前先做红队实测：四组误报（K8s 默认值 + 数量、Helm values 跨环境、单位族、非 YAML 格式）在 dyff / diffyml / confdiff 三家上**全部复现**。存在理由是实测出来的，不是推测出来的 |
| 2026-10-20 | 形态收敛为**单静态二进制 CLI**（唯一形态） | 无 GUI / 无 Web / 无桌面；形态唯一才能给出形态化验收（退出码语义 + 字节确定性 + 裸二进制直跑）。候选语言按"单静态二进制"筛选，M1 前在 `plan/02` 写死 |
| 2026-10-20 | **风险①：confdiff（41★，2026-08 建、npm 0.17.2、迭代极快、已支持 8 格式）可能在 3 个月内补上数量归一** | **证伪条件（可机械判定）：其 CHANGELOG / release notes 出现 `quantity` / `duration` / 单位归一 相关条目，或 golden 复现脚本显示其测试C 组不再误报**。触发后本题的响应不是加功能对抗，而是把差异化进一步收窄到"默认值双侧抵消 + 影响分类 + 门禁退出码"（这一层它不做） |
| 2026-10-20 | **风险②：默认值表是长期负债** | 默认值随 K8s 版本变化，且每个 kind 都有一批。硬纪律：**必须外置为版本化 JSON，profile 名带版本（如 `k8s/v1.29`）**；未命中规则时**明确降级为纯结构 diff 并在输出头部标注**，禁止静默假装已比较。完整 defaulting 表列为扩展项，不进首期承诺 |
| 2026-10-20 | **风险③：`indeterminate` 会被嫌吵** | 用户会抱怨"这工具老是让我看 indeterminate"。硬纪律：**默认只对已知族归一，未知族按原样比较**（不升级为 indeterminate）；`--strict` 才把未知族升级。`indeterminate` 一律带理由码，不允许无理由 `unknown` |
| 2026-10-20 | 首期范围：**YAML / JSON 双格式 + 3 值族 + 5 影响类**，共 4 个必做模块 + 1 个延后模块 | 原本想一次上 8 格式（对齐 confdiff），会把 M1 的代数引擎做薄。砍到双格式后 M1 能把**精确有理数**做透——这是与所有竞品的分水岭 |
| 2026-10-20 | 值族只做三个：**K8s Quantity / Go Duration / 十进制数值（含整值浮点折叠）** | 三者覆盖测试A/B/C 的全部误报。布尔族与 null 三态属"顺手做但不扩张"：布尔族严格按 YAML 1.1 全族，null 三态不折叠 |
| 2026-10-20 | **百分比默认当字符串**，仅 profile 显式声明为 ratio 时才换算 | 百分比在不同语义下含义不同（CPU 占比 / 阈值 / 权重），默认换算会产生新的静默等价。宁可保守 |
| 2026-10-20 | **非 K8s 字节单位（MB vs MiB）判 `indeterminate`** | 十进/二进存在真实书写歧义，判等是错的，判不等也是错的。这是 `indeterminate` 存在的最好例证，必须写进 README |
| 2026-10-20 | **禁止浮点参与相等判定**（任何路径出现即视为 bug） | `1e3` vs `1000`、`1.50` vs `1.5`、`1Gi` vs `1024Mi` 全部走精确有理数与整数纳秒。浮点一旦进入比较，diffyml 那种"同一行数字相同却判变更"的错误就会以另一种形式复现 |
| 2026-10-20 | 序列**三模式显式选择**（`ordered` / `set` / `keyed:<field>`），不做自动猜测；`keyed:` 重复键直接报错 | 自动猜测会把"结构不确定"伪装成"无差异"；重复键静默取首会让真实差异消失。**不做模糊匹配、不做相似度** |
| 2026-10-20 | 默认值算法：**双侧同时施加 → 相同默认值相互抵消 → 只有显式非默认才报差异** | 单侧施加会把"一侧写了默认值、一侧没写"判成差异（这正是 dyff 那两条 `+ imagePullPolicy` / `+ protocol` 的成因）。必须构造"显式写非默认值"的对照组证明没有过度静默 |
| 2026-10-20 | 影响类定为 **5 行为类 + 1 非行为类**（资源·收缩 / 部署·收缩 / 镜像·变更（semver 细分 major·minor·patch）/ 接口·暴露 / 配置·新增·移除 / 纯书写） | **纯书写只计数不逐条列**——这是"知道就够了"的那一类，逐条列只会把输出变回 dyff 的样子 |
| 2026-10-20 | 退出码 `0/1/2/3` **一处定义并逐位断言** | `0` 等价 / `1` 有行为差异 / `2` 有 indeterminate / `3` 解析或用法错误。`gate --fail-on` 只让列出的类触发 `1`，其余照常输出但不阻断 |
| 2026-10-20 | **弃 qdiff（纯 Quantity 判定器）** | 形态是 library / 单函数，没有首屏、没有门禁、没有影响分类；写出来是"30 行代码 + 一个 README"，无法作为独立工具发布。Quantity 归一作为 M1 的一部分保留，不单独成题 |
| 2026-10-20 | **弃 helmvalues-delta（helm values 跨环境对照器）** | 三面夹击：helm-diff（3507★）+ argocd-diff-preview（766★）+ dyff（1898★）；且真实使用必然需要 helm 二进制或模板求值，与"单静态二进制、不调外部命令"的形态冲突。需求原话（HN 40861457）保留作本题证据 |
| 2026-10-20 | **弃 envparity（多来源环境变量对齐）** | 需求证据不足：`env var diff dotenv` / `dotenv semantic diff` 检索全 0 且**没有找到任何用户原话**。0 结果本身不能当"空白"，无原话则无从证明痛点。降为扩展项 |
| 2026-10-20 | **弃"更好的 YAML 结构化 diff"路线** | dyff / graphtage / diffyml / diffx / confdiff 已占，且这条路线的天花板是"渲染得更漂亮"，不解决 CI 误报。本题结构比较只做"够用且确定" |
| 2026-10-20 | **弃集群交互**（kubectl / helm / API Server 探测） | 需要集群即失去"单静态二进制可在任何 CI 上跑"的核心优势；且 k8s#118325 表明 server-side diff 自身还会卡 immutable 字段。默认值表来自公开 manifest 语义，不靠探测 |
| 2026-10-20 | 交付措辞纪律：只能写"零运行时依赖 / 单静态二进制 / 不调外部命令" | **不得宣称"无第三方库"**——构建期允许第三方解析库，运行期不分发动态依赖才是真实承诺。夸大会在评审时被逐字核对 |
| 2026-10-20 | 数值口径一处定义（`data/README.md §四`） | 文法、浮点禁令、序列三模式、默认值施加顺序、退出码、影响分类表**只定义一次**；plan/01、plan/03、代码、golden 全部引用同一份 |
| 2026-10-20 | 发布里程碑采用"Release 附产物"口径 | M6 DoD 三件套：**① git tag ② GitHub Release 附产物（≥1 个手工上传附件：三平台二进制 + `SHA256SUMS`；本地留副本 + sha256 可复验）③ 发布留档文件**（`plan/RELEASE-M6.md`，须在 tag 之前进 tag 树） |

## 三、承诺-证据台账

> 全项目**只此一份**。plan/01 与 `data/README.md` 的相关表述均为引用，不得复制出第二份台账。状态列须与里程碑同步翻转。

| 承诺项 | 素材来源 | 交付物内可达性 | 核验方式 | 状态 |
|---|---|---|---|---|
| Quantity 精确有理数归一 | apimachinery 文法（公开）+ 测试C 实测误报 | M1 引擎内嵌 | 测试C 七组全等 + `1Gi` vs `1000M` 判不等 | ✅ M2（golden c01–c03 判等、c08 反例 rc=1；`TestQuantityGrammar`） |
| Duration 纳秒归一 | Go `time.ParseDuration` 文法（公开）+ 测试C | M1 引擎内嵌 | `90s`==`1m30s`、`1h`==`60m` 断言 | ✅ M2（golden c04/c05；`TestDurationGrammar` + `TestGrammarBoundaries`） |
| 十进制精确归一 + 整值浮点折叠 | 测试C 第 6/7 组实测 | M1 引擎内嵌 | `1e3`==`1000`、`1.50`==`1.5` 断言 | ✅ M2（golden c06/c07；`+1≡1`/`-0≡0` 由 `TestDecimalSignFolding` 钉死） |
| YAML 1.1 布尔全族 | YAML 1.1 规范（公开） | M1 引擎内嵌 | 全族归一断言 + `"true"` 不强转断言 | ✅ M2（`TestYAML11BoolFamily`） |
| null 三态不折叠 | 口径定义（data/README §四） | M1 引擎内嵌 | 三态各自可见断言 | ✅ M2（`TestNullThreeStates`） |
| `indeterminate` 带理由码 | MB/MiB 歧义实测 | M1 引擎内嵌 + 退出码 2 | `MB` vs `MiB` → 理由码断言 | ✅ M2（golden c09 exit 2 + `indeterminate(ambiguous_unit_family)`） |
| 浮点禁令 | 决策（禁止浮点相等） | 代码评审 + 测试 | 静态扫描 + 断言 | ✅ M2（`tests/invariants_test.go:TestNoFloatInComparisonPath`，扫 20 个非测试文件 0 命中） |
| 键序无关比较 | 测试A/B 实测（env 顺序） | M2 引擎内嵌 | 键重排零差异断言 | ✅ M3（golden `testM3-noise-keys` + `TestFourStructuralNoiseClasses`） |
| 序列三模式 | 决策（不做自动猜测） | M2 引擎内嵌 | `set` 乱序零差异；`keyed:` 重复键报错 | ✅ M3（golden `testM3-set-reorder` / `testM3-set-duplicate` / `testM3-keyed-order` / `testM3-keyed-duplicate`） |
| 注释/缩进/引号零噪声 | 覆盖矩阵（dyff 此列已 ✅） | M2 解析阶段 | 四类输入零差异条目断言 | ✅ M3（golden `testM3-noise-{keys,indent,quotes,comments}`，四例均 0 条目 exit 0） |
| K8s 默认值双侧抵消 | 测试A 实测（dyff 4 条 → 目标 0 条） | 模块 M3（里程碑 M4）+ 版本化 JSON profile | 测试A 4 → 0；显式非默认对照组仍报 1 条 | ✅ M4（`testA-profile` exit 0 / behavioral 0 / cosmetic 2；`testA-ctrl-profile` exit 1；golden `testM4-rule01..10-cancel/nondefault` 共 20 例；变异：禁用施加 → 21 例红，覆盖显式值 → 11 例红） |
| 10 条高频默认值 | 公开 manifest 语义（候选清单见 plan/01 M3） | `data/profiles/k8s/v1.29.json` | 逐条 golden + 版本号进输出头 | ✅ M4（`testM4/rule01–rule10`：每条「缺失 vs 显式默认」抵消 + 「显式非默认」仍报 1 条；头部带 `profile k8s/v1.29 (hits: N)`） |
| profile 未命中降级 | 决策（风险②） | 输出头部标注 | 未知 kind 输入 → 头部出现降级标注 | ✅ M4（golden `testM4-degraded-header`：ConfigMap 双侧未命中 → 头部含 `degraded: structural-only` 与 `hits: 0`；变异：删标注 → 该例红） |
| 影响分类五类 + 纯书写 | plan/01 M4 表 | 模块 M4（里程碑 M5）引擎内嵌 | 五类各一例 + 纯书写只计数断言 | ✅ M5（六类各一例端到端：`classify-resource-shrink` / `classify-deploy-shrink` / `classify-image-*` / `classify-exposure` / `classify-config-remove` / `classify-cosmetic`；纯书写只计数不逐条列） |
| 镜像 semver 细分 | semver 规范（公开） | 模块 M4（里程碑 M5）引擎内嵌 | major/minor/patch/non-semver 四例断言 | ✅ M5（`classify-image-{major,minor,patch,nonsemver}` 四条 golden 的 `contains` 断言 `镜像·变更/<sub_class>` 出现在输出面；`TestImageSemverClassification` 单测四条） |
| 退出码 0/1/2/3 | 决策（一处定义） | CLI + README 命令表 | 逐码参数化断言 | ✅ 单点定义 + `TestExitCodeContract` + golden `exit-*` 逐码断言 |
| `gate --fail-on` 选择性阻断 | 决策（门禁语义） | CLI `gate` 子命令 | 未列出的行为差异不阻断断言 | ✅ `TestGateSelectiveBlocking` + golden `gate-ignore-shrink` |
| TOML/INI/.env/HCL 适配 | 测试D 实测 | M5 适配器 | 测试D TOML 零噪声 + HCL 重排零差异 | ✅ M5（`testD-toml` / `testD-hcl` / `testD-ini` / `testD-env` 均 0 条目 exit 0；按扩展名显式分派，未知扩展名 exit 3 不回落） |
| HCL 不求值 | 决策（只做结构解析） | M5 `unevaluated` 标注 | 表达式节点标注断言 | ✅ M5（`testD-hcl-unevaluated`：exit 2，输出含 `indeterminate(unevaluated_expression)` 与左右原文；`TestHCLExpressionIsUnevaluated` / `TestUnevaluatedExpressions`） |
| golden 四组夹具 | 测试A/B/C/D 实测 | `testdata/golden/` | `selftest` 一条命令全跑 | ✅ 已落盘（76 例，76 pass / 0 显式 skip） |
| 三家基线留档 | dyff v1.12.0 / diffyml v1.8.2 / confdiff 0.17.2 实测 | `testdata/baseline/` | 留档含版本 + 命令 + 输出 | ✅ 已落盘（40 个 `.out` + 四件齐全守门） |
| 单静态二进制 | 形态决策 | Release 附件 | 干净机器直接运行；`file` / 依赖检查 | ✅ M6（`file dist-release/cfgdelta-linux-amd64` → `ELF 64-bit LSB executable, x86-64, statically linked, stripped`；二进制内无 `ld-linux` 动态加载器字符串；windows/amd64 产物在本机裸跑 `--version` + `selftest` 76 passed） |
| README 首屏 4→0 对照 | 测试A golden | `docs/testA-before-after.png` | 文件存在 + 内容与 golden 一致 | ✅ M5（1280×760 PNG；左 `total 4 · behavioral 2 · cosmetic 2` exit 1，右 `total 2 · behavioral 0 · cosmetic 2` exit 0；README 首屏引用） |
| 发布可用性 | GitHub Releases | 源码 + 二进制 + SHA256SUMS | 干净环境 clone 跑通 README；真下载 sha256 复验 | ✅ M6（tag `v0.1.0`；Release 四件已发布；`gh release download` 后 `sha256sum -c SHA256SUMS` 三行全 OK 并真跑 `--version` + `selftest`；干净 clone 按 README 跑通——证据见 `plan/RELEASE-M6.md §1.6 / §2.2 / §2.4`） |
| 完整 defaulting 表（扩展项） | — | 不承诺首期 | 非首期 DoD | 扩展项 |
| helm values 覆盖链（扩展项） | 需求真实（HN 40861457），形态冲突 | 不承诺首期 | 非首期 DoD | 扩展项 |
| 多来源 env parity（扩展项） | 需求证据不足（检索全 0、无原话） | 不承诺首期 | 非首期 DoD | 扩展项 |

## 四、里程碑

| 里程碑 | 内容 | DoD 要点 | 状态 |
|---|---|---|---|
| **M0 装订与骨架** | 第零步装订（AGENTS.md + 布局门 + plan/00 装订行）+ plan/00–06 + Go 骨架（5 子命令）+ CI | 布局门 9 组变异全红；`go vet` / `go test` / `selftest` 全绿；单静态二进制可跑；CI workflow 本地解析通过 | ✅ 完成（2026-10-20，本地等价校验，未接远端） |
| **M1 数据先行** | golden 四组夹具（测试A/B/C/D）+ 三家基线留档 + 10 条默认值表 + `plan/02`/`03`/`04` 落盘 | 夹具最小可复现；基线留档含版本号与调用命令；默认值表为版本化 JSON 且每条带来源标注；`plan/03` 模块数与 plan/01 一致 | ✅ 完成（2026-10-20：三家 40 个 `.out` 留档 + 守门 7 组变异全红；10 条规则逐条核对到上游源码，抓出并修掉 1 条条件性默认值） |
| **M2 归一化** | M1 模块（normalize 引擎）：Quantity / Duration / 十进制 / 布尔族 / null 三态 / `indeterminate` | 测试C 七组 100% 判等；`1Gi` vs `1000M` 判不等；`MB` vs `MiB` 判 indeterminate 且带理由码；浮点禁令有静态拦截；双跑字节一致 | ✅ 完成（2026-10-20：golden 35 例 33 pass / 2 显式 skip；测试C 七组全等、c08 反例 rc=1、c09 exit 2；浮点门 0 命中；双跑 `cmp` 逐字节一致；修掉 `ParseDecimal` 不接受符号导致 `+1`/`-0` 报错的缺陷，新增 5 条文法边界 + 1 条非法 YAML golden） |
| **M3 结构比较** | M2 模块（diff 引擎）：键序无关 / 序列三模式 / 注释零噪声 | 四类结构差异零条目；`keyed:` 重复键报错；`set` 乱序零差异；双跑字节一致 | ✅ 完成（2026-10-20：新增 `testM3` 10 条 golden——四类噪声均 0 条目 exit 0、set 乱序 0 条、set 重复 exit 2、keyed 乱序 0 条、keyed 重复 exit 3 且输出带 `/items/1`、RFC 6901 `/a~1b`+`/c~0d`；`selftest` 45 例 43 pass / 2 显式 skip；双跑 `cmp` 三组逐字节一致） |
| **M4 默认值** | M3 模块（profile）：双侧施加 + 抵消 + 未命中降级 | **测试A 4 条 → 0 条**；显式非默认对照组仍报 1 条；未知 profile 版本明确报错；`--dump-normalized` 可见双侧默认值 | ✅ 完成（2026-10-20：`testA-profile` exit 0 / behavioral 0 / cosmetic 2、`testA-ctrl-profile` exit 1、未知版本 exit 3；新增 `testM4/` 32 个夹具 21 条 golden——10 条规则各一对「抵消 + 显式非默认」+ 降级标注；施加顺序改为 §4.11 的路径序并暴露 `Applied` 供断言；selftest 66 例 64 pass / 2 显式 skip；4 组变异全红后还原） |
| **M5 分类门禁** | M4 模块 + M5 模块（多格式适配器）；README 首屏截图 | **测试B 5 条 exit 1 → 0 行为差异 exit 0**；五类各一例；退出码 0/1/2/3 逐位断言；测试D TOML 零噪声；`docs/testA-before-after.png` 落盘 | ✅ 完成（2026-10-20：测试B 的 5 行语义在 `keyed:name` / `set` 下 0 行为差异（`ordered` 下 4+2 是显式模式的诚实结果，口径已查清）；六类各一例 + semver 四例端到端；TOML/HCL/INI/.env 适配器按扩展名分派，未知格式 exit 3；HCL 表达式 `unevaluated_expression`；selftest 76 例 76 pass / 0 skip；首屏截图落盘；4 组变异全红后还原） |
| **M6 发布** | 发布 **GitHub 公开仓库**（署名 `yuluo554`） | **三件套：① git tag ② GitHub Release 附产物（≥1 个手工上传附件：三平台二进制 + `SHA256SUMS`；本地留副本 + sha256 可复验）③ 发布留档（`plan/RELEASE-M6.md`，须在 tag 之前进 tag 树）**；缺任一项即未达成。另：干净环境 clone 按 README 跑通；提交面脱敏扫描 0 命中 | ✅ 完成（2026-10-20：tag `v0.1.0` → `de5599eb`；Release 四件（三平台二进制 + `SHA256SUMS`）已发布并真下载 `sha256sum -c` 复验 OK；`plan/RELEASE-M6.md` 在 tag 前落盘；干净 clone 按 README 跑通 selftest 76 passed；脱敏门 0 命中。已知偏差：v0.1.0 的 `publish` job 因说明文件路径缺陷失败，Release 用该 run 的真实产物手工发布，修正提交在 tag 之后且不回改 tag——详见 `plan/RELEASE-M6.md §2.3`） |

## 五、纪律

- **不写会漂移的硬数字**（测试数 / 用例数 / 通过数 / 代码行数）；指标只写口径与门槛来源，确需数字"以末态回填"。竞品星数与检索结果是**带基准日的快照**（软论据），照写基准日。
- **数值口径一处定义**（`data/README.md §四`），引擎 / golden / 文档全部引用同一份，禁止手写第二份口径。
- **禁止编造文号、数值、日期**：所有 issue 编号、HN item id、版本号、星数、检索结果都必须来自本轮实际查证；0 结果标 ⬜ 软论据。
- **浮点禁令**：任何路径上出现 `float64` 参与相等判定即视为 bug，评审 + 测试双拦。
- **默认值表改动必须走版本**：新建 profile 文件（如 `k8s/v1.30`）而不是原地改 `v1.29`；冻结后的 profile 字节不再变动。
- **`indeterminate` 不许静默**：不许出现无理由的 `unknown`；默认不升级未知族，`--strict` 才升级。
- **诚实口径**：未实测的不写成"实测"；未实现的标 ⬜ 并注明目标里程碑；扩展项不写成首期承诺。
- **"零依赖"措辞**：只能写"零运行时依赖 / 单静态二进制 / 不调外部命令"，不得宣称"无第三方库"。
- **品牌隔离**：文档不提名任何其他项目；不复制其他项目代码。
- **每个里程碑出口的硬约束**：DoD 验证 → 本文件状态回写（台账 + 里程碑表同一次提交）→ 写 `plan/HANDOFF-M<next>.md` → commit →（有远端则 push 等 CI 绿）。
- **发布纪律**：M6 三件套措辞不裁剪；发布留档在 tag 前落盘，tag 后才产生的事实（附件 sha256 / 真下载复核）作为追加提交并**分栏**记录。
- **守门自证纪律**：新增/改写门禁必须做一次变异验证（故意弄坏被测行为 → 门必须变红）。
