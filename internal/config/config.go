// Package config 提供从环境变量读取配置的能力。
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 服务配置。
type Config struct {
	ListenAddr string        // HTTP 监听地址
	APIKeys    []string      // 允许的 API Key 列表
	PoolSize   int           // 通达信连接池大小
	Timeout    time.Duration // 单次请求通达信的超时时间
	Debug      bool          // 是否输出底层调试日志
}

// FromEnv 从环境变量构造配置。
func FromEnv() Config {
	c := Config{
		ListenAddr: ":8080",
		PoolSize:   4,
		Timeout:    30 * time.Second,
		Debug:      false,
	}

	if v := strings.TrimSpace(os.Getenv("TDX_LISTEN_ADDR")); v != "" {
		c.ListenAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("TDX_API_KEYS")); v != "" {
		for _, k := range strings.Split(v, ",") {
			if k = strings.TrimSpace(k); k != "" {
				c.APIKeys = append(c.APIKeys, k)
			}
		}
	}
	if v := strings.TrimSpace(os.Getenv("TDX_POOL_SIZE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.PoolSize = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("TDX_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.Timeout = d
		}
	}
	if v := strings.TrimSpace(os.Getenv("TDX_DEBUG")); v != "" {
		c.Debug = strings.EqualFold(v, "true") || v == "1"
	}
	return c
}
