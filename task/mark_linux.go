//go:build linux

package task

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// markControl 返回一个 Dialer.Control，在 socket 创建后、connect 前为其设置 SO_MARK，
// 使测速发出的连接带上指定 mark（需要 root 或 CAP_NET_ADMIN 权限）。
func markControl(mark int) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		var sockErr error
		if err := c.Control(func(fd uintptr) {
			sockErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_MARK, mark)
		}); err != nil {
			return err
		}
		return sockErr
	}
}
