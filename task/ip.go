package task

import (
	"bufio"
	"log"
	"math/big"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/XIU2/CloudflareSpeedTest/utils"
)

const defaultInputFile = "ip.txt"

// defaultIPv6PrefixCount 指定了 [-ip6suffix] 时，每个 IPv6 段默认前缀数量
const defaultIPv6PrefixCount = 1024

// defaultIPv6PrefixAlign 指定了 [-ip6suffix] 时，前缀默认对齐位数（64 即前 64 位内都可随机）
const defaultIPv6PrefixAlign = 64

var (
	// TestAll test all ip
	TestAll = false
	// IPFile is the filename of IP Rangs
	IPFile = defaultInputFile
	IPText string
	// IPv6Suffix 由 [-ip6suffix] 指定的 IPv6 后缀（仅后 64 位），英文逗号分隔，为空则后 64 位随机
	IPv6Suffix string
	// IPv6PrefixCount 由 [-ip6n] 指定，为每个 IPv6 段在段内均匀抽取的前缀数量
	IPv6PrefixCount = defaultIPv6PrefixCount
	// IPv6PrefixAlign 由 [-ip6align] 指定，为前缀对齐位数（如 56 则前缀低 8 位恒为 0）
	IPv6PrefixAlign = defaultIPv6PrefixAlign

	ipv6Suffixes []net.IP // 解析后的后缀列表，每项只有后 64 位（8 字节），为空表示未指定
)

func InitRandSeed() {
	rand.Seed(time.Now().UnixNano())
}

func isIPv4(ip string) bool {
	return strings.Contains(ip, ".")
}

func randIPEndWith(num byte) byte {
	if num == 0 { // 对于 /32 这种单独的 IP
		return byte(0)
	}
	return byte(rand.Intn(int(num)))
}

type IPRanges struct {
	ips     []*net.IPAddr
	mask    string
	firstIP net.IP
	ipNet   *net.IPNet
}

func newIPRanges() *IPRanges {
	return &IPRanges{
		ips: make([]*net.IPAddr, 0),
	}
}

// 如果是单独 IP 则加上子网掩码，反之则获取子网掩码(r.mask)
func (r *IPRanges) fixIP(ip string) string {
	// 如果不含有 '/' 则代表不是 IP 段，而是一个单独的 IP，因此需要加上 /32 /128 子网掩码
	if i := strings.IndexByte(ip, '/'); i < 0 {
		if isIPv4(ip) {
			r.mask = "/32"
		} else {
			r.mask = "/128"
		}
		ip += r.mask
	} else {
		r.mask = ip[i:]
	}
	return ip
}

// 解析 IP 段，获得 IP、IP 范围、子网掩码
func (r *IPRanges) parseCIDR(ip string) {
	var err error
	if r.firstIP, r.ipNet, err = net.ParseCIDR(r.fixIP(ip)); err != nil {
		log.Fatalln("ParseCIDR err", err)
	}
}

func (r *IPRanges) appendIPv4(d byte) {
	r.appendIP(net.IPv4(r.firstIP[12], r.firstIP[13], r.firstIP[14], d))
}

func (r *IPRanges) appendIP(ip net.IP) {
	r.ips = append(r.ips, &net.IPAddr{IP: ip})
}

// 返回第四段 ip 的最小值及可用数目
func (r *IPRanges) getIPRange() (minIP, hosts byte) {
	minIP = r.firstIP[15] & r.ipNet.Mask[3] // IP 第四段最小值

	// 根据子网掩码获取主机数量
	m := net.IPv4Mask(255, 255, 255, 255)
	for i, v := range r.ipNet.Mask {
		m[i] ^= v
	}
	total, _ := strconv.ParseInt(m.String(), 16, 32) // 总可用 IP 数
	if total > 255 {                                 // 矫正 第四段 可用 IP 数
		hosts = 255
		return
	}
	hosts = byte(total)
	return
}

func (r *IPRanges) chooseIPv4() {
	if r.mask == "/32" { // 单个 IP 则无需随机，直接加入自身即可
		r.appendIP(r.firstIP)
	} else {
		minIP, hosts := r.getIPRange()    // 返回第四段 IP 的最小值及可用数目
		for r.ipNet.Contains(r.firstIP) { // 只要该 IP 没有超出 IP 网段范围，就继续循环随机
			if TestAll { // 如果是测速全部 IP
				for i := 0; i <= int(hosts); i++ { // 遍历 IP 最后一段最小值到最大值
					r.appendIPv4(byte(i) + minIP)
				}
			} else { // 随机 IP 的最后一段 0.0.0.X
				r.appendIPv4(minIP + randIPEndWith(hosts))
			}
			r.firstIP[14]++ // 0.0.(X+1).X
			if r.firstIP[14] == 0 {
				r.firstIP[13]++ // 0.(X+1).X.X
				if r.firstIP[13] == 0 {
					r.firstIP[12]++ // (X+1).X.X.X
				}
			}
		}
	}
}

// parseIPv6Suffixes 解析 [-ip6suffix] 参数，得到 IPv6 地址后 64 位的后缀列表。
// 支持 ::1、1、0:0:0:1、1234:5678:9abc:def0 等写法，不含 :: 时左侧自动补零成 8 组再解析。
func parseIPv6Suffixes() {
	ipv6Suffixes = nil // 重置，避免重复解析时追加
	if IPv6Suffix == "" {
		return
	}
	for _, suffix := range strings.Split(IPv6Suffix, ",") {
		text := strings.TrimSpace(suffix) // 原始写法，仅用于报错信息
		// 跳过空的（即开头、结尾或连续多个 ,, 的情况）
		if text == "" {
			continue
		}
		suffix = text
		if !strings.Contains(suffix, "::") { // 没有 :: 压缩，说明只写了后面几组，左侧补零到 8 组
			groups := strings.Split(suffix, ":")
			if len(groups) > net.IPv6len/2 {
				log.Fatalf("[-ip6suffix] 后缀 %s 格式有误，请只填写后 64 位，如 ::1 或 1234:5678:9abc:def0", text)
			}
			suffix = strings.Repeat("0:", net.IPv6len/2-len(groups)) + suffix
		}
		ip := net.ParseIP(suffix)
		if ip == nil || ip.To4() != nil { // 必须能解析且是 IPv6 地址
			log.Fatalf("[-ip6suffix] 后缀 %s 格式有误，请只填写后 64 位，如 ::1 或 1234:5678:9abc:def0", text)
		}
		ip = ip.To16()
		for _, b := range ip[:net.IPv6len/2] { // 后缀只能占用后 64 位，前 64 位必须为 0
			if b != 0 {
				log.Fatalf("[-ip6suffix] 后缀 %s 只能是后 64 位（前 64 位必须为 0），如 ::1 或 1234:5678:9abc:def0", text)
			}
		}
		ipv6Suffixes = append(ipv6Suffixes, ip[net.IPv6len/2:]) // 只保留后 64 位（前 64 位已校验为 0）
	}
}

// stratifiedIPv6Prefix 生成该 IP 段内第 bucket 个前缀：把段内可随机的前缀空间均分成 count 个桶，
// 再在桶内随机取值，使各前缀均匀分散在整个空间里（比各自独立随机更容易覆盖到不同的地域块）。
// ones 为 IP 段前缀长度，align 为对齐位数（前缀低 64-align 位恒为 0，如 56 即第 4 组 hextet 低字节为 0）。
func (r *IPRanges) stratifiedIPv6Prefix(ones, align, count, bucket int, rnd *rand.Rand) net.IP {
	// 可随机的前缀空间大小为 2^freeBits，均分给 count 个桶后，取本桶内的随机值
	freeBits := align - ones
	shift := uint(net.IPv6len*8/2 - align) // 可随机位在前 64 位中的起始偏移
	size := new(big.Int).Lsh(big.NewInt(1), uint(freeBits))
	step := new(big.Int).Div(size, big.NewInt(int64(count)))
	value := new(big.Int).Mul(step, big.NewInt(int64(bucket)))
	value.Add(value, new(big.Int).Rand(rnd, step))

	// 把随机值写回前 64 位的可随机位，其余位沿用段地址（对齐位与掩码约束位为 0）
	var random uint64
	for _, b := range value.FillBytes(make([]byte, net.IPv6len/2)) {
		random = random<<8 | uint64(b)
	}
	var base uint64
	for i := 0; i < net.IPv6len/2; i++ {
		base = base<<8 | uint64(r.ipNet.IP[i])
	}
	freeMask := ((uint64(1) << uint(freeBits)) - 1) << shift
	full := base | ((random << shift) & freeMask)

	prefix := make(net.IP, net.IPv6len)
	for i := 0; i < net.IPv6len/2; i++ {
		prefix[i] = byte(full >> uint(8*(net.IPv6len/2-1-i)))
	}
	return prefix
}

// chooseIPv6WithSuffix 生成「段内均匀抽取的前 64 位 + 指定的后 64 位后缀」的 IPv6 地址。
// 掩码长度超过 /64 时后 64 位已被 IP 段约束，无法套用后缀，因此直接跳过该段。
func (r *IPRanges) chooseIPv6WithSuffix() {
	ones, _ := r.ipNet.Mask.Size()
	if ones > net.IPv6len*8/2 {
		utils.Yellow.Printf("[提示] [-ip6suffix] 仅支持前缀长度不超过 /64 的 IPv6 段，已跳过 %s/%d ...\n", r.ipNet.IP, ones)
		return
	}

	// 对齐位数：不能超过 /64，也不能短于 IP 段本身的前缀长度
	align := IPv6PrefixAlign
	if align > net.IPv6len*8/2 {
		utils.Yellow.Printf("[提示] [-ip6align] 不能超过 /64，已按 /64 处理...\n")
		align = net.IPv6len * 8 / 2
	}
	if align < ones {
		align = ones
	}

	// 前缀数量不能超过该段能容纳的前缀数（如 /48 段对齐到 /56 只有 256 个、对齐到 /64 只有 1 个）
	freeBits := align - ones
	count := IPv6PrefixCount
	if count <= 0 {
		count = defaultIPv6PrefixCount
	}
	if capacity := new(big.Int).Lsh(big.NewInt(1), uint(freeBits)); capacity.IsInt64() && int64(count) > capacity.Int64() {
		count = int(capacity.Int64())
	}

	rnd := rand.New(rand.NewSource(rand.Int63()))
	for bucket := 0; bucket < count; bucket++ {
		// 各桶区间互不重叠，因此生成的前缀天然不重复
		prefix := r.stratifiedIPv6Prefix(ones, align, count, bucket, rnd)
		for _, suffix := range ipv6Suffixes { // 每个前缀与每个后缀分别组合
			targetIP := make(net.IP, net.IPv6len)
			copy(targetIP, prefix[:net.IPv6len/2]) // 段内抽取的前 64 位
			copy(targetIP[net.IPv6len/2:], suffix) // 指定的后 64 位后缀
			r.appendIP(targetIP)
		}
	}
}

func (r *IPRanges) chooseIPv6() {
	if r.mask == "/128" { // 单个 IP 则无需随机，直接加入自身即可
		if len(ipv6Suffixes) > 0 { // 单个 IP 已由用户写全（含后 64 位），无需再套用后缀
			utils.Yellow.Printf("[提示] [-ip6suffix] 对单个 IPv6 地址 %s 无效，已按原样测速...\n", r.firstIP.String())
		}
		r.appendIP(r.firstIP)
		return
	}
	if len(ipv6Suffixes) > 0 { // 指定了后缀，则后 64 位固定为后缀，只随机前 64 位
		r.chooseIPv6WithSuffix()
		return
	}

	var tempIP uint8                  // 临时变量，用于记录前一位的值
	for r.ipNet.Contains(r.firstIP) { // 只要该 IP 没有超出 IP 网段范围，就继续循环随机
		r.firstIP[15] = randIPEndWith(255) // 随机 IP 的最后一段
		r.firstIP[14] = randIPEndWith(255) // 随机 IP 的最后一段

		targetIP := make([]byte, len(r.firstIP))
		copy(targetIP, r.firstIP)
		r.appendIP(targetIP) // 加入 IP 地址池

		for i := 13; i >= 0; i-- { // 从倒数第三位开始往前随机
			tempIP = r.firstIP[i]              // 保存前一位的值
			r.firstIP[i] += randIPEndWith(255) // 随机 0~255，加到当前位上
			if r.firstIP[i] >= tempIP {        // 如果当前位的值大于等于前一位的值，说明随机成功了，可以退出该循环
				break
			}
		}
	}
}

func loadIPRanges() []*net.IPAddr {
	ranges := newIPRanges()
	parseIPv6Suffixes() // 解析 [-ip6suffix] 参数，得到 IPv6 后 64 位的后缀列表
	if IPText != "" {   // 从参数中获取 IP 段数据
		IPs := strings.Split(IPText, ",") // 以逗号分隔为数组并循环遍历
		for _, IP := range IPs {
			IP = strings.TrimSpace(IP) // 去除首尾的空白字符（空格、制表符、换行符等）
			if IP == "" {              // 跳过空的（即开头、结尾或连续多个 ,, 的情况）
				continue
			}
			ranges.parseCIDR(IP) // 解析 IP 段，获得 IP、IP 范围、子网掩码
			if isIPv4(IP) {      // 生成要测速的所有 IPv4 / IPv6 地址（单个/随机/全部）
				ranges.chooseIPv4()
			} else {
				ranges.chooseIPv6()
			}
		}
	} else { // 从文件中获取 IP 段数据
		if IPFile == "" {
			IPFile = defaultInputFile
		}
		file, err := os.Open(IPFile)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() { // 循环遍历文件每一行
			line := strings.TrimSpace(scanner.Text()) // 去除首尾的空白字符（空格、制表符、换行符等）
			if line == "" {                           // 跳过空行
				continue
			}
			ranges.parseCIDR(line) // 解析 IP 段，获得 IP、IP 范围、子网掩码
			if isIPv4(line) {      // 生成要测速的所有 IPv4 / IPv6 地址（单个/随机/全部）
				ranges.chooseIPv4()
			} else {
				ranges.chooseIPv6()
			}
		}
	}
	return ranges.ips
}
