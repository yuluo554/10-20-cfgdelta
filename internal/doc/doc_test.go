package doc

import "testing"

// yaml.v3 的 Decode 根是 DocumentNode —— 忘了剥这层会把整棵文档读成空节点。
// 这条测试是那个缺陷的回归（M0 实测：整份 diff 变成 "0 差异"）。
func TestYAMLRootIsUnwrapped(t *testing.T) {
	n, format, err := ParseBytes("x.yaml", []byte("a: 1\nb: two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if format != YAML {
		t.Fatalf("format = %v, want yaml", format)
	}
	if n.Kind != Mapping {
		t.Fatalf("root kind = %v, want mapping (DocumentNode not unwrapped?)", n.Kind)
	}
	if got := n.Get("a"); got == nil || got.Raw != "1" {
		t.Fatalf("a = %+v", got)
	}
	if got := n.Get("b"); got == nil || got.Raw != "two" {
		t.Fatalf("b = %+v", got)
	}
}

func TestYAMLStyleDistinguishesQuoted(t *testing.T) {
	n, _, err := ParseBytes("x.yaml", []byte("plain: yes\nquoted: \"yes\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n.Get("plain").IsQuoted() {
		t.Error("plain scalar must not be quoted")
	}
	if !n.Get("quoted").IsQuoted() {
		t.Error(`"yes" must be reported as quoted (so it stays a string)`)
	}
}

func TestJSONNumbersKeepRawText(t *testing.T) {
	n, format, err := ParseBytes("x.json", []byte(`{"a": 1.50, "b": 1e3, "c": 1000}`))
	if err != nil {
		t.Fatal(err)
	}
	if format != JSON {
		t.Fatalf("format = %v, want json", format)
	}
	// 原文必须保留：1.50 不能变成 1.5，1e3 不能变成 1000（否则丢失"书写形式"这条信息）。
	for key, want := range map[string]string{"a": "1.50", "b": "1e3", "c": "1000"} {
		got := n.Get(key)
		if got == nil || got.Raw != want {
			t.Errorf("%s raw = %v, want %q", key, got, want)
		}
	}
}

func TestJSONKeyOrderIsSorted(t *testing.T) {
	n, _, err := ParseBytes("x.json", []byte(`{"z": 1, "a": 2, "m": 3}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "m", "z"}
	got := n.Keys()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys = %v, want %v", got, want)
		}
	}
}

func TestMultiDocumentYAML(t *testing.T) {
	n, _, err := ParseBytes("x.yaml", []byte("a: 1\n---\nb: 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n.Kind != Sequence {
		t.Fatalf("multi-doc root kind = %v, want sequence", n.Kind)
	}
	if len(n.Children) != 2 {
		t.Fatalf("documents = %d, want 2", len(n.Children))
	}
}

func TestEmptyFileIsNullNotError(t *testing.T) {
	n, _, err := ParseBytes("x.yaml", []byte(""))
	if err != nil {
		t.Fatalf("empty file must not be a parse error: %v", err)
	}
	if n.Tag != "!!null" {
		t.Fatalf("empty file tag = %q, want !!null", n.Tag)
	}
}

func TestParseErrorCarriesPosition(t *testing.T) {
	_, _, err := ParseBytes("x.yaml", []byte("a: [1, 2\n"))
	if err == nil {
		t.Fatal("malformed YAML must be a parse error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("error type = %T, want *ParseError", err)
	}
	if pe.File != "x.yaml" {
		t.Errorf("file = %q", pe.File)
	}
	if pe.Line == 0 {
		t.Error("parse error should carry a line number")
	}
}

func TestCommentsAreDropped(t *testing.T) {
	n, _, err := ParseBytes("x.yaml", []byte("# top\na: 1 # trailing\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n.Get("a") == nil || n.Get("a").Raw != "1" {
		t.Fatal("comment handling changed the value")
	}
	if len(n.Children) != 2 {
		t.Fatalf("children = %d, want 2 (one key + one value; comments must not create nodes)", len(n.Children))
	}
}

// ── M5：多格式适配器（按扩展名分派，只做结构解析）────────────────────────────

func TestDetectFormatIsExtensionDriven(t *testing.T) {
	cases := map[string]Format{
		"a.yaml": YAML, "a.yml": YAML, "a.json": JSON,
		"a.toml": TOML, "a.ini": INI, "a.env": DotEnv,
		"a.hcl": HCL, "a.tf": HCL,
	}
	for path, want := range cases {
		if got := DetectFormat(path); got != want {
			t.Errorf("DetectFormat(%q) = %v, want %v", path, got, want)
		}
	}
}

// 未知扩展名必须明确报错，**不回落到 YAML**。
// 回归靶子：`.toml` 以 `[service]` 开头时曾被内容嗅探送进 encoding/json。
func TestUnknownExtensionIsExplicitError(t *testing.T) {
	if got := DetectFormat("a.txt"); got != Unknown {
		t.Fatalf("DetectFormat(a.txt) = %v, want Unknown", got)
	}
	_, _, err := ParseBytes("a.txt", []byte("a: 1\n"))
	if err == nil {
		t.Fatal("unknown extension must be an explicit error, not a YAML fallback")
	}
	pe, ok := err.(*ParseError)
	if !ok || pe.Line == 0 {
		t.Fatalf("error = %v, want *ParseError with a position", err)
	}
}

// `[service]` 必须是表：这是测试D 的 dyff 噪声来源（段被当成字符串）。
func TestTOMLSectionIsTable(t *testing.T) {
	n, format, err := ParseBytes("a.toml", []byte("[service]\nname = \"synth\"\nport = 8080\n"))
	if err != nil {
		t.Fatal(err)
	}
	if format != TOML {
		t.Fatalf("format = %v, want toml", format)
	}
	svc := n.Get("service")
	if svc == nil || svc.Kind != Mapping {
		t.Fatalf("[service] parsed as %+v, want Mapping", svc)
	}
	if got := svc.Get("name").Raw; got != "synth" {
		t.Errorf("name = %q", got)
	}
	if got := svc.Get("port"); got == nil || got.Tag != TagInt || got.Raw != "8080" {
		t.Errorf("port = %+v, want !!int 8080", got)
	}
}

func TestTOMLKeyOrderIsIrrelevant(t *testing.T) {
	a, _, err := ParseBytes("a.toml", []byte("[service]\nname = \"synth\"\nport = 8080\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := ParseBytes("b.toml", []byte("[service]\nport = 8080\nname = \"synth\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Children) != len(b.Children) || len(a.Get("service").Children) != len(b.Get("service").Children) {
		t.Fatal("table shape differs")
	}
	if a.Get("service").Get("port").Raw != b.Get("service").Get("port").Raw {
		t.Fatal("scalar raw differs")
	}
}

func TestTOMLDuplicateKeyIsError(t *testing.T) {
	if _, _, err := ParseBytes("a.toml", []byte("a = 1\na = 2\n")); err == nil {
		t.Fatal("duplicate TOML key must be a parse error")
	}
}

func TestTOMLArraysAndNumbers(t *testing.T) {
	n, _, err := ParseBytes("a.toml", []byte("nums = [1, 2, 3,]\nhex = 0x10\nf = 1.50\nok = true\nwhen = 1979-05-27T07:32:00Z\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := n.Get("nums"); got == nil || got.Kind != Sequence || len(got.Children) != 3 {
		t.Fatalf("nums = %+v, want 3-element sequence", got)
	}
	if got := n.Get("hex").Raw; got != "16" {
		t.Errorf("hex = %q, want 16 (decimal form, no float)", got)
	}
	if got := n.Get("f").Tag; got != TagFloat {
		t.Errorf("f tag = %q, want !!float", got)
	}
	if got := n.Get("ok").Tag; got != TagBool {
		t.Errorf("ok tag = %q, want !!bool", got)
	}
	if got := n.Get("when"); got == nil || got.Tag != TagStr || !got.IsQuoted() {
		t.Errorf("datetime must stay a (quoted-style) string, got %+v", got)
	}
}

func TestINISectionsAndComments(t *testing.T) {
	n, format, err := ParseBytes("a.ini", []byte("; top\n[service]\nname = synth ; trailing\nport = \"8080\"\n[service.tls]\nenabled = true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if format != INI {
		t.Fatalf("format = %v, want ini", format)
	}
	svc := n.Get("service")
	if svc == nil || svc.Kind != Mapping {
		t.Fatalf("service = %+v", svc)
	}
	if got := svc.Get("name").Raw; got != "synth" {
		t.Errorf("name = %q (trailing comment not stripped?)", got)
	}
	// INI 没有类型系统：`true` / `8080` 都按字符串落树。
	for _, k := range []string{"port", "enabled"} {
		target := svc
		if k == "enabled" {
			target = svc.Get("tls")
		}
		if got := target.Get(k); got == nil || got.Tag != TagStr || !got.IsQuoted() {
			t.Errorf("%s must stay a string, got %+v", k, got)
		}
	}
}

func TestINIDuplicateKeyIsError(t *testing.T) {
	if _, _, err := ParseBytes("a.ini", []byte("[s]\na = 1\na = 2\n")); err == nil {
		t.Fatal("duplicate INI key must be a parse error")
	}
}

func TestDotEnvQuotingAndExport(t *testing.T) {
	n, format, err := ParseBytes("a.env", []byte("export APP=synth\nQ=\"a b # not comment\"\nHASH=abc # comment\n"))
	if err != nil {
		t.Fatal(err)
	}
	if format != DotEnv {
		t.Fatalf("format = %v, want env", format)
	}
	if got := n.Get("APP").Raw; got != "synth" {
		t.Errorf("APP = %q", got)
	}
	if got := n.Get("Q").Raw; got != "a b # not comment" {
		t.Errorf("Q = %q (quoted # must not start a comment)", got)
	}
	if got := n.Get("HASH").Raw; got != "abc" {
		t.Errorf("HASH = %q (inline comment not stripped?)", got)
	}
}

func TestHCLBlockShapeAndAttributeOrder(t *testing.T) {
	src := "resource \"thing\" \"b\" {\n  count = 1\n  name  = \"synth\"\n}\n"
	n, format, err := ParseBytes("a.hcl", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if format != HCL {
		t.Fatalf("format = %v, want hcl", format)
	}
	b := n.Get("resource").Get("thing").Get("b")
	if b == nil || b.Kind != Mapping {
		t.Fatalf("resource.thing.b = %+v", b)
	}
	if got := b.Get("name").Raw; got != "synth" {
		t.Errorf("name = %q", got)
	}
	if got := b.Get("count"); got == nil || got.Tag != TagInt || got.Raw != "1" {
		t.Errorf("count = %+v", got)
	}
}

// HCL 表达式保留原文、标 unevaluated —— 绝不求值、绝不静默当字符串比。
func TestHCLExpressionIsUnevaluated(t *testing.T) {
	n, _, err := ParseBytes("a.hcl", []byte("resource \"thing\" \"b\" {\n  count = var.n + 1\n  name  = \"x-${var.env}\"\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	b := n.Get("resource").Get("thing").Get("b")
	if got := b.Get("count"); !IsUnevaluated(got) {
		t.Fatalf("count = %+v, want unevaluated", got)
	}
	if got := b.Get("name"); !IsUnevaluated(got) {
		t.Fatalf("interpolated string = %+v, want unevaluated", got)
	}
}

func TestHCLParseErrorHasPosition(t *testing.T) {
	_, _, err := ParseBytes("a.hcl", []byte("resource \"x\" {\n  a = \n}\n"))
	if err == nil {
		t.Fatal("missing expression must be a parse error")
	}
	pe, ok := err.(*ParseError)
	if !ok || pe.Line == 0 {
		t.Fatalf("error = %v, want *ParseError with line", err)
	}
}
