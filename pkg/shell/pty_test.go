package shell

import (
	"errors"
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/libtnb/assert/check"
	"github.com/libtnb/assert/must"
)

func TestIsPTYError(t *testing.T) {
	other := errors.New("other")

	check.NoError(t, isPTYError(&os.PathError{Op: "read", Path: "/dev/ptmx", Err: syscall.EIO}))
	check.NoError(t, isPTYError(io.EOF))
	check.ErrorIs(t, isPTYError(other), other)
	check.ErrorIs(t, isPTYError(&os.PathError{Op: "read", Path: "/dev/ptmx", Err: syscall.EBADF}), syscall.EBADF)
}

// 命令退出后从端关闭，读主端在 Linux 上得到 EIO、在 macOS 上得到 EOF，都算正常结束
func TestTurnPipeEndsCleanlyAfterExit(t *testing.T) {
	turn, err := NewPTYTurn(t.Context(), nil, "true")
	must.NoError(t, err)
	defer turn.Close()

	check.NoError(t, turn.Pipe(t.Context()))
	turn.Wait()
}
