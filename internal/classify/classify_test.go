package classify

import (
	"testing"

	"github.com/yuluo554/10-20-cfgdelta/internal/diff"
	"github.com/yuluo554/10-20-cfgdelta/internal/value"
)

func rat(num, den int64) *value.Canonical {
	return &value.Canonical{Family: value.FamilyQuantity, Rat: value.SetBigRat(num, den)}
}

// gate 语义：只有列出的类阻断；未列出的行为差异照常输出但不阻断（§4.12）。
func TestGateSelectiveBlocking(t *testing.T) {
	sum := Summary{Behavioral: 2, ByClass: map[Class]int{ResourceShrink: 2}}
	if !Gate(sum, []Class{ResourceShrink}, false) {
		t.Fatal("listed class present → must block")
	}
	if Gate(sum, []Class{Exposure}, false) {
		t.Fatal("unlisted class only → must NOT block")
	}
	if !Gate(sum, nil, false) {
		t.Fatal("no --fail-on → any behavioural diff blocks")
	}
}

// indeterminate 恒阻断（除非 --allow-indeterminate），且优先于 1。
func TestGateIndeterminate(t *testing.T) {
	sum := Summary{Behavioral: 0, Indeterminate: 1, ByClass: map[Class]int{}}
	if !Gate(sum, []Class{ResourceShrink}, false) {
		t.Fatal("indeterminate must block regardless of --fail-on")
	}
	if Gate(sum, []Class{ResourceShrink}, true) {
		t.Fatal("--allow-indeterminate must suppress the indeterminate block")
	}
}

// 资源·收缩：归一后变小才命中；变大不命中。
func TestResourceShrinkDirection(t *testing.T) {
	entries := []diff.Entry{
		{Path: "/spec/template/spec/containers/0/resources/limits/memory", Kind: diff.Changed, Left: rat(1073741824, 1), Right: rat(536870912, 1)},
	}
	res := Apply(entries)
	if got := res.Entries[0].Class; got != string(ResourceShrink) {
		t.Fatalf("class = %q, want %q", got, ResourceShrink)
	}

	grow := []diff.Entry{
		{Path: "/spec/template/spec/containers/0/resources/limits/memory", Kind: diff.Changed, Left: rat(536870912, 1), Right: rat(1073741824, 1)},
	}
	res = Apply(grow)
	if got := res.Entries[0].Class; got == string(ResourceShrink) {
		t.Fatalf("growing a limit must not be classified as 资源·收缩")
	}
}

// 镜像 semver 细分：major / minor / patch / non-semver 四例。
func TestImageSemverClassification(t *testing.T) {
	mk := func(l, r string) diff.Entry {
		return diff.Entry{
			Path: "/spec/template/spec/containers/0/image", Kind: diff.Changed,
			Left:  &value.Canonical{Family: value.FamilyString, Str: l},
			Right: &value.Canonical{Family: value.FamilyString, Str: r},
		}
	}
	cases := []struct{ l, r, want string }{
		{"registry.example/x:1.2.3", "registry.example/x:2.0.0", "major"},
		{"registry.example/x:1.2.3", "registry.example/x:1.3.0", "minor"},
		{"registry.example/x:1.2.3", "registry.example/x:1.2.4", "patch"},
		{"registry.example/x:latest", "registry.example/x:1.2.4", "non-semver"},
	}
	for _, c := range cases {
		res := Apply([]diff.Entry{mk(c.l, c.r)})
		if res.Entries[0].Class != string(ImageChange) {
			t.Fatalf("%s -> %s: class = %q, want %q", c.l, c.r, res.Entries[0].Class, ImageChange)
		}
		if res.Entries[0].SubClass != c.want {
			t.Errorf("%s -> %s: sub_class = %q, want %q", c.l, c.r, res.Entries[0].SubClass, c.want)
		}
	}
}

// 纯书写只计数、不进行为类；且计数从明细算得。
func TestCosmeticCounting(t *testing.T) {
	entries := []diff.Entry{
		{Path: "/a", Kind: diff.CosmeticKind},
		{Path: "/b", Kind: diff.CosmeticKind},
		{Path: "/c", Kind: diff.Changed, Left: rat(1, 1), Right: rat(2, 1)},
	}
	res := Apply(entries)
	if res.Summary.Cosmetic != 2 {
		t.Errorf("cosmetic = %d, want 2", res.Summary.Cosmetic)
	}
	if res.Summary.Behavioral != 1 {
		t.Errorf("behavioral = %d, want 1", res.Summary.Behavioral)
	}
	if res.Summary.Total != 3 {
		t.Errorf("total = %d, want 3", res.Summary.Total)
	}
}

// 分类不确定 → ConfigChange + class_guess，**禁止**归 Cosmetic（§4.13）。
func TestUnknownNeverBecomesCosmetic(t *testing.T) {
	entries := []diff.Entry{{
		Path: "/spec/somethingUnknown", Kind: diff.Changed,
		Left:  &value.Canonical{Family: value.FamilyString, Str: "a"},
		Right: &value.Canonical{Family: value.FamilyString, Str: "b"},
	}}
	res := Apply(entries)
	if res.Entries[0].Class == string(Cosmetic) {
		t.Fatal("uncertain classification must never be folded into 纯书写")
	}
	if !res.Entries[0].Guess {
		t.Fatal("uncertain classification must carry class_guess: true")
	}
}

// 分类排序固定：先按影响类，再按路径字节序（不依赖 map 迭代）。
func TestFixedOrdering(t *testing.T) {
	entries := []diff.Entry{
		{Path: "/z", Kind: diff.CosmeticKind},
		{Path: "/b", Kind: diff.Changed, Left: rat(3, 1), Right: rat(2, 1)},
		{Path: "/a", Kind: diff.Changed, Left: rat(3, 1), Right: rat(2, 1)},
	}
	res := Apply(entries)
	if res.Entries[0].Path != "/a" || res.Entries[1].Path != "/b" {
		t.Fatalf("within a class, paths must be byte-sorted: %+v", res.Entries)
	}
	if res.Entries[2].Class != string(Cosmetic) {
		t.Fatalf("cosmetic is last in the fixed class order, got %+v", res.Entries[2])
	}
}
