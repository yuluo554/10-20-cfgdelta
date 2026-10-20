package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuluo554/10-20-cfgdelta/internal/cli"
)

// TestReadmeCommandsAreRunnable 做两件事，都是干净环境里最贵的那族缺陷的拦截点：
//
//  1. **反射对账**：README 里每条 `cfgdelta ...` 命令都真喂给 CLI 的参数解析面。
//     任何"未知旗标 / 缺必选参数 / 未知子命令"（exit 3）都判红 —— 文档写的形态
//     parser 不认，是"读者照着敲就炸"的最短路径。
//  2. **可执行性**：命令里引用的**夹具文件**必须真在盘上（存在性，非内容）。
//
// 占位符（`left.yaml` / `right.yaml`）是**示例**，只做解析面检查、不做存在性检查；
// 这条豁免显式写在 PLACEHOLDERS 里，避免"把示例当实物"与"把实物当示例"两种误判。
func TestReadmeCommandsAreRunnable(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}

	commands := extractCfgdeltaCommands(string(data))
	if len(commands) < 3 {
		t.Fatalf("extracted %d cfgdelta commands from README — scan is vacuous (expected the tour block)", len(commands))
	}

	placeholders := map[string]bool{"left.yaml": true, "right.yaml": true, "a.yaml": true, "b.yaml": true}

	checked := 0
	for _, raw := range commands {
		args, files := splitCommand(raw, root)
		if len(args) == 0 {
			continue
		}
		// 词法里可能残留行尾 `; echo $?` 之类，已由 splitCommand 截断。
		if args[0] != "normalize" && args[0] != "diff" && args[0] != "classify" &&
			args[0] != "gate" && args[0] != "selftest" && args[0] != "--version" && args[0] != "--help" {
			t.Errorf("README command %q starts with an unknown subcommand", raw)
			continue
		}

		// ① 反射对账：只跑解析面。用不存在的占位文件也能暴露"旗标不认"。
		var out, errb strings.Builder
		code := cli.Run(cli.Env{Args: args, Stdout: &out, Stderr: &errb})
		if code == 3 && isParserRejection(errb.String()) {
			t.Errorf("README command is rejected by the parser (exit 3): %q\n  stderr: %s",
				raw, strings.TrimSpace(errb.String()))
		}

		// ② 夹具存在性（占位符豁免）。
		for _, f := range files {
			base := filepath.Base(f)
			if placeholders[base] {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, f)); err != nil {
				t.Errorf("README command references a file that does not exist: %q (in %q)", f, raw)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no README fixture paths were verified — the existence half is vacuous")
	}
	t.Logf("README: %d command(s) parsed, %d fixture reference(s) verified", len(commands), checked)
}

// isParserRejection 区分"参数面被拒"与"业务上的解析/用法错误"。
// 只关心前者：文档里的旗标/子命令形态必须被 parser 认。
func isParserRejection(stderr string) bool {
	for _, marker := range []string{
		"unknown flag", "unknown subcommand", "requires",
		"expected exactly two input files", "wants <path-glob>",
		"unknown class", "unknown --seq-mode", "unknown --family", "unknown profile",
	} {
		if strings.Contains(stderr, marker) {
			return true
		}
	}
	// "matched no node" / "no such file" 是**数据**问题（含占位符），不是参数面问题。
	return false
}

// extractCfgdeltaCommands 从 README 的围栏代码块里抽出以 cfgdelta 开头的命令行。
func extractCfgdeltaCommands(md string) []string {
	var out []string
	inFence := false
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		body := strings.TrimPrefix(trimmed, "$ ")
		if strings.HasPrefix(body, "cfgdelta ") {
			out = append(out, body)
		}
	}
	return out
}

// splitCommand 做最小的 shell 分词：处理引号、去掉 `; echo $?` 之类的尾缀，
// 并把看起来像**输入文件**的参数原样返回（供存在性检查）。
func splitCommand(raw, root string) (args []string, files []string) {
	// 截断到第一个未加引号的 `;` 或 `|`。
	var cleaned strings.Builder
	inSingle, inDouble := false, false
	for _, r := range raw {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ';' || r == '|') && !inSingle && !inDouble:
			goto done
		}
		cleaned.WriteRune(r)
	}
done:

	var cur strings.Builder
	inSingle, inDouble = false, false
	flush := func() {
		if cur.Len() > 0 {
			args = append(args, cur.String())
			cur.Reset()
		}
	}
	for _, r := range cleaned.String() {
		switch {
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()

	// 去掉可执行名本身（`cfgdelta` 或 `./cfgdelta` 或 `./dist/cfgdelta`）。
	if len(args) > 0 && strings.HasSuffix(args[0], "cfgdelta") {
		args = args[1:]
	}
	// 文件参数：含 `/` 或以 .yaml/.json 结尾，且不是旗标值。
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if isFlagValueAt(args, i) {
			continue
		}
		if strings.Contains(a, "/") || strings.HasSuffix(a, ".yaml") || strings.HasSuffix(a, ".json") {
			files = append(files, a)
		}
	}
	return args, files
}

func isFlagValueAt(args []string, i int) bool {
	if i == 0 {
		return false
	}
	switch args[i-1] {
	case "--profile", "--seq-mode", "--coerce-at", "--fail-on", "--at", "--family":
		return true
	}
	return false
}
