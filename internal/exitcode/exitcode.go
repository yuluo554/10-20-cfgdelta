// Package exitcode 是退出码契约的**唯一**定义处（plan/05 K-07 / data/README.md §4.12）。
//
//	0 = 等价（无行为差异）
//	1 = 有行为差异
//	2 = 存在 indeterminate
//	3 = 解析错误或用法错误
//
// 优先级 3 > 2 > 1 > 0：多个条件同时成立时取最高优先级。
// 任何子命令**不得**自己调用 os.Exit，只能返回 Code（或 error，由 cli 翻译成 Code）。
package exitcode

// Code 是进程退出码。
type Code int

const (
	Equal         Code = 0
	Behavioral    Code = 1
	Indeterminate Code = 2
	Usage         Code = 3
)

// Combine 返回两者中优先级更高（数值更大）的那个。
func Combine(a, b Code) Code {
	if a > b {
		return a
	}
	return b
}

// String 用于诊断输出，不参与判定。
func (c Code) String() string {
	switch c {
	case Equal:
		return "0/equivalent"
	case Behavioral:
		return "1/behavioral-diff"
	case Indeterminate:
		return "2/indeterminate"
	case Usage:
		return "3/usage-or-parse-error"
	}
	return "unknown"
}
