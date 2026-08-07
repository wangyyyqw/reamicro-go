package main

// 阅微同步 host：TCP 服务器 + mDNS 广播。

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

// hostState 管理同步服务状态
type hostState struct {
	mu       sync.Mutex
	running  bool
	port     int
	listener net.Listener
	deviceID string
	nickname string
}

var host = &hostState{}

// HostStatus 返回给前端的运行状态
type HostStatus struct {
	Running  bool   `json:"running"`
	Port     int    `json:"port"`
	DeviceID string `json:"deviceId"`
	Nickname string `json:"nickname"`
}

func (h *hostState) status() HostStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	return HostStatus{
		Running:  h.running,
		Port:     h.port,
		DeviceID: h.deviceID,
		Nickname: h.nickname,
	}
}

// handleConn 处理一个客户端连接
func (h *hostState) handleConn(conn net.Conn) {
	defer conn.Close()
	cfg := loadConfig()
	dir := cfg.BooksDir
	deviceID := h.deviceID
	reader := bufio.NewReader(conn)
	emitLog("客户端接入 " + conn.RemoteAddr().String())

	// 统计：本次连接拉取的文件数 / 总字节
	sentFiles := 0
	sentBytes := int64(0)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF && sentFiles > 0 {
				emitLog(fmt.Sprintf("本次同步完成：%d 个文件 / %d MB", sentFiles, sentBytes/1024/1024))
			}
			return
		}
		line = trimNewline(line)
		if line == CMD_BYE {
			if sentFiles > 0 {
				emitLog(fmt.Sprintf("本次同步完成：%d 个文件 / %d MB", sentFiles, sentBytes/1024/1024))
			}
			emitLog("连接关闭")
			return
		}
		// 解析 "CMD [len]"
		cmd := line
		var bodyLen int
		if sp := indexSpace(line); sp >= 0 {
			cmd = line[:sp]
			fmt.Sscanf(line[sp+1:], "%d", &bodyLen)
		}
		var body []byte
		if bodyLen > 0 {
			body = make([]byte, bodyLen)
			if _, err := io.ReadFull(reader, body); err != nil {
				return
			}
		}

		switch cmd {
		case CMD_HELLO:
			var hello SyncHello
			if err := json.Unmarshal(body, &hello); err != nil {
				_ = writeFrame(conn, CMD_ERR, []byte("bad hello"))
				continue
			}
			if hello.UserID != cfg.UserID {
				emitLog(fmt.Sprintf("拒绝：客户端 userId=%d 与本机 %d 不匹配", hello.UserID, cfg.UserID))
				_ = writeFrame(conn, CMD_ERR, []byte("只能与同一账号设备同步"))
				continue
			}
			reply := SyncHello{
				UserID:   cfg.UserID,
				DeviceID: deviceID,
				Nickname: h.nickname,
				IsVip:    cfg.IsVip,
			}
			_ = writeJSONFrame(conn, CMD_HELLO_OK, reply)
			emitLog("设备已连接")
		case CMD_MANIFEST:
			manifest, err := buildManifest(dir, deviceID, cfg.UserID)
			if err != nil {
				_ = writeFrame(conn, CMD_ERR, []byte(err.Error()))
				continue
			}
			_ = writeJSONFrame(conn, CMD_MANIFEST, manifest)
			emitLog(fmt.Sprintf("提供 %d 本书", len(manifest.Books)))
		case CMD_GET_BOOK_FILES:
			var req struct {
				UUIDs []string `json:"uuids"`
			}
			_ = json.Unmarshal(body, &req)
			var result []SyncBookFiles
			for _, u := range req.UUIDs {
				if b, ok := findBook(dir, u); ok {
					var files []FileSnapshot
					for _, f := range b.Files {
						files = append(files, FileSnapshot{
							RelativePath: f.RelPath,
							Size:         f.Size,
							Mtime:        f.Mtime,
						})
					}
					result = append(result, SyncBookFiles{UUID: b.UUID, Files: files})
				}
			}
			_ = writeJSONFrame(conn, CMD_BOOK_FILES, result)
		case CMD_GET_FILE:
			var req struct {
				UUID string `json:"uuid"`
				Path string `json:"path"`
			}
			_ = json.Unmarshal(body, &req)
			data, err := readBookFile(dir, req.UUID, req.Path)
			if err != nil {
				_ = writeFrame(conn, CMD_ERR, []byte("文件不存在"))
				continue
			}
			_ = writeFrame(conn, CMD_FILE, data)
			sentFiles++
			sentBytes += int64(len(data))
		case CMD_GET_FILES:
			var req struct {
				UUID  string   `json:"uuid"`
				Paths []string `json:"paths"`
			}
			_ = json.Unmarshal(body, &req)
			var entries [][2][]byte
			for _, p := range req.Paths {
				data, err := readBookFile(dir, req.UUID, p)
				if err != nil {
					continue
				}
				entries = append(entries, [2][]byte{[]byte(p), data})
			}
			_ = writeFrame(conn, CMD_FILES, packFiles(entries))
			for _, e := range entries {
				sentFiles++
				sentBytes += int64(len(e[1]))
			}
		default:
			_ = writeFrame(conn, CMD_ERR, []byte("unknown command"))
		}
	}
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func indexSpace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

func filepathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}

// StartHost 启动 TCP 同步服务并广播 mDNS
func (a *App) StartHost() (HostStatus, error) {
	host.mu.Lock()
	if host.running {
		host.mu.Unlock()
		return HostStatus{Running: true, Port: host.port, DeviceID: host.deviceID, Nickname: host.nickname}, nil
	}
	cfg := loadConfig()
	if cfg.UserID <= 0 {
		host.mu.Unlock()
		return HostStatus{}, fmt.Errorf("请先登录")
	}
	host.mu.Unlock()

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		return HostStatus{}, err
	}

	host.mu.Lock()
	host.running = true
	host.listener = ln
	host.port = ln.Addr().(*net.TCPAddr).Port
	host.deviceID = cfg.DeviceID
	if host.deviceID == "" {
		host.deviceID = "pc-" + deviceSuffix(cfg.Email)
	}
	host.nickname = cfg.Nickname
	deviceID := host.deviceID
	port := host.port
	host.mu.Unlock()

	// 保存 deviceID
	if cfg.DeviceID == "" {
		cfg.DeviceID = deviceID
		_ = saveConfig(cfg)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go host.handleConn(conn)
		}
	}()

	// mDNS 广播
	startMDNS(cfg.UserID, deviceID, cfg.Nickname, port)

	log.Printf("同步服务已启动，端口 %d", port)
	return host.status(), nil
}

// StopHost 停止同步服务
func (a *App) StopHost() {
	host.mu.Lock()
	defer host.mu.Unlock()
	if host.listener != nil {
		_ = host.listener.Close()
		host.listener = nil
	}
	stopMDNS()
	host.running = false
	cleanupUnpackCache()
	log.Printf("同步服务已停止")
}
