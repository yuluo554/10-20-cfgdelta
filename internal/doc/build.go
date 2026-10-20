package doc

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// 本文件是各**非 YAML 适配器**共用的建树原语。
//
// 纪律：适配器只做结构解析，产出的节点与 YAML/JSON 路径进入**同一棵**文档树；
// 标量必须带可比较的 tag 与原文（Raw），绝不在这里做值归一（那是 internal/value 的事）。

// Value tags used by the structural adapters. YAML's scalar tags are reused so
// the downstream normalizer does not need a per-format branch.
const (
	TagStr         = "!!str"
	TagInt         = "!!int"
	TagFloat       = "!!float"
	TagBool        = "!!bool"
	TagNull        = "!!null"
	TagUnevaluated = "!!unevaluated" // HCL expression kept verbatim (never evaluated)
)

func newMapping(line, col int) *Node {
	return &Node{Kind: Mapping, Line: line, Column: col}
}

func newSequence(line, col int) *Node {
	return &Node{Kind: Sequence, Line: line, Column: col}
}

func newString(s string, quoted bool, line, col int) *Node {
	style := yaml.Style(0)
	if quoted {
		style = yaml.DoubleQuotedStyle
	}
	return &Node{Kind: Scalar, Tag: TagStr, Style: style, Raw: s, Line: line, Column: col}
}

func newNumber(tag, raw string, line, col int) *Node {
	return &Node{Kind: Scalar, Tag: tag, Raw: raw, Line: line, Column: col}
}

func newBool(raw string, line, col int) *Node {
	return &Node{Kind: Scalar, Tag: TagBool, Raw: raw, Line: line, Column: col}
}

func newUnevaluated(raw string, line, col int) *Node {
	return &Node{Kind: Scalar, Tag: TagUnevaluated, Raw: raw, Line: line, Column: col}
}

// IsUnevaluated 判定节点是否为"保留原文、不参与求值"的表达式节点。
func IsUnevaluated(n *Node) bool { return n != nil && n.Tag == TagUnevaluated }

// setKey 在 mapping 上插入一个键；键已存在时返回 false（调用方决定报错还是合并）。
func (n *Node) setKey(key string, v *Node, line, col int) bool {
	for i := 0; i+1 < len(n.Children); i += 2 {
		if n.Children[i].Raw == key {
			return false
		}
	}
	n.Children = append(n.Children, newString(key, false, line, col), v)
	return true
}

// lookup 按键取子节点（不区分"不存在"与"显式 null"由调用方判断）。
func (n *Node) lookup(key string) *Node { return n.Get(key) }

// ensureTable 取得（必要时创建）一个子 table。
func ensureTable(parent *Node, key string, line, col int) (*Node, error) {
	if existing := parent.Get(key); existing != nil {
		if existing.Kind != Mapping {
			return nil, fmt.Errorf("key %q at line %d is not a table", key, line)
		}
		return existing, nil
	}
	child := newMapping(line, col)
	parent.Children = append(parent.Children, newString(key, false, line, col), child)
	return child, nil
}
