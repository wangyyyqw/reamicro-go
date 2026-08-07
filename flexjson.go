package main

// 宽松 JSON 数字类型：接受字符串或数字（服务器返回类型不一致）。

import (
	"encoding/json"
	"strconv"
)

// FlexInt64 接受 JSON 数字或字符串形式的 64 位整数
type FlexInt64 struct {
	Value int64
}

func (f *FlexInt64) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" || s == "" {
		f.Value = 0
		return nil
	}
	// 去掉引号
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// 尝试解析浮点（如 "1.0"）
		fl, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return nil // 容忍无法解析的值
		}
		f.Value = int64(fl)
		return nil
	}
	f.Value = v
	return nil
}

// FlexInt 接受 JSON 数字或字符串形式的整数
type FlexInt struct {
	Value int
}

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" || s == "" {
		f.Value = 0
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	f.Value = v
	return nil
}

// FlexBool 接受 JSON 布尔或字符串形式的布尔
type FlexBool struct {
	Value bool
}

func (f *FlexBool) UnmarshalJSON(data []byte) error {
	s := string(data)
	switch s {
	case "true", "1", "\"true\"", "\"1\"":
		f.Value = true
		return nil
	case "false", "0", "\"false\"", "\"0\"", "null", "":
		f.Value = false
		return nil
	}
	return nil
}

var _ json.Unmarshaler = (*FlexInt64)(nil)
var _ json.Unmarshaler = (*FlexInt)(nil)
var _ json.Unmarshaler = (*FlexBool)(nil)
