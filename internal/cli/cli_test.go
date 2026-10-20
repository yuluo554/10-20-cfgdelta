package cli

import (
	"strings"
	"testing"

	"github.com/yuluo554/10-20-cfgdelta/internal/exitcode"
)

func run(args ...string) (exitcode.Code, string, string) {
	var out, errb writerBuffer
	code := Run(Env{Args: args, Stdout: &out, Stderr: &errb})
	return code, out.String(), errb.String()
}

// 退出码契约（§4.12）：0/1/2/3 逐位断言，且 3 优先级最高。
func TestExitCodeContract(t *testing.T) {
	root := "../.."
	cases := []struct {
		name string
		args []string
		want exitcode.Code
	}{
		{"version", []string{"--version"}, exitcode.Equal},
		{"help", []string{"--help"}, exitcode.Equal},
		{"unknown subcommand", []string{"nope"}, exitcode.Usage},
		{"missing positional", []string{"diff", root + "/testdata/golden/testA/k1.yaml"}, exitcode.Usage},
		{"missing file", []string{"diff", root + "/testdata/golden/testA/nope.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Usage},
		{"unknown profile", []string{"diff", "--profile", "k8s/v0.0", root + "/testdata/golden/testA/k1.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Usage},
		{"bad seq mode", []string{"diff", "--seq-mode", "fuzzy", root + "/testdata/golden/testA/k1.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Usage},
		{"gate without fail-on", []string{"gate", root + "/testdata/golden/testA/k1.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Usage},
		{"unknown class", []string{"gate", "--fail-on", "bogus", root + "/testdata/golden/testA/k1.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Usage},
		{"coerce miss", []string{"diff", "--coerce-at", "/nope/*=quantity", root + "/testdata/golden/testA/k1.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Usage},
		{"equivalent", []string{"diff", "--profile", "k8s/v1.29", root + "/testdata/golden/testA/k1.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Equal},
		{"behavioral", []string{"diff", root + "/testdata/golden/testA/k1.yaml", root + "/testdata/golden/testA/k2.yaml"}, exitcode.Behavioral},
		{"indeterminate", []string{"diff", "--coerce-at", "/limits/*=quantity", root + "/testdata/golden/testC/c09-left.yaml", root + "/testdata/golden/testC/c09-right.yaml"}, exitcode.Indeterminate},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, out, errs := run(c.args...)
			if code != c.want {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, c.want, out, errs)
			}
		})
	}
}

// 优先级 3 > 2 > 1：未知类 + indeterminate 输入同时成立时必须是 3。
func TestExitCodePriority(t *testing.T) {
	code, _, _ := run("gate", "--fail-on", "bogus",
		"../../testdata/golden/testC/c09-left.yaml",
		"../../testdata/golden/testC/c09-right.yaml")
	if code != exitcode.Usage {
		t.Fatalf("exit = %d, want 3 (usage error outranks indeterminate)", code)
	}
}

// gate 选择性阻断：未列出的类不阻断，但仍要打印。
func TestGateSelectiveBlocking(t *testing.T) {
	blockCode, out, _ := run("gate", "--fail-on", "资源·收缩",
		"../../testdata/golden/testGate/shrink-left.yaml",
		"../../testdata/golden/testGate/shrink-right.yaml")
	if blockCode != exitcode.Behavioral {
		t.Fatalf("listed class present: exit = %d, want 1", blockCode)
	}
	if !strings.Contains(out, "资源·收缩") {
		t.Fatalf("output must name the class:\n%s", out)
	}

	passCode, passOut, _ := run("gate", "--fail-on", "接口·暴露",
		"../../testdata/golden/testGate/shrink-left.yaml",
		"../../testdata/golden/testGate/shrink-right.yaml")
	if passCode != exitcode.Equal {
		t.Fatalf("unlisted class: exit = %d, want 0", passCode)
	}
	// 不阻断 ≠ 不输出：行为差异仍要打印出来供人读。
	if !strings.Contains(passOut, "资源·收缩") {
		t.Fatalf("unlisted behavioural differences must still be printed:\n%s", passOut)
	}
}

// 纯书写只计数、不逐条列（§4.13）。
func TestCosmeticIsCountedNotListed(t *testing.T) {
	code, out, _ := run("diff", "../../testdata/golden/testA/k1.yaml", "../../testdata/golden/testA/k2.yaml")
	if code != exitcode.Behavioral {
		t.Fatalf("exit = %d", code)
	}
	if strings.Contains(out, "memory") && strings.Contains(out, "cosmetic") {
		// memory 是纯书写（1Gi vs 1024Mi），不应作为条目行出现
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "~") && strings.Contains(line, "memory") {
				t.Fatalf("cosmetic entry was listed individually: %q", line)
			}
		}
	}
	if !strings.Contains(out, "cosmetic 2") {
		t.Fatalf("expected cosmetic count in summary:\n%s", out)
	}
}

// 输出确定性：同输入两次运行逐字节一致（K-02）。
func TestOutputIsDeterministic(t *testing.T) {
	args := []string{"classify", "../../testdata/golden/testA/k1.yaml", "../../testdata/golden/testA/k2.yaml"}
	_, out1, err1 := run(args...)
	_, out2, err2 := run(args...)
	if out1 != out2 || err1 != err2 {
		t.Fatal("two runs of the same input produced different output")
	}
}

// --dump-normalized 必须让人看见双侧默认值同时出现（测试A 的卖点）。
func TestDumpNormalizedShowsBothSides(t *testing.T) {
	_, out, _ := run("diff", "--profile", "k8s/v1.29", "--dump-normalized",
		"../../testdata/golden/testA/k1.yaml", "../../testdata/golden/testA/k2.yaml")
	if !strings.Contains(out, "normalized left") || !strings.Contains(out, "normalized right") {
		t.Fatalf("--dump-normalized must print both trees:\n%s", out)
	}
	// 左侧原本没有 imagePullPolicy，施加默认值后必须出现。
	parts := strings.Split(out, "--- normalized right ---")
	if len(parts) != 2 {
		t.Fatalf("unexpected dump layout:\n%s", out)
	}
	if !strings.Contains(parts[0], "imagePullPolicy") || !strings.Contains(parts[1], "imagePullPolicy") {
		t.Fatalf("both sides must show imagePullPolicy after defaulting:\n%s", out)
	}
}

// normalize 子命令输出精确有理数（不经过浮点）。
func TestNormalizeSubcommand(t *testing.T) {
	code, out, errs := run("normalize", "--at", "/limits/memory", "--family", "quantity",
		"../../testdata/golden/testC/c01-left.yaml")
	if code != exitcode.Equal {
		t.Fatalf("exit = %d, stderr = %s", code, errs)
	}
	if strings.TrimSpace(out) != "quantity:1073741824/1" {
		t.Fatalf("normalize output = %q, want exact rational form", out)
	}
}

// 前导 `/` 可省略（Git Bash / MSYS2 会把 `/x` 改写成 `C:/...`）。
// 两种写法必须给出同一结果——这是干净 clone 实测抓出的可用性缺陷的回归。
func TestAtPathLeadingSlashOptional(t *testing.T) {
	for _, at := range []string{"/limits/memory", "limits/memory"} {
		code, out, errs := run("normalize", "--at", at, "--family", "quantity",
			"../../testdata/golden/testC/c01-left.yaml")
		if code != exitcode.Equal {
			t.Fatalf("--at %q: exit = %d, stderr = %s", at, code, errs)
		}
		if strings.TrimSpace(out) != "quantity:1073741824/1" {
			t.Fatalf("--at %q: output = %q", at, out)
		}
	}
}

// `--flag=value` 形态必须与 `--flag value` 等价（README 与习惯各异）。
func TestEqualsFlagForm(t *testing.T) {
	code, out, errs := run("normalize", "--at=limits/memory", "--family=quantity",
		"../../testdata/golden/testC/c01-left.yaml")
	if code != exitcode.Equal {
		t.Fatalf("exit = %d, stderr = %s", code, errs)
	}
	if strings.TrimSpace(out) != "quantity:1073741824/1" {
		t.Fatalf("output = %q", out)
	}

	code, _, _ = run("diff", "--profile=k8s/v1.29",
		"../../testdata/golden/testA/k1.yaml", "../../testdata/golden/testA/k2.yaml")
	if code != exitcode.Equal {
		t.Fatalf("--profile=k8s/v1.29 exit = %d, want 0", code)
	}
}

// shell 把 `/limits/memory` 改写成 `D:/...` 时，必须给出**解释性**错误而不是
// 含糊的 "matched no node"——静默失败最贵。
func TestShellRewrittenPathIsExplained(t *testing.T) {
	code, _, errs := run("normalize", "--at", "D:/program/Git/limits/memory", "--family", "quantity",
		"../../testdata/golden/testC/c01-left.yaml")
	if code != exitcode.Usage {
		t.Fatalf("exit = %d, want 3", code)
	}
	if !strings.Contains(errs, "Windows absolute path") || !strings.Contains(errs, "leading slash") {
		t.Fatalf("stderr must explain the shell rewrite:\n%s", errs)
	}
}

// JSON 输出是机读面且确定性（不用 map 迭代）。
func TestJSONOutput(t *testing.T) {
	code, out, _ := run("classify", "--json",
		"../../testdata/golden/testGate/shrink-left.yaml",
		"../../testdata/golden/testGate/shrink-right.yaml")
	if code != exitcode.Behavioral {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{`"summary"`, `"entries"`, `"class": "资源·收缩"`, `"behavioral": 2`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json output missing %s:\n%s", want, out)
		}
	}
}
