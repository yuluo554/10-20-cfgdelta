package doc

import "sort"

// sortStringsImpl 用 Go 的字符串比较（字节序 / UTF-8 lexicographic）。
// 单独抽出是为了让"排序唯一入口"可被测试定位（K-02）。
func sortStringsImpl(s []string) {
	sort.Strings(s)
}
