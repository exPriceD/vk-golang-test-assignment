package main

import (
	"flag"
	"fmt"
	"go.uber.org/zap"
	"vk-worker-pool/internal/config"
	"vk-worker-pool/internal/logger"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "путь до конфиг-файла")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Ошибка загрузки конфигурации: %v\n", err)
		return
	}
	fmt.Println(cfg)

	log, err := logger.NewLogger(cfg.Logger)
	if err != nil {
		fmt.Printf("Ошибка создания логгера: %v\n", err)
		return
	}
	defer func(log *zap.Logger) {
		_ = log.Sync()
	}(log)
	log.Info("Test INFO msg")
}
