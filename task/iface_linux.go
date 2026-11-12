//go:build linux

package task

import (
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// bindInterface 将 dialer 创建的 socket 绑定到 ifname 指定的网卡（SO_BINDTODEVICE），
// 由内核把出口路由约束到该网卡。该选项与地址族无关，IPv4 与 IPv6 连接均生效，
// 源地址由内核从该网卡上的同族地址中自动选择。
// 返回 true 表示当前平台支持设备绑定。
func bindInterface(dialer *net.Dialer, ifname string) bool {
	device := func(network, address string, c syscall.RawConn) error {
		var sockErr error
		if err := c.Control(func(fd uintptr) {
			sockErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, ifname)
		}); err != nil {
			return err
		}
		return sockErr
	}
	// 与 [-mark] 等已有的 Control 串联，任一失败即终止
	if prev := dialer.Control; prev != nil {
		dialer.Control = func(network, address string, c syscall.RawConn) error {
			if err := prev(network, address, c); err != nil {
				return err
			}
			return device(network, address, c)
		}
	} else {
		dialer.Control = device
	}
	return true
}
