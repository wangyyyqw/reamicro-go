package main

// 阅微账号登录：邮箱验证码 → token → 用户信息。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const apiBaseURL = "https://api.reamicro.zhendong.ltd/"

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36"

// httpClient 禁用 keep-alive，避免服务器对复用连接的 EOF 问题
var httpClient = &http.Client{
	Transport: &http.Transport{
		DisableKeepAlives: true,
	},
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type userInfo struct {
	ID            FlexInt64  `json:"id"`
	UserType      FlexInt    `json:"userType"`
	NickName      string     `json:"nickName"`
	UserTitle     string     `json:"userTitle"`
	TitleLevel    FlexInt    `json:"titleLevel"`
	Gender        FlexInt    `json:"gender"`
	Avatar        string     `json:"avatar"`
	VipExpireTime string     `json:"vipExpireTime"`
	Email         string     `json:"email"`
	ThirdBinds    []string   `json:"thirdBinds"`
	Level         FlexInt    `json:"level"`
	Exp           FlexInt    `json:"exp"`
	ExpMax        FlexInt    `json:"expMax"`
	Coin          FlexInt    `json:"coin"`
	Gem           FlexInt64  `json:"gem"`
	Voucher       FlexInt64  `json:"voucher"`
	CallingCardID FlexInt64  `json:"callingCardId"`
	ReadFinishedBookNum FlexInt `json:"readFinishedBookNum"`
	TotalReadTime FlexInt64  `json:"totalReadTime"`
	CreateTime    FlexInt64  `json:"createTime"`
	PayAmount     FlexInt64  `json:"payAmount"`
}

// apiPost 发送 POST，body 为 JSON，返回 envelope 的 data
func apiPost(session *http.Client, path string, body map[string]interface{}, token string) (json.RawMessage, error) {
	payload, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", apiBaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("platform", "desktop")
	req.Header.Set("Connection", "close")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := session.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("响应非 JSON: %s", string(raw))
	}
	if env.Code != 0 {
		return nil, fmt.Errorf("%s", env.Message)
	}
	return env.Data, nil
}

// SendEmailCode 发送验证码
func SendEmailCode(email string) error {
	session := httpClient
	_, err := apiPost(session, "rest/user/send-email-code", map[string]interface{}{"email": email}, "")
	return err
}

// LoginOrRegister 邮箱验证码登录，返回 token
func LoginOrRegister(email, code string) (string, error) {
	session := httpClient
	data, err := apiPost(session, "rest/user/login-or-register", map[string]interface{}{
		"type": "email", "email": email, "code": code,
	}, "")
	if err != nil {
		return "", err
	}
	var loginRes struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &loginRes); err != nil {
		return "", fmt.Errorf("解析登录结果失败")
	}
	if loginRes.Token == "" {
		return "", fmt.Errorf("登录未返回 token")
	}
	return loginRes.Token, nil
}

// FetchUserInfo 用 token 获取用户信息
func FetchUserInfo(token string) (*userInfo, error) {
	session := httpClient
	data, err := apiPost(session, "rest/user/get-user-info", map[string]interface{}{}, token)
	if err != nil {
		return nil, err
	}
	var info userInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %s", string(data))
	}
	return &info, nil
}
