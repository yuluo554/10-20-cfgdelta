package doc

// .env 适配器：只做结构解析。
//
// 形态约定（唯一实现处）：
//   - 每行 `KEY=VALUE`，可带 `export ` 前缀；`#` 起注释（整行或行内，引号内除外）；
//   - 成对引号去掉并标记 quoted（.env 的引号规则与 TOML/YAML **不**互通，不做跨格式合并）；
//   - 值一律字符串：.env 没有类型系统；
//   - 重复键 → exit 3（不取首、不静默覆盖）。

import (
	"fmt"
	"strings"
)

func parseDotEnv(path string, data []byte) (*Node, error) {
	root := newMapping(1, 1)
	lines := strings.Split(strings.ReplaceAll(stripBOM(string(data)), "\r\n", "\n"), "\n")
	for i, line := range lines {
		ln := i + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "export ")
		trimmed = strings.TrimSpace(trimmed)
		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: fmt.Sprintf("expected KEY=VALUE, got %q", trimmed)}
		}
		key := strings.TrimSpace(trimmed[:eq])
		if key == "" {
			return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: "empty key"}
		}
		if !validEnvKey(key) {
			return nil, &ParseError{File: path, Line: ln, Col: 1, Msg: fmt.Sprintf("invalid key %q (want [A-Za-z_][A-Za-z0-9_]*)", key)}
		}
		rawVal := strings.TrimSpace(stripEnvComment(trimmed[eq+1:]))
		val, _ := unquoteEnv(rawVal)
		if root.Get(key) != nil {
			return nil, &ParseError{File: path, Line: ln, Col: eq + 2, Msg: fmt.Sprintf("duplicate key %q", key)}
		}
		// .env 没有类型系统：所有值都按字符串落树（quoted 样式），不猜数字。
		root.Children = append(root.Children, newString(key, false, ln, 1), newString(val, true, ln, eq+2))
	}
	return root, nil
}

func validEnvKey(k string) bool {
	for i := 0; i < len(k); i++ {
		c := k[i]
		ok := c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (i > 0 && c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return k != ""
}

// stripEnvComment 去掉行内注释（引号内的 `#` 不算）。
func stripEnvComment(s string) string {
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
		case c == '#':
			return s[:i]
		}
	}
	return s
}

// unquoteEnv 去掉成对引号；双引号内按 .env 规则展开 \n / \t / \" / \\。
func unquoteEnv(s string) (string, bool) {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			inner := s[1 : len(s)-1]
			inner = strings.ReplaceAll(inner, `\n`, "\n")
			inner = strings.ReplaceAll(inner, `\t`, "\t")
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			return inner, true
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1], true
		}
	}
	return s, false
}
