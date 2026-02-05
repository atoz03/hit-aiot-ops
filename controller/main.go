package main

import (
	"context"
	"log"
	"time"
)

func main() {
	args := parseArgs()

	cfgPath := args.configPath
	if cfgPath == "" {
		p, err := defaultConfigPath()
		if err != nil {
			log.Fatalf("加载配置失败：%v", err)
		}
		cfgPath = p
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("读取配置失败：%v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置校验失败：%v", err)
	}

	store, err := NewStore(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败：%v", err)
	}
	defer func() { _ = store.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.ApplyMigrations(ctx, cfg.MigrationDir); err != nil {
		log.Fatalf("数据库迁移失败：%v", err)
	}

	srv := NewServer(cfg, store)
	r := srv.Router()

	log.Printf("控制器启动：listen=%s dry_run=%v", cfg.ListenAddr, cfg.DryRun)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("服务启动失败：%v", err)
	}
}
