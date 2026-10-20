// Command cfgdelta 是单静态二进制 CLI 的入口。
//
// 构建：CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=<ver>" ./cmd/cfgdelta
// 退出码：0 等价 / 1 行为差异 / 2 存在 indeterminate / 3 解析或用法错误。
package main

import (
	"os"

	"github.com/yuluo554/10-20-cfgdelta/internal/cli"
)

// version 可被 -ldflags "-X main.version=..." 覆盖；未覆盖时用包内 VERSION。
var version string

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
