package main

import (
	"tiny-feed/internal/config"
	"tiny-feed/internal/db"
	apphttp "tiny-feed/internal/http"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func main() {
	// 加载 .env（本地开发）
	if err := godotenv.Load(); err != nil {
		log.Println("未找到 .env 文件，继续启动")
	}

	// 加载配置
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}
	log.Printf("正在加载配置：%s", configPath)
	cfg, usedDefault, err := config.LoadLocalDev(configPath)
	if err != nil {
		log.Fatalf("加载配置失败：%v", err)
	}
	if usedDefault {
		log.Printf("配置文件 %s 不存在，使用默认本地配置", configPath)
	} else {
		log.Printf("已从文件加载配置：%s", configPath)
	}

	// 连接数据库
	sqlDB, err := db.NewDB(cfg.Database)
	if err != nil {
		log.Fatalf("连接数据库失败：%v", err)
	}
	if err := db.AutoMigrate(sqlDB); err != nil {
		log.Fatalf("数据库自动迁移失败：%v", err)
	}
	defer db.CloseDB(sqlDB)

	// 设置路由
	r := apphttp.SetRouter(sqlDB)
	log.Printf("服务已启动，监听端口：%d", cfg.Server.Port)
	if err := r.Run(":" + strconv.Itoa(cfg.Server.Port)); err != nil {
		log.Fatalf("服务运行失败：%v", err)
	}
}
