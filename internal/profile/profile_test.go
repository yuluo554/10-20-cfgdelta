package profile

import (
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
func TestApplyCancelsOnBothSides(t *testing.T) {
	left := parse(t, "kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n")
	right := parse(t, "kind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          imagePullPolicy: IfNotPresent\n")

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

func parse(t *testing.T, src string) *doc.Node {
	t.Helper()
	n, _, err := doc.ParseBytes("inline.yaml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return n
}
