package main

// 阅微局域网同步协议：线协议编码/解码与 JSON 数据模型。
// 参考安卓端 SyncWire.kt / SyncModels.kt。

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// 命令常量
const (
	CMD_HELLO         = "HELLO"
	CMD_HELLO_OK      = "HELLO_OK"
	CMD_MANIFEST      = "MANIFEST"
	CMD_FILE          = "FILE"
	CMD_FILES         = "FILES"
	CMD_GET_FILE      = "GET_FILE"
	CMD_GET_FILES     = "GET_FILES"
	CMD_GET_BOOK_FILES = "GET_BOOK_FILES"
	CMD_BOOK_FILES    = "BOOK_FILES"
	CMD_ERR           = "ERR"
	CMD_BYE           = "BYE"

	SERVICE_TYPE = "_reamicro-sync._tcp"
)

// ---------- 数据模型 ----------
type SyncHello struct {
	UserID     int64  `json:"userId"`
	DeviceID   string `json:"deviceId"`
	Nickname   string `json:"nickname"`
	AppVersion int    `json:"appVersion"`
	IsVip      bool   `json:"isVip"`
}

type FileSnapshot struct {
	RelativePath string `json:"relativePath"`
	Size         int64  `json:"size"`
	MD5          string `json:"md5"`
	Mtime        int64  `json:"mtime"`
}

type BookSnapshot struct {
	UUID          string  `json:"uuid"`
	Title         string  `json:"title"`
	Subtitle      string  `json:"subtitle"`
	Author        string  `json:"author"`
	Cover         string  `json:"cover"`
	Size          int64   `json:"size"`
	Group         string  `json:"group"`
	Created       int64   `json:"created"`
	CfiVersion    int     `json:"cfiVersion"`
	EmbeddedFonts int     `json:"embeddedFonts"`
	Epubcfi       string  `json:"epubcfi"`
	Chapter       string  `json:"chapter"`
	Progress      float64 `json:"progress"`
	Total         int64   `json:"total"`
	Finished      int64   `json:"finished"`
	Updated       int64   `json:"updated"`
	PinnedAt      int64   `json:"pinnedAt"`
	CloudID       int64   `json:"cloudId"`
	BackupType    int     `json:"backupType"`
	BackupID      string  `json:"backupId"`
	BackupCode    string  `json:"backupCode"`
	Publisher     string  `json:"publisher"`
}

type MarkSnapshot struct {
	StartCfi  string `json:"startCfi"`
	EndCfi    string `json:"endCfi"`
	Kind      int    `json:"kind"`
	Chapter   string `json:"chapter"`
	Quote     string `json:"quote"`
	Note      string `json:"note"`
	Style     int    `json:"style"`
	Color     string `json:"color"`
	Synced    int    `json:"synced"`
	CloudID   int64  `json:"cloudId"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

type BookSyncEntry struct {
	UUID              string         `json:"uuid"`
	MetaUpdatedAt     int64          `json:"metaUpdatedAt"`
	ProgressUpdatedAt int64          `json:"progressUpdatedAt"`
	Book              BookSnapshot   `json:"book"`
	Marks             []MarkSnapshot `json:"marks"`
	Files             []FileSnapshot `json:"files"`
}

type SyncManifest struct {
	UserID        int64            `json:"userId"`
	DeviceID      string           `json:"deviceId"`
	AppVersion    int              `json:"appVersion"`
	GeneratedAt   int64            `json:"generatedAt"`
	Books         []BookSyncEntry  `json:"books"`
	ReaderProfile json.RawMessage  `json:"readerProfile,omitempty"`
}

type SyncBookFiles struct {
	UUID  string         `json:"uuid"`
	Files []FileSnapshot `json:"files"`
}

// ---------- 线协议 ----------

// writeControlLine 生成 "CMD" 或 "CMD <len>" 控制行
func controlLine(cmd string, bodyLen int) string {
	if bodyLen <= 0 {
		return cmd
	}
	return fmt.Sprintf("%s %d", cmd, bodyLen)
}

// writeFrame 写一帧（文本 body）
func writeFrame(conn net.Conn, cmd string, body []byte) error {
	line := controlLine(cmd, len(body)) + "\n"
	if _, err := conn.Write([]byte(line)); err != nil {
		return err
	}
	if len(body) > 0 {
		if _, err := conn.Write(body); err != nil {
			return err
		}
	}
	return nil
}

// writeJSONFrame 写一个 JSON 对象帧
func writeJSONFrame(conn net.Conn, cmd string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return writeFrame(conn, cmd, data)
}

// packFiles 打包文件列表为二进制 body：u32 count，每个条目 u32 nameLen+name+u32 dataLen+data
func packFiles(entries [][2][]byte) []byte {
	buf := make([]byte, 0, 256)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(entries)))
	for _, e := range entries {
		name, data := e[0], e[1]
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(name)))
		buf = append(buf, name...)
		buf = binary.BigEndian.AppendUint32(buf, uint32(len(data)))
		buf = append(buf, data...)
	}
	return buf
}

// unpackFiles 解析 packFiles 的 body
func unpackFiles(data []byte) ([][2][]byte, error) {
	if len(data) < 4 {
		return nil, nil
	}
	count := binary.BigEndian.Uint32(data[:4])
	off := 4
	var entries [][2][]byte
	for i := uint32(0); i < count; i++ {
		if off+4 > len(data) {
			break
		}
		nameLen := binary.BigEndian.Uint32(data[off:])
		off += 4
		if off+int(nameLen) > len(data) {
			break
		}
		name := data[off : off+int(nameLen)]
		off += int(nameLen)
		if off+4 > len(data) {
			break
		}
		dataLen := binary.BigEndian.Uint32(data[off:])
		off += 4
		if off+int(dataLen) > len(data) {
			break
		}
		body := data[off : off+int(dataLen)]
		off += int(dataLen)
		entries = append(entries, [2][]byte{name, body})
	}
	return entries, nil
}

// serviceName 生成 mDNS 服务名：ReaMicroSync-<userId>-<suffix6>-<label>
func serviceName(userID int64, deviceID, label string) string {
	suffix := deviceSuffix(deviceID)
	return fmt.Sprintf("ReaMicroSync-%d-%s-%s", userID, suffix, label)
}

// deviceSuffix 取 deviceID 的 sha256 前 6 位
func deviceSuffix(deviceID string) string {
	sum := sha256Sum(deviceID)
	if len(sum) >= 6 {
		return sum[:6]
	}
	return sum
}

// parseServiceName 解析服务名为 userId + label
func parseServiceName(name string) (int64, string) {
	s := strings.TrimSpace(name)
	// 去掉类型后缀
	if i := strings.Index(s, "."); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimPrefix(s, "ReaMicroSync-")
	// userId-suffix[-label]
	parts := strings.SplitN(s, "-", 3)
	if len(parts) < 2 {
		return 0, ""
	}
	var uid int64
	fmt.Sscanf(parts[0], "%d", &uid)
	label := ""
	if len(parts) >= 3 {
		label = parts[2]
	}
	return uid, label
}
