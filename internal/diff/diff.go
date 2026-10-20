// Package diff 实现结构无关比较（plan/03 §三）。
//
// 三条硬纪律：键按 UTF-8 字节序递归比较（键序差异不产生条目）；序列三模式显式选择、
// 不自动猜测；不做模糊匹配 / 相似度 / 启发式对齐。
package diff

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
	"github.com/yuluo554/10-20-cfgdelta/internal/value"
)

// EntryKind 是差异条目类别。
type EntryKind string

const (
	Added         EntryKind = "added"
	Removed       EntryKind = "removed"
	Changed       EntryKind = "changed"
	// CosmeticKind = 归一后语义相同、只是书写不同（§4.13 纯书写：**只计数，不逐条列**）。
	CosmeticKind      EntryKind = "cosmetic"
	IndeterminateKind EntryKind = "indeterminate"
)

// Entry 是一条差异。
type Entry struct {
	Path  string
	Kind  EntryKind
	Left  *value.Canonical
	Right *value.Canonical
	Reason string // Indeterminate 的理由码
	Class string  // classify 阶段回填
	SubClass string
	Guess bool // class_guess: true（分类不确定）
}

// SeqMode 是序列比较模式（§4.10），显式选择、不做自动猜测。
type SeqMode struct {
	Kind  string // ordered | set | keyed
	Field string // keyed 模式下的对齐字段
}

// ParseSeqMode 解析 --seq-mode 取值。
func ParseSeqMode(s string) (SeqMode, error) {
	switch {
	case s == "" || s == "ordered":
		return SeqMode{Kind: "ordered"}, nil
	case s == "set":
		return SeqMode{Kind: "set"}, nil
	case strings.HasPrefix(s, "keyed:"):
		field := strings.TrimPrefix(s, "keyed:")
		if field == "" {
			return SeqMode{}, fmt.Errorf("--seq-mode keyed: requires a field name (keyed:<field>)")
		}
		return SeqMode{Kind: "keyed", Field: field}, nil
	}
	return SeqMode{}, fmt.Errorf("unknown --seq-mode %q (want ordered | set | keyed:<field>)", s)
}

// Options 控制比较行为。
type Options struct {
	Seq      SeqMode
	Coerce   *value.Coercion
	Strict   bool
}

// Result 是比较结果。
type Result struct {
	Entries []Entry
	HasIndeterminate bool
}

// Compare 递归比较两棵文档树。
func Compare(left, right *doc.Node, opt Options) (Result, error) {
	var res Result
	if err := compareNode("", left, right, opt, &res); err != nil {
		return res, err
	}
	return res, nil
}

func compareNode(p string, l, r *doc.Node, opt Options, res *Result) error {
	switch {
	case l == nil && r == nil:
		return nil
	case l == nil:
		return recordRemoved(p, r, opt, res)
	case r == nil:
		return recordRemoved(p, l, opt, res)
	}

	lk, rk := l.Kind, r.Kind
	if lk != rk {
		// 一方 mapping 一方 sequence：不猜对齐，按 remove+add 成对列出。
		if err := recordRemoved(p, l, opt, res); err != nil {
			return err
		}
		return recordAdded(p, r, opt, res)
	}

	switch lk {
	case doc.Mapping:
		return compareMapping(p, l, r, opt, res)
	case doc.Sequence:
		return compareSequence(p, l, r, opt, res)
	default:
		return compareScalar(p, l, r, opt, res)
	}
}

func compareMapping(p string, l, r *doc.Node, opt Options, res *Result) error {
	// 键按 UTF-8 字节序递归比较；键序差异**不产生条目**。
	keys := unionSorted(l.SortedKeys(), r.SortedKeys())
	for _, key := range keys {
		lv, rv := l.Get(key), r.Get(key)
		if err := compareNode(value.JoinPointer(p, key), lv, rv, opt, res); err != nil {
			return err
		}
	}
	return nil
}

func compareSequence(p string, l, r *doc.Node, opt Options, res *Result) error {
	switch opt.Seq.Kind {
	case "set":
		return compareSet(p, l, r, opt, res)
	case "keyed":
		return compareKeyed(p, l, r, opt, res)
	default:
		return compareOrdered(p, l, r, opt, res)
	}
}

func compareOrdered(p string, l, r *doc.Node, opt Options, res *Result) error {
	n := len(l.Children)
	if len(r.Children) > n {
		n = len(r.Children)
	}
	for i := 0; i < n; i++ {
		child := value.JoinPointer(p, strconv.Itoa(i))
		var lc, rc *doc.Node
		if i < len(l.Children) {
			lc = l.Children[i]
		}
		if i < len(r.Children) {
			rc = r.Children[i]
		}
		if err := compareNode(child, lc, rc, opt, res); err != nil {
			return err
		}
	}
	return nil
}

func compareSet(p string, l, r *doc.Node, opt Options, res *Result) error {
	// 元素按规范化后的字节序排序后比对（§4.10）；顺序差异不产生条目。
	lk, ldup, err := sortedElementKeys(p, l, opt)
	if err != nil {
		return err
	}
	rk, rdup, err := sortedElementKeys(p, r, opt)
	if err != nil {
		return err
	}
	if len(ldup) > 0 {
		res.Entries = append(res.Entries, Entry{
			Path: p, Kind: IndeterminateKind, Reason: value.ReasonDuplicateInSet,
		})
		res.HasIndeterminate = true
	}
	if len(rdup) > 0 {
		res.Entries = append(res.Entries, Entry{
			Path: p, Kind: IndeterminateKind, Reason: value.ReasonDuplicateInSet,
		})
		res.HasIndeterminate = true
	}
	lset, rset := toSet(lk), toSet(rk)
	for _, k := range unionSorted(keysOf(lset), keysOf(rset)) {
		switch {
		case !lset[k] && rset[k]:
			res.Entries = append(res.Entries, Entry{Path: p + "[" + k + "]", Kind: Added})
		case lset[k] && !rset[k]:
			res.Entries = append(res.Entries, Entry{Path: p + "[" + k + "]", Kind: Removed})
		}
	}
	return nil
}

func compareKeyed(p string, l, r *doc.Node, opt Options, res *Result) error {
	lmap, err := keyedIndex(p, l, opt.Seq.Field)
	if err != nil {
		return err
	}
	rmap, err := keyedIndex(p, r, opt.Seq.Field)
	if err != nil {
		return err
	}
	for _, k := range unionSorted(keysOfNode(lmap), keysOfNode(rmap)) {
		li, lok := lmap[k]
		ri, rok := rmap[k]
		child := value.JoinPointer(p, k)
		switch {
		case !lok:
			if err := recordAdded(child, ri, opt, res); err != nil {
				return err
			}
		case !rok:
			if err := recordRemoved(child, li, opt, res); err != nil {
				return err
			}
		default:
			if err := compareNode(child, li, ri, opt, res); err != nil {
				return err
			}
		}
	}
	return nil
}

// keyedIndex 建键索引；键必须唯一，重复即 **报错**（K-05，禁止取第一个）。
func keyedIndex(p string, n *doc.Node, field string) (map[string]*doc.Node, error) {
	out := make(map[string]*doc.Node, len(n.Children))
	for i, item := range n.Children {
		fv := item.Get(field)
		if fv == nil {
			return nil, fmt.Errorf("keyed:%s: element %s/%d has no field %q", field, p, i, field)
		}
		key := fv.Raw
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("keyed:%s: duplicate key %q at %s/%d (keys must be unique)", field, key, p, i)
		}
		out[key] = item
	}
	return out, nil
}

func compareScalar(p string, l, r *doc.Node, opt Options, res *Result) error {
	lfam := familyFor(p, l, opt)
	rfam := familyFor(p, r, opt)

	lc, lerr := value.Normalize(l, p, lfam, opt.Strict)
	rc, rerr := value.Normalize(r, p, rfam, opt.Strict)

	// indeterminate 是**一等结果**，不是错误：把它记成条目并继续，
	// 否则一个歧义值会让整次 diff 中断、连汇总都打不出来。
	if ind := asIndeterminate(lerr); ind != nil {
		res.Entries = append(res.Entries, Entry{Path: p, Kind: IndeterminateKind, Reason: ind.Reason})
		res.HasIndeterminate = true
		return nil
	}
	if ind := asIndeterminate(rerr); ind != nil {
		res.Entries = append(res.Entries, Entry{Path: p, Kind: IndeterminateKind, Reason: ind.Reason})
		res.HasIndeterminate = true
		return nil
	}
	if lerr != nil {
		return lerr
	}
	if rerr != nil {
		return rerr
	}

	eq, ind := value.Compare(lc, rc)
	switch {
	case ind != nil:
		res.Entries = append(res.Entries, Entry{
			Path: p, Kind: IndeterminateKind, Reason: ind.Reason,
			Left: &lc, Right: &rc,
		})
		res.HasIndeterminate = true
		return nil
	case eq && l.Raw != r.Raw:
		// 归一后相等但原文不同 → 纯书写差异：**计数但不逐条列**（§4.13）。
		res.Entries = append(res.Entries, Entry{
			Path: p, Kind: CosmeticKind, Left: &lc, Right: &rc,
		})
		return nil
	case eq:
		return nil
	case l.Raw != r.Raw:
		res.Entries = append(res.Entries, Entry{
			Path: p, Kind: Changed, Left: &lc, Right: &rc,
		})
	}
	return nil
}

func familyFor(p string, n *doc.Node, opt Options) value.Family {
	if !opt.Coerce.Empty() {
		if fam, ok := opt.Coerce.Match(p); ok {
			return fam
		}
	}
	return value.InferFamily(n, p)
}

// asIndeterminate 把 error 归一成 *value.Indeterminate（不是则返回 nil）。
func asIndeterminate(err error) *value.Indeterminate {
	if err == nil {
		return nil
	}
	if ind, ok := err.(*value.Indeterminate); ok {
		return ind
	}
	return nil
}

func recordAdded(p string, n *doc.Node, opt Options, res *Result) error {
	c, err := value.Normalize(n, p, familyFor(p, n, opt), opt.Strict)
	if err != nil {
		if _, ok := err.(*value.Indeterminate); ok {
			res.Entries = append(res.Entries, Entry{Path: p, Kind: IndeterminateKind, Reason: err.(*value.Indeterminate).Reason})
			res.HasIndeterminate = true
			return nil
		}
		return err
	}
	res.Entries = append(res.Entries, Entry{Path: p, Kind: Added, Right: &c})
	return nil
}

func recordRemoved(p string, n *doc.Node, opt Options, res *Result) error {
	c, err := value.Normalize(n, p, familyFor(p, n, opt), opt.Strict)
	if err != nil {
		if _, ok := err.(*value.Indeterminate); ok {
			res.Entries = append(res.Entries, Entry{Path: p, Kind: IndeterminateKind, Reason: err.(*value.Indeterminate).Reason})
			res.HasIndeterminate = true
			return nil
		}
		return err
	}
	res.Entries = append(res.Entries, Entry{Path: p, Kind: Removed, Left: &c})
	return nil
}

func sortedElementKeys(p string, n *doc.Node, opt Options) ([]string, []string, error) {
	keys := make([]string, 0, len(n.Children))
	dups := make([]string, 0)
	seen := make(map[string]int, len(n.Children))
	for _, item := range n.Children {
		// 元素键 = 元素自身的结构规范形式（mapping 元素也有确定的键）。
		key := canonicalKey(item)
		if seen[key] == 1 {
			dups = append(dups, key)
		}
		seen[key]++
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys, dups, nil
}

func toSet(keys []string) map[string]bool {
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysOfNode(m map[string]*doc.Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func unionSorted(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sortStrings(out)
	return out
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
