package benchmark

import "syscall"

// smallPages 让 buf 只用普通页：透明大页开没开、当时分不分得到都会改变随机访问的延迟，成绩就不只取决于硬件了
func smallPages(buf []byte) {
	_ = syscall.Madvise(buf, syscall.MADV_NOHUGEPAGE)
}
