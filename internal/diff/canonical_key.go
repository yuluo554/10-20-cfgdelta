package diff

import (
	"fmt"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
	"github.com/yuluo554/10-20-cfgdelta/internal/value"
)

// canonicalKey 为任意子树生成**冻结的**结构键（用于 set 模式）。
//
// 为什么需要它：`set` 模式的元素常常是 mapping（env 条目），把整棵子树归一成
// 一个标量会退化成空串，导致"所有元素都相等"。这里用"键字节序 + 标量按族归一"
// 生成确定字符串，只依赖语义内容（不依赖文档顺序 / 缩进 / 引号 / 注释）。
func canonicalKey(n *doc.Node) string {
	var b strings.Builder
	writeCanonicalKey(&b, n)
	return b.String()
}

func writeCanonicalKey(b *strings.Builder, n *doc.Node) {
	if n == nil {
		b.WriteString("<absent>")
		return
	}
	switch n.Kind {
	case doc.Mapping:
		b.WriteByte('{')
		for i, k := range n.SortedKeys() {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(fmt.Sprintf("%q", k))
			b.WriteByte(':')
			writeCanonicalKey(b, n.Get(k))
		}
		b.WriteByte('}')
	case doc.Sequence:
		b.WriteByte('[')
		for i, c := range n.Children {
			if i > 0 {
				b.WriteByte(',')
			}
			writeCanonicalKey(b, c)
		}
		b.WriteByte(']')
	default:
		// 空 pointer：让 InferFamily 走"非资源上下文"分支，纯数字按 decimal。
		c, err := value.Normalize(n, "", value.FamilyUnknown, false)
		if err != nil {
			// 判不了时退回到"带 tag 的原文"，仍保持确定性，且不把不同值折成同一键。
			b.WriteString(fmt.Sprintf("%s:%q", n.Tag, n.Raw))
			return
		}
		b.WriteString(c.String())
	}
}
