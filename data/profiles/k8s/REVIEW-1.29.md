# data/profiles/k8s/v1.29.json — 十条规则逐条核对记录

> 核对日期：**2026-10-20**。核对对象：`data/profiles/k8s/v1.29.json`（构建镜像：`internal/profile/profiles/k8s/v1.29.json`）。
> 核对口径：每条规则的 `source` 必须能指到**上游公开源码的具体位置与行为**，不接受"看起来合理"。
> 证据获取方式：`api.github.com` contents API 拉取 `kubernetes/kubernetes` **v1.29.0** 与 `kubernetes/api` **kubernetes-1.29.0** 的源文件原文后逐行比对（本机 `raw.githubusercontent.com` 直连失败，走 API 通道）。

## 结论一览

| # | 规则 | 行为（K8s 1.29 源码） | 无条件? | 本次核对动作 |
|---|---|---|---|---|
| 1 | `/spec/template/spec/containers/*/imagePullPolicy` = `IfNotPresent` | `tag == "latest"` → `Always`，否则 `IfNotPresent`；**无 tag 会被归一成 `latest`** | ❌ **有条件** | **修**：新增 `only_if_image_tag_not_latest` 条件字段并在 `Apply` 中执行 |
| 2 | `/spec/template/spec/containers/*/ports/*/protocol` = `TCP` | `ContainerPort.Protocol == ""` → `"TCP"` | ✅ | 核对通过；`source` 补到生成文件与测试用例 |
| 3 | `/spec/containers/*/ports/*/protocol` = `TCP`（Pod） | 同上 | ✅ | 核对通过 |
| 4 | `/spec/template/spec/terminationGracePeriodSeconds` = `30` | `nil` → `DefaultTerminationGracePeriodSeconds`（常量 = 30） | ✅ | 核对通过；补常量出处 |
| 5 | `/spec/template/spec/restartPolicy` = `Always` | `""` → `RestartPolicyAlways` | ✅ | 核对通过 |
| 6 | `/spec/template/spec/dnsPolicy` = `ClusterFirst` | `""` → `DNSClusterFirst` | ✅ | 核对通过 |
| 7 | `/spec/template/spec/serviceAccountName` = `default` | `DefaultServiceAccountName = "default"`，由 **ServiceAccount admission 插件**填入 | ✅（但机制不同） | 核对通过；**`source` 明确写成 admission 插件**，不再写成"registry defaulting" |
| 8 | `/spec/revisionHistoryLimit` = `10` | `nil` → `10` | ✅ | 核对通过 |
| 9 | `/spec/strategy/type` = `RollingUpdate` | `""` → `RollingUpdateDeploymentStrategyType` | ✅ | 核对通过 |
| 10 | `/spec/progressDeadlineSeconds` = `600` | `nil` → `600` | ✅ | 核对通过 |

## 逐条证据

### 1. imagePullPolicy — 唯一一条**有条件**的规则

`kubernetes/kubernetes` v1.29.0 · `pkg/apis/core/v1/defaults.go`

```go
func SetDefaults_Container(obj *v1.Container) {
	if obj.ImagePullPolicy == "" {
		_, tag, _, _ := parsers.ParseImageName(obj.Image)
		if tag == "latest" {
			obj.ImagePullPolicy = v1.PullAlways
		} else {
			obj.ImagePullPolicy = v1.PullIfNotPresent
		}
	}
	...
}
```

`pkg/util/parsers/parsers.go`

```go
	// If no tag was specified, use the default "latest".
	if len(tag) == 0 && len(digest) == 0 {
		tag = "latest"
	}
```

**判定**：默认值不是常量，取决于同容器 `image`。
- `image: repo/web:latest` → `Always`
- `image: repo/web`（无 tag）→ `Always`（ParseImageName 归一）
- `image: repo/web@sha256:...`（无 tag 有 digest）→ tag 为空、不等 `latest` → `IfNotPresent`
- `image: repo/web:1.0.0` → `IfNotPresent`

**不修会怎样**（两个方向）：
- 假阳性：左 `:latest` 无 policy（真实默认 `Always`）vs 右 `:latest` 显式 `Always` → 我们补成 `IfNotPresent`，凭空报一条差异。
- **假阴性（更危险）**：左 `:latest` 无 policy vs 右 `:latest` 显式 `IfNotPresent`（**真实有差异**）→ 双侧都被补成 `IfNotPresent`，静默判等。

**修法**：规则加 `"only_if_image_tag_not_latest": true`；`internal/profile` 在补默认值前读同容器的 `image`，tag 为 `latest`/缺失时**不补**（无从判定就不猜）。回归：`TestImagePullPolicyDependsOnImageTag`（6 个子用例，含 registry 端口 `registry.example:5000/web` 不被误当 tag）。

> 这是"逐条核对"真正抓到的东西：M0 时这 10 条看着都合理，核对后只有这一条是**条件性**的，而它恰好是测试A 的主角。

### 2/3. ContainerPort.protocol = TCP

`pkg/apis/core/v1/zz_generated.defaults.go`（由 defaulting 生成器产出）：

```go
			if b.Protocol == "" {
				b.Protocol = "TCP"
			}
```

`pkg/apis/core/v1/defaults_test.go` 的 `ContainerPort` 用例：输入只有 `Name: "default"`，期望 `Protocol: v1.ProtocolTCP`。
**注意**：`pkg/apis/core/v1/defaults.go` 里 line 119 的 `sp.Protocol = "TCP"` 是 **Service** 的端口默认值，**不是** ContainerPort —— 首次核对时差点记错出处，以此条为准。
**判定**：无条件，两条路径（Deployment 模板 / 裸 Pod）成立。

### 4. terminationGracePeriodSeconds = 30

`pkg/apis/core/v1/defaults.go` `SetDefaults_PodSpec`：

```go
	if obj.TerminationGracePeriodSeconds == nil {
		period := int64(v1.DefaultTerminationGracePeriodSeconds)
		obj.TerminationGracePeriodSeconds = &period
	}
```

`kubernetes/api` `kubernetes-1.29.0` · `core/v1/types.go`：

```go
	DefaultTerminationGracePeriodSeconds = 30
```

**判定**：无条件。

### 5/6. restartPolicy = Always / dnsPolicy = ClusterFirst

`pkg/apis/core/v1/defaults.go` `SetDefaults_PodSpec`：

```go
	if obj.DNSPolicy == "" {
		obj.DNSPolicy = v1.DNSClusterFirst
	}
	if obj.RestartPolicy == "" {
		obj.RestartPolicy = v1.RestartPolicyAlways
	}
```

**判定**：无条件。

### 7. serviceAccountName = "default" — 机制与其它 9 条不同

`plugin/pkg/admission/serviceaccount/admission.go`：

```go
	// DefaultServiceAccountName is the name of the default service account to set on pods which do not specify a service account
	DefaultServiceAccountName = "default"
...
	// Set the default service account if needed
	if pod.Spec.ServiceAccountName == "" {
		pod.Spec.ServiceAccountName = DefaultServiceAccountName
	}
```

**判定**：值正确，但这是 **admission 插件**的行为，不是 API 机器层面的 registry defaulting。含义：它只在 ServiceAccount 插件启用时发生（默认安装启用；`--disable-admission-plugins=ServiceAccount` 时不发生）。
处置：`source` 已改成点明 admission 插件；`scope_note` 仍声明"非全量"，不把这条包装成"API Server 无条件默认值"。

### 8/9/10. Deployment 三件

`pkg/apis/apps/v1/defaults.go`：

```go
	// Set default DeploymentStrategyType as RollingUpdate.
	if strategy.Type == "" {
		strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	}
...
	if obj.Spec.RevisionHistoryLimit == nil {
		obj.Spec.RevisionHistoryLimit = new(int32)
		*obj.Spec.RevisionHistoryLimit = 10
	}
	if obj.Spec.ProgressDeadlineSeconds == nil {
		obj.Spec.ProgressDeadlineSeconds = new(int32)
		*obj.Spec.ProgressDeadlineSeconds = 600
	}
```

**判定**：无条件。

## plan/04 候选清单的对账（一处有意偏差）

`plan/04-数据计划.md` 的"首期 10 条候选"第 10 条写的是 **`securityContext.runAsNonRoot`**，实际 JSON 里没有它，改为把 `protocol` 拆成 Deployment / Pod 两条。

**核对结论：`runAsNonRoot` 在 K8s 1.29 里没有 API Server 默认值。**
- `pkg/apis/core/v1/defaults.go` 全文无 `RunAsNonRoot`；
- `pkg/apis/core/v1/zz_generated.defaults.go` 全文无 `RunAsNonRoot`；
- 它只可能由 **PodSecurity admission**（restricted 级别）或用户显式设置产生，属于"策略注入"而非"默认值"。

因此**不纳入默认值表是正确的**（否则会把"策略未启用时的不存在"当作默认值，制造假阴性）。本次把该偏差写进 `plan/04` 的候选清单注记，避免下一棒又把 `runAsNonRoot` 加回来。

## 关于"冻结"的说明

`plan/04` 纪律 2 要求"版本进文件名与 name，冻结后不改字节；新版本 = 新文件"。本次修改了 `v1.29.json` 的字节，属**首次冻结前的修订**：

- 冻结时点定义为 **M6 打发布 tag 时**，当前 `VERSION=0.1.0-dev`，尚未发布；
- 修订内容仅限 `source` 措辞与一条**纠错性**字段（`only_if_image_tag_not_latest`），未增删规则、未改默认值；
- 镜像已用 `node tools/sync-profiles.mjs` 同步，`--check` 为 0 drift。

M6 之后若再发现同类错误，按纪律**新增 `v1.29.x` 文件**，不改本文件。
