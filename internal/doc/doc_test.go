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
