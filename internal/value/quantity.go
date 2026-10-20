package value

import (
	"math/big"
	"strings"
)

// 后缀表是 §4.1 文法在代码里的**唯一**实现处（K-04 / plan/04 §五）。
// binary：1024^1 .. 1024^6；decimal：10^-9 .. 10^18。
var (
	binarySuffix = map[string]int{
		"Ki": 1, "Mi": 2, "Gi": 3, "Ti": 4, "Pi": 5, "Ei": 6,
	}
	decimalSuffix = map[string]int{
		"n": -9, "u": -6, "m": -3, "": 0,
		"k": 3, "M": 6, "G": 9, "T": 12, "P": 15, "E": 18,
	}
)

// byteSuffixAmbiguous 是"十进/二进书写歧义"的非 K8s 字节单位（§4.7）。
// 命中即 *Indeterminate{ambiguous_unit_family}，**绝不静默判相等**。
var byteSuffixAmbiguous = map[string]bool{
	"B": true, "KB": true, "MB": true, "GB": true, "TB": true, "PB": true, "EB": true,
	"KiB": true, "MiB": true, "GiB": true, "TiB": true, "PiB": true, "EiB": true,
}

var (
	big1024 = big.NewInt(1024)
	big10   = big.NewInt(10)
)

// ParseQuantity 按 apimachinery 文法把字符串解析为精确有理数（§4.1）。
//
//	quantity := sign? number suffix?
//	number   := digits ("." digits)? | "." digits | digits ("e"|"E") sign? digits
//
// 指数式与后缀不可同时出现（1e3Mi 非法）。
func ParseQuantity(s string) (*big.Rat, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return nil, errParse("empty quantity")
	}
	neg := false
	switch raw[0] {
	case '+':
		raw = raw[1:]
	case '-':
		neg = true
		raw = raw[1:]
	}
	if raw == "" {
		return nil, errParse("quantity has sign but no number")
	}

	// 歧义字节单位优先判定：MB/MiB 这类一律 indeterminate。
	for suf := range byteSuffixAmbiguous {
		if strings.HasSuffix(raw, suf) {
			mantissa := raw[:len(raw)-len(suf)]
			if isNumericString(mantissa) {
				return nil, &Indeterminate{Reason: ReasonAmbiguousUnitFamily, Detail: raw}
			}
		}
	}

	num, suffix, exp, err := splitQuantity(raw)
	if err != nil {
		return nil, err
	}
	if exp != "" && suffix != "" {
		return nil, errParse("exponent and suffix are mutually exclusive: " + s)
	}

	mant, err := parseDecimalRat(num)
	if err != nil {
		return nil, errParse("bad quantity number: " + s)
	}

	scale := big.NewRat(1, 1)
	if exp != "" {
		e, ok := new(big.Int).SetString(strings.TrimPrefix(exp, "+"), 10)
		if !ok {
			return nil, errParse("bad exponent: " + s)
		}
		if e.Sign() < 0 {
			scale = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big10, new(big.Int).Neg(e), nil))
		} else {
			scale = new(big.Rat).SetInt(new(big.Int).Exp(big10, e, nil))
		}
	} else if suffix != "" {
		if p, ok := binarySuffix[suffix]; ok {
			numr := new(big.Int).Exp(big1024, big.NewInt(int64(p)), nil)
			scale = new(big.Rat).SetInt(numr)
		} else if p, ok := decimalSuffix[suffix]; ok {
			if p >= 0 {
				scale = new(big.Rat).SetInt(new(big.Int).Exp(big10, big.NewInt(int64(p)), nil))
			} else {
				scale = new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big10, big.NewInt(int64(-p)), nil))
			}
		} else {
			return nil, errParse("unknown quantity suffix: " + suffix)
		}
	}

	out := new(big.Rat).Mul(mant, scale)
	if neg {
		out.Neg(out)
	}
	return out, nil
}

// splitQuantity 拆出数字部分 / 后缀 / 指数部分。
func splitQuantity(raw string) (num, suffix, exp string, err error) {
	i := 0
	for i < len(raw) {
		c := raw[i]
		if (c >= '0' && c <= '9') || c == '.' {
			i++
			continue
		}
		break
	}
	num = raw[:i]
	if num == "" {
		return "", "", "", errParse("quantity has no digits")
	}
	rest := raw[i:]
	if rest != "" && (rest[0] == 'e' || rest[0] == 'E') {
		body := rest[1:]
		if body == "" {
			return "", "", "", errParse("dangling exponent")
		}
		digits := body
		if body[0] == '+' || body[0] == '-' {
			digits = body[1:]
		}
		if !allDigits(digits) || digits == "" {
			return "", "", "", errParse("bad exponent: " + raw)
		}
		return num, "", body, nil
	}
	return num, rest, "", nil
}

func parseDecimalRat(s string) (*big.Rat, error) {
	if !isNumericString(s) {
		if mant, exp, ok := splitExponent(s); ok {
			m, err := parseDecimalRat(mant)
			if err != nil {
				return nil, err
			}
			e, ok := new(big.Int).SetString(strings.TrimPrefix(exp, "+"), 10)
			if !ok {
				return nil, errParse("bad exponent: " + s)
			}
			if e.Sign() < 0 {
				return new(big.Rat).Quo(m, new(big.Rat).SetInt(new(big.Int).Exp(big10, new(big.Int).Neg(e), nil))), nil
			}
			return new(big.Rat).Mul(m, new(big.Rat).SetInt(new(big.Int).Exp(big10, e, nil))), nil
		}
		return nil, errParse("not a decimal literal: " + s)
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, errParse("not a decimal literal: " + s)
	}
	return r, nil
}

// splitExponent 把 "1e3" 拆成 ("1", "3")；不带指数返回 ok=false。
func splitExponent(s string) (mantissa, exp string, ok bool) {
	idx := strings.IndexAny(s, "eE")
	if idx <= 0 || idx == len(s)-1 {
		return "", "", false
	}
	mantissa, exp = s[:idx], s[idx+1:]
	if !isNumericString(mantissa) {
		return "", "", false
	}
	if exp[0] == '+' || exp[0] == '-' {
		exp = exp[1:]
	}
	if !allDigits(exp) {
		return "", "", false
	}
	if s[idx+1] == '-' {
		exp = "-" + exp
	}
	return mantissa, exp, true
}

func isNumericString(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	digits := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			digits++
		case c == '.':
			if dot {
				return false
			}
			dot = true
		default:
			return false
		}
	}
	return digits > 0
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

type parseErr struct{ msg string }

func (e *parseErr) Error() string { return e.msg }

func errParse(msg string) error { return &parseErr{msg: msg} }

// IsParseError 判定是否为"文法不匹配"（→ exit 3），区别于 *Indeterminate（→ exit 2）。
func IsParseError(err error) bool {
	_, ok := err.(*parseErr)
	return ok
}
