package cli

import (
	"os"

	"github.com/yuluo554/10-20-cfgdelta/internal/selftest"
)

// init 把"跑一条用例"的能力注入 selftest，避免 selftest → cli 的循环 import。
func init() {
	selftest.SetRunner(func(args []string) (int, string) {
		var out, errb writerBuffer
		code := Run(Env{Args: args, Stdout: &out, Stderr: &errb})
		return int(code), out.String() + errb.String()
	})
}

// Main 是进程入口的唯一出口（只做 os.Exit 翻译）。
func Main(args []string) int {
	return int(Run(Env{Args: args, Stdout: os.Stdout, Stderr: os.Stderr}))
}
