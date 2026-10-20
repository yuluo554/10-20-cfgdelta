package value

import (
	"math/big"
	"testing"
)

// 测试C 的七组是本题的**核心真值**（plan/01 §1.2 测试C）。
// 真值由文法机械推导（data/README.md §4.1–4.3），不来自任何工具的观测。
func TestQuantityGrammar(t *testing.T) {
	cases := []struct {
		in   string
		want string // 规范值，冻结形态
	}{
		{"1Gi", "1073741824/1"},
		{"1024Mi", "1073741824/1"},
		{"1000M", "1000000000/1"},
		{"1G", "1000000000/1"},
		{"8000m", "8/1"},
		{"8", "8/1"},
		{"500m", "1/2"},
		{"0.5", "1/2"},
		{"1e3", "1000/1"},
		{"1000", "1000/1"},
		{"1.50", "3/2"},
		{"1.5", "3/2"},
		{"1Ki", "1024/1"},
		{"1n", "1/1000000000"},
		{"-2Gi", "-2147483648/1"},
	}
	for _, c := range cases {
		r, err := ParseQuantity(c.in)
		if err != nil {
			t.Fatalf("ParseQuantity(%q): %v", c.in, err)
		}
		got := ratString(r)
		if got != c.want {
			t.Errorf("ParseQuantity(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// 反例：1Gi != 1000M —— 1024^3 != 1000^3。这是"防过度归一"的哨兵（plan/01 §1.2 M1 验收）。
func TestQuantityCounterexample(t *testing.T) {
	gi, err := ParseQuantity("1Gi")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseQuantity("1000M")
	if err != nil {
		t.Fatal(err)
	}
	if gi.Cmp(m) == 0 {
		t.Fatal("1Gi must NOT equal 1000M (1024^3 != 1000^3)")
	}
}

// 歧义字节单位必须是 indeterminate，绝不静默判等（data/README.md §4.7）。
func TestAmbiguousByteUnit(t *testing.T) {
	for _, in := range []string{"1MB", "1MiB", "1GiB", "1KB"} {
		_, err := ParseQuantity(in)
		ind, ok := err.(*Indeterminate)
		if !ok {
			t.Fatalf("ParseQuantity(%q) err = %v, want *Indeterminate", in, err)
		}
		if ind.Reason != ReasonAmbiguousUnitFamily {
			t.Errorf("ParseQuantity(%q) reason = %q, want %q", in, ind.Reason, ReasonAmbiguousUnitFamily)
		}
	}
}

func TestDurationGrammar(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"90s", 90_000_000_000},
		{"1m30s", 90_000_000_000},
		{"1h", 3_600_000_000_000},
		{"60m", 3_600_000_000_000},
		{"1ms", 1_000_000},
		{"1us", 1_000},
		{"1µs", 1_000},
		{"1ns", 1},
		{"0", 0},
		{"1.5s", 1_500_000_000},
		{"2h45m", 9_900_000_000_000},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if err != nil {
			t.Fatalf("ParseDuration(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ParseDuration(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestDurationRejectsBareNumber(t *testing.T) {
	// "90" 没有单位 → 非法（Go time.ParseDuration 同款语义，§4.2）。
	if _, err := ParseDuration("90"); err == nil {
		t.Fatal(`ParseDuration("90") must fail: a bare number is not a duration`)
	}
}

// 浮点禁令的**行为级**哨兵：1e3 与 1000 必须走完全相同的规范值。
// （静态扫描见 tests/float_ban_test.go。）
func TestNoFloatInEquality(t *testing.T) {
	a, err := ParseDecimal("1e3")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseDecimal("1000")
	if err != nil {
		t.Fatal(err)
	}
	if a.Cmp(b) != 0 {
		t.Fatalf("1e3 != 1000 (%s vs %s)", a.RatString(), b.RatString())
	}
	// 精确有理数：浮点会在 0.1+0.2 这类值上暴露差异。
	x, _ := ParseDecimal("0.1")
	y, _ := ParseDecimal("0.2")
	z, _ := ParseDecimal("0.3")
	sum := new(big.Rat).Add(x, y)
	if sum.Cmp(z) != 0 {
		t.Fatal("0.1 + 0.2 != 0.3 under exact rational arithmetic")
	}
}

func TestRatioDefaultIsString(t *testing.T) {
	// 百分比默认当字符串（§4.6）：InferFamily 不把 "50%" 认成数值。
	n := scalarNode("50%")
	if got := InferFamily(n, "/some/path"); got != FamilyString {
		t.Fatalf("InferFamily(50%%) = %s, want string (ratio is opt-in only)", got)
	}
	r, err := ParseRatio("50%")
	if err != nil {
		t.Fatal(err)
	}
	if r.Cmp(big.NewRat(1, 2)) != 0 {
		t.Fatalf(`ParseRatio("50%%") = %s, want 1/2`, r.RatString())
	}
}

func TestYAML11BoolFamily(t *testing.T) {
	trues := []string{"y", "Y", "yes", "Yes", "YES", "true", "True", "TRUE", "on", "On", "ON"}
	falses := []string{"n", "N", "no", "No", "NO", "false", "False", "FALSE", "off", "Off", "OFF"}
	for _, s := range trues {
		v, ok := LookupYAML11Bool(s)
		if !ok || !v {
			t.Errorf("LookupYAML11Bool(%q) = (%v,%v), want (true,true)", s, v, ok)
		}
	}
	for _, s := range falses {
		v, ok := LookupYAML11Bool(s)
		if !ok || v {
			t.Errorf("LookupYAML11Bool(%q) = (%v,%v), want (false,true)", s, v, ok)
		}
	}
	// 引号包裹的 "true" 不自动强转（§4.4）：InferFamily 应把它当 string。
	quoted := scalarNode("true")
	quoted.Style = 2
	if got := InferFamily(quoted, "/x"); got != FamilyString {
		t.Fatalf(`quoted "true" inferred as %s, want string`, got)
	}
	plain := scalarNode("true")
	if got := InferFamily(plain, "/x"); got != FamilyBool {
		t.Fatalf("plain true inferred as %s, want bool", got)
	}
}

func TestNullThreeStates(t *testing.T) {
	absent, _ := Normalize(nil, "/x", FamilyNull, false)
	explicit, _ := Normalize(docNode("!!null", ""), "/x", FamilyNull, false)
	empty, _ := Normalize(docNode("!!str", ""), "/x", FamilyNull, false)
	if absent.Null != NullAbsent {
		t.Errorf("absent = %q", absent.Null)
	}
	if explicit.Null != NullExplicit {
		t.Errorf("explicit = %q", explicit.Null)
	}
	if empty.Null != NullEmpty {
		t.Errorf("empty = %q", empty.Null)
	}
	if absent.String() == explicit.String() || explicit.String() == empty.String() {
		t.Fatal("null three states must not fold into each other")
	}
}

func TestCanonicalStringIsFrozen(t *testing.T) {
	r, _ := ParseQuantity("1Gi")
	c := Canonical{Family: FamilyQuantity, Rat: r}
	if got := c.String(); got != "quantity:1073741824/1" {
		t.Fatalf("canonical string = %q, want frozen form quantity:1073741824/1", got)
	}
	d := Canonical{Family: FamilyDuration, Ns: 90_000_000_000}
	if got := d.String(); got != "duration:90000000000" {
		t.Fatalf("canonical string = %q, want duration:90000000000", got)
	}
}
