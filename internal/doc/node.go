// Package doc 定义统一内部文档树与 YAML/JSON 解析入口（plan/03 §一）。
//
// 设计要点：
//   - 保留原文 Raw 与解析 tag Tag、书写风格 Style —— 精确归一的前提；
//   - 注释在解析阶段即丢弃（不参与比较），由输出头标注；
//   - 绝不经过 float64：JSON 用 UseNumber，YAML 保留标量文本。
package doc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kind 是节点种类。
type Kind int

const (
	Scalar Kind = iota
	Mapping
	Sequence
)

func (k Kind) String() string {
	switch k {
	case Mapping:
		return "mapping"
	case Sequence:
		return "sequence"
	}
	return "scalar"
}

// Node 是内部文档树节点。
type Node struct {
	Kind     Kind
	Tag      string // yaml 原生 tag：!!str / !!int / !!float / !!bool / !!null
	Style    yaml.Style
	Raw      string  // 标量原文
	Children []*Node // Mapping: [k0,v0,k1,v1,...] 保持文档顺序；Sequence: 按序
	Line     int
	Column   int
}

// IsQuoted 表示该标量是否被引号包裹（YAML 布尔族只对 plain 标量生效，§4.4）。
func (n *Node) IsQuoted() bool {
	return n != nil && n.Kind == Scalar && n.Style != 0
}

// Format 描述输入格式。
type Format int

const (
	Unknown Format = iota
	YAML
	JSON
	TOML
	INI
	DotEnv
	HCL
)

func (f Format) String() string {
	switch f {
	case YAML:
		return "yaml"
	case JSON:
		return "json"
	case TOML:
		return "toml"
	case INI:
		return "ini"
	case DotEnv:
		return "env"
	case HCL:
		return "hcl"
	}
	return "unknown"
}

// SupportedFormats 是错误信息里列出的完整支持面（未知扩展名必须明确报错）。
var SupportedFormats = ".yaml/.yml/.json/.toml/.ini/.env/.hcl/.tf"

// ParseError 携带 file:line:col，可直接映射到 exit 3。
type ParseError struct {
	File string
	Line int
	Col  int
	Msg  string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Col, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Msg)
}

// newParseError 从 yaml/json 的错误文本里抽出 "line N" / "offset N" 定位信息。
// 语法错误必须能指到行，否则用户拿到 exit 3 也不知道改哪一行（plan/03 失败态矩阵）。
func newParseError(file, msg string) *ParseError {
	pe := &ParseError{File: file, Line: 1, Col: 1, Msg: msg}
	for _, marker := range []string{"line "} {
		if idx := strings.Index(msg, marker); idx >= 0 {
			rest := msg[idx+len(marker):]
			n := 0
			digits := 0
			for _, r := range rest {
				if r < '0' || r > '9' {
					break
				}
				n = n*10 + int(r-'0')
				digits++
			}
			if digits > 0 {
				pe.Line = n
			}
		}
	}
	return pe
}

// DetectFormat 按**扩展名**显式分派。
//
// 不做内容嗅探回落：`.toml` 文件以 `[service]` 开头时，任何"首字节是 [ 就当 JSON"
// 的嗅探都会把它送进 encoding/json（M4 实测的真实缺陷）。未知扩展名返回 Unknown，
// 由 ParseBytes 明确报错（exit 3），**绝不**静默按 YAML 解析成一个大标量。
func DetectFormat(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return YAML
	case ".json":
		return JSON
	case ".toml":
		return TOML
	case ".ini":
		return INI
	case ".env":
		return DotEnv
	case ".hcl", ".tf":
		return HCL
	}
	return Unknown
}

// ParseFile 读文件并解析为统一文档树。
func ParseFile(path string) (*Node, Format, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Unknown, &ParseError{File: path, Msg: err.Error()}
	}
	return ParseBytes(path, data)
}

// ParseBytes 解析字节：按扩展名分派到对应适配器。
func ParseBytes(path string, data []byte) (*Node, Format, error) {
	format := DetectFormat(path)
	switch format {
	case JSON:
		node, err := parseJSON(path, data)
		return node, JSON, err
	case YAML:
		node, err := parseYAML(path, data)
		return node, YAML, err
	case TOML:
		node, err := parseTOML(path, data)
		return node, TOML, err
	case INI:
		node, err := parseINI(path, data)
		return node, INI, err
	case DotEnv:
		node, err := parseDotEnv(path, data)
		return node, DotEnv, err
	case HCL:
		node, err := parseHCL(path, data)
		return node, HCL, err
	}
	return nil, Unknown, &ParseError{
		File: path,
		Line: 1, Col: 1,
		Msg: fmt.Sprintf("unsupported input format %q (supported: %s)", filepath.Ext(path), SupportedFormats),
	}
}

func parseJSON(path string, data []byte) (*Node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			return &Node{Kind: Scalar, Tag: "!!null", Raw: ""}, nil
		}
		return nil, &ParseError{File: path, Msg: err.Error()}
	}
	if dec.More() {
		return nil, &ParseError{File: path, Msg: "trailing content after JSON value"}
	}
	return jsonToNode(raw, path), nil
}

func jsonToNode(raw any, path string) *Node {
	switch v := raw.(type) {
	case nil:
		return &Node{Kind: Scalar, Tag: "!!null", Raw: ""}
	case bool:
		if v {
			return &Node{Kind: Scalar, Tag: "!!bool", Raw: "true"}
		}
		return &Node{Kind: Scalar, Tag: "!!bool", Raw: "false"}
	case json.Number:
		text := v.String()
		tag := "!!int"
		if strings.ContainsAny(text, ".eE") {
			tag = "!!float"
		}
		return &Node{Kind: Scalar, Tag: tag, Raw: text}
	case string:
		return &Node{Kind: Scalar, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Raw: v}
	case []any:
		n := &Node{Kind: Sequence}
		for _, item := range v {
			n.Children = append(n.Children, jsonToNode(item, path))
		}
		return n
	case map[string]any:
		n := &Node{Kind: Mapping}
		// 键集合按字节序落树：JSON 对象本无序，提前定序让下游不依赖 map 迭代。
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, k := range keys {
			n.Children = append(n.Children,
				&Node{Kind: Scalar, Tag: "!!str", Style: yaml.DoubleQuotedStyle, Raw: k},
				jsonToNode(v[k], path))
		}
		return n
	}
	return &Node{Kind: Scalar, Tag: "!!str", Raw: fmt.Sprint(raw)}
}

func parseYAML(path string, data []byte) (*Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var docs []*Node
	for {
		var yn yaml.Node
		err := dec.Decode(&yn)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, newParseError(path, err.Error())
		}
		docs = append(docs, fromYAML(&yn))
	}
	switch len(docs) {
	case 0:
		return &Node{Kind: Scalar, Tag: "!!null", Raw: ""}, nil
	case 1:
		return docs[0], nil
	default:
		return &Node{Kind: Sequence, Children: docs}, nil
	}
}

func fromYAML(yn *yaml.Node) *Node {
	if yn == nil {
		return &Node{Kind: Scalar, Tag: "!!null", Raw: ""}
	}
	// yaml.v3 的 Decode 根是 DocumentNode：必须先剥掉这一层，否则整棵文档被当成空节点。
	if yn.Kind == yaml.DocumentNode {
		if len(yn.Content) == 0 {
			return &Node{Kind: Scalar, Tag: "!!null", Raw: ""}
		}
		return fromYAML(yn.Content[0])
	}
	n := &Node{Tag: yn.Tag, Style: yn.Style, Raw: yn.Value, Line: yn.Line, Column: yn.Column}
	switch yn.Kind {
	case yaml.MappingNode:
		n.Kind = Mapping
		for i := 0; i+1 < len(yn.Content); i += 2 {
			n.Children = append(n.Children, fromYAML(yn.Content[i]), fromYAML(yn.Content[i+1]))
		}
	case yaml.SequenceNode:
		n.Kind = Sequence
		for _, c := range yn.Content {
			n.Children = append(n.Children, fromYAML(c))
		}
	case yaml.AliasNode:
		// 别名不展开：下游按标量处理并标 indeterminate(unevaluated_expression)。
		n.Kind = Scalar
		n.Tag = "!!alias"
	default:
		n.Kind = Scalar
	}
	return n
}

// sortStrings 是字节序排序的单一入口（UTF-8 lexicographic）。
func sortStrings(s []string) { sortStringsImpl(s) }

// Get 按键取 Mapping 的子节点；不存在返回 nil（与"显式 null"严格区分）。
func (n *Node) Get(key string) *Node {
	if n == nil || n.Kind != Mapping {
		return nil
	}
	for i := 0; i+1 < len(n.Children); i += 2 {
		if n.Children[i].Raw == key {
			return n.Children[i+1]
		}
	}
	return nil
}

// Keys 返回 Mapping 的键（文档顺序）。
func (n *Node) Keys() []string {
	if n == nil || n.Kind != Mapping {
		return nil
	}
	keys := make([]string, 0, len(n.Children)/2)
	for i := 0; i+1 < len(n.Children); i += 2 {
		keys = append(keys, n.Children[i].Raw)
	}
	return keys
}

// SortedKeys 返回按 UTF-8 字节序排序的键（比较器的唯一入口）。
func (n *Node) SortedKeys() []string {
	keys := n.Keys()
	sortStrings(keys)
	return keys
}
