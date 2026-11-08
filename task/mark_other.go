//go:build !linux

package task

import (
	"sync"
	"syscall"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

// markWarnOnce 保证非 Linux 平台的提示只输出一次（newDialer 会按 IP 反复调用）
var markWarnOnce sync.Once

// markControl 在非 Linux 平台不支持 SO_MARK，返回 nil 表示不设置 Dialer.Control，
// 仅在调试模式下提示一次，随后照常测速。
func markControl(mark int) func(network, address string, c syscall.RawConn) error {
	if utils.Debug {
		markWarnOnce.Do(func() {
			utils.Yellow.Println("[提示] 当前系统不支持 SO_MARK 套接字选项，[-mark] 已忽略，测速将照常进行...")
		})
	}
	return nil
}
