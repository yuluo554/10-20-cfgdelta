// Package selftest 跑打包在仓库里的 golden 套件（plan/03 §六）。
//
// 纪律：不联网、不调外部命令、不读时钟；golden 夹具随仓库分发。
// 夹具未落盘时**显式 skip 并说明原因**，绝不假装通过。
package selftest

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/exitcode"
)

// Case 是 manifest.json 中的一条用例（真值只存输入与期望，K-08）。
//
// Contains 是可选的**输出面**断言：退出码相同、计数也相同，仍可能把路径打错
// （例如 set 模式的成员标识、RFC 6901 转义、keyed 重复键的定位）。只对期望成立
// 的字面串断言，不存整段观测输出。
type Case struct {
	Case          string   `json:"case"`
	Args          []string `json:"args"`
	Exit          int      `json:"exit"`
	Behavioral    int      `json:"behavioral"`
	Cosmetic      int      `json:"cosmetic"`
	Indeterminate []string `json:"indeterminate"`
	Contains      []string `json:"contains,omitempty"`
	Note          string   `json:"note,omitempty"`
}

// Manifest 是 golden 清单。
type Manifest struct {
	GeneratedBy string `json:"generated_by"`
	ScopeNote   string `json:"scope_note"`
	Cases       []Case `json:"cases"`
}

// Runner 抽象"跑一条用例"，由 cli 注入以避免 import 环。
type Runner func(args []string) (exit int, stdout string)

// Run 执行 selftest 子命令。
func Run(stdout, stderr io.Writer, root string, args []string) exitcode.Code {
	verbose := false
	for _, a := range args {
		switch a {
		case "--verbose":
			verbose = true
		case "", "--json":
		default:
			fmt.Fprintf(stderr, "cfgdelta: selftest: unknown flag %s\n", a)
			return exitcode.Usage
		}
	}

	base, err := locateGolden(root)
	if err != nil {
		fmt.Fprintf(stderr, "cfgdelta: selftest: %v\n", err)
		return exitcode.Usage
	}

	mfPath := filepath.Join(base, "manifest.json")
	data, err := os.ReadFile(mfPath)
	if err != nil {
		fmt.Fprintf(stderr, "cfgdelta: selftest: golden manifest not found at %s\n", mfPath)
		fmt.Fprintln(stderr, "  hint: golden fixtures land in M1; run `node tools/gen-golden-manifest.mjs`")
		return exitcode.Usage
	}
	var mf Manifest
	if err := json.Unmarshal(data, &mf); err != nil {
		fmt.Fprintf(stderr, "cfgdelta: selftest: manifest is not valid JSON: %v\n", err)
		return exitcode.Usage
	}
	if len(mf.Cases) == 0 {
		fmt.Fprintln(stdout, "selftest: manifest lists 0 cases — nothing was proven (positive control failed)")
		return exitcode.Behavioral
	}

	fmt.Fprintf(stdout, "selftest: %d golden cases from %s\n", len(mf.Cases), filepath.ToSlash(base))
	if mf.ScopeNote != "" {
		fmt.Fprintf(stdout, "scope: %s\n", mf.ScopeNote)
	}

	// exec 由调用方注入，避免 selftest → cli → selftest 的循环 import。
	exec := currentRunner
	if exec == nil {
		fmt.Fprintln(stderr, "cfgdelta: selftest: no runner wired")
		return exitcode.Usage
	}

	pass, fail, skip := 0, 0, 0
	names := make([]string, 0, len(mf.Cases))
	for _, c := range mf.Cases {
		names = append(names, c.Case)
	}
	sort.Strings(names)
	byName := make(map[string]Case, len(mf.Cases))
	for _, c := range mf.Cases {
		byName[c.Case] = c
	}

	for _, name := range names {
		c := byName[name]
		if strings.HasPrefix(c.Note, "pending:") {
			skip++
			fmt.Fprintf(stdout, "SKIP  %s  (%s)\n", c.Case, strings.TrimPrefix(c.Note, "pending:"))
			continue
		}
		code, out := exec(resolveArgs(base, c.Args))
		if code != c.Exit {
			fail++
			fmt.Fprintf(stdout, "FAIL  %s  exit %d want %d\n", c.Case, code, c.Exit)
			if verbose {
				fmt.Fprintf(stdout, "      args: %v\n      out: %s\n", c.Args, strings.TrimSpace(out))
			}
			continue
		}
		// 退出码只是代理信号：同类退出码可以掩盖"条目数全错"。
		// 因此对带 summary 的输出**再对账一次计数**（K-08 的真值必须被真读）。
		if reason, bad := checkSummary(c, out); bad {
			fail++
			fmt.Fprintf(stdout, "FAIL  %s  exit ok (%d) but %s\n", c.Case, code, reason)
			if verbose {
				fmt.Fprintf(stdout, "      args: %v\n      out: %s\n", c.Args, strings.TrimSpace(out))
			}
			continue
		}
		// 输出面断言：路径转义 / 集合成员标识 / 重复键定位这类事实，计数对账看不出来。
		if reason, bad := checkContains(c, out); bad {
			fail++
			fmt.Fprintf(stdout, "FAIL  %s  exit ok (%d) but %s\n", c.Case, code, reason)
			if verbose {
				fmt.Fprintf(stdout, "      args: %v\n      out: %s\n", c.Args, strings.TrimSpace(out))
			}
			continue
		}
		pass++
		if verbose {
			fmt.Fprintf(stdout, "PASS  %s  exit %d\n", c.Case, code)
		}
	}
	fmt.Fprintf(stdout, "selftest: %d passed, %d failed, %d skipped\n", pass, fail, skip)
	if fail > 0 {
		return exitcode.Behavioral
	}
	return exitcode.Equal
}

// currentRunner 由 cli 在 init 时注入（唯一 import 方向：cli → selftest）。
var currentRunner Runner

// SetRunner 由 cli 调用。
func SetRunner(r Runner) { currentRunner = r }

// checkSummary 解析输出里的 `summary:` 行并与 manifest 的期望计数对账。
//
// 对账规则：
//   - 输出里**没有 summary 行**（如 --version / 用法错误）→ 视为不适用，通过；
//   - 有 summary 行 → behavioral / cosmetic / indeterminate 三项必须逐项相等。
func checkSummary(c Case, out string) (string, bool) {
	var line string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "summary:") {
			line = l
			break
		}
	}
	if line == "" {
		if c.Behavioral != 0 || c.Cosmetic != 0 || len(c.Indeterminate) != 0 {
			return "manifest expects counts but output has no summary line (engine did not run?)", true
		}
		return "", false
	}
	got := parseSummary(line)
	if got.behavioral != c.Behavioral {
		return fmt.Sprintf("behavioral=%d want %d (%s)", got.behavioral, c.Behavioral, strings.TrimSpace(line)), true
	}
	if got.cosmetic != c.Cosmetic {
		return fmt.Sprintf("cosmetic=%d want %d (%s)", got.cosmetic, c.Cosmetic, strings.TrimSpace(line)), true
	}
	if got.indeterminate != len(c.Indeterminate) {
		return fmt.Sprintf("indeterminate=%d want %d (%s)", got.indeterminate, len(c.Indeterminate), strings.TrimSpace(line)), true
	}
	return "", false
}

// checkContains 逐条断言输出里出现期望片段（stdout 与 stderr 已由 runner 合并）。
//
// 规则：manifest 没写 contains → 不适用；写了 → 每一条都必须出现。
func checkContains(c Case, out string) (string, bool) {
	for _, want := range c.Contains {
		if !strings.Contains(out, want) {
			return fmt.Sprintf("output does not contain %q", want), true
		}
	}
	return "", false
}

type summaryCounts struct{ behavioral, cosmetic, indeterminate int }

// parseSummary 解析形如
// `summary: total 3 · behavioral 0 · cosmetic 3 · indeterminate 0`。
func parseSummary(line string) summaryCounts {
	var s summaryCounts
	for _, field := range []struct {
		key string
		dst *int
	}{
		{"behavioral", &s.behavioral},
		{"cosmetic", &s.cosmetic},
		{"indeterminate", &s.indeterminate},
	} {
		idx := strings.Index(line, field.key+" ")
		if idx < 0 {
			continue
		}
		rest := line[idx+len(field.key)+1:]
		num := 0
		digits := 0
		for _, r := range rest {
			if r < '0' || r > '9' {
				break
			}
			num = num*10 + int(r-'0')
			digits++
		}
		if digits > 0 {
			*field.dst = num
		}
	}
	return s
}

// resolveArgs 把 manifest 里的**相对夹具路径**解析成相对 base 的绝对路径。
// 只有"看起来像夹具路径且文件真实存在"的参数才改写；旗标与取值原样保留
// （否则 `--profile k8s/v1.29` 会被误当成文件）。
func resolveArgs(base string, args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		if strings.HasPrefix(a, "-") || strings.HasPrefix(a, "/") || strings.Contains(a, "=") {
			continue
		}
		if !strings.Contains(a, ".") {
			continue
		}
		if isFlagValue(args, i) {
			continue
		}
		candidate := filepath.Join(base, filepath.FromSlash(a))
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			out[i] = candidate
		}
	}
	return out
}

// isFlagValue 判定 args[i] 是否为某个旗标的取值。
func isFlagValue(args []string, i int) bool {
	if i == 0 {
		return false
	}
	prev := args[i-1]
	switch prev {
	case "--profile", "--seq-mode", "--coerce-at", "--fail-on", "--at", "--family":
		return true
	}
	return false
}

// locateGolden 依次尝试：--root、CWD 及上溯、可执行文件所在目录上溯。
func locateGolden(root string) (string, error) {
	candidates := []string{}
	if root != "" {
		candidates = append(candidates, filepath.Join(root, "testdata", "golden"))
	}
	if wd, err := os.Getwd(); err == nil {
		for p := wd; ; p = filepath.Dir(p) {
			candidates = append(candidates, filepath.Join(p, "testdata", "golden"))
			if filepath.Dir(p) == p {
				break
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		for p := filepath.Dir(exe); ; p = filepath.Dir(p) {
			candidates = append(candidates, filepath.Join(p, "testdata", "golden"))
			if filepath.Dir(p) == p {
				break
			}
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("testdata/golden not found (searched %d locations from CWD / executable)", len(candidates))
}
