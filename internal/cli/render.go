package cli

import (
	"fmt"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/classify"
	"github.com/yuluo554/10-20-cfgdelta/internal/diff"
	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
)

// renderText 是人读面：纯文本、无颜色、无时间戳（K-02 双跑逐字节一致）。
//
// 纯书写条目**只计数、不逐条列**（§4.13）——把它们逐条打出来，
// 输出就退化回 dyff 那个样子，正是本题要避免的。
func renderText(r classify.Result) string {
	var b strings.Builder
	printed := 0
	for _, e := range r.Entries {
		if e.Kind == diff.CosmeticKind {
			continue
		}
		b.WriteString(renderEntry(e))
		b.WriteString("\n")
		printed++
	}
	if printed == 0 {
		b.WriteString("equivalent (no behavioural differences)\n")
	}
	// 汇总计数从明细计算得出；纯书写只在这行体现。
	b.WriteString(fmt.Sprintf("summary: total %d · behavioral %d · cosmetic %d · indeterminate %d\n",
		r.Summary.Total, r.Summary.Behavioral, r.Summary.Cosmetic, r.Summary.Indeterminate))
	return b.String()
}

func renderEntry(e diff.Entry) string {
	left := diff.CanonicalOf(e.Left)
	right := diff.CanonicalOf(e.Right)
	class := e.Class
	if e.SubClass != "" {
		class += "/" + e.SubClass
	}
	if e.Guess {
		class += " (class_guess)"
	}
	switch e.Kind {
	case diff.Added:
		return fmt.Sprintf("+ %s  %s  [%s]", e.Path, right, class)
	case diff.Removed:
		return fmt.Sprintf("- %s  %s  [%s]", e.Path, left, class)
	case diff.IndeterminateKind:
		line := fmt.Sprintf("? %s  indeterminate(%s)", e.Path, e.Reason)
		if e.Detail != "" {
			line += "  " + e.Detail
		}
		return line
	}
	return fmt.Sprintf("~ %s  %s -> %s  [%s]", e.Path, left, right, class)
}

// renderJSON 是机读面：手写有序输出，不经 map（K-02 确定性）。
func renderJSON(head string, r classify.Result, ar interface{}) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "header": ` + jsonStr(head) + ",\n")
	b.WriteString(`  "summary": {` + "\n")
	b.WriteString(fmt.Sprintf(`    "total": %d,`+"\n", r.Summary.Total))
	b.WriteString(fmt.Sprintf(`    "behavioral": %d,`+"\n", r.Summary.Behavioral))
	b.WriteString(fmt.Sprintf(`    "cosmetic": %d,`+"\n", r.Summary.Cosmetic))
	b.WriteString(fmt.Sprintf(`    "indeterminate": %d`+"\n", r.Summary.Indeterminate))
	b.WriteString("  },\n")
	b.WriteString(`  "entries": [`)
	if len(r.Entries) == 0 {
		b.WriteString("]\n")
	} else {
		b.WriteString("\n")
		for i, e := range r.Entries {
			b.WriteString("    {")
			b.WriteString(`"path": ` + jsonStr(e.Path) + ", ")
			b.WriteString(`"kind": ` + jsonStr(string(e.Kind)) + ", ")
			b.WriteString(`"class": ` + jsonStr(e.Class))
			if e.SubClass != "" {
				b.WriteString(", " + `"sub_class": ` + jsonStr(e.SubClass))
			}
			if e.Guess {
				b.WriteString(`, "class_guess": true`)
			}
			if e.Reason != "" {
				b.WriteString(", " + `"reason": ` + jsonStr(e.Reason))
			}
			if e.Detail != "" {
				b.WriteString(", " + `"detail": ` + jsonStr(e.Detail))
			}
			if e.Left != nil {
				b.WriteString(", " + `"left": ` + jsonStr(diff.CanonicalOf(e.Left)))
			}
			if e.Right != nil {
				b.WriteString(", " + `"right": ` + jsonStr(diff.CanonicalOf(e.Right)))
			}
			b.WriteString("}")
			if i != len(r.Entries)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("  ]\n")
	}
	b.WriteString("}")
	return b.String()
}

func jsonStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(fmt.Sprintf(`\u%04x`, r))
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// renderTree 把（已施加默认值的）文档树渲染为稳定的缩进形式。
// 键按文档顺序输出，方便人读与 --dump-normalized 对照。
func renderTree(n *doc.Node) string {
	var b strings.Builder
	renderTreeAt(&b, n, 0)
	return b.String()
}

func renderTreeAt(b *strings.Builder, n *doc.Node, depth int) {
	indent := strings.Repeat("  ", depth)
	if n == nil {
		b.WriteString(indent + "<absent>\n")
		return
	}
	switch n.Kind {
	case doc.Mapping:
		if len(n.Children) == 0 {
			b.WriteString(indent + "{}\n")
			return
		}
		keys := n.SortedKeys()
		for _, k := range keys {
			v := n.Get(k)
			if v != nil && (v.Kind == doc.Mapping || v.Kind == doc.Sequence) {
				b.WriteString(indent + k + ":\n")
				renderTreeAt(b, v, depth+1)
				continue
			}
			b.WriteString(indent + k + ": " + scalarText(v) + "\n")
		}
	case doc.Sequence:
		if len(n.Children) == 0 {
			b.WriteString(indent + "[]\n")
			return
		}
		for _, c := range n.Children {
			if c != nil && (c.Kind == doc.Mapping || c.Kind == doc.Sequence) {
				b.WriteString(indent + "-\n")
				renderTreeAt(b, c, depth+1)
				continue
			}
			b.WriteString(indent + "- " + scalarText(c) + "\n")
		}
	default:
		b.WriteString(indent + scalarText(n) + "\n")
	}
}

func scalarText(n *doc.Node) string {
	if n == nil {
		return "<absent>"
	}
	if n.Tag == "!!null" {
		return "null"
	}
	return n.Raw
}
