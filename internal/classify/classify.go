// Package classify 把差异分为 5 个行为类 + 1 个非行为类（plan/03 §五，data/README.md §4.13）。
//
// 硬纪律：分类顺序固定（先影响类、再路径字节序）；汇总计数从明细计算得出；
// 分类不确定 → ConfigChange 且 class_guess: true，**禁止**归入 Cosmetic。
package classify

import (
	"sort"
	"strconv"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/diff"
	"github.com/yuluo554/10-20-cfgdelta/internal/value"
)

// Class 是影响类。
type Class string

const (
	ResourceShrink Class = "资源·收缩"
	DeployShrink   Class = "部署·收缩"
	ImageChange    Class = "镜像·变更"
	Exposure       Class = "接口·暴露"
	ConfigChange   Class = "配置·新增·移除"
	Cosmetic       Class = "纯书写"
)

// Behavioral 是全部行为类（Cosmetic 不在内）。
var Behavioral = []Class{ResourceShrink, DeployShrink, ImageChange, Exposure, ConfigChange}

// All 是全部类，顺序即输出顺序（固定）。
var All = []Class{ResourceShrink, DeployShrink, ImageChange, Exposure, ConfigChange, Cosmetic}

// classOrder 给出固定排序权重。
var classOrder = func() map[Class]int {
	m := make(map[Class]int, len(All))
	for i, c := range All {
		m[c] = i
	}
	return m
}()

// IsBehavioral 判定类是否行为类。
func IsBehavioral(c Class) bool {
	for _, b := range Behavioral {
		if b == c {
			return true
		}
	}
	return false
}

// ParseClass 解析 --fail-on 的一项；未知类返回 false。
func ParseClass(s string) (Class, bool) {
	for _, c := range All {
		if string(c) == s {
			return c, true
		}
	}
	return "", false
}

// Summary 是分类汇总（计数全部从明细算得）。
type Summary struct {
	Total          int
	ByClass        map[Class]int
	Behavioral     int
	Cosmetic       int
	Indeterminate  int
}

// Result 是分类输出。
type Result struct {
	Entries []diff.Entry
	Summary Summary
}

// Apply 给每条差异打上 Class；返回带固定排序的结果。
func Apply(entries []diff.Entry) Result {
	out := make([]diff.Entry, len(entries))
	copy(out, entries)
	for i := range out {
		out[i].Class, out[i].SubClass, out[i].Guess = classifyEntry(out[i])
	}
	sort.SliceStable(out, func(i, j int) bool {
		oi, oj := classOrder[Class(out[i].Class)], classOrder[Class(out[j].Class)]
		if oi != oj {
			return oi < oj
		}
		return out[i].Path < out[j].Path
	})
	return Result{Entries: out, Summary: summarize(out)}
}

func summarize(entries []diff.Entry) Summary {
	s := Summary{Total: len(entries), ByClass: map[Class]int{}}
	for _, e := range entries {
		if e.Kind == diff.IndeterminateKind {
			s.Indeterminate++
			continue
		}
		c := Class(e.Class)
		s.ByClass[c]++
		if IsBehavioral(c) {
			s.Behavioral++
		} else {
			s.Cosmetic++
		}
	}
	return s
}

// classifyEntry 返回 (类, 子类, class_guess)。
func classifyEntry(e diff.Entry) (string, string, bool) {
	if e.Kind == diff.IndeterminateKind {
		return string(Cosmetic), "", false
	}
	if e.Kind == diff.CosmeticKind {
		// 归一后语义相同、只是书写不同 → 纯书写（只计数）。
		return string(Cosmetic), "", false
	}
	p := e.Path
	base := lastSegment(p)

	// 镜像·变更：image 引用变化，按 semver 细分。
	if base == "image" && e.Kind == diff.Changed {
		return string(ImageChange), semverBump(e.Left, e.Right), false
	}

	// 资源·收缩：limits/requests 同族归一后变小；replicas 减少。
	if (strings.Contains(p, "/resources/limits") || strings.Contains(p, "/resources/requests")) && e.Kind == diff.Changed {
		if shrank(e.Left, e.Right) {
			return string(ResourceShrink), "", false
		}
		return string(ConfigChange), "", false
	}
	if base == "replicas" && e.Kind == diff.Changed {
		if shrank(e.Left, e.Right) {
			return string(ResourceShrink), "", false
		}
		return string(ConfigChange), "", false
	}

	// 部署·收缩：maxUnavailable 变大 / minReadySeconds 变大。
	if base == "maxUnavailable" && e.Kind == diff.Changed && grew(e.Left, e.Right) {
		return string(DeployShrink), "", false
	}
	if base == "minReadySeconds" && e.Kind == diff.Changed && grew(e.Left, e.Right) {
		return string(DeployShrink), "", false
	}

	// 接口·暴露：Service 端口/类型/selector、Ingress 路由。
	if isExposurePath(p) && (e.Kind == diff.Changed || e.Kind == diff.Added || e.Kind == diff.Removed) {
		return string(Exposure), "", false
	}

	// 配置·新增·移除：env / ConfigMap / Secret key 增删。
	if isConfigPath(p) {
		return string(ConfigChange), "", false
	}

	// 归一后语义相同（同族等值但书写不同）→ 纯书写。
	if e.Kind == diff.Changed && e.Left != nil && e.Right != nil {
		if eq, ind := value.Compare(*e.Left, *e.Right); ind == nil && eq {
			return string(Cosmetic), "", false
		}
	}

	// 其余变化一律按配置变更，并标记"分类不确定"（§4.13）。
	return string(ConfigChange), "", true
}

// isExposurePath：暴露面只认"端口号 / 服务类型 / selector / 路由"这类**可达性**变更。
// `protocol` 属于容器端口默认值，不是暴露面（否则加一行 protocol: TCP 就被判成接口变更）。
func isExposurePath(p string) bool {
	if strings.Contains(p, "/protocol") {
		return false
	}
	// Service 的 type 在 /spec/type；Deployment 的是 /spec/strategy/type（不受影响）。
	if p == "/spec/type" {
		return true
	}
	for _, marker := range []string{"/ports", "/serviceType", "/selector", "/ingress", "/rules", "/host", "/http/paths"} {
		if strings.Contains(p, marker) {
			return true
		}
	}
	return false
}

func isConfigPath(p string) bool {
	for _, marker := range []string{"/env", "/data", "/stringData", "/envFrom", "/configMap", "/secret"} {
		if strings.Contains(p, marker) {
			return true
		}
	}
	return false
}

func lastSegment(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return p
	}
	return p[i+1:]
}

// shrank 判定右侧归一后是否**变小**（同族有理数比较）。
func shrank(l, r *value.Canonical) bool {
	if l == nil || r == nil {
		return false
	}
	if l.Family != r.Family || l.Rat == nil || r.Rat == nil {
		return false
	}
	return r.Rat.Cmp(l.Rat) < 0
}

func grew(l, r *value.Canonical) bool {
	if l == nil || r == nil {
		return false
	}
	if l.Family == value.FamilyDuration && r.Family == value.FamilyDuration {
		return r.Ns > l.Ns
	}
	if l.Family != r.Family || l.Rat == nil || r.Rat == nil {
		return false
	}
	return r.Rat.Cmp(l.Rat) > 0
}

// semverBump 只在两侧都是纯数字点分标签时给 major/minor/patch；
// 其它（latest / sha- 前缀 / 含摘要）一律 non-semver。
func semverBump(l, r *value.Canonical) string {
	lt, rt := tagOf(l), tagOf(r)
	if lt == "" || rt == "" {
		return "non-semver"
	}
	lp, lok := parseSemver(lt)
	rp, rok := parseSemver(rt)
	if !lok || !rok {
		return "non-semver"
	}
	switch {
	case lp[0] != rp[0]:
		return "major"
	case lp[1] != rp[1]:
		return "minor"
	case lp[2] != rp[2]:
		return "patch"
	}
	return "patch"
}

func tagOf(c *value.Canonical) string {
	if c == nil {
		return ""
	}
	s := c.Str
	if i := strings.LastIndex(s, ":"); i >= 0 && strings.Contains(s, "/") {
		s = s[i+1:]
	}
	if s == "" {
		return ""
	}
	return s
}

func parseSemver(s string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// Gate 按 --fail-on 选择性地判定是否阻断（§4.12）。
//
// 未列出的行为差异**照常输出但不阻断**；indeterminate 恒返回 true（除 allowIndeterminate）。
func Gate(sum Summary, failOn []Class, allowIndeterminate bool) bool {
	if sum.Indeterminate > 0 && !allowIndeterminate {
		return true
	}
	if len(failOn) == 0 {
		return sum.Behavioral > 0
	}
	for _, c := range failOn {
		if sum.ByClass[c] > 0 {
			return true
		}
	}
	return false
}
