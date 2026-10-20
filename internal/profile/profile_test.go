package profile

import (
	"strings"
	"testing"

	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
)

func TestLoadKnownProfile(t *testing.T) {
	p, err := Load("k8s/v1.29")
	if err != nil {
		t.Fatalf("Load(k8s/v1.29): %v", err)
	}
	if p.Name != "k8s/v1.29" {
		t.Errorf("name = %q", p.Name)
	}
	if len(p.Rules) != 10 {
		t.Errorf("rules = %d, want the 10 high-frequency defaults promised in plan/01 M3", len(p.Rules))
	}
	// 每条规则都必须带来源（data/README.md §二）。
	for i, r := range p.Rules {
		if r.Source == "" {
			t.Errorf("rule %d (%s) has no source attribution", i, r.Path)
		}
		if r.Path == "" || r.Default == "" {
			t.Errorf("rule %d is incomplete: %+v", i, r)
		}
	}
	// 未命中任何规则 → 明确降级，不静默（K-09 / §4.11 第 4 条）。
	if p.ScopeNote == "" {
		t.Error("profile must carry a scope note stating it is not a complete defaulting table")
	}
}

// 未知版本必须**明确报错**，不得回落到"最近的版本"（§4.11 第 6 条）。
func TestUnknownProfileIsError(t *testing.T) {
	if _, err := Load("k8s/v9.99"); err == nil {
		t.Fatal("unknown profile version must be an error, never a silent fallback")
	}
	if _, err := Load(""); err == nil {
		t.Fatal("empty profile name must be an error")
	}
}

// 双侧施加：一侧显式写了默认值、另一侧没写 → 抵消后两侧相同。
// 夹具必须带 image 且 tag 非 latest：imagePullPolicy 的默认值是条件性的（见
// TestImagePullPolicyDependsOnImageTag），而 K8s 里 image 本身是必填字段。
func TestApplyCancelsOnBothSides(t *testing.T) {
	left := parse(t, "kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: registry.example/synth/web:1.0.0\n")
	right := parse(t, "kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: registry.example/synth/web:1.0.0\n          imagePullPolicy: IfNotPresent\n")

	p, err := Load("k8s/v1.29")
	if err != nil {
		t.Fatal(err)
	}
	la := Apply(left, p)
	ra := Apply(right, p)
	if la.Hits == 0 || ra.Hits == 0 {
		t.Fatalf("both sides must record hits: left=%+v right=%+v", la, ra)
	}
	lv := left.Get("spec").Get("template").Get("spec").Get("containers").Children[0].Get("imagePullPolicy")
	rv := right.Get("spec").Get("template").Get("spec").Get("containers").Children[0].Get("imagePullPolicy")
	if lv == nil || rv == nil {
		t.Fatal("defaulting did not fill the missing key on the left side")
	}
	if lv.Raw != rv.Raw {
		t.Fatalf("defaults must cancel: left=%q right=%q", lv.Raw, rv.Raw)
	}
}

// 默认值**不覆盖**显式值（§4.11 第 5 条）——这是防"过度静默"的关键。
func TestApplyDoesNotOverwriteExplicit(t *testing.T) {
	n := parse(t, "kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          imagePullPolicy: Always\n")
	p, err := Load("k8s/v1.29")
	if err != nil {
		t.Fatal(err)
	}
	Apply(n, p)
	got := n.Get("spec").Get("template").Get("spec").Get("containers").Children[0].Get("imagePullPolicy")
	if got == nil || got.Raw != "Always" {
		t.Fatalf("explicit value was overwritten: %+v", got)
	}
}

// 施加顺序必须可观测（§4.11 第 2/3 条）：外层先于内层、父先于子、
// 同层按路径 UTF-8 字节序。只断言"最终值"看不出顺序，所以 Apply 返回
// 逐个补上的具体路径（Applied），这里对**顺序本身**断言。
func TestApplyOrderIsOuterFirstThenBytewise(t *testing.T) {
	// 两个容器、各一个端口：通配符必须展开，且展开顺序确定。
	n := parse(t, "kind: Deployment\n"+
		"spec:\n"+
		"  template:\n"+
		"    spec:\n"+
		"      containers:\n"+
		"        - name: b\n"+
		"          image: registry.example/synth/b:1.0.0\n"+
		"          ports:\n"+
		"            - containerPort: 80\n"+
		"        - name: a\n"+
		"          image: registry.example/synth/a:1.0.0\n"+
		"          ports:\n"+
		"            - containerPort: 81\n")

	p, err := Load("k8s/v1.29")
	if err != nil {
		t.Fatal(err)
	}
	res := Apply(n, p)

	want := []string{
		// 深度 2：同层按字节序 progressDeadlineSeconds < revisionHistoryLimit
		"/spec/progressDeadlineSeconds",
		"/spec/revisionHistoryLimit",
		// 深度 3：strategy 缺失（中间层不猜结构）→ 不补，故不出现
		// 深度 4：同层字节序
		"/spec/template/spec/dnsPolicy",
		"/spec/template/spec/restartPolicy",
		"/spec/template/spec/serviceAccountName",
		"/spec/template/spec/terminationGracePeriodSeconds",
		// 深度 6：父先于子（容器内的键晚于 spec 层）
		"/spec/template/spec/containers/0/imagePullPolicy",
		"/spec/template/spec/containers/1/imagePullPolicy",
		// 深度 8：最深，最后
		"/spec/template/spec/containers/0/ports/0/protocol",
		"/spec/template/spec/containers/1/ports/0/protocol",
	}
	if strings.Join(res.Applied, "\n") != strings.Join(want, "\n") {
		t.Fatalf("apply order = \n%s\nwant \n%s", strings.Join(res.Applied, "\n"), strings.Join(want, "\n"))
	}
	if res.Hits != len(want) {
		t.Fatalf("hits = %d, want %d (one per concrete path)", res.Hits, len(want))
	}
	if res.Degraded {
		t.Fatal("a document that matched 10 rules must not be reported as degraded")
	}

	// 反证：顺序**不是** profile 文件的书写顺序（JSON 里 imagePullPolicy 排第一），
	// 否则"同层按字节序"只是文档里的一句话。
	if strings.HasPrefix(res.Applied[0], "/spec/template/spec/containers") {
		t.Fatalf("apply order follows the profile file order, not path order: %v", res.Applied)
	}
}

// 未命中任何规则 → Degraded=true（输出头会标 degraded: structural-only）。
func TestApplyDegradesWhenNothingMatches(t *testing.T) {
	n := parse(t, "kind: ConfigMap\ndata:\n  a: b\n")
	p, err := Load("k8s/v1.29")
	if err != nil {
		t.Fatal(err)
	}
	res := Apply(n, p)
	if !res.Degraded {
		t.Fatalf("unmatched profile must report degraded, got %+v", res)
	}
}

// imagePullPolicy 的默认值**取决于同容器 image 的 tag**（K8s 1.29 SetDefaults_Container
// + ParseImageName）：`:latest` 与"完全没写 tag"都归到 Always，此时补 IfNotPresent
// 会把"左 `:latest` 无 policy / 右 `:latest` 显式 `IfNotPresent`"这种**真实差异**抵消掉。
func TestImagePullPolicyDependsOnImageTag(t *testing.T) {
	cases := []struct {
		name     string
		image    string // 空串 = 容器没有 image 字段
		wantFill bool
	}{
		{"tagged non-latest", "registry.example/synth/web:1.0.0", true},
		{"tagged latest", "registry.example/synth/web:latest", false},
		{"untagged (ParseImageName -> latest)", "registry.example/synth/web", false},
		{"digest only (empty tag -> IfNotPresent)", "registry.example/synth/web@sha256:0123456789abcdef", true},
		{"registry port is not a tag", "registry.example:5000/synth/web", false},
		{"no image field", "", false},
	}

	p, err := Load("k8s/v1.29")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			src := "kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n"
			if tc.image != "" {
				src += "          image: " + tc.image + "\n"
			}
			n := parse(t, src)
			Apply(n, p)
			got := n.Get("spec").Get("template").Get("spec").Get("containers").Children[0].Get("imagePullPolicy")
			if tc.wantFill && got == nil {
				t.Fatalf("image %q: expected imagePullPolicy to be filled with IfNotPresent", tc.image)
			}
			if !tc.wantFill && got != nil {
				t.Fatalf("image %q: imagePullPolicy must NOT be filled (API Server defaults it to Always), got %q", tc.image, got.Raw)
			}
		})
	}
}

// 显式值永远不会被条件规则覆盖（与 TestApplyDoesNotOverwriteExplicit 互补）。
func TestImagePullPolicyConditionDoesNotOverwriteExplicit(t *testing.T) {
	n := parse(t, "kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: registry.example/synth/web:latest\n          imagePullPolicy: IfNotPresent\n")
	p, err := Load("k8s/v1.29")
	if err != nil {
		t.Fatal(err)
	}
	Apply(n, p)
	got := n.Get("spec").Get("template").Get("spec").Get("containers").Children[0].Get("imagePullPolicy")
	if got == nil || got.Raw != "IfNotPresent" {
		t.Fatalf("explicit value was overwritten: %+v", got)
	}
}

func parse(t *testing.T, src string) *doc.Node {
	t.Helper()
	n, _, err := doc.ParseBytes("inline.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return n
}
