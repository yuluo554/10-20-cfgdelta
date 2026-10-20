package doc

// HCL 适配器：**只做结构解析，不做表达式求值**（plan/01 模块 5）。
//
// 形态约定（唯一实现处）：
//   - 块 `type "label" ... { body }` 落成 type → label → body 的嵌套表；同名块重复时转成序列；
//   - 属性 `name = <literal>` 中，字面量（字符串 / 数字 / bool / null / 列表 / 对象）按值落树；
//   - 其余表达式（`var.x`、函数调用、插值、`for` 表达式、运算符组合）一律保留原文，
//     标 `!!unevaluated` —— 下游把它当 indeterminate(unevaluated_expression)，**不猜结果**；
//   - 语法错误给 file:line:col（exit 3）。

import (
	"fmt"
	"strings"
)

type hclTokKind int

const (
	hclEOF hclTokKind = iota
	hclNewline
	hclIdent
	hclString
	hclNumber
	hclPunct
)

type hclTok struct {
	kind   hclTokKind
	text   string // 解码后的内容（字符串 / 标识符 / 数字原文）
	start  int    // 源码字节偏移（保留表达式原文）
	end    int
	line   int
	col    int
	interp bool // 字符串里含未转义的 ${...} 插值
}

func parseHCL(path string, data []byte) (*Node, error) {
	toks, err := lexHCL(path, stripBOM(string(data)))
	if err != nil {
		return nil, err
	}
	p := &hclParser{file: path, src: stripBOM(string(data)), toks: toks}
	root := newMapping(1, 1)
	if err := p.parseBody(root); err != nil {
		return nil, err
	}
	return root, nil
}

// ── 词法 ───────────────────────────────────────────────────────────────────

type hclLexer struct {
	file string
	src  string
	pos  int
	line int
	col  int
	toks []hclTok
}

func lexHCL(path, src string) ([]hclTok, error) {
	l := &hclLexer{file: path, src: src, line: 1, col: 1}
	for {
		l.skipSpace()
		if l.eof() {
			l.emit(hclEOF, "", l.pos, l.pos)
			return l.toks, nil
		}
		c := l.cur()
		switch {
		case c == '\n':
			start := l.pos
			l.advance()
			l.emit(hclNewline, "\n", start, l.pos)
		case c == '#' || (c == '/' && l.peekAt(1) == '/'):
			for !l.eof() && l.cur() != '\n' {
				l.advance()
			}
		case c == '/' && l.peekAt(1) == '*':
			l.advanceN(2)
			for {
				if l.eof() {
					return nil, l.errorf("unterminated block comment")
				}
				if l.cur() == '*' && l.peekAt(1) == '/' {
					l.advanceN(2)
					break
				}
				l.advance()
			}
		case c == '"':
			if err := l.lexString(); err != nil {
				return nil, err
			}
		case c == '<' && l.peekAt(1) == '<':
			if err := l.lexHeredoc(); err != nil {
				return nil, err
			}
		case isHCLIdentStart(c):
			start, sl, sc := l.pos, l.line, l.col
			for !l.eof() && isHCLIdentPart(l.cur()) {
				l.advance()
			}
			l.toks = append(l.toks, hclTok{kind: hclIdent, text: l.src[start:l.pos], start: start, end: l.pos, line: sl, col: sc})
		case c >= '0' && c <= '9':
			start, sl, sc := l.pos, l.line, l.col
			for !l.eof() && isHCLNumberPart(l.cur()) {
				l.advance()
			}
			l.toks = append(l.toks, hclTok{kind: hclNumber, text: l.src[start:l.pos], start: start, end: l.pos, line: sl, col: sc})
		case strings.IndexByte("{}[]()=,.:?*+-/%<>!&|~", c) >= 0:
			start, sl, sc := l.pos, l.line, l.col
			l.advance()
			l.toks = append(l.toks, hclTok{kind: hclPunct, text: string(c), start: start, end: l.pos, line: sl, col: sc})
		default:
			return nil, l.errorf("unexpected character %q", string(c))
		}
	}
}

func (l *hclLexer) lexString() error {
	start, sl, sc := l.pos, l.line, l.col
	l.advance() // opening quote
	var b strings.Builder
	interp := false
	for {
		if l.eof() {
			return &ParseError{File: l.file, Line: sl, Col: sc, Msg: "unterminated string"}
		}
		c := l.cur()
		switch c {
		case '"':
			l.advance()
			l.toks = append(l.toks, hclTok{kind: hclString, text: b.String(), start: start, end: l.pos, line: sl, col: sc, interp: interp})
			return nil
		case '\n':
			return &ParseError{File: l.file, Line: l.line, Col: l.col, Msg: "newline in string (use a heredoc)"}
		case '\\':
			l.advance()
			if l.eof() {
				return &ParseError{File: l.file, Line: sl, Col: sc, Msg: "dangling escape"}
			}
			switch e := l.cur(); e {
			case 'n':
				b.WriteByte('\n')
				l.advance()
			case 'r':
				b.WriteByte('\r')
				l.advance()
			case 't':
				b.WriteByte('\t')
				l.advance()
			case '"':
				b.WriteByte('"')
				l.advance()
			case '\\':
				b.WriteByte('\\')
				l.advance()
			case 'u', 'U':
				n := 4
				if e == 'U' {
					n = 8
				}
				l.advance()
				r, err := l.readHex(n)
				if err != nil {
					return err
				}
				b.WriteRune(r)
			default:
				return &ParseError{File: l.file, Line: l.line, Col: l.col, Msg: fmt.Sprintf("invalid escape \\%c", e)}
			}
		case '$':
			if l.peekAt(1) == '$' && l.peekAt(2) == '{' {
				// `$${` 是字面量 `${`，不是插值。
				b.WriteString("${")
				l.advanceN(3)
				continue
			}
			if l.peekAt(1) == '{' {
				interp = true
			}
			b.WriteByte(c)
			l.advance()
		case '%':
			if l.peekAt(1) == '%' && l.peekAt(2) == '{' {
				b.WriteString("%{")
				l.advanceN(3)
				continue
			}
			b.WriteByte(c)
			l.advance()
		default:
			b.WriteByte(c)
			l.advance()
		}
	}
}

// lexHeredoc 读 `<<EOF` / `<<-EOF`；内容按原样保留（不做模板求值）。
func (l *hclLexer) lexHeredoc() error {
	start, sl, sc := l.pos, l.line, l.col
	l.advanceN(2)
	indented := false
	if !l.eof() && l.cur() == '-' {
		indented = true
		l.advance()
	}
	markerStart := l.pos
	for !l.eof() && (isHCLIdentPart(l.cur()) || l.cur() == '-') {
		l.advance()
	}
	if l.pos == markerStart {
		return &ParseError{File: l.file, Line: sl, Col: sc, Msg: "heredoc marker is missing"}
	}
	marker := l.src[markerStart:l.pos]
	for !l.eof() && l.cur() != '\n' {
		l.advance()
	}
	if l.eof() {
		return &ParseError{File: l.file, Line: sl, Col: sc, Msg: "unterminated heredoc"}
	}
	l.advance() // newline
	var content strings.Builder
	for {
		if l.eof() {
			return &ParseError{File: l.file, Line: sl, Col: sc, Msg: fmt.Sprintf("unterminated heredoc %q", marker)}
		}
		lineStart := l.pos
		for !l.eof() && l.cur() != '\n' {
			l.advance()
		}
		lineText := l.src[lineStart:l.pos]
		done := false
		if indented {
			done = strings.TrimSpace(lineText) == marker
		} else {
			done = strings.TrimSuffix(lineText, "\r") == marker
		}
		if done {
			l.advance() // newline
			break
		}
		if indented {
			lineText = strings.TrimPrefix(lineText, "\t")
		}
		content.WriteString(lineText)
		content.WriteString("\n")
		if !l.eof() {
			l.advance() // newline
		}
	}
	body := content.String()
	interp := strings.Contains(body, "${")
	l.toks = append(l.toks, hclTok{kind: hclString, text: body, start: start, end: l.pos, line: sl, col: sc, interp: interp})
	return nil
}

func (l *hclLexer) readHex(n int) (rune, error) {
	if l.pos+n > len(l.src) {
		return 0, l.errorf("truncated unicode escape")
	}
	v := 0
	for i := 0; i < n; i++ {
		c := l.cur()
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return 0, l.errorf("invalid hex digit %q", c)
		}
		v = v*16 + d
		l.advance()
	}
	return rune(v), nil
}

func (l *hclLexer) skipSpace() {
	for !l.eof() && (l.cur() == ' ' || l.cur() == '\t' || l.cur() == '\r') {
		l.advance()
	}
}

func (l *hclLexer) emit(kind hclTokKind, text string, start, end int) {
	l.toks = append(l.toks, hclTok{kind: kind, text: text, start: start, end: end, line: l.line, col: l.col})
}

func (l *hclLexer) eof() bool { return l.pos >= len(l.src) }
func (l *hclLexer) cur() byte { return l.src[l.pos] }
func (l *hclLexer) peekAt(n int) byte {
	if l.pos+n >= len(l.src) {
		return 0
	}
	return l.src[l.pos+n]
}

func (l *hclLexer) advance() {
	if l.src[l.pos] == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	l.pos++
}

func (l *hclLexer) advanceN(n int) {
	for i := 0; i < n && !l.eof(); i++ {
		l.advance()
	}
}

func (l *hclLexer) errorf(format string, args ...any) *ParseError {
	return &ParseError{File: l.file, Line: l.line, Col: l.col, Msg: fmt.Sprintf(format, args...)}
}

func isHCLIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isHCLIdentPart(c byte) bool {
	return isHCLIdentStart(c) || (c >= '0' && c <= '9') || c == '-'
}

func isHCLNumberPart(c byte) bool {
	return (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-'
}

// ── 语法 ───────────────────────────────────────────────────────────────────

type hclParser struct {
	file string
	src  string
	toks []hclTok
	i    int
}

func (p *hclParser) parseBody(into *Node) error {
	for {
		p.skipNewlines()
		if p.atEOF() {
			return nil
		}
		if p.isPunct("}") {
			return nil
		}
		tok := p.peek()
		if tok.kind != hclIdent {
			return p.errAt(tok, "expected an attribute or block, got %q", tok.text)
		}
		p.i++
		if p.isPunct("=") {
			p.i++
			node, err := p.parseExpression()
			if err != nil {
				return err
			}
			if into.Get(tok.text) != nil {
				return p.errAt(tok, "duplicate attribute %q", tok.text)
			}
			into.Children = append(into.Children, newString(tok.text, false, tok.line, tok.col), node)
			continue
		}

		var labels []string
		for {
			lab := p.peek()
			if lab.kind != hclString {
				break
			}
			if lab.interp {
				return p.errAt(lab, "block label must be a literal string")
			}
			labels = append(labels, lab.text)
			p.i++
		}
		if !p.isPunct("{") {
			return p.errAt(p.peek(), "expected `{` to start block %q", tok.text)
		}
		p.i++
		body := newMapping(tok.line, tok.col)
		if err := p.parseBody(body); err != nil {
			return err
		}
		if !p.isPunct("}") {
			return p.errAt(p.peek(), "expected `}` to close block %q", tok.text)
		}
		p.i++
		if err := addHCLBlock(into, tok.text, labels, body, tok.line, tok.col); err != nil {
			return p.errAt(tok, "%v", err)
		}
	}
}

func addHCLBlock(parent *Node, typ string, labels []string, body *Node, line, col int) error {
	if len(labels) == 0 {
		return appendOrSequence(parent, typ, body, line, col)
	}
	// 块类型必须留在路径里（`resource "thing" "b"` → /resource/thing/b），
	// 否则 resource 与 data 同名标签会互相覆盖。
	cur, err := ensureTable(parent, typ, line, col)
	if err != nil {
		return err
	}
	for i, lab := range labels {
		if i == len(labels)-1 {
			return appendOrSequence(cur, lab, body, line, col)
		}
		sub, err := ensureTable(cur, lab, line, col)
		if err != nil {
			return err
		}
		cur = sub
	}
	return nil
}

// appendOrSequence：同名块第二次出现时转成序列（保留两个 body，不静默覆盖）。
func appendOrSequence(parent *Node, key string, body *Node, line, col int) error {
	existing := parent.Get(key)
	if existing == nil {
		parent.Children = append(parent.Children, newString(key, false, line, col), body)
		return nil
	}
	if existing.Kind == Sequence {
		existing.Children = append(existing.Children, body)
		return nil
	}
	if existing.Kind == Mapping {
		seq := newSequence(line, col)
		seq.Children = append(seq.Children, existing, body)
		for i := 0; i+1 < len(parent.Children); i += 2 {
			if parent.Children[i].Raw == key {
				parent.Children[i+1] = seq
				return nil
			}
		}
	}
	return fmt.Errorf("block %q collides with a non-block value", key)
}

// parseExpression 收集一个属性的表达式 token 区间：字面量落结构，其余保留原文。
func (p *hclParser) parseExpression() (*Node, error) {
	startIdx := p.i
	depth := 0
	startTok := p.peek()
	for !p.atEOF() {
		tok := p.peek()
		if tok.kind == hclNewline && depth == 0 {
			// 行尾属于 body 的分隔，不属于表达式（literal 解析不能吃到一个多余 token）。
			break
		}
		if tok.kind == hclPunct {
			switch tok.text {
			case "{", "[", "(":
				depth++
			case "}", "]", ")":
				if depth == 0 {
					// `}` 属于外层块：表达式到此结束。
					goto done
				}
				depth--
			}
		}
		p.i++
	}
done:
	if p.i <= startIdx {
		return nil, p.errAt(startTok, "missing expression after `=`")
	}
	toks := p.toks[startIdx:p.i]
	end := toks[len(toks)-1].end
	raw := strings.TrimSpace(p.src[startTok.start:end])

	if node, ok := literalFromHCLTokens(toks); ok {
		return node, nil
	}
	return newUnevaluated(collapseHCLSpace(raw), startTok.line, startTok.col), nil
}

// collapseHCLSpace 只折叠换行与连续空白 —— 表达式按**书写原文**比较，
// 但不让纯缩进差异制造假差异。
func collapseHCLSpace(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// literalFromHCLTokens 尝试把 token 序列读成纯字面量；含引用/调用/运算符即失败。
func literalFromHCLTokens(toks []hclTok) (*Node, bool) {
	lp := &hclLitParser{toks: toks}
	node, ok := lp.parse()
	if !ok || lp.i != len(toks) {
		return nil, false
	}
	return node, true
}

type hclLitParser struct {
	toks []hclTok
	i    int
}

func (p *hclLitParser) peek() hclTok {
	if p.i >= len(p.toks) {
		return hclTok{kind: hclEOF}
	}
	return p.toks[p.i]
}

func (p *hclLitParser) parse() (*Node, bool) {
	tok := p.peek()
	switch tok.kind {
	case hclString:
		if tok.interp {
			return nil, false
		}
		p.i++
		return newString(tok.text, true, tok.line, tok.col), true
	case hclNumber:
		p.i++
		tag := TagInt
		if strings.ContainsAny(tok.text, ".eE") {
			tag = TagFloat
		}
		return newNumber(tag, tok.text, tok.line, tok.col), true
	case hclIdent:
		p.i++
		switch tok.text {
		case "true", "false":
			return newBool(tok.text, tok.line, tok.col), true
		case "null":
			return &Node{Kind: Scalar, Tag: TagNull, Raw: "", Line: tok.line, Column: tok.col}, true
		}
		return nil, false
	case hclPunct:
		switch tok.text {
		case "-":
			p.i++
			next := p.peek()
			if next.kind != hclNumber {
				return nil, false
			}
			p.i++
			tag := TagInt
			if strings.ContainsAny(next.text, ".eE") {
				tag = TagFloat
			}
			return newNumber(tag, "-"+next.text, next.line, next.col), true
		case "[":
			return p.parseList()
		case "{":
			return p.parseObject()
		}
	}
	return nil, false
}

func (p *hclLitParser) parseList() (*Node, bool) {
	open := p.peek()
	p.i++
	seq := newSequence(open.line, open.col)
	for {
		if p.peek().kind == hclEOF {
			return nil, false
		}
		if p.peek().kind == hclPunct && p.peek().text == "]" {
			p.i++
			return seq, true
		}
		item, ok := p.parse()
		if !ok {
			return nil, false
		}
		seq.Children = append(seq.Children, item)
		if p.peek().kind == hclPunct && p.peek().text == "," {
			p.i++
			continue
		}
		if p.peek().kind == hclPunct && p.peek().text == "]" {
			p.i++
			return seq, true
		}
		return nil, false
	}
}

func (p *hclLitParser) parseObject() (*Node, bool) {
	open := p.peek()
	p.i++
	m := newMapping(open.line, open.col)
	for {
		if p.peek().kind == hclEOF {
			return nil, false
		}
		if p.peek().kind == hclPunct && p.peek().text == "}" {
			p.i++
			return m, true
		}
		keyTok := p.peek()
		var key string
		switch keyTok.kind {
		case hclIdent, hclString:
			if keyTok.interp {
				return nil, false
			}
			key = keyTok.text
			p.i++
		default:
			return nil, false
		}
		sep := p.peek()
		if sep.kind != hclPunct || (sep.text != "=" && sep.text != ":") {
			return nil, false
		}
		p.i++
		val, ok := p.parse()
		if !ok {
			return nil, false
		}
		if m.Get(key) != nil {
			return nil, false
		}
		m.Children = append(m.Children, newString(key, false, keyTok.line, keyTok.col), val)
		if p.peek().kind == hclPunct && p.peek().text == "," {
			p.i++
			continue
		}
		if p.peek().kind == hclPunct && p.peek().text == "}" {
			p.i++
			return m, true
		}
		return nil, false
	}
}

func (p *hclParser) skipNewlines() {
	for !p.atEOF() && p.peek().kind == hclNewline {
		p.i++
	}
}

func (p *hclParser) atEOF() bool { return p.peek().kind == hclEOF }

func (p *hclParser) peek() hclTok {
	if p.i >= len(p.toks) {
		return hclTok{kind: hclEOF}
	}
	return p.toks[p.i]
}

func (p *hclParser) isPunct(s string) bool {
	t := p.peek()
	return t.kind == hclPunct && t.text == s
}

func (p *hclParser) errAt(t hclTok, format string, args ...any) *ParseError {
	return &ParseError{File: p.file, Line: t.line, Col: t.col, Msg: fmt.Sprintf(format, args...)}
}
