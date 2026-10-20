// Package version 是产品版本号的运行时来源。
//
// 单一来源 = 仓库根 `VERSION` 文件；本包内的 VERSION 是其逐字节镜像
// （由 `tools/sync-version.mjs` 同步，`tests/version_test.go` 断言两者一致）。
// 构建期可用 -ldflags "-X main.version=<ver>" 覆盖，终值仍由测试与 VERSION 对账。
package version

import _ "embed"

//go:embed VERSION
var embedded string

// Value 返回去掉行尾空白后的版本号。
func Value() string {
	v := embedded
	for len(v) > 0 && (v[len(v)-1] == '\n' || v[len(v)-1] == '\r' || v[len(v)-1] == ' ') {
		v = v[:len(v)-1]
	}
	return v
}
