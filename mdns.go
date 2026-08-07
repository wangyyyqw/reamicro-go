package main

// mDNS 广播阅微同步服务（使用 github.com/hashicorp/mdns）。

import (
	"log"
	"net"
	"runtime"
	"sync"

	"github.com/hashicorp/mdns"
)

var (
	mdnsMu  sync.Mutex
	mdnsSrv *mdns.Server
)

// startMDNS 注册并广播阅微同步服务
func startMDNS(userID int64, deviceID, nickname string, port int) {
	mdnsMu.Lock()
	defer mdnsMu.Unlock()
	stopMDNSLocked()
	suffix := deviceSuffix(deviceID)
	// 设备名必须为 ASCII（DNS-SD 标签不支持中文）。
	// 安卓端优先从服务名标签解析设备名，服务名标签 + TXT 都用 ASCII 设备名。
	asciiName := asciiDeviceName(nickname)
	instance := "ReaMicroSync-" + itoa(userID) + "-" + suffix + "-" + asciiName

	// 服务信息
	service, err := mdns.NewMDNSService(
		instance,
		"_reamicro-sync._tcp.",
		"",
		"", port,
		[]net.IP{bestLocalIP()},
		[]string{
			"userId=" + itoa(userID),
			"deviceId=" + deviceID,
			"nickname=" + asciiName,
		})
	if err != nil {
		log.Printf("创建 mDNS 服务失败: %v", err)
		return
	}

	// 启动服务器，显式绑定真实局域网接口（避免代理/虚拟网卡导致组播失效）
	iface := bestLocalInterface()
	server, err := mdns.NewServer(&mdns.Config{
		Zone:  service,
		Iface: iface,
	})
	if err != nil {
		log.Printf("mDNS 服务器启动失败: %v", err)
		return
	}
	mdnsSrv = server
	log.Printf("mDNS 广播: %s 端口 %d 接口 %v", instance, port, ifaceName(iface))
}

// asciiDeviceName 设备名 = 平台名（Win / Mac），mDNS 不支持中文，直接用平台名最清晰
func asciiDeviceName(nickname string) string {
	switch runtime.GOOS {
	case "windows":
		return "Win"
	case "darwin":
		return "Mac"
	case "linux":
		return "Linux"
	default:
		return "PC"
	}
}

func stopMDNS() {
	mdnsMu.Lock()
	defer mdnsMu.Unlock()
	stopMDNSLocked()
}

func stopMDNSLocked() {
	if mdnsSrv != nil {
		_ = mdnsSrv.Shutdown()
		mdnsSrv = nil
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
