//go:build !linux

package task

import "net"

// bindInterface 在非 Linux 平台不支持 SO_BINDTODEVICE，
// 返回 false 表示调用方应回退到绑定该网卡的本地地址。
func bindInterface(dialer *net.Dialer, ifname string) bool {
	return false
}
