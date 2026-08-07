package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 结构（Wails 绑定对象）
type App struct {
	ctx context.Context
}

// 全局 app 实例引用，供 host 发日志事件到前端
var currentApp *App

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	currentApp = a
}

// emitLog 向前端发送日志事件（供 host 等模块调用）
func emitLog(msg string) {
	if currentApp != nil && currentApp.ctx != nil {
		runtime.EventsEmit(currentApp.ctx, "log", msg)
	}
	log.Printf("%s", msg)
}

// ---------- 登录 ----------
func (a *App) SendEmailCode(email string) error {
	return SendEmailCode(email)
}

type LoginResult struct {
	UserID   int64  `json:"userId"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	IsVip    bool   `json:"isVip"`
}

func (a *App) Login(email, code string) (*LoginResult, error) {
	token, err := LoginOrRegister(email, code)
	if err != nil {
		return nil, err
	}
	info, err := FetchUserInfo(token)
	if err != nil {
		return nil, err
	}
	cfg := loadConfig()
	cfg.Token = token
	cfg.UserID = info.ID.Value
	cfg.Nickname = info.NickName
	if cfg.Nickname == "" {
		cfg.Nickname = info.Email
	}
	cfg.Email = info.Email
	cfg.IsVip = info.VipExpireTime != ""
	if cfg.DeviceID == "" {
		buf := make([]byte, 8)
		_, _ = rand.Read(buf)
		cfg.DeviceID = "pc-" + hex.EncodeToString(buf)
	}
	if err := saveConfig(cfg); err != nil {
		return nil, err
	}
	log.Printf("登录成功 userId=%d nickname=%s", info.ID.Value, cfg.Nickname)
	return &LoginResult{
		UserID:   cfg.UserID,
		Nickname: cfg.Nickname,
		Email:    cfg.Email,
		IsVip:    cfg.IsVip,
	}, nil
}

// ---------- 会话 ----------
type SessionInfo struct {
	LoggedIn bool   `json:"loggedIn"`
	UserID   int64  `json:"userId"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	BooksDir string `json:"booksDir"`
}

func (a *App) GetSession() *SessionInfo {
	cfg := loadConfig()
	return &SessionInfo{
		LoggedIn: cfg.Token != "" && cfg.UserID > 0,
		UserID:   cfg.UserID,
		Nickname: cfg.Nickname,
		Email:    cfg.Email,
		BooksDir: cfg.BooksDir,
	}
}

func (a *App) Logout() error {
	cfg := loadConfig()
	cfg.Token = ""
	cfg.UserID = 0
	cfg.Nickname = ""
	cfg.Email = ""
	return saveConfig(cfg)
}

// ---------- 书籍 ----------
type BookInfo struct {
	UUID  string `json:"uuid"`
	Title string `json:"title"`
	Size  int64  `json:"size"`
}

func (a *App) SetBooksDir(dir string) error {
	cfg := loadConfig()
	cfg.BooksDir = dir
	return saveConfig(cfg)
}

// ChooseBooksDir 弹出系统目录选择对话框，返回选中的目录
func (a *App) ChooseBooksDir() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("应用上下文不可用")
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择书籍目录",
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", nil // 用户取消
	}
	cfg := loadConfig()
	cfg.BooksDir = dir
	if err := saveConfig(cfg); err != nil {
		return "", err
	}
	return dir, nil
}

func (a *App) ScanBooks(dir string) ([]BookInfo, error) {
	if dir == "" {
		cfg := loadConfig()
		dir = cfg.BooksDir
	}
	books, err := scanBooks(dir)
	if err != nil {
		return nil, err
	}
	var result []BookInfo
	for _, b := range books {
		result = append(result, BookInfo{UUID: b.UUID, Title: b.Title, Size: b.OrigSize})
	}
	return result, nil
}

func (a *App) GetHostStatus() HostStatus {
	return host.status()
}

// ---------- 日志推送 ----------
func (a *App) Log(message string) {
	log.Printf("%s", message)
}
