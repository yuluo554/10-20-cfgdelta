// Package profile 是 K8s 默认值等价表（plan/03 §四）。
//
// 数据驱动：默认值**不写在代码里**，而是版本化 JSON（data/profiles/<name>.json），
// 构建期由 tools/sync-profiles.mjs 逐字节镜像进本包的 profiles/ 目录并 embed。
package profile

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/yuluo554/10-20-cfgdelta/internal/doc"
	"github.com/yuluo554/10-20-cfgdelta/internal/value"
)

//go:embed profiles/*/*.json
var embedded embed.FS

// Rule 是一条默认值规则。
type Rule struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Default string `json:"default"`
	Family  string `json:"family"`
	Source  string `json:"source"`

	// OnlyIfImageTagNotLatest 标记"默认值取决于同容器 image 的 tag"的规则。
	//
	// API Server 的 imagePullPolicy 默认值不是常量：tag 为 `latest`（**含完全没写
	// tag**——parsers.ParseImageName 会把空 tag 归一成 `latest`）时默认 `Always`，
	// 其余 `IfNotPresent`（K8s 1.29 pkg/apis/core/v1/defaults.go SetDefaults_Container）。
	// 把这条规则当成无条件 `IfNotPresent` 会造成两个方向的误判：
	// 左 `:latest` 无 policy / 右 `:latest` 显式 `Always` → 假阳性；
	// 左 `:latest` 无 policy / 右 `:latest` 显式 `IfNotPresent` → **假阴性（更危险）**。
	OnlyIfImageTagNotLatest bool `json:"only_if_image_tag_not_latest,omitempty"`
}

// Profile 是一份版本化默认值表。
type Profile struct {
	Name       string `json:"name"`
	K8sVersion string `json:"k8s_version"`
	ScopeNote  string `json:"scope_note"`
	Rules      []Rule `json:"rules"`
}

// Load 按名称（如 "k8s/v1.29"）加载内嵌 profile。
// 未知版本 → 明确错误（**不回落**到"最近的版本"，§4.11 第 6 条）。
func Load(name string) (*Profile, error) {
	if name == "" {
		return nil, fmt.Errorf("profile name is required (e.g. k8s/v1.29)")
	}
	path := "profiles/" + name + ".json"
	data, err := embedded.ReadFile(path)
	if err != nil {
		names, _ := Names()
		return nil, fmt.Errorf("unknown profile %q (available: %s)", name, strings.Join(names, ", "))
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("profile %q is not valid JSON: %w", name, err)
	}
	if p.Name != name {
		return nil, fmt.Errorf("profile file %q declares name %q (must match its path)", path, p.Name)
	}
	return &p, nil
}

// Names 列出可用 profile 名（排序后，稳定输出）。
func Names() ([]string, error) {
	var out []string
	err := fs.WalkDir(embedded, "profiles", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".json") {
			return nil
		}
		name := strings.TrimSuffix(strings.TrimPrefix(p, "profiles/"), ".json")
		out = append(out, name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// ApplyResult 记录施加结果（用于 --dump-normalized 与降级标注）。
type ApplyResult struct {
	Hits     int
	Degraded bool // 未命中任何规则 → 明确降级为纯结构 diff
}

// Apply 对整棵文档树**双侧**施加默认值（§4.11）。
//
// 施加顺序：外层先于内层、父先于子，同层按路径 UTF-8 字节序。
// 只补"缺失"的键，**不覆盖显式值** —— 显式非默认值因此会正常报差异。
func Apply(root *doc.Node, p *Profile) ApplyResult {
	if p == nil || root == nil {
		return ApplyResult{Degraded: true}
	}
	hits := 0
	kind := kindOf(root)
	for _, r := range p.Rules {
		if r.Kind != "" && !strings.EqualFold(r.Kind, kind) {
			continue
		}
		if applyRule(root, r) {
			hits++
		}
	}
	return ApplyResult{Hits: hits, Degraded: hits == 0}
}

func applyRule(root *doc.Node, r Rule) bool {
	segs := splitPointer(r.Path)
	if len(segs) == 0 {
		return false
	}
	return insertDefault(root, segs, r)
}

func insertDefault(n *doc.Node, segs []string, r Rule) bool {
	if n == nil || len(segs) == 0 {
		return false
	}
	seg := segs[0]

	// 序列 + 通配：对每个元素递归（`containers/*/imagePullPolicy` 的形态）。
	if seg == "*" && n.Kind == doc.Sequence {
		hit := false
		for _, item := range n.Children {
			if insertDefault(item, segs[1:], r) {
				hit = true
			}
		}
		return hit
	}

	if n.Kind != doc.Mapping {
		return false
	}

	if seg == "*" {
		hit := false
		for i := 0; i+1 < len(n.Children); i += 2 {
			if insertDefault(n.Children[i+1], segs[1:], r) {
				hit = true
			}
		}
		return hit
	}

	if child := n.Get(seg); child != nil {
		return insertDefault(child, segs[1:], r)
	}

	// 键不存在：只在**最后一个段**补默认值；中间层缺失不猜结构。
	if len(segs) != 1 {
		return false
	}
	// 条件性默认值：image tag 为 latest（或无 tag）时 API Server 给的是 Always，
	// 此时补 IfNotPresent 会把真实差异"抵消"掉（假阴性）。
	if r.OnlyIfImageTagNotLatest && !imageTagNotLatest(n) {
		return false
	}
	key := &doc.Node{Kind: doc.Scalar, Tag: "!!str", Style: 0, Raw: seg}
	val := &doc.Node{Kind: doc.Scalar, Tag: "!!str", Style: 0, Raw: r.Default}
	switch r.Family {
	case "bool":
		val.Tag = "!!bool"
	case "int", "quantity", "decimal", "duration":
		val.Tag = "!!int"
	}
	n.Children = append(n.Children, key, val)
	return true
}

func kindOf(root *doc.Node) string {
	kv := root.Get("kind")
	if kv == nil {
		return ""
	}
	return kv.Raw
}

// imageTagNotLatest 复刻 K8s 的 imagePullPolicy 默认值判定（K8s 1.29）：
//
//	SetDefaults_Container: tag == "latest" ? Always : IfNotPresent
//	parsers.ParseImageName: 既无 tag 又无 digest 时，把 tag 归一为 "latest"
//
// 所以"无 tag"与"tag=latest"都**不满足**本条件；带 digest 且无 tag 的是空 tag，
// 落到 IfNotPresent，满足本条件。没有 image 字段时无从判定 → 不补（宁可不抵消，也不猜）。
func imageTagNotLatest(container *doc.Node) bool {
	if container == nil || container.Kind != doc.Mapping {
		return false
	}
	img := container.Get("image")
	if img == nil || img.Raw == "" {
		return false
	}
	hasDigest := strings.Contains(img.Raw, "@")
	tag := imageTagOf(img.Raw)
	if tag == "" {
		// 无 tag：有 digest → ParseImageName 保留空 tag（→ IfNotPresent）；否则归一为 latest。
		return hasDigest
	}
	return tag != "latest"
}

// imageTagOf 抽出镜像引用的 tag（不含 digest）。registry 端口（registry:5000/img）
// 不是 tag，所以只看最后一个 path segment。
func imageTagOf(image string) string {
	base := image
	if i := strings.Index(base, "@"); i >= 0 {
		base = base[:i]
	}
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, ":"); i >= 0 {
		return base[i+1:]
	}
	return ""
}

func splitPointer(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		s = strings.ReplaceAll(s, "~1", "/")
		s = strings.ReplaceAll(s, "~0", "~")
		segs[i] = s
	}
	return segs
}

// FamilyOf 把 profile 的 family 字段翻译成 value.Family。
func FamilyOf(r Rule) value.Family {
	switch strings.ToLower(r.Family) {
	case "quantity":
		return value.FamilyQuantity
	case "duration":
		return value.FamilyDuration
	case "decimal", "int":
		return value.FamilyDecimal
	case "bool":
		return value.FamilyBool
	case "ratio":
		return value.FamilyRatio
	}
	return value.FamilyString
}
