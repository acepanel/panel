package confval

import "regexp"

var (
	heapInitRegexp = regexp.MustCompile(`(?m)^[^#\n]*?-Xms(\S+)`)
	heapMaxRegexp  = regexp.MustCompile(`(?m)^[^#\n]*?-Xmx(\S+)`)
)

// JVMHeap 从 JVM 参数中取出 -Xms 与 -Xmx 的值，jvm.options 里被注释掉的示例行不算
func JVMHeap(opts string) (initSize, maxSize string) {
	if m := heapInitRegexp.FindStringSubmatch(opts); m != nil {
		initSize = m[1]
	}
	if m := heapMaxRegexp.FindStringSubmatch(opts); m != nil {
		maxSize = m[1]
	}

	return initSize, maxSize
}
