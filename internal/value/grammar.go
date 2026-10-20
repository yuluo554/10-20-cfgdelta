package value

import (
	"math/big"
	"strings"
)

// yaml11Bools 是 YAML 1.1 布尔全族（§4.4）。唯一实现处。
// 只对 **plain 标量** 生效；被引号包裹的 "true" 若也落在此表中，调用方必须先查 IsQuoted。
var yaml11Bools = map[string]bool{
	"y": true, "Y": true, "yes": true, "Yes": true, "YES": true,
	"true": true, "True": true, "TRUE": true,
	"on": true, "On": true, "ON": true,
	"n": false, "N": false, "no": false, "No": false, "NO": false,
	"false": false, "False": false, "FALSE": false,
	"off": false, "Off": false, "OFF": false,
}

// LookupYAML11Bool 查 YAML 1.1 布尔族；命中返回 (值, true)。
func LookupYAML11Bool(s string) (bool, bool) {
	v, ok := yaml11Bools[s]
	return v, ok
}

// ParseDecimal 把十进制字面量解析为精确有理数（§4.3）。
// 支持符号（`+1` ≡ `1`、`-0` ≡ `0`）、整数、小数、指数式（1e3）；
// 1.50 与 1.5 自然约分为同一 Rat。
func ParseDecimal(s string) (*big.Rat, error) {
	raw := strings.TrimSpace(s)
	neg := false
	switch {
	case strings.HasPrefix(raw, "+"):
		raw = raw[1:]
	case strings.HasPrefix(raw, "-"):
		neg = true
		raw = raw[1:]
	}
	if raw == "" {
		return nil, errParse("decimal has sign but no number: " + s)
	}
	r, err := parseDecimalRat(raw)
	if err != nil {
		return nil, err
	}
	if neg {
		r.Neg(r)
	}
	return r, nil
}

// ParseRatio 把 "50%" 解析为 1/2（§4.6，仅在显式声明 ratio 族时调用）。
func ParseRatio(s string) (*big.Rat, error) {
	raw := strings.TrimSpace(s)
	if !strings.HasSuffix(raw, "%") {
		return nil, errParse("ratio must end with %: " + s)
	}
	body := strings.TrimSpace(strings.TrimSuffix(raw, "%"))
	r, err := parseDecimalRat(body)
	if err != nil {
		return nil, err
	}
	return new(big.Rat).Quo(r, big.NewRat(100, 1)), nil
}

// LooksLikeDuration 判定书写形态是否像 Go Duration（至少一个时间单位）。
// 只用于**默认族推断**；不做值判定。
func LooksLikeDuration(s string) bool {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return false
	}
	// 纯数字（含小数/指数）不是 duration。
	if isNumericString(strings.TrimPrefix(strings.TrimPrefix(raw, "+"), "-")) {
		return false
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] >= '0' && raw[i] <= '9' {
			continue
		}
		if raw[i] == '.' || raw[i] == '+' || raw[i] == '-' {
			continue
		}
		unit := raw[i:]
		if _, ok := durationUnits[trimUnitTail(unit)]; ok {
			return true
		}
		return false
	}
	return false
}

func trimUnitTail(s string) string {
	i := 0
	for i < len(s) && !(s[i] >= '0' && s[i] <= '9') && s[i] != '.' {
		i++
	}
	return s[:i]
}

// LooksLikeQuantity 判定书写形态是否像 K8s Quantity（数字 + 可选后缀/指数）。
func LooksLikeQuantity(s string) bool {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return false
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "+"), "-")
	num, suffix, exp, err := splitQuantity(raw)
	if err != nil || !isNumericString(num) {
		return false
	}
	if exp != "" {
		return true
	}
	if suffix == "" {
		return true
	}
	if _, ok := binarySuffix[suffix]; ok {
		return true
	}
	_, ok := decimalSuffix[suffix]
	return ok
}
