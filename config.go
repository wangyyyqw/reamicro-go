package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"time"
)

const configDir = ".reamicro_pc"

type Config struct {
	Token    string `json:"token"`
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	BooksDir string `json:"books_dir"`
	DeviceID string `json:"device_id"`
	IsVip    bool   `json:"is_vip"`
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", configDir, "config.json")
	}
	return filepath.Join(home, configDir, "config.json")
}

func loadConfig() *Config {
	cfg := &Config{}
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, cfg)
	return cfg
}

func saveConfig(cfg *Config) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}

// bestLocalIP 返回本机最适合局域网 mDNS 广播的 IPv4 地址
func bestLocalIP() net.IP {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var fallback net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			ip := ipnet.IP.To4()
			// 排除代理 fake-ip 网段
			if ip[0] == 198 && (ip[1] == 18 || ip[1] == 19) {
				continue
			}
			if ip[0] == 169 && ip[1] == 254 {
				continue
			}
			// 私有网段优先
			if ip[0] == 192 && ip[1] == 168 {
				return ip
			}
			if ip[0] == 10 {
				return ip
			}
			if fallback == nil {
				fallback = ip
			}
		}
	}
	return fallback
}

// bestLocalInterface 返回适合 mDNS 广播的真实局域网接口
func bestLocalInterface() *net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var fallback *net.Interface
	for i := range ifaces {
		iface := &ifaces[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			ip := ipnet.IP.To4()
			if ip[0] == 198 && (ip[1] == 18 || ip[1] == 19) {
				continue // 代理 fake-ip
			}
			if ip[0] == 169 && ip[1] == 254 {
				continue // 链路本地
			}
			if ip[0] == 192 && ip[1] == 168 {
				return iface
			}
			if ip[0] == 10 {
				return iface
			}
			if fallback == nil {
				fallback = iface
			}
		}
	}
	return fallback
}

func ifaceName(iface *net.Interface) string {
	if iface == nil {
		return "(默认)"
	}
	return iface.Name
}

func sha256Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
