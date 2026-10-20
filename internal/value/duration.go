package value

import (
	"math/big"
	"strings"
)

// durationUnits 是 §4.2 的单位表（纳秒倍数）。唯一实现处。
var durationUnits = map[string]*big.Rat{
	"ns": big.NewRat(1, 1),
	"us": big.NewRat(1000, 1),
	"µs": big.NewRat(1000, 1),
	"μs": big.NewRat(1000, 1), // U+03BC 与 U+00B5 都接受
	"ms": big.NewRat(1000000, 1),
	"s":  big.NewRat(1000000000, 1),
	"m":  big.NewRat(60000000000, 1),
	"h":  big.NewRat(3600000000000, 1),
}

// ParseDuration 按 Go time.ParseDuration 文法把字符串解析为纳秒整数（§4.2）。
//
//	duration := ( number unit )+       ; 至少一组
//	特例:     "0" 合法（Go 同款）
func ParseDuration(s string) (int64, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return 0, errParse("empty duration")
	}
	neg := false
	if raw[0] == '+' || raw[0] == '-' {
		neg = raw[0] == '-'
		raw = raw[1:]
	}
	if raw == "0" || raw == "0s" {
		return 0, nil
	}
	if raw == "" {
		return 0, errParse("duration has sign but no value")
	}

	total := new(big.Rat)
	groups := 0
	i := 0
	for i < len(raw) {
		start := i
		dot := false
		digits := 0
		for i < len(raw) {
			c := raw[i]
			if c >= '0' && c <= '9' {
				digits++
				i++
				continue
			}
			if c == '.' && !dot {
				dot = true
				i++
				continue
			}
			break
		}
		if digits == 0 {
			return 0, errParse("duration: expected number at offset " + itoa(start) + ": " + s)
		}
		numText := raw[start:i]

		unitStart := i
		for i < len(raw) && !(raw[i] >= '0' && raw[i] <= '9') && raw[i] != '.' {
			i++
		}
		unit := raw[unitStart:i]
		mult, ok := durationUnits[unit]
		if !ok {
			return 0, errParse("duration: unknown unit " + quote(unit) + " in " + s)
		}
		part, err := parseDecimalRat(numText)
		if err != nil {
			return 0, err
		}
		total.Add(total, new(big.Rat).Mul(part, mult))
		groups++
	}
	if groups == 0 {
		return 0, errParse("duration: no groups parsed: " + s)
	}
	if total.Sign() < 0 {
		return 0, errParse("duration: negative total is not allowed: " + s)
	}
	// 纳秒必须是整数：big.Rat 的 RatString 已在解析期保证，此处断言不丢精度。
	if !total.IsInt() {
		return 0, errParse("duration: sub-nanosecond precision: " + s)
	}
	ns := total.Num()
	if !ns.IsInt64() {
		return 0, errParse("duration: overflows int64 nanoseconds: " + s)
	}
	out := ns.Int64()
	if neg {
		out = -out
	}
	return out, nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func quote(s string) string { return "\"" + s + "\"" }
