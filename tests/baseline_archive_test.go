package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// baselineTools 是 testdata/baseline 下必须存在的工具目录（data/README §1.2 的三家）。
var baselineTools = []string{"dyff", "diffyml", "confdiff"}

// baselineRequiredFiles 是每个工具目录的"四件"（data/README §1.2 复现纪律）。
var baselineRequiredFiles = []string{"VERSION.txt", "COMMANDS.txt", "exit_code.txt"}

// TestBaselineArchiveIsComplete 守 data/README.md §1.2 的留档纪律。
//
// 它拦的是三类"看起来有档、其实复验不了"的形态：
//  1. 工具目录缺件（少 VERSION.txt / COMMANDS.txt / exit_code.txt / 一个 .out 都没有）；
//  2. COMMANDS.txt 里登记的 case 与 exit_code.txt、`<case>.out` **三方不成对**
//     —— 只登记命令却没落输出，是最常见的"声称跑过"；
//  3. VERSION.txt 是空的或没有版本号 —— 版本非空自证。
//
// 注意：本门**不要求本机装有这三家工具**（它们在仓库外，不入库），只校验留档自身
// 的自洽性与版本可读性。重跑比对是人工动作，见 testdata/baseline/README.md §五。
func TestBaselineArchiveIsComplete(t *testing.T) {
	root := repoRoot(t)

	for _, tool := range baselineTools {
		tool := tool
		t.Run(tool, func(t *testing.T) {
			dir := filepath.Join(root, "testdata", "baseline", tool)
			if _, err := os.Stat(dir); err != nil {
				t.Fatalf("baseline dir missing for %q: %v", tool, err)
			}

			// ① 四件齐全（.out 按"至少一个"计，逐 case 的完整性由 ② 覆盖）。
			for _, name := range baselineRequiredFiles {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Errorf("%s/%s missing", tool, name)
				}
			}
			outs, err := filepath.Glob(filepath.Join(dir, "*.out"))
			if err != nil {
				t.Fatalf("glob %s/*.out: %v", tool, err)
			}
			if len(outs) == 0 {
				t.Fatalf("%s has no *.out capture — archive without output is not evidence", tool)
			}

			// ③ 版本非空自证：必须回显出一个数字版本号。
			version := readTrimmed(t, filepath.Join(dir, "VERSION.txt"))
			if version == "" {
				t.Errorf("%s/VERSION.txt is empty", tool)
			}
			if !regexp.MustCompile(`\d+\.\d+`).MatchString(version) {
				t.Errorf("%s/VERSION.txt has no version number: %q", tool, version)
			}

			// ② 三方对账：COMMANDS.txt 的 case 集合 == exit_code.txt 的 case 集合，
			//    且每个 case 都有 <case>.out。
			cmdCases := parseCommandCases(t, filepath.Join(dir, "COMMANDS.txt"))
			rcCases := parseExitCodes(t, filepath.Join(dir, "exit_code.txt"))

			if len(cmdCases) == 0 {
				t.Fatalf("%s/COMMANDS.txt lists no cases — scan is vacuous", tool)
			}

			for _, c := range cmdCases {
				if _, ok := rcCases[c]; !ok {
					t.Errorf("%s: case %q is in COMMANDS.txt but has no exit code", tool, c)
				}
				// `version` 的原样输出就是 VERSION.txt（版本非空自证另有一条），
				// 不重复落一份 version.out。
				if c == "version" {
					continue
				}
				outPath := filepath.Join(dir, c+".out")
				info, err := os.Stat(outPath)
				if err != nil {
					t.Errorf("%s: case %q has no raw output file %s.out", tool, c, c)
					continue
				}
				if info.Size() == 0 {
					t.Errorf("%s: case %q output is empty", tool, c)
				}
			}
			for c := range rcCases {
				if !containsString(cmdCases, c) {
					t.Errorf("%s: case %q has an exit code but no command line in COMMANDS.txt", tool, c)
				}
			}

			// 每条登记命令都必须点名工具本身（避免把别的工具的调用混进来）。
			for _, line := range strings.Split(readTrimmed(t, filepath.Join(dir, "COMMANDS.txt")), "\n") {
				if strings.TrimSpace(line) == "" {
					continue
				}
				if !strings.Contains(line, "cmd="+tool+" ") {
					t.Errorf("%s/COMMANDS.txt line does not invoke %q: %q", tool, tool, line)
				}
			}
		})
	}
}

// TestBaselineCoversEveryDocumentedCase 把 data/README §1.2 点名的用例与留档对齐：
// 测试A/B/C(9 组)/D(TOML+HCL) 必须都能在留档里找到对应用例，防止"只留了一半"。
func TestBaselineCoversEveryDocumentedCase(t *testing.T) {
	root := repoRoot(t)

	// requireCase[进具名约定] = 至少一个工具留了它。
	required := []struct {
		name  string
		label string
	}{
		{"A-dyff", "测试A（K8s 默认值 + 数量）"},
		{"B-dyff", "测试B（Helm values 跨环境）"},
		{"C01-dyff", "测试C 第 1 组 1Gi/1024Mi"},
		{"C02-dyff", "测试C 第 2 组 1000M/1G"},
		{"C03-dyff", "测试C 第 3 组 8000m/8"},
		{"C04-dyff", "测试C 第 4 组 90s/1m30s"},
		{"C05-dyff", "测试C 第 5 组 1h/60m"},
		{"C06-dyff", "测试C 第 6 组 1e3/1000"},
		{"C07-dyff", "测试C 第 7 组 1.50/1.5"},
		{"C08-dyff", "测试C 第 8 组反例 1Gi/1000M"},
		{"C09-dyff", "测试C 第 9 组歧义 1MiB/1MB"},
		{"D-toml-dyff", "测试D TOML"},
		{"D-hcl-dyff", "测试D HCL"},
	}

	dyffDir := filepath.Join(root, "testdata", "baseline", "dyff")
	present := map[string]bool{}
	outs, _ := filepath.Glob(filepath.Join(dyffDir, "*.out"))
	for _, p := range outs {
		present[strings.TrimSuffix(filepath.Base(p), ".out")] = true
	}

	for _, r := range required {
		if !present[r.name] {
			t.Errorf("baseline is missing case %q (%s) — data/README §1.2 names it explicitly", r.name, r.label)
		}
	}
}

func parseCommandCases(t *testing.T, path string) []string {
	t.Helper()
	var cases []string
	for _, line := range strings.Split(readTrimmed(t, path), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.Index(line, "case=")
		if i < 0 {
			t.Errorf("%s: line has no case= prefix: %q", path, line)
			continue
		}
		rest := line[i+len("case="):]
		j := strings.IndexAny(rest, " \t")
		if j < 0 {
			cases = append(cases, rest)
		} else {
			cases = append(cases, rest[:j])
		}
	}
	return cases
}

func parseExitCodes(t *testing.T, path string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, line := range strings.Split(readTrimmed(t, path), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			t.Errorf("%s: malformed line %q (want case=rc)", path, line)
			continue
		}
		out[parts[0]] = parts[1]
		if _, err := os.Stat(path); err == nil {
			// rc 必须是整数
			if !regexp.MustCompile(`^\d+$`).MatchString(parts[1]) {
				t.Errorf("%s: exit code for %q is not an integer: %q", path, parts[0], parts[1])
			}
		}
	}
	return out
}

func readTrimmed(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.TrimSpace(string(b))
}

func containsString(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
