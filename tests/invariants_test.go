// Package tests 存放跨包的不变量守门（不是单元测试）。
//
// 这里的两条是**可证伪的门**：
//   - K-01 浮点禁令：比较路径上不允许出现 float32/float64 类型；
//   - 版本单一来源：仓库根 VERSION 与 internal/version/VERSION 必须逐字节一致。
package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoFloatInComparisonPath 静态拦截"浮点参与相等判定"（K-01 / §4.9）。
//
// 判据：`internal/` 下任何非测试文件都**不得**出现 float32/float64 标识符
// （类型声明、转换、字面量除外——本项目根本不用浮点，一刀切更安全且更易审计）。
//
// 变异验证：在 internal/value/canonical.go 里插入 `var _ float64` → 本测试必须红。
func TestNoFloatInComparisonPath(t *testing.T) {
	root := repoRoot(t)
	pkgs, err := filepath.Glob(filepath.Join(root, "internal", "*"))
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	offenders := []string{}
	for _, dir := range pkgs {
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			continue
		}
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			scanned++
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, f, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", f, err)
			}
			ast.Inspect(parsed, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				if id.Name == "float32" || id.Name == "float64" {
					pos := fset.Position(id.Pos())
					rel, _ := filepath.Rel(root, pos.Filename)
					offenders = append(offenders, rel+":"+itoa(pos.Line))
				}
				return true
			})
		}
	}
	// 非空跑自证：扫到 0 个文件时这条判据什么都没证明。
	if scanned == 0 {
		t.Fatal("scanned 0 non-test files under internal/ — the guard is vacuous")
	}
	if len(offenders) > 0 {
		t.Fatalf("float types are forbidden on the comparison path (K-01); found %d: %s",
			len(offenders), strings.Join(offenders, ", "))
	}
	t.Logf("float ban: scanned %d non-test files under internal/ — 0 float identifiers", scanned)
}

// TestVersionSingleSource 断言 VERSION 与 internal/version/VERSION 逐字节一致（K-03）。
//
// 变异验证：改 internal/version/VERSION 一个字节 → 本测试必须红。
func TestVersionSingleSource(t *testing.T) {
	root := repoRoot(t)
	rootVer, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	mirrorVer, err := os.ReadFile(filepath.Join(root, "internal", "version", "VERSION"))
	if err != nil {
		t.Fatalf("read internal/version/VERSION: %v", err)
	}
	if string(rootVer) != string(mirrorVer) {
		t.Fatalf("version drift: VERSION=%q mirror=%q (run `node tools/sync-version.mjs`)",
			strings.TrimSpace(string(rootVer)), strings.TrimSpace(string(mirrorVer)))
	}
	if strings.TrimSpace(string(rootVer)) == "" {
		t.Fatal("VERSION is empty")
	}
}

// TestProfilesMirrored 断言 data/profiles/ 与内嵌镜像逐字节一致。
//
// 变异验证：改 data/profiles/k8s/v1.29.json 而不跑同步 → 本测试必须红。
func TestProfilesMirrored(t *testing.T) {
	root := repoRoot(t)
	src := filepath.Join(root, "data", "profiles")
	dst := filepath.Join(root, "internal", "profile", "profiles")

	var srcFiles []string
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, ".json") {
			srcFiles = append(srcFiles, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(srcFiles) == 0 {
		t.Fatal("data/profiles/ contains 0 JSON files — the guard is vacuous")
	}
	for _, s := range srcFiles {
		rel, err := filepath.Rel(src, s)
		if err != nil {
			t.Fatal(err)
		}
		srcBytes, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		dstBytes, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatalf("mirror missing for %s: %v (run `node tools/sync-profiles.mjs`)", rel, err)
		}
		if string(srcBytes) != string(dstBytes) {
			t.Fatalf("profile drift at %s — source and embedded mirror differ", rel)
		}
	}
	t.Logf("profile mirror: %d file(s) byte-identical", len(srcFiles))
}

// TestWorkflowParses 在本地解析 CI workflow 并断言 job/step 数。
//
// 为什么必须有这条：YAML 非法时 GitHub 连 job 都不建（run 显示 failure 但 jobs=0），
// 而文件字节、workflow state 都看起来正常。"推之前先在本地解析"是唯一便宜的拦截点。
//
// M6 起同时钉住发布面的形状（K-11：判据有几条，变异就覆盖几条）：
// 三平台产物名齐、publish 真跑 sha256sum 且用 gh release create 上传附件。
// 变异验证：删一条 matrix include / 把 sha256sum 换成 md5sum / 把 publish 的
// permissions 改回只读 → 本测试必须红。
func TestWorkflowParses(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	var doc struct {
		Jobs map[string]struct {
			If          string            `yaml:"if"`
			Needs       string            `yaml:"needs"`
			Permissions map[string]string `yaml:"permissions"`
			Strategy    struct {
				Matrix struct {
					Include []struct {
						OS    string `yaml:"os"`
						Asset string `yaml:"asset"`
					} `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
				Uses string `yaml:"uses"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("workflow YAML is invalid (GitHub would create 0 jobs): %v", err)
	}
	if len(doc.Jobs) != 5 {
		t.Fatalf("job count = %d, want 5 (layout / consistency / test / release-build / publish)", len(doc.Jobs))
	}
	for _, required := range []string{"layout", "consistency", "test", "release-build", "publish"} {
		if _, ok := doc.Jobs[required]; !ok {
			t.Fatalf("required job %q is missing", required)
		}
	}
	// 布局门必须在 CI 里有一条步骤（方法论装订的硬要求）。
	layoutSteps := doc.Jobs["layout"].Steps
	sawGate := false
	for _, s := range layoutSteps {
		if strings.Contains(s.Run, "check-sprint-layout.mjs") {
			sawGate = true
		}
	}
	if !sawGate {
		t.Fatal("the layout gate has no CI step — it would only exist on the developer's machine")
	}

	// 发布面：三平台原生构建必须都在 matrix 里，且产物名与 Release 附件一致。
	release := doc.Jobs["release-build"]
	if !strings.Contains(release.If, "refs/tags/v") {
		t.Fatal("release-build must be gated on v* tags — VERSION==tag is only meaningful there")
	}
	wantAssets := map[string]string{
		"ubuntu-latest":  "cfgdelta-linux-amd64",
		"macos-latest":   "cfgdelta-darwin-arm64",
		"windows-latest": "cfgdelta-windows-amd64.exe",
	}
	sawAssets := map[string]bool{}
	for _, inc := range release.Strategy.Matrix.Include {
		if want, ok := wantAssets[inc.OS]; ok && inc.Asset == want {
			sawAssets[inc.Asset] = true
		}
	}
	for _, asset := range wantAssets {
		if !sawAssets[asset] {
			t.Fatalf("release matrix is missing the native asset %q", asset)
		}
	}

	publish := doc.Jobs["publish"]
	if publish.Needs != "release-build" {
		t.Fatalf("publish needs = %q, want release-build", publish.Needs)
	}
	if publish.Permissions["contents"] != "write" {
		t.Fatalf("publish permissions.contents = %q, want write (uploading assets needs it)", publish.Permissions["contents"])
	}
	var publishRuns strings.Builder
	for _, s := range publish.Steps {
		publishRuns.WriteString(s.Run)
		publishRuns.WriteString("\n")
	}
	for _, needle := range []string{"sha256sum cfgdelta-*", "SHA256SUMS", "gh release create"} {
		if !strings.Contains(publishRuns.String(), needle) {
			t.Fatalf("publish steps never mention %q — assets would ship unverifiable", needle)
		}
	}

	// 发布说明文件的落点：`Checksums + manifest` 步骤先 `cd dist`，而 `Create GitHub
	// Release` 步骤在仓库根跑。两个步骤共用同一个相对路径就是 v0.1.0 上真实发生的
	// `open RELEASE_NOTES.md: no such file or directory`。判据：说明文件必须写到
	// 绝对路径（$GITHUB_WORKSPACE），且 publish 里不得出现裸相对重定向。
	// 变异验证：把 `"$notes"` 改回 `RELEASE_NOTES.md` → 本测试必须红。
	if !strings.Contains(publishRuns.String(), "GITHUB_WORKSPACE/RELEASE_NOTES.md") {
		t.Fatal("publish never anchors RELEASE_NOTES.md to $GITHUB_WORKSPACE — a step that cd'd into dist/ would strand it")
	}
	if strings.Contains(publishRuns.String(), "> RELEASE_NOTES.md") {
		t.Fatal("publish writes RELEASE_NOTES.md at a bare relative path — that path is cwd-dependent and already broke one release")
	}

	// 重跑必须幂等：tag 上第一次失败后重跑不得因 "release already exists" 再红。
	if !strings.Contains(publishRuns.String(), "gh release upload") ||
		!strings.Contains(publishRuns.String(), "--clobber") {
		t.Fatal("publish is not idempotent — a re-run on the same tag would fail on an existing release")
	}

	total := 0
	for name, job := range doc.Jobs {
		total += len(job.Steps)
		t.Logf("job %s: %d steps", name, len(job.Steps))
	}
	if total < 18 {
		t.Fatalf("total steps = %d, suspiciously few (expected the full pipeline)", total)
	}
}

// repoRoot 从测试文件位置向上找 go.mod。
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for p := wd; ; p = filepath.Dir(p) {
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
			return p
		}
		if filepath.Dir(p) == p {
			t.Fatal("go.mod not found above test directory")
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
