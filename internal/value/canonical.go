package value

import (
	"fmt"
	"math/big"
)

// Family 是值族声明（data/README.md §4.8 的 family 取值）。
type Family string

const (
	FamilyQuantity Family = "quantity"
	FamilyDuration Family = "duration"
	FamilyDecimal  Family = "decimal"
	FamilyBool     Family = "bool"
	FamilyString   Family = "string"
	FamilyNull     Family = "null"
	FamilyRatio    Family = "ratio"
	FamilyUnknown  Family = "unknown"
)

// Valid 判定是否为 --coerce-at 允许出现的族名。
func (f Family) Valid() bool {
	switch f {
	case FamilyQuantity, FamilyDuration, FamilyDecimal, FamilyBool, FamilyString, FamilyRatio:
		return true
	}
	return false
}

// 理由码（§4.7）：indeterminate 必须带理由码，禁止无理由的 unknown。
const (
	ReasonAmbiguousUnitFamily   = "ambiguous_unit_family"
	ReasonUnknownFamily         = "unknown_family"
	ReasonUnevaluatedExpression = "unevaluated_expression"
	ReasonDuplicateInSet        = "duplicate_in_set"
)

// Indeterminate 是"判不了"这个一等结果，不是错误。
type Indeterminate struct {
	Reason string
	Detail string
}

func (e *Indeterminate) Error() string {
	if e.Detail == "" {
		return "indeterminate: " + e.Reason
	}
	return fmt.Sprintf("indeterminate: %s (%s)", e.Reason, e.Detail)
}

// NullState 是 null 三态（§4.5）：缺失 / 显式 null / 空字符串，严格区分不折叠。
const (
	NullAbsent = "absent"
	NullExplicit = "null"
	NullEmpty = "empty"
)

// Canonical 是归一化后的规范值 —— 比较路径上**唯一**允许出现的值类型。
//
// 禁止浮点（K-01）：数值一律 *big.Rat，时长一律 int64 纳秒。
type Canonical struct {
	Family Family
	Rat    *big.Rat // quantity / decimal / ratio
	Ns     int64    // duration
	Bool   bool     // bool
	Str    string   // string 原文；null 态时为 ""
	Null   string   // "" | NullAbsent | NullExplicit | NullEmpty
}

// String 是冻结的规范值表单（K-04）：排序与序列化只认它。
func (c Canonical) String() string {
	switch c.Family {
	case FamilyQuantity, FamilyDecimal, FamilyRatio:
		if c.Rat == nil {
			return string(c.Family) + ":nil"
		}
		return string(c.Family) + ":" + ratString(c.Rat)
	case FamilyDuration:
		return fmt.Sprintf("%s:%d", c.Family, c.Ns)
	case FamilyBool:
		return fmt.Sprintf("%s:%t", c.Family, c.Bool)
	case FamilyNull:
		return string(c.Family) + ":" + c.Null
	}
	return string(c.Family) + ":" + c.Str
}

// ratString 精确格式化：整数就写整数，否则写 "num/den"（绝不转浮点）。
func ratString(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String() + "/1"
	}
	return r.RatString()
}

// Compare 返回两值是否相等；判不了时返回 *Indeterminate。
//
// 语义（§4.1 / §4.3）：
//   - quantity 与 decimal **同属精确有理数代数**（§4.1 的 "" 后缀就是 10^0），因此可跨族比较；
//   - 其余跨族 → 不等（不做隐式强转）；
//   - 任一侧 unknown → 由调用方按 --strict 决定是否升级为 indeterminate。
func Compare(a, b Canonical) (bool, *Indeterminate) {
	if a.Family == FamilyUnknown || b.Family == FamilyUnknown {
		return false, &Indeterminate{Reason: ReasonUnknownFamily}
	}
	if isRationalFamily(a.Family) && isRationalFamily(b.Family) {
		if a.Rat == nil || b.Rat == nil {
			return false, &Indeterminate{Reason: ReasonUnknownFamily, Detail: "nil rational"}
		}
		return a.Rat.Cmp(b.Rat) == 0, nil
	}
	if a.Family != b.Family {
		return false, nil
	}
	switch a.Family {
	case FamilyDuration:
		return a.Ns == b.Ns, nil
	case FamilyBool:
		return a.Bool == b.Bool, nil
	case FamilyNull:
		return a.Null == b.Null, nil
	}
	return a.Str == b.Str, nil
}

// isRationalFamily：quantity / decimal / ratio 三者共用 *big.Rat。
func isRationalFamily(f Family) bool {
	return f == FamilyQuantity || f == FamilyDecimal || f == FamilyRatio
}
