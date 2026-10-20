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
// （静态扫描见 tests/invariants_test.go:TestNoFloatInComparisonPath。）
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

// §4.3 符号折叠：`+1` ≡ `1`、`-0` ≡ `0` —— 规范串必须逐字节相同，
// 而不是"看起来相等"。符号若被当字符串残留，这里会立刻红。
func TestDecimalSignFolding(t *testing.T) {
	cases := []struct{ in, want string }{
		{"+1", "1/1"},
		{"1", "1/1"},
		{"-0", "0/1"},
		{"0", "0/1"},
		{"+0.5", "1/2"},
		{"-1.50", "-3/2"},
		{"+1e3", "1000/1"},
	}
	for _, c := range cases {
		r, err := ParseDecimal(c.in)
		if err != nil {
			t.Fatalf("ParseDecimal(%q): %v", c.in, err)
		}
		if got := ratString(r); got != c.want {
			t.Errorf("ParseDecimal(%q) = %s, want %s", c.in, got, c.want)
		}
	}
	for _, pair := range [][2]string{{"+1", "1"}, {"-0", "0"}} {
		a, err := ParseDecimal(pair[0])
		if err != nil {
			t.Fatalf("ParseDecimal(%q): %v", pair[0], err)
		}
		b, err := ParseDecimal(pair[1])
		if err != nil {
			t.Fatalf("ParseDecimal(%q): %v", pair[1], err)
		}
		ca := Canonical{Family: FamilyDecimal, Rat: a}.String()
		cb := Canonical{Family: FamilyDecimal, Rat: b}.String()
		if ca != cb {
			t.Errorf("canonical strings differ: %q vs %q (%q ≡ %q)", ca, cb, pair[0], pair[1])
		}
	}
	// 只有符号、没有数字 → 文法错误（不是 0）。
	if _, err := ParseDecimal("+"); err == nil || !IsParseError(err) {
		t.Fatalf(`ParseDecimal("+") err = %v, want parse error`, err)
	}
}

// §4.1 / §4.2 的文法边界：这些形态在规范里明写，golden 之外单独钉死，
// 防止"顺手放宽文法"把非法输入吞成合法值。
func TestGrammarBoundaries(t *testing.T) {
	// 指数与后缀不可同现（§4.1 边界）。
	for _, in := range []string{"1e3Mi", "1E3Gi", "1e-3Ki"} {
		_, err := ParseQuantity(in)
		if err == nil {
			t.Errorf("ParseQuantity(%q) must fail: exponent and suffix are mutually exclusive", in)
			continue
		}
		if !IsParseError(err) {
			t.Errorf("ParseQuantity(%q) err = %T, want *parseErr", in, err)
		}
	}
	// 前导点 / 显式正号是合法书写（§4.1）。
	for _, c := range []struct{ in, want string }{
		{".5", "1/2"},
		{"+.5", "1/2"},
		{"+1", "1/1"},
		{"+1Gi", "1073741824/1"},
	} {
		r, err := ParseQuantity(c.in)
		if err != nil {
			t.Fatalf("ParseQuantity(%q): %v", c.in, err)
		}
		if got := ratString(r); got != c.want {
			t.Errorf("ParseQuantity(%q) = %s, want %s", c.in, got, c.want)
		}
	}
	// µs（U+00B5）与 μs（U+03BC）两种拼写都接受，且与 us 等值（§4.2）。
	for _, in := range []string{"1us", "1µs", "1μs"} {
		ns, err := ParseDuration(in)
		if err != nil {
			t.Fatalf("ParseDuration(%q): %v", in, err)
		}
		if ns != 1000 {
			t.Errorf("ParseDuration(%q) = %d, want 1000", in, ns)
		}
	}
	// "0" 是 duration 唯一允许的无单位值；"0s" 折叠为同一规范值。
	for _, in := range []string{"0", "0s"} {
		ns, err := ParseDuration(in)
		if err != nil {
			t.Fatalf("ParseDuration(%q): %v", in, err)
		}
		if ns != 0 {
			t.Errorf("ParseDuration(%q) = %d, want 0", in, ns)
		}
	}
}
