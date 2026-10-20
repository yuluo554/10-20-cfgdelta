package value

import (
	"math/big"
	"path"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
)

// CoerceRule 是 --coerce-at 或 profile 声明的一条"指定路径按指定族解释"（§4.8）。
type CoerceRule struct {
	Pattern string
	Family  Family
}

// Coercion 是一组已解析的强制族规则，可对指针路径查询。
type Coercion struct {
	rules []CoerceRule
}

// NewCoercion 构造；pattern 用 `*` 匹配单段（glob 风格，整段通配）。
func NewCoercion(rules []CoerceRule) *Coercion { return &Coercion{rules: rules} }

// Empty 表示没有任何强制规则。
func (c *Coercion) Empty() bool { return c == nil || len(c.rules) == 0 }

// Match 返回该路径上声明的族；未声明返回 (FamilyUnknown, false)。
// 多条规则命中同一路径由调用方判定为用法错误（§4.8）。
func (c *Coercion) Match(p string) (Family, bool) {
	if c == nil {
		return FamilyUnknown, false
	}
	for _, r := range c.rules {
		if globSegmentMatch(r.Pattern, p) {
			return r.Family, true
		}
	}
	return FamilyUnknown, false
}

// MatchesAny 判定该 glob 是否命中过任何路径（用于 --coerce-at 未命中的用法错误）。
func (c *Coercion) MatchesAny(paths []string) map[string]bool {
	hit := make(map[string]bool, len(c.rules))
	for _, r := range c.rules {
		for _, p := range paths {
			if globSegmentMatch(r.Pattern, p) {
				hit[r.Pattern] = true
				break
			}
		}
	}
	return hit
}

// globSegmentMatch：`*` 只匹配一段（不含 `/`），其余按字面量比较。
//
// 前导 `/` 视为可省略：`/limits/*` 与 `limits/*` 等价。理由不是风格 ——
// 在 Git Bash / MSYS2 下，以 `/` 开头的参数会被**静默改写成 Windows 路径**
// （`--at /limits/memory` → `--at D:/program/Git/limits/memory`），
// 用户写不带前导斜杠的形式才能在任何 shell 里稳定工作。
func globSegmentMatch(pattern, p string) bool {
	ps := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	ss := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(ps) != len(ss) {
		return false
	}
	for i := range ps {
		if ps[i] == "*" {
			continue
		}
		if ps[i] != ss[i] {
			return false
		}
	}
	return true
}

// InferFamily 推断默认值族（K-06）：只推断 bool / quantity / duration / decimal，其余按 string。
//
// 路径上下文（唯一实现处）：只有 resources.limits / resources.requests 下的裸数字才默认
// quantity —— 这是防止 `image.tag: 1.25` 被当成数字的关键（测试B 第 2 项）。
func InferFamily(n *doc.Node, jsonPointer string) Family {
	fam, _ := InferFamilyDetailed(n, jsonPointer)
	return fam
}

// InferFamilyDetailed 额外返回 `fallback`：true 表示该值**没有命中任何已声明的族**，
// 只是按"其余 → string"兜底（plan/03 §二 第 5 步）。
//
// `--strict` 只用 fallback 那一类升级为 indeterminate(unknown_family)（第 6 步）——
// 显式声明的字符串（引号包裹、百分比）**不**升级，否则工具会变得吵（风险③）。
func InferFamilyDetailed(n *doc.Node, jsonPointer string) (fam Family, fallback bool) {
	if n == nil {
		return FamilyNull, false
	}
	if n.Tag == "!!alias" {
		return FamilyUnknown, false
	}
	if n.Tag == "!!null" {
		return FamilyNull, false
	}
	if n.Tag == "!!bool" {
		return FamilyBool, false
	}
	if n.Kind != doc.Scalar {
		return FamilyUnknown, false
	}
	raw := strings.TrimSpace(n.Raw)
	if raw == "" {
		return FamilyString, false
	}

	// YAML 1.1 布尔族只对 plain 标量生效（§4.4）；引号包裹的 "true" 是字符串。
	if !n.IsQuoted() {
		if _, ok := LookupYAML11Bool(raw); ok {
			return FamilyBool, false
		}
	}
	if n.IsQuoted() {
		// 引用类型书写不改变**数值**语义（测试B：`replicaCount: 3` vs `"3"` 相同；
		// `image.tag: "1.25"` vs `1.25` 相同）。但布尔与百分比例外：
		// `"true"` 仍是字符串（§4.4），`"50%"` 仍是字符串（§4.6）。
		if isPlainNumeric(raw) {
			return FamilyDecimal, false
		}
		return FamilyString, false
	}
	if strings.HasSuffix(raw, "%") {
		// 百分比默认当字符串（§4.6），只有显式声明才换算。
		return FamilyString, false
	}
	// K8s 资源上下文优先判 quantity：`500m` 在 limits/requests 里是 0.5，
	// 不是 Go duration 的"500 分钟"。这条顺序是文法的消歧点（§4.1 vs §4.2）。
	if inResourceContext(jsonPointer) && LooksLikeQuantity(raw) {
		return FamilyQuantity, false
	}
	if LooksLikeDuration(raw) {
		return FamilyDuration, false
	}
	if isAmbiguousByteUnit(raw) {
		// 歧义字节单位（MB/MiB）：走 quantity 路径，由 ParseQuantity 产出
		// indeterminate(ambiguous_unit_family)，**绝不静默判相等**（§4.7）。
		return FamilyQuantity, false
	}
	if LooksLikeQuantity(raw) {
		// 裸数字与指数式（1e3）在语义上属于"十进制数值"（§4.3）；
		// 只有 resources 上下文里的裸数字才默认 quantity。
		if !hasQuantitySuffix(raw) {
			if inResourceContext(jsonPointer) {
				return FamilyQuantity, false
			}
			return FamilyDecimal, false
		}
		return FamilyQuantity, false
	}
	// 其余 → string 兜底；`--strict` 会把"两侧书写不同"的这一类升级为 indeterminate。
	return FamilyString, true
}

// hasQuantitySuffix 判定是否带非空后缀（含指数式之外的 Ki/Mi/500m 等）。
func hasQuantitySuffix(s string) bool {
	raw := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "+"), "-")
	_, suffix, exp, err := splitQuantity(raw)
	if err != nil || exp != "" {
		return false
	}
	return suffix != ""
}

// isPlainNumeric 判定是否为纯十进制/指数字面量（不含单位）。
func isPlainNumeric(s string) bool {
	raw := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "+"), "-")
	if raw == "" {
		return false
	}
	num, suffix, exp, err := splitQuantity(raw)
	if err != nil || suffix != "" {
		return false
	}
	if !isNumericString(num) {
		return false
	}
	_ = exp
	return true
}

func inResourceContext(p string) bool {
	return strings.Contains(p, "/resources/limits/") ||
		strings.Contains(p, "/resources/requests/") ||
		strings.Contains(p, "/resources/limits") ||
		strings.Contains(p, "/resources/requests")
}

func isNumericOnly(s string) bool {
	return isNumericString(strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "+"), "-"))
}

func isAmbiguousByteUnit(s string) bool {
	raw := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "+"), "-")
	for suf := range byteSuffixAmbiguous {
		if strings.HasSuffix(raw, suf) && len(raw) > len(suf) {
			if isNumericString(raw[:len(raw)-len(suf)]) {
				return true
			}
		}
	}
	return false
}

// Normalize 把一个标量节点按给定族（或推断族）归一为规范值（§四）。
//
// 返回 error 的两类：
//   - *Indeterminate → 一等结果，映射 exit 2；
//   - 其它 → 文法错误，映射 exit 3。
func Normalize(n *doc.Node, jsonPointer string, family Family, strict bool) (Canonical, error) {
	if family == FamilyUnknown {
		family = InferFamily(n, jsonPointer)
	}
	raw := ""
	if n != nil {
		raw = strings.TrimSpace(n.Raw)
	}

	switch family {
	case FamilyNull:
		switch {
		case n == nil:
			return Canonical{Family: FamilyNull, Null: NullAbsent}, nil
		case n.Tag == "!!null":
			return Canonical{Family: FamilyNull, Null: NullExplicit}, nil
		case n.Tag == "!!str" && raw == "":
			return Canonical{Family: FamilyNull, Null: NullEmpty}, nil
		}
		return Canonical{Family: FamilyNull, Null: NullExplicit}, nil

	case FamilyBool:
		if n != nil && n.Tag == "!!bool" {
			return Canonical{Family: FamilyBool, Bool: raw == "true"}, nil
		}
		if n != nil && n.IsQuoted() {
			// 引号包裹的 "true" 是字符串，不自动强转（§4.4）。
			return Canonical{Family: FamilyString, Str: raw}, nil
		}
		if b, ok := LookupYAML11Bool(raw); ok {
			return Canonical{Family: FamilyBool, Bool: b}, nil
		}
		return Canonical{}, errParse("not a YAML 1.1 boolean: " + raw)

	case FamilyQuantity:
		r, err := ParseQuantity(raw)
		if err != nil {
			return Canonical{}, err
		}
		return Canonical{Family: FamilyQuantity, Rat: r}, nil

	case FamilyDuration:
		ns, err := ParseDuration(raw)
		if err != nil {
			return Canonical{}, err
		}
		return Canonical{Family: FamilyDuration, Ns: ns}, nil

	case FamilyDecimal:
		r, err := ParseDecimal(raw)
		if err != nil {
			return Canonical{}, err
		}
		return Canonical{Family: FamilyDecimal, Rat: r}, nil

	case FamilyRatio:
		r, err := ParseRatio(raw)
		if err != nil {
			return Canonical{}, err
		}
		return Canonical{Family: FamilyRatio, Rat: r}, nil

	case FamilyString:
		if strict && n != nil && n.Tag == "!!alias" {
			return Canonical{}, &Indeterminate{Reason: ReasonUnevaluatedExpression, Detail: raw}
		}
		return Canonical{Family: FamilyString, Str: raw}, nil
	}

	// unknown 族：默认按原样比较；--strict 才升级为 indeterminate（§4.7）。
	if strict {
		return Canonical{}, &Indeterminate{Reason: ReasonUnknownFamily, Detail: raw}
	}
	return Canonical{Family: FamilyString, Str: raw}, nil
}

// SetBigRat 便于测试构造期望值。
func SetBigRat(num, den int64) *big.Rat { return big.NewRat(num, den) }

// JoinPointer 拼接 JSON Pointer 段（RFC 6901 转义）。
func JoinPointer(base, seg string) string {
	seg = strings.ReplaceAll(seg, "~", "~0")
	seg = strings.ReplaceAll(seg, "/", "~1")
	if base == "" {
		return "/" + seg
	}
	return base + "/" + seg
}

// PointerBase 返回路径最后一段（用于 kind 判定等）。
func PointerBase(p string) string { return path.Base(p) }
