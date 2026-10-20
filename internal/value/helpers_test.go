package value

import "github.com/yuluo554/10-20-cfgdelta/internal/doc"

// 测试辅助：构造最小的标量节点。
func scalarNode(raw string) *doc.Node {
	return &doc.Node{Kind: doc.Scalar, Tag: "!!str", Raw: raw}
}

// docNode 构造带指定 tag 的标量节点。
func docNode(tag, raw string) *doc.Node {
	return &doc.Node{Kind: doc.Scalar, Tag: tag, Raw: raw}
}
