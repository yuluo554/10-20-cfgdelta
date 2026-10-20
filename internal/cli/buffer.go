package cli

import "strings"

// writerBuffer 是最小的字符串缓冲 writer（避免引入 bytes 之外的额外依赖）。
type writerBuffer struct{ b strings.Builder }

func (w *writerBuffer) Write(p []byte) (int, error) { return w.b.Write(p) }

func (w *writerBuffer) String() string { return w.b.String() }
