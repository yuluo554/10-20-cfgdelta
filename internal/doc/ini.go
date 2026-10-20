package doc

// INI 适配器：只做结构解析。
//
// 形态约定（唯一实现处）：
//   - `[section]` 是表；`[a.b]` 是嵌套表；
//   - `key = value` 的键值都按**字符串**落树：INI 没有类型系统，猜数字只会制造假差异；
//   - `;` 与 `#` 起注释；值两侧空白去掉；成对引号去掉并标记 quoted；
//   - 重复键 → exit 3（与 keyed 的纪律一致：不取首、不静默覆盖）。

import (
	"fmt"
	"strings"
)

func parseINI(path string, data []byte) (*Node, error) {
	root := newMapping(1, 1)
	cur := root
	seen := map[string]bool{}

	lines := strings.Split(strings.ReplaceAll(stripBOM(string(data)), "\r\n", "\n"), "\n")
	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == ';' || trimmed[0] == '#' {
			continue
		}
		if trimmed[0] == '[' {
			end := strings.Index(trimmed, "]")
			if end < 0 {
				return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: "unterminated section header"}
			}
			if rest := strings.TrimSpace(trimmed[end+1:]); rest != "" && rest[0] != ';' && rest[0] != '#' {
				return nil, &ParseError{File: path, Line: ln, Col: end + 2, Msg: "unexpected content after section header"}
			}
			name := strings.TrimSpace(trimmed[1:end])
			if name == "" {
				return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: "empty section name"}
			}
			if seen[name] {
				return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: fmt.Sprintf("section [%s] defined twice", name)}
			}
			seen[name] = true
			target := root
			segs := strings.Split(name, ".")
			for j, seg := range segs {
				seg = strings.TrimSpace(seg)
				if seg == "" {
					return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: fmt.Sprintf("empty segment in section [%s]", name)}
				}
				sub, err := ensureTable(target, seg, ln, 1)
				if err != nil {
					return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: err.Error()}
				}
				target = sub
				_ = j
			}
			cur = target
			continue
		}

		eq := indexINISeparator(trimmed)
		if eq < 0 {
			return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: fmt.Sprintf("expected `key = value`, got %q", trimmed)}
		}
		key := strings.TrimSpace(trimmed[:eq])
		if key == "" {
			return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: "empty key"}
		}
		rawVal := strings.TrimSpace(stripINIComment(trimmed[eq+1:]))
		val, _ := unquoteINI(rawVal)
		if cur.Get(key) != nil {
			return nil, &ParseError{File: path, Line: ln, Col: eq + 2, Msg: fmt.Sprintf("duplicate key %q", key)}
		}
		// INI 没有类型系统：所有值都按字符串落树（quoted 样式），不猜数字。
		cur.Children = append(cur.Children, newString(key, false, ln, 1), newString(val, true, ln, eq+2))
	}
	return root, nil
}

// indexINISeparator 找第一个 `=` 或 `:`（`: ` 形态只在没有 `=` 时使用）。
func indexINISeparator(s string) int {
	if i := strings.Index(s, "="); i >= 0 {
		return i
	}
	return strings.Index(s, ":")
}

// stripINIComment 去掉值尾部的行内注释（引号内的 `;`/`#` 不算注释）。
func stripINIComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ';' || c == '#':
			return s[:i]
		}
	}
	return s
}

// unquoteINI 去掉成对引号；返回 (值, 是否被引号包裹)。
func unquoteINI(s string) (string, bool) {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			inner := s[1 : len(s)-1]
			if s[0] == '"' {
				inner = strings.ReplaceAll(inner, `\"`, `"`)
				inner = strings.ReplaceAll(inner, `\\`, `\`)
			}
			return inner, true
		}
	}
	return s, false
}
