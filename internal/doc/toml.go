package doc

// TOML 适配器：只做结构解析，产出的树与 YAML/JSON 同形。
//
// 设计取舍（与 plan/01 模块 5 一致）：
//   - 不引第三方 TOML 库：库通常把浮点解成 float64，而本项目的比较路径禁止浮点（K-01）。
//     这里保留**原文标量**，数值归一交给 internal/value 的精确有理数。
//   - `[service]` 必须是表（M4 实测的缺陷正是把 `[service]` 当 JSON 数组起始）；
//     解析失败给 file:line:col，映射 exit 3。
//   - 日期/时间、inf/nan 按字符串标量保留（TOML 的数值族不覆盖它们，求值不在本工具范围）。

import (
	"fmt"
	"math/big"
	"strings"
)

type tomlParser struct {
	file string
	src  string
	pos  int
	line int
	col  int
}

func parseTOML(path string, data []byte) (*Node, error) {
	p := &tomlParser{file: path, src: stripBOM(string(data)), line: 1, col: 1}
	root := newMapping(1, 1)
	cur := root
	declared := map[string]bool{}

	for {
		p.skipBlank()
		if p.eof() {
			return root, nil
		}
		line, col := p.line, p.col
		if p.cur() == '[' {
			array := strings.HasPrefix(p.src[p.pos:], "[[")
			if array {
				p.advanceN(2)
			} else {
				p.advance()
			}
			p.skipSpaces()
			keys, err := p.parseKeyPath()
			if err != nil {
				return nil, err
			}
			p.skipSpaces()
			if array {
				if !strings.HasPrefix(p.src[p.pos:], "]]") {
					return nil, p.errorf("expected ]] to close array-of-tables header")
				}
				p.advanceN(2)
			} else {
				if p.eof() || p.cur() != ']' {
					return nil, p.errorf("expected ] to close table header")
				}
				p.advance()
			}
			if err := p.endOfLine(); err != nil {
				return nil, err
			}
			path := strings.Join(keys, ".")
			if !array {
				if declared[path] {
					return nil, &ParseError{File: p.file, Line: line, Col: col, Msg: fmt.Sprintf("table [%s] defined twice", path)}
				}
				declared[path] = true
			}
			target, err := resolveTOMLTable(root, keys, array, line, col)
			if err != nil {
				return nil, &ParseError{File: p.file, Line: line, Col: col, Msg: err.Error()}
			}
			cur = target
			continue
		}

		keys, err := p.parseKeyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.eof() || p.cur() != '=' {
			return nil, p.errorf("expected `=` after key %q", strings.Join(keys, "."))
		}
		p.advance()
		p.skipSpaces()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		target := cur
		for _, k := range keys[:len(keys)-1] {
			sub, err := ensureTable(target, k, line, col)
			if err != nil {
				return nil, &ParseError{File: p.file, Line: line, Col: col, Msg: err.Error()}
			}
			target = sub
		}
		last := keys[len(keys)-1]
		if target.Get(last) != nil {
			return nil, &ParseError{File: p.file, Line: line, Col: col, Msg: fmt.Sprintf("duplicate key %q", last)}
		}
		target.Children = append(target.Children, newString(last, false, line, col), val)
		if err := p.endOfLine(); err != nil {
			return nil, err
		}
	}
}

// resolveTOMLTable 解析 `[a.b]` / `[[a]]` 头。数组表取最后一个元素继续。
func resolveTOMLTable(root *Node, keys []string, array bool, line, col int) (*Node, error) {
	parent := root
	for i, k := range keys {
		last := i == len(keys)-1
		if last && array {
			node := parent.Get(k)
			if node == nil {
				node = newSequence(line, col)
				parent.Children = append(parent.Children, newString(k, false, line, col), node)
			}
			if node.Kind != Sequence {
				return nil, fmt.Errorf("key %q is not an array of tables", k)
			}
			tbl := newMapping(line, col)
			node.Children = append(node.Children, tbl)
			return tbl, nil
		}
		if last {
			return ensureTable(parent, k, line, col)
		}
		node := parent.Get(k)
		if node == nil {
			node = newMapping(line, col)
			parent.Children = append(parent.Children, newString(k, false, line, col), node)
		}
		switch node.Kind {
		case Mapping:
			parent = node
		case Sequence:
			// 穿过数组表：`[[a]]` 之后再写 `[a.b]`，b 属于最后一个元素。
			if len(node.Children) == 0 {
				return nil, fmt.Errorf("key %q is an empty array of tables", k)
			}
			tbl := node.Children[len(node.Children)-1]
			if tbl.Kind != Mapping {
				return nil, fmt.Errorf("key %q is not a table", k)
			}
			parent = tbl
		default:
			return nil, fmt.Errorf("key %q is not a table", k)
		}
	}
	return parent, nil
}

func (p *tomlParser) parseKeyPath() ([]string, error) {
	var out []string
	for {
		p.skipSpaces()
		k, err := p.parseKey()
		if err != nil {
			return nil, err
		}
		out = append(out, k)
		p.skipSpaces()
		if !p.eof() && p.cur() == '.' {
			p.advance()
			continue
		}
		return out, nil
	}
}

func (p *tomlParser) parseKey() (string, error) {
	if p.eof() {
		return "", p.errorf("unexpected end of file, expected a key")
	}
	switch p.cur() {
	case '"':
		if strings.HasPrefix(p.src[p.pos:], `"""`) {
			return "", p.errorf("multi-line strings cannot be keys")
		}
		return p.parseBasicString()
	case '\'':
		if strings.HasPrefix(p.src[p.pos:], "'''") {
			return "", p.errorf("multi-line strings cannot be keys")
		}
		return p.parseLiteralString()
	}
	start := p.pos
	for !p.eof() {
		c := p.cur()
		if isTOMLBareKeyChar(c) {
			p.advance()
			continue
		}
		break
	}
	if p.pos == start {
		return "", p.errorf("expected a bare key")
	}
	return p.src[start:p.pos], nil
}

func (p *tomlParser) parseValue() (*Node, error) {
	if p.eof() {
		return nil, p.errorf("unexpected end of file, expected a value")
	}
	line, col := p.line, p.col
	switch c := p.cur(); {
	case c == '"':
		s, err := p.parseBasicString()
		if err != nil {
			return nil, err
		}
		return newString(s, true, line, col), nil
	case c == '\'':
		s, err := p.parseLiteralString()
		if err != nil {
			return nil, err
		}
		return newString(s, true, line, col), nil
	case c == '[':
		return p.parseArray()
	case c == '{':
		return p.parseInlineTable()
	default:
		raw, err := p.parseBareToken()
		if err != nil {
			return nil, err
		}
		return tomlScalar(raw, line, col, p.file)
	}
}

func (p *tomlParser) parseBasicString() (string, error) {
	if strings.HasPrefix(p.src[p.pos:], `"""`) {
		p.advanceN(3)
		// TOML 规范：紧跟在开引号后的第一个换行不属于内容。
		if !p.eof() && p.cur() == '\r' {
			p.advance()
		}
		if !p.eof() && p.cur() == '\n' {
			p.advance()
		}
		var b strings.Builder
		for {
			if p.eof() {
				return "", p.errorf("unterminated multi-line string")
			}
			if strings.HasPrefix(p.src[p.pos:], `"""`) {
				p.advanceN(3)
				return b.String(), nil
			}
			if p.cur() == '\\' {
				if err := p.decodeBasicEscape(&b, true); err != nil {
					return "", err
				}
				continue
			}
			b.WriteByte(p.advance())
		}
	}
	p.advance()
	var b strings.Builder
	for {
		if p.eof() {
			return "", p.errorf("unterminated string")
		}
		switch p.cur() {
		case '"':
			p.advance()
			return b.String(), nil
		case '\n', '\r':
			return "", p.errorf("newline in single-line string")
		case '\\':
			if err := p.decodeBasicEscape(&b, false); err != nil {
				return "", err
			}
		default:
			b.WriteByte(p.advance())
		}
	}
}

func (p *tomlParser) parseLiteralString() (string, error) {
	if strings.HasPrefix(p.src[p.pos:], "'''") {
		p.advanceN(3)
		if !p.eof() && p.cur() == '\r' {
			p.advance()
		}
		if !p.eof() && p.cur() == '\n' {
			p.advance()
		}
		start := p.pos
		for {
			if p.eof() {
				return "", p.errorf("unterminated multi-line literal string")
			}
			if strings.HasPrefix(p.src[p.pos:], "'''") {
				body := p.src[start:p.pos]
				p.advanceN(3)
				return body, nil
			}
			p.advance()
		}
	}
	p.advance()
	start := p.pos
	for {
		if p.eof() {
			return "", p.errorf("unterminated literal string")
		}
		switch p.cur() {
		case '\'':
			body := p.src[start:p.pos]
			p.advance()
			return body, nil
		case '\n', '\r':
			return "", p.errorf("newline in single-line literal string")
		}
		p.advance()
	}
}

// decodeBasicEscape 处理 TOML 基本字符串转义（含多行行尾续行）。
func (p *tomlParser) decodeBasicEscape(b *strings.Builder, multiline bool) error {
	p.advance() // backslash
	if p.eof() {
		return p.errorf("dangling backslash")
	}
	switch c := p.cur(); c {
	case 'b':
		p.advance()
		b.WriteByte('\b')
	case 't':
		p.advance()
		b.WriteByte('\t')
	case 'n':
		p.advance()
		b.WriteByte('\n')
	case 'f':
		p.advance()
		b.WriteByte('\f')
	case 'r':
		p.advance()
		b.WriteByte('\r')
	case '"':
		p.advance()
		b.WriteByte('"')
	case '\\':
		p.advance()
		b.WriteByte('\\')
	case 'u', 'U':
		n := 4
		if c == 'U' {
			n = 8
		}
		p.advance()
		r, err := p.readHexRune(n)
		if err != nil {
			return err
		}
		b.WriteRune(r)
	case '\r', '\n':
		if !multiline {
			return p.errorf("invalid escape in single-line string")
		}
		// 行尾续行：吃掉换行与其后的所有空白（含更多换行）。
		for !p.eof() {
			switch p.cur() {
			case ' ', '\t', '\r', '\n':
				p.advance()
			default:
				return nil
			}
		}
		return nil
	default:
		return p.errorf("invalid escape sequence \\%c", c)
	}
	return nil
}

func (p *tomlParser) readHexRune(n int) (rune, error) {
	if p.pos+n > len(p.src) {
		return 0, p.errorf("truncated unicode escape")
	}
	v := 0
	for i := 0; i < n; i++ {
		c := p.cur()
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return 0, p.errorf("invalid hex digit %q in unicode escape", c)
		}
		v = v*16 + d
		p.advance()
	}
	if v > 0x10FFFF {
		return 0, p.errorf("unicode escape out of range")
	}
	return rune(v), nil
}

func (p *tomlParser) parseArray() (*Node, error) {
	line, col := p.line, p.col
	p.advance() // [
	n := newSequence(line, col)
	for {
		p.skipBlank()
		if p.eof() {
			return nil, p.errorf("unterminated array")
		}
		if p.cur() == ']' {
			p.advance()
			return n, nil
		}
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, v)
		p.skipBlank()
		if p.eof() {
			return nil, p.errorf("unterminated array")
		}
		switch p.cur() {
		case ',':
			p.advance()
		case ']':
			p.advance()
			return n, nil
		default:
			return nil, p.errorf("expected `,` or `]` in array")
		}
	}
}

func (p *tomlParser) parseInlineTable() (*Node, error) {
	line, col := p.line, p.col
	p.advance() // {
	n := newMapping(line, col)
	p.skipSpaces()
	if !p.eof() && p.cur() == '}' {
		p.advance()
		return n, nil
	}
	for {
		p.skipSpaces()
		kline, kcol := p.line, p.col
		keys, err := p.parseKeyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpaces()
		if p.eof() || p.cur() != '=' {
			return nil, p.errorf("expected `=` in inline table")
		}
		p.advance()
		p.skipSpaces()
		v, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		target := n
		for _, k := range keys[:len(keys)-1] {
			sub, err := ensureTable(target, k, kline, kcol)
			if err != nil {
				return nil, &ParseError{File: p.file, Line: kline, Col: kcol, Msg: err.Error()}
			}
			target = sub
		}
		last := keys[len(keys)-1]
		if target.Get(last) != nil {
			return nil, &ParseError{File: p.file, Line: kline, Col: kcol, Msg: fmt.Sprintf("duplicate key %q in inline table", last)}
		}
		target.Children = append(target.Children, newString(last, false, kline, kcol), v)
		p.skipSpaces()
		if p.eof() {
			return nil, p.errorf("unterminated inline table")
		}
		switch p.cur() {
		case ',':
			p.advance()
		case '}':
			p.advance()
			return n, nil
		default:
			return nil, p.errorf("expected `,` or `}` in inline table")
		}
	}
}

// parseBareToken 读一个裸标量（bool / 数字 / 日期时间）。
func (p *tomlParser) parseBareToken() (string, error) {
	start := p.pos
	for !p.eof() {
		c := p.cur()
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' || c == ']' || c == '}' || c == '#' {
			break
		}
		p.advance()
	}
	if p.pos == start {
		return "", p.errorf("expected a value")
	}
	return p.src[start:p.pos], nil
}

// tomlScalar 把裸标量按 TOML 词法分类。数值只保留原文（去下划线 / 进制换算），
// 不经过 float64（K-01）。
func tomlScalar(raw string, line, col int, file string) (*Node, error) {
	switch raw {
	case "true", "false":
		return newBool(raw, line, col), nil
	}
	if isTOMLDatetime(raw) || isTOMLSpecialFloat(raw) {
		// 日期时间与 inf/nan 不属于本工具的比较族，按字符串保留。
		return newString(raw, true, line, col), nil
	}
	body := strings.ReplaceAll(raw, "_", "")
	if !validTOMLNumber(body) {
		return nil, &ParseError{File: file, Line: line, Col: col, Msg: fmt.Sprintf("invalid TOML value %q", raw)}
	}
	if strings.ContainsAny(body, ".eE") {
		return newNumber(TagFloat, body, line, col), nil
	}
	if dec, ok := tomlIntegerToDecimal(body); ok {
		return newNumber(TagInt, dec, line, col), nil
	}
	return nil, &ParseError{File: file, Line: line, Col: col, Msg: fmt.Sprintf("invalid TOML integer %q", raw)}
}

func validTOMLNumber(s string) bool {
	if s == "" {
		return false
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "+"), "-")
	if s == "" {
		return false
	}
	switch {
	case strings.HasPrefix(s, "0x"), strings.HasPrefix(s, "0o"), strings.HasPrefix(s, "0b"):
		return len(s) > 2
	case strings.ContainsAny(s, "eE"):
		return validTOMLFloatBody(s)
	}
	gotDigit, gotDot := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			gotDigit = true
		case c == '.' && !gotDot:
			gotDot = true
		default:
			return false
		}
	}
	return gotDigit
}

func validTOMLFloatBody(s string) bool {
	mant := s
	exp := ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant, exp = s[:i], s[i+1:]
		if exp == "" {
			return false
		}
		exp = strings.TrimPrefix(strings.TrimPrefix(exp, "+"), "-")
		if exp == "" {
			return false
		}
		for i := 0; i < len(exp); i++ {
			if exp[i] < '0' || exp[i] > '9' {
				return false
			}
		}
	}
	if mant == "" {
		return false
	}
	digits, dot := 0, 0
	for i := 0; i < len(mant); i++ {
		switch {
		case mant[i] >= '0' && mant[i] <= '9':
			digits++
		case mant[i] == '.':
			dot++
			if dot > 1 {
				return false
			}
		default:
			return false
		}
	}
	return digits > 0
}

func tomlIntegerToDecimal(s string) (string, bool) {
	neg := false
	switch {
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	case strings.HasPrefix(s, "-"):
		neg = true
		s = s[1:]
	}
	base := 10
	switch {
	case strings.HasPrefix(s, "0x"):
		base, s = 16, s[2:]
	case strings.HasPrefix(s, "0o"):
		base, s = 8, s[2:]
	case strings.HasPrefix(s, "0b"):
		base, s = 2, s[2:]
	}
	v := new(big.Int)
	if _, ok := v.SetString(s, base); !ok {
		return "", false
	}
	if neg {
		v.Neg(v)
	}
	return v.String(), true
}

func isTOMLSpecialFloat(s string) bool {
	switch strings.ToLower(s) {
	case "inf", "+inf", "-inf", "nan", "+nan", "-nan":
		return true
	}
	return false
}

// isTOMLDatetime 识别 1979-05-27 / 07:32:00 / 1979-05-27T07:32:00Z 等形态。
func isTOMLDatetime(s string) bool {
	if len(s) < 8 {
		return false
	}
	digits := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			digits++
		}
	}
	if digits < 6 {
		return false
	}
	hasDate := len(s) >= 10 && s[4] == '-'
	hasTime := strings.Contains(s, ":")
	return (hasDate && (hasTime || len(s) == 10)) || (hasTime && !hasDate)
}

// endOfLine 吃掉行尾空白与注释，并要求确实到了行尾。
func (p *tomlParser) endOfLine() error {
	p.skipSpaces()
	p.skipComment()
	if p.eof() {
		return nil
	}
	if p.cur() == '\r' {
		p.advance()
	}
	if !p.eof() && p.cur() == '\n' {
		p.advance()
		return nil
	}
	if p.eof() {
		return nil
	}
	return p.errorf("unexpected trailing content %q", string(p.cur()))
}

func (p *tomlParser) skipBlank() {
	for !p.eof() {
		switch p.cur() {
		case ' ', '\t', '\r', '\n':
			p.advance()
		case '#':
			p.skipComment()
		default:
			return
		}
	}
}

func (p *tomlParser) skipSpaces() {
	for !p.eof() && (p.cur() == ' ' || p.cur() == '\t') {
		p.advance()
	}
}

func (p *tomlParser) skipComment() {
	if p.eof() || p.cur() != '#' {
		return
	}
	for !p.eof() && p.cur() != '\n' {
		p.advance()
	}
}

func (p *tomlParser) eof() bool { return p.pos >= len(p.src) }
func (p *tomlParser) cur() byte { return p.src[p.pos] }

func (p *tomlParser) advance() byte {
	c := p.src[p.pos]
	p.pos++
	if c == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	return c
}

func (p *tomlParser) advanceN(n int) {
	for i := 0; i < n && !p.eof(); i++ {
		p.advance()
	}
}

func (p *tomlParser) errorf(format string, args ...any) *ParseError {
	return &ParseError{File: p.file, Line: p.line, Col: p.col, Msg: fmt.Sprintf(format, args...)}
}

func isTOMLBareKeyChar(c byte) bool {
	return c == '_' || c == '-' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func stripBOM(s string) string {
	return strings.TrimPrefix(s, "\ufeff")
}
