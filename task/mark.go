package task

import (
	"fmt"
	"strconv"
	"strings"
)

// SocketMark 由 [-mark] 指定的 socket mark 值（即 SO_MARK），0 表示不设置
var SocketMark int

// ParseSocketMark 解析 [-mark] 参数，支持 0x 前缀十六进制与十进制，取值范围 0 ~ 0xFFFFFFFF
func ParseSocketMark(text string) error {
	const errTip = "[-mark] 格式有误，示例：-mark 0x02 或 -mark 2"
	text = strings.TrimSpace(text)
	if text == "" { // 未指定则视为不设置
		SocketMark = 0
		return nil
	}
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") { // 十六进制写法
		base = 16
		text = text[2:]
		if text == "" {
			return fmt.Errorf("%s", errTip)
		}
	}
	// 不使用 0 作为进制，避免 02 这类写法被当成八进制；bitSize 为 32 即限制最大值为 0xFFFFFFFF
	value, err := strconv.ParseUint(text, base, 32)
	if err != nil {
		return fmt.Errorf("%s", errTip)
	}
	SocketMark = int(value)
	return nil
}
