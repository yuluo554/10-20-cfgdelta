package diff

import (
	"sort"

	"github.com/yuluo554/10-20-cfgdelta/internal/value"
)

// sortStrings 是 diff 包内的字节序排序入口（与 doc 包同一语义：UTF-8 lexicographic）。
func sortStrings(s []string) { sort.Strings(s) }

// SortEntries 按 (path 字节序) 稳定排序；分类顺序由 classify 阶段另行固定。
func SortEntries(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].Kind < entries[j].Kind
	})
}

// Counts 是条目计数（纯书写只计数不逐条列，§4.13）。
type Counts struct {
	Added         int
	Removed       int
	Changed       int
	Indeterminate int
}

// Tally 从明细**计算得出**计数，不单独维护计数变量。
func Tally(entries []Entry) Counts {
	var c Counts
	for _, e := range entries {
		switch e.Kind {
		case Added:
			c.Added++
		case Removed:
			c.Removed++
		case Changed:
			c.Changed++
		case IndeterminateKind:
			c.Indeterminate++
		}
	}
	return c
}

// canonicalOf 便于测试断言：取条目某一侧规范值的冻结字符串。
func canonicalOf(c *value.Canonical) string {
	if c == nil {
		return ""
	}
	return c.String()
}

// CanonicalOf 导出给测试与渲染层用。
func CanonicalOf(c *value.Canonical) string { return canonicalOf(c) }
