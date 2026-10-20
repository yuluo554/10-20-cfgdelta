// Package cli 是子命令解析与编排层 —— 唯一知道子命令名的地方（plan/03 §六）。
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/classify"
	"github.com/yuluo554/10-20-cfgdelta/internal/diff"
	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
	"github.com/yuluo554/10-20-cfgdelta/internal/exitcode"
	"github.com/yuluo554/10-20-cfgdelta/internal/profile"
	"github.com/yuluo554/10-20-cfgdelta/internal/selftest"
	"github.com/yuluo554/10-20-cfgdelta/internal/value"
	"github.com/yuluo554/10-20-cfgdelta/internal/version"
)

// Env 是 CLI 的注入点（测试用，生产走 os.Stdout/os.Stderr/os.Args）。
type Env struct {
	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	Root   string // 仓库根（selftest 用）；空则自动定位
}

// Run 执行一次 CLI 调用，返回退出码（不调用 os.Exit）。
func Run(env Env) exitcode.Code {
	if env.Stdout == nil {
		env.Stdout = os.Stdout
	}
	if env.Stderr == nil {
		env.Stderr = os.Stderr
	}
	if len(env.Args) == 0 {
		usage(env.Stderr)
		return exitcode.Usage
	}

	// 全局旗标：--version / --help（任一位置均可）。
	for _, a := range env.Args {
		switch a {
		case "--version", "-v":
			fmt.Fprintf(env.Stdout, "cfgdelta %s\n", version.Value())
			return exitcode.Equal
		case "--help", "-h":
			usage(env.Stdout)
			return exitcode.Equal
		}
	}

	cmd, rest := env.Args[0], env.Args[1:]
	switch cmd {
	case "normalize":
		return runNormalize(env, rest)
	case "diff":
		return runDiff(env, rest, false)
	case "classify":
		return runDiff(env, rest, true)
	case "gate":
		return runGate(env, rest)
	case "selftest":
		return selftest.Run(env.Stdout, env.Stderr, env.Root, rest)
	default:
		fmt.Fprintf(env.Stderr, "cfgdelta: unknown subcommand %q\n\n", cmd)
		usage(env.Stderr)
		// 用法错误统一 exit 3（K-07），不使用 2。
		return exitcode.Usage
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `cfgdelta — behavioral config diff (normalization layer + impact classification)

USAGE
  cfgdelta <subcommand> [flags] [args]

SUBCOMMANDS
  normalize --at <path> --family <fam> <file>
      Normalize one value at a path and print its exact canonical form.
  diff <left> <right>
      Structurally-independent comparison. Exit 0 = equivalent, 1 = behavioral
      diff, 2 = indeterminate present, 3 = parse/usage error.
  classify <left> <right>
      Same as diff, with the impact class attached to every entry.
  gate --fail-on <class,...> <left> <right>
      CI gate: only the listed classes block (exit 1). Unlisted behavioral
      differences are still printed. indeterminate still exits 2 unless
      --allow-indeterminate.
  selftest
      Run the bundled golden suite (no network, no external commands).

COMMON FLAGS
  --profile <name>        versioned defaults table, e.g. k8s/v1.29
  --seq-mode <mode>       ordered | set | keyed:<field>   (default: ordered)
  --coerce-at <glob>=<fam> declare a value family at a path (repeatable)
  --strict                upgrade unknown families to indeterminate
  --json                  machine-readable output
  --dump-normalized       print both trees after defaulting
  --allow-indeterminate   do not let indeterminate force exit 2
  --version / --help
`)
}

// ── normalize ──────────────────────────────────────────────────────────────

func runNormalize(env Env, args []string) exitcode.Code {
	var at, fam string
	var file string
	for i := 0; i < len(args); i++ {
		a := args[i]
		// 同时支持 `--flag value` 与 `--flag=value` 两种形态（README 与习惯各异）。
		if key, val, ok := splitFlag(a); ok {
			switch key {
			case "--at":
				at = val
			case "--family":
				fam = val
			case "--json":
				// 单值场景下 JSON 与文本同形，保留旗标以保持契约一致。
			default:
				return usageErr(env, "unknown flag "+key)
			}
			continue
		}
		switch a {
		case "--at":
			if i+1 >= len(args) {
				return usageErr(env, "--at requires a path")
			}
			at = args[i+1]
			i++
		case "--family":
			if i+1 >= len(args) {
				return usageErr(env, "--family requires a value")
			}
			fam = args[i+1]
			i++
		case "--json":
		default:
			if strings.HasPrefix(a, "-") {
				return usageErr(env, "unknown flag "+a)
			}
			file = a
		}
	}
	if file == "" || at == "" || fam == "" {
		return usageErr(env, "normalize requires --at <path> --family <fam> <file>")
	}
	if hint := shellRewrotePath(at); hint != "" {
		return usageErr(env, hint)
	}
	f := value.Family(fam)
	if !f.Valid() {
		return usageErr(env, "unknown --family "+fam+" (want quantity|duration|decimal|bool|string|ratio)")
	}

	root, _, err := doc.ParseFile(file)
	if err != nil {
		return reportErr(env, err)
	}
	node := lookupPointer(root, at)
	if node == nil {
		return usageErr(env, "--at "+at+" matched no node in "+file)
	}
	c, err := value.Normalize(node, at, f, false)
	if err != nil {
		return reportErr(env, err)
	}
	fmt.Fprintf(env.Stdout, "%s\n", c.String())
	return exitcode.Equal
}

// splitFlag 拆 `--flag=value`；不含 `=` 或不是旗标时返回 ok=false。
func splitFlag(a string) (key, val string, ok bool) {
	if !strings.HasPrefix(a, "--") {
		return "", "", false
	}
	idx := strings.Index(a, "=")
	if idx < 0 {
		return "", "", false
	}
	return a[:idx], a[idx+1:], true
}

// shellRewrotePath 识别"参数被 shell 改写成了 Windows 绝对路径"这种事故。
//
// Git Bash / MSYS2 会把以 `/` 开头的参数当成 POSIX 路径转写成 `C:/...`。用户看到的
// 是"我明明写了 /limits/memory"，工具收到的却是 `D:/program/Git/limits/memory`。
// 静默失败最贵，所以这里显式解释原因并给出两条可用写法。
func shellRewrotePath(v string) string {
	if len(v) < 3 {
		return ""
	}
	isDrive := (v[0] >= 'A' && v[0] <= 'Z' || v[0] >= 'a' && v[0] <= 'z') && v[1] == ':' && (v[2] == '/' || v[2] == '\\')
	if !isDrive {
		return ""
	}
	return "path " + v + " looks like a Windows absolute path: your shell (Git Bash / MSYS2) " +
		"rewrote a leading '/' into a drive letter. Write the path without the leading slash " +
		"(e.g. `--at limits/memory`) or set MSYS2_ARG_CONV_EXCL=\"*\" for the invocation."
}

// ── diff / classify ────────────────────────────────────────────────────────

type diffFlags struct {
	profileName        string
	seqMode            string
	coerce             []value.CoerceRule
	strict             bool
	json               bool
	dumpNormalized     bool
	allowIndeterminate bool
	failOn             []classify.Class
	left, right        string
}

func parseDiffFlags(env Env, args []string, needFailOn bool) (diffFlags, exitcode.Code, bool) {
	var f diffFlags
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		// 支持 `--flag=value`；同时**不**对路径取值做任何 shell 补全假设。
		if key, val, ok := splitFlag(a); ok {
			switch key {
			case "--profile":
				f.profileName = val
			case "--seq-mode":
				f.seqMode = val
			case "--coerce-at":
				eq := strings.LastIndex(val, "=")
				if eq <= 0 {
					return f, usageErr(env, "--coerce-at wants <path-glob>=<family>, got "+val), false
				}
				fam := value.Family(val[eq+1:])
				if !fam.Valid() {
					return f, usageErr(env, "--coerce-at unknown family "+string(fam)), false
				}
				f.coerce = append(f.coerce, value.CoerceRule{Pattern: val[:eq], Family: fam})
			case "--fail-on":
				for _, item := range strings.Split(val, ",") {
					item = strings.TrimSpace(item)
					if item == "" {
						continue
					}
					c, ok := classify.ParseClass(item)
					if !ok {
						return f, usageErr(env, "unknown class "+item+" (want "+classList()+")"), false
					}
					f.failOn = append(f.failOn, c)
				}
			case "--strict":
				f.strict = true
			case "--json":
				f.json = true
			case "--dump-normalized":
				f.dumpNormalized = true
			case "--allow-indeterminate":
				f.allowIndeterminate = true
			default:
				return f, usageErr(env, "unknown flag "+key), false
			}
			continue
		}
		next := func(flag string) (string, bool) {
			if i+1 >= len(args) {
				usageErr(env, flag+" requires a value")
				return "", false
			}
			i++
			return args[i], true
		}
		switch {
		case a == "--profile":
			v, ok := next(a)
			if !ok {
				return f, exitcode.Usage, false
			}
			f.profileName = v
		case a == "--seq-mode":
			v, ok := next(a)
			if !ok {
				return f, exitcode.Usage, false
			}
			f.seqMode = v
		case a == "--coerce-at":
			v, ok := next(a)
			if !ok {
				return f, exitcode.Usage, false
			}
			eq := strings.LastIndex(v, "=")
			if eq <= 0 {
				return f, usageErr(env, "--coerce-at wants <path-glob>=<family>, got "+v), false
			}
			fam := value.Family(v[eq+1:])
			if !fam.Valid() {
				return f, usageErr(env, "--coerce-at unknown family "+string(fam)), false
			}
			f.coerce = append(f.coerce, value.CoerceRule{Pattern: v[:eq], Family: fam})
		case a == "--fail-on":
			v, ok := next(a)
			if !ok {
				return f, exitcode.Usage, false
			}
			for _, item := range strings.Split(v, ",") {
				item = strings.TrimSpace(item)
				if item == "" {
					continue
				}
				c, ok := classify.ParseClass(item)
				if !ok {
					return f, usageErr(env, "unknown class "+item+" (want "+classList()+")"), false
				}
				f.failOn = append(f.failOn, c)
			}
		case a == "--strict":
			f.strict = true
		case a == "--json":
			f.json = true
		case a == "--dump-normalized":
			f.dumpNormalized = true
		case a == "--allow-indeterminate":
			f.allowIndeterminate = true
		case strings.HasPrefix(a, "-"):
			return f, usageErr(env, "unknown flag "+a), false
		default:
			positional = append(positional, a)
		}
	}
	if needFailOn && len(f.failOn) == 0 {
		return f, usageErr(env, "gate requires --fail-on <class,...>"), false
	}
	if len(positional) != 2 {
		return f, usageErr(env, "expected exactly two input files, got "+itoa(len(positional))), false
	}
	f.left, f.right = positional[0], positional[1]
	return f, exitcode.Equal, true
}

func runDiff(env Env, args []string, forceClassify bool) exitcode.Code {
	f, code, ok := parseDiffFlags(env, args, false)
	if !ok {
		return code
	}
	return executeCompare(env, f, forceClassify, false)
}

func runGate(env Env, args []string) exitcode.Code {
	f, code, ok := parseDiffFlags(env, args, true)
	if !ok {
		return code
	}
	return executeCompare(env, f, true, true)
}

func executeCompare(env Env, f diffFlags, forceClassify, isGate bool) exitcode.Code {
	seq, err := diff.ParseSeqMode(f.seqMode)
	if err != nil {
		return usageErr(env, err.Error())
	}

	left, _, err := doc.ParseFile(f.left)
	if err != nil {
		return reportErr(env, err)
	}
	right, _, err := doc.ParseFile(f.right)
	if err != nil {
		return reportErr(env, err)
	}

	var prof *profile.Profile
	if f.profileName != "" {
		prof, err = profile.Load(f.profileName)
		if err != nil {
			return usageErr(env, err.Error())
		}
	}

	// 收集全部路径，用于 --coerce-at 未命中的用法错误（§4.8）。
	coercion := value.NewCoercion(f.coerce)
	if !coercion.Empty() {
		hit := coercion.MatchesAny(collectPaths(left, right))
		for _, r := range f.coerce {
			if !hit[r.Pattern] {
				return usageErr(env, "--coerce-at "+r.Pattern+" matched no node (silently ineffective rules are a bug)")
			}
		}
	}

	// 双侧施加默认值：相同默认值相互抵消（§4.11）。
	var applyRes profile.ApplyResult
	if prof != nil {
		l2, r2 := left.Clone(), right.Clone()
		la := profile.Apply(l2, prof)
		ra := profile.Apply(r2, prof)
		applyRes = profile.ApplyResult{Hits: la.Hits + ra.Hits, Degraded: la.Degraded && ra.Degraded}
		left, right = l2, r2
	}

	res, err := diff.Compare(left, right, diff.Options{Seq: seq, Coerce: coercion, Strict: f.strict})
	if err != nil {
		return reportErr(env, err)
	}
	diff.SortEntries(res.Entries)

	classified := classify.Apply(res.Entries)

	head := header(f, prof, applyRes, seq)
	if f.dumpNormalized {
		fmt.Fprintln(env.Stdout, head)
		fmt.Fprintln(env.Stdout, "--- normalized left ---")
		fmt.Fprint(env.Stdout, renderTree(left))
		fmt.Fprintln(env.Stdout, "--- normalized right ---")
		fmt.Fprint(env.Stdout, renderTree(right))
		fmt.Fprintln(env.Stdout, "--- entries ---")
	}

	if f.json {
		fmt.Fprintln(env.Stdout, renderJSON(head, classified, applyRes))
	} else {
		fmt.Fprintln(env.Stdout, head)
		fmt.Fprint(env.Stdout, renderText(classified))
	}

	// 退出码：3 已在上面的错误分支；此处 2 > 1 > 0。
	if classified.Summary.Indeterminate > 0 && !f.allowIndeterminate {
		return exitcode.Indeterminate
	}
	if isGate {
		if classify.Gate(classified.Summary, f.failOn, f.allowIndeterminate) {
			return exitcode.Behavioral
		}
		return exitcode.Equal
	}
	if forceClassify {
		if classified.Summary.Behavioral > 0 {
			return exitcode.Behavioral
		}
		return exitcode.Equal
	}
	if classified.Summary.Behavioral > 0 {
		return exitcode.Behavioral
	}
	return exitcode.Equal
}

func header(f diffFlags, prof *profile.Profile, ar profile.ApplyResult, seq diff.SeqMode) string {
	profName := "none"
	hits := "n/a"
	if prof != nil {
		profName = prof.Name
		hits = itoa(ar.Hits)
	}
	seqDesc := seq.Kind
	if seq.Kind == "keyed" {
		seqDesc = "keyed:" + seq.Field
	}
	parts := []string{
		"cfgdelta " + version.Value(),
		"profile " + profName + " (hits: " + hits + ")",
		"seq-mode " + seqDesc,
		"comments: not compared",
	}
	if prof != nil && ar.Degraded {
		parts = append(parts, "degraded: structural-only")
	}
	return strings.Join(parts, " · ")
}

func classList() string {
	names := make([]string, 0, len(classify.All))
	for _, c := range classify.All {
		names = append(names, string(c))
	}
	return strings.Join(names, ",")
}

// ── helpers ────────────────────────────────────────────────────────────────

func usageErr(env Env, msg string) exitcode.Code {
	fmt.Fprintf(env.Stderr, "cfgdelta: %s\n", msg)
	fmt.Fprintln(env.Stderr, "hint: run `cfgdelta --help`")
	return exitcode.Usage
}

func reportErr(env Env, err error) exitcode.Code {
	var ind *value.Indeterminate
	if errors.As(err, &ind) {
		fmt.Fprintf(env.Stderr, "cfgdelta: indeterminate (%s)\n", ind.Reason)
		if ind.Detail != "" {
			fmt.Fprintf(env.Stderr, "  detail: %s\n", ind.Detail)
		}
		return exitcode.Indeterminate
	}
	fmt.Fprintf(env.Stderr, "cfgdelta: %s\n", err.Error())
	return exitcode.Usage
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}

// lookupPointer 按 JSON Pointer 取值（只支持对象/数组下标）。
func lookupPointer(n *doc.Node, p string) *doc.Node {
	if n == nil || p == "" || p == "/" {
		return n
	}
	cur := n
	for _, seg := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		seg = strings.ReplaceAll(seg, "~1", "/")
		seg = strings.ReplaceAll(seg, "~0", "~")
		if cur == nil {
			return nil
		}
		switch cur.Kind {
		case doc.Mapping:
			cur = cur.Get(seg)
		case doc.Sequence:
			idx := -1
			for _, c := range seg {
				if c < '0' || c > '9' {
					idx = -1
					break
				}
			}
			if idx == -1 {
				num := 0
				ok := seg != ""
				for _, c := range seg {
					if c < '0' || c > '9' {
						ok = false
						break
					}
					num = num*10 + int(c-'0')
				}
				if !ok || num >= len(cur.Children) {
					return nil
				}
				cur = cur.Children[num]
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return cur
}

func collectPaths(left, right *doc.Node) []string {
	var out []string
	var walk func(p string, n *doc.Node)
	walk = func(p string, n *doc.Node) {
		if n == nil {
			return
		}
		out = append(out, p)
		switch n.Kind {
		case doc.Mapping:
			for i := 0; i+1 < len(n.Children); i += 2 {
				walk(value.JoinPointer(p, n.Children[i].Raw), n.Children[i+1])
			}
		case doc.Sequence:
			for i, c := range n.Children {
				walk(value.JoinPointer(p, itoa(i)), c)
			}
		}
	}
	walk("", left)
	walk("", right)
	sort.Strings(out)
	return out
}
