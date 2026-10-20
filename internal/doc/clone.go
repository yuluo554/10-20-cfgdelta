package doc

// Clone 深拷贝子树。默认值施加会改写树，双侧施加必须各在独立副本上做。
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	out := &Node{
		Kind:   n.Kind,
		Tag:    n.Tag,
		Style:  n.Style,
		Raw:    n.Raw,
		Line:   n.Line,
		Column: n.Column,
	}
	if len(n.Children) > 0 {
		out.Children = make([]*Node, len(n.Children))
		for i, c := range n.Children {
			out.Children[i] = c.Clone()
		}
	}
	return out
}
