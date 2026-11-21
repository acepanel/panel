package app

import (
	"sync/atomic"
	"time"
)

// StartTime 面板启动时间
var StartTime = time.Now()

// 面板状态常量
const (
	StatusNormal int32 = iota
	StatusUpgrade
)

// 面板全局变量
var (
	Key    string       // 密钥
	Root   string       // 根目录
	Locale string       // 语言
	IsCli  bool         // 是否命令行
	Status atomic.Int32 // 面板状态，零值即 StatusNormal
)

// 自动注入
var (
	Version    = "0.0.0"
	BuildTime  string
	CommitHash string
	GoVersion  string
	BuildID    string
	BuildUser  string
	BuildHost  string
)
