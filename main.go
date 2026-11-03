package main

import (
	"log"
	"net/http"
	"time"

	"github.com/Garretqaq/tdx-api/internal/api"
	"github.com/Garretqaq/tdx-api/internal/config"
	"github.com/Garretqaq/tdx-api/internal/tdx"
)

func main() {
	config.LoadEnvFile(".env")
	cfg := config.FromEnv()

	if len(cfg.APIKeys) == 0 {
		log.Println("警告: 未配置 TDX_API_KEYS，所有请求将免鉴权放行")
	}

	log.Printf("正在连接通达信服务器，连接池大小 %d...", cfg.PoolSize)
	cli, err := tdx.NewClient(cfg.PoolSize, cfg.Timeout)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.New(cli, cfg.APIKeys),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("服务已启动，监听 %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务异常退出: %v", err)
	}
}
