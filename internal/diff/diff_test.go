package diff

import (
	"strings"
	"testing"

	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
	"github.com/yuluo554/10-20-cfgdelta/internal/value"
)

func parse(t *testing.T, src string) *doc.Node {
	t.Helper()
	n, _, err := doc.ParseBytes("inline.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return n
}

func compare(t *testing.T, left, right string, opt Options) Result {
	t.Helper()
	res, err := Compare(parse(t, left), parse(t, right), opt)
	if err != nil {
		t.Fatalf("compare: %v", err)
	}
	return res
}

// M3 验收：键重排 / 缩进变化 / 引号风格变化 / 注释增删 → **零差异条目**。
func TestFourStructuralNoiseClasses(t *testing.T) {
	base := "a: 1\nb:\n  c: 2\n  d: 3\nlist:\n  - x\n  - y\n"

	cases := []struct {
		name  string
		right string
	}{
		{
			name:  "key reorder",
			right: "list:\n  - x\n  - y\nb:\n  d: 3\n  c: 2\na: 1\n",
		},
		{
			name:  "indentation",
			right: "a: 1\nb:\n    c: 2\n    d: 3\nlist:\n- x\n- y\n",
		},
		{
			name:  "quote style",
			right: "a: 1\nb:\n  c: 2\n  d: 3\nlist:\n  - \"x\"\n  - 'y'\n",
		},
		{
			name:  "comments added and removed",
			right: "# leading comment\na: 1  # trailing\nb:\n  c: 2\n  # inner\n  d: 3\nlist:\n  - x\n  - y\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := compare(t, base, c.right, Options{Seq: SeqMode{Kind: "ordered"}})
			if len(res.Entries) != 0 {
				t.Fatalf("%s produced %d entries, want 0: %+v", c.name, len(res.Entries), res.Entries)
			}
		})
	}
}

// set 模式：乱序元素零差异；重复元素 → indeterminate(duplicate_in_set)。
func TestSetMode(t *testing.T) {
	left := "items:\n  - a\n  - b\n  - c\n"
	right := "items:\n  - c\n  - a\n  - b\n"
	res := compare(t, left, right, Options{Seq: SeqMode{Kind: "set"}})
	if len(res.Entries) != 0 {
		t.Fatalf("set mode reorder produced %d entries, want 0: %+v", len(res.Entries), res.Entries)
	}

	dup := "items:\n  - a\n  - a\n  - b\n"
	res = compare(t, left, dup, Options{Seq: SeqMode{Kind: "set"}})
	if !res.HasIndeterminate {
		t.Fatalf("duplicate set elements must be indeterminate, got %+v", res.Entries)
	}
	found := false
	for _, e := range res.Entries {
		if e.Reason == value.ReasonDuplicateInSet {
			found = true
		}
	}
	if !found {
		t.Fatalf("want reason %q in %+v", value.ReasonDuplicateInSet, res.Entries)
	}
}

// keyed: 重复键 → **报错**（exit 3），禁止取第一个（K-05）。
func TestKeyedDuplicateIsError(t *testing.T) {
	left := "items:\n  - name: a\n    v: 1\n  - name: a\n    v: 2\n"
	right := "items:\n  - name: a\n    v: 1\n"
	_, err := Compare(parse(t, left), parse(t, right), Options{Seq: SeqMode{Kind: "keyed", Field: "name"}})
	if err == nil {
		t.Fatal("duplicate keyed field must be an error, not a silent first-wins")
	}
	if !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("error = %v, want it to mention the duplicate key", err)
	}
}

// ordered 模式：长度差成 Added/Removed，不猜配对。
func TestOrderedLengthDifference(t *testing.T) {
	left := "items:\n  - a\n  - b\n"
	right := "items:\n  - a\n"
	res := compare(t, left, right, Options{Seq: SeqMode{Kind: "ordered"}})
	if len(res.Entries) != 1 || res.Entries[0].Kind != Removed {
		t.Fatalf("want exactly one removed entry, got %+v", res.Entries)
	}
	if res.Entries[0].Path != "/items/1" {
		t.Fatalf("path = %q, want /items/1", res.Entries[0].Path)
	}
}

// 路径转义（RFC 6901）：键含 "/" 与 "~" 时必须正确转义。
func TestPathEscaping(t *testing.T) {
	left := `"a/b": 1` + "\n" + `"c~d": 2` + "\n"
	right := `"a/b": 9` + "\n" + `"c~d": 2` + "\n"
	res := compare(t, left, right, Options{Seq: SeqMode{Kind: "ordered"}})
	if len(res.Entries) != 1 {
		t.Fatalf("want 1 entry, got %+v", res.Entries)
	}
	if got, want := res.Entries[0].Path, "/a~1b"; got != want {
		t.Fatalf("path = %q, want %q (RFC 6901)", got, want)
	}
}

// set 模式路径是**成员标识**（pointer[quoted canonical key]），不是 JSON Pointer：
// canonical key 可能含 `]` / `"`，裸拼会截断，所以整段必须被引号包裹。
func TestSetMemberPathIsQuotedIdentity(t *testing.T) {
	left := "items:\n  - alpha\n  - beta\n"
	right := "items:\n  - alpha\n  - beta\n  - charlie\n"
	res := compare(t, left, right, Options{Seq: SeqMode{Kind: "set"}})
	if len(res.Entries) != 1 || res.Entries[0].Kind != Added {
		t.Fatalf("want one added entry, got %+v", res.Entries)
	}
	if got, want := res.Entries[0].Path, `/items["string:charlie"]`; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}

	// 元素自带 `]`：引号外的裸拼接会被误读成路径结束，这里必须保持完整。
	left = "items:\n  - alpha\n"
	right = "items:\n  - alpha\n  - alpha]beta\n"
	res = compare(t, left, right, Options{Seq: SeqMode{Kind: "set"}})
	if len(res.Entries) != 1 {
		t.Fatalf("want one added entry, got %+v", res.Entries)
	}
	if got, want := res.Entries[0].Path, `/items["string:alpha]beta"]`; got != want {
		t.Fatalf("path with embedded ']' = %q, want %q", got, want)
	}
}

// 归一后相等 → CosmeticKind，**不是** Changed（纯书写只计数）。
func TestNormalizedEqualIsCosmetic(t *testing.T) {
	res := compare(t, "m: 1Gi\n", "m: 1024Mi\n", Options{
		Seq:    SeqMode{Kind: "ordered"},
		Coerce: value.NewCoercion([]value.CoerceRule{{Pattern: "/m", Family: value.FamilyQuantity}}),
	})
	if len(res.Entries) != 1 {
		t.Fatalf("want 1 entry, got %+v", res.Entries)
	}
	if res.Entries[0].Kind != CosmeticKind {
		t.Fatalf("kind = %s, want cosmetic", res.Entries[0].Kind)
	}
}

// 反例：1Gi vs 1000M 必须是 Changed（不得被误判成 cosmetic）。
func TestCounterexampleIsChanged(t *testing.T) {
	res := compare(t, "m: 1Gi\n", "m: 1000M\n", Options{
		Seq:    SeqMode{Kind: "ordered"},
		Coerce: value.NewCoercion([]value.CoerceRule{{Pattern: "/m", Family: value.FamilyQuantity}}),
	})
	if len(res.Entries) != 1 || res.Entries[0].Kind != Changed {
		t.Fatalf("1Gi vs 1000M must be a real change, got %+v", res.Entries)
	}
}

// --strict 把"未知族 + 书写不同"升级为 indeterminate(unknown_family)（§4.7）；
// 不升级显式声明的字符串（引号包裹 / 百分比），否则工具会变吵（风险③）。
func TestStrictUpgradesOnlyUnknownFallback(t *testing.T) {
	opt := Options{Seq: SeqMode{Kind: "ordered"}, Strict: true}

	res := compare(t, "weird: foobar\n", "weird: 100%\n", opt)
	if !res.HasIndeterminate || len(res.Entries) != 1 {
		t.Fatalf("strict must upgrade an unknown-family change, got %+v", res.Entries)
	}
	if res.Entries[0].Reason != value.ReasonUnknownFamily {
		t.Fatalf("reason = %q, want %q", res.Entries[0].Reason, value.ReasonUnknownFamily)
	}

	// 引号包裹的字符串是**已声明**的族，不升级。
	res = compare(t, `note: "alpha"`+"\n", `note: "beta"`+"\n", opt)
	if res.HasIndeterminate {
		t.Fatalf("explicitly quoted strings must not be upgraded, got %+v", res.Entries)
	}

	// 百分比默认是字符串族，也不升级。
	res = compare(t, "ratio: 10%\n", "ratio: 20%\n", opt)
	if res.HasIndeterminate {
		t.Fatalf("percentages are strings by default and must not be upgraded, got %+v", res.Entries)
	}
}

// 非 strict 下未知族按原样比较（默认不吵）。
func TestNonStrictComparesUnknownAsIs(t *testing.T) {
	res := compare(t, "weird: foobar\n", "weird: 100%\n", Options{Seq: SeqMode{Kind: "ordered"}})
	if res.HasIndeterminate {
		t.Fatalf("non-strict must not produce indeterminate, got %+v", res.Entries)
	}
	if len(res.Entries) != 1 || res.Entries[0].Kind != Changed {
		t.Fatalf("want one changed entry, got %+v", res.Entries)
	}
}

// M5：HCL 表达式节点。原文相同 → 无条目；原文不同 → 一等 indeterminate，
// 并且**不带** Left/Right 的规范值（因为根本没有求值）。
func TestUnevaluatedExpressions(t *testing.T) {
	parseHCL := func(src string) *doc.Node {
		n, _, err := doc.ParseBytes("inline.hcl", []byte(src))
		if err != nil {
			t.Fatalf("parse hcl: %v", err)
		}
		return n
	}
	a := "resource \"x\" \"y\" {\n  count = var.n\n}\n"
	b := "resource \"x\" \"y\" {\n  count = var.n\n}\n"
	c := "resource \"x\" \"y\" {\n  count = var.m\n}\n"

	res, err := Compare(parseHCL(a), parseHCL(b), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 0 {
		t.Fatalf("identical expressions must produce no entry, got %+v", res.Entries)
	}

	res, err = Compare(parseHCL(a), parseHCL(c), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 1 || res.Entries[0].Kind != IndeterminateKind {
		t.Fatalf("differing expressions = %+v, want one indeterminate entry", res.Entries)
	}
	if res.Entries[0].Reason != value.ReasonUnevaluatedExpression {
		t.Fatalf("reason = %q", res.Entries[0].Reason)
	}
	if !strings.Contains(res.Entries[0].Detail, "var.n -> var.m") {
		t.Fatalf("detail must expose both expression texts, got %q", res.Entries[0].Detail)
	}
}

// 表达式节点**只存在于一侧**时也必须是 indeterminate：既不能当"新增字符串"，
// 也不能当"删除字符串"。这条钉死 value.Normalize 的 unevaluated 分支。
func TestUnevaluatedOneSidedIsIndeterminate(t *testing.T) {
	with := "resource \"x\" \"y\" {\n  count = var.n\n}\n"
	without := "resource \"x\" \"y\" {\n}\n"
	a, _, err := doc.ParseBytes("a.hcl", []byte(with))
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := doc.ParseBytes("b.hcl", []byte(without))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compare(a, b, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 1 || res.Entries[0].Kind != IndeterminateKind {
		t.Fatalf("one-sided expression = %+v, want one indeterminate entry", res.Entries)
	}
	if res.Entries[0].Reason != value.ReasonUnevaluatedExpression {
		t.Fatalf("reason = %q", res.Entries[0].Reason)
	}
}
