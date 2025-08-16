package main

import (
	"flag"
	"log"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/database"
	"xiaozhi/manager/backend/router"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	// 定义命令行参数
	var configFile string
	var resetDB bool
	flag.StringVar(&configFile, "config", "manager/backend/config/config.json", "配置文件路径")
	flag.StringVar(&configFile, "c", "manager/backend/config/config.json", "配置文件路径 (简写)")
	flag.BoolVar(&resetDB, "reset-db", false, "重置数据库表（删除所有数据）")
	flag.Parse()

	// 加载配置
	cfg := config.LoadFromFile(configFile)

	// 初始化数据库
	var db *gorm.DB
	if resetDB {
		log.Println("使用数据库重置模式")
		db = database.InitWithReset(cfg.Database)
	} else {
		db = database.Init(cfg.Database)
	}
	defer database.Close(db)

	// 设置Gin模式
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化路由
	r := router.Setup(db)

	// 启动服务器
	log.Printf("使用配置文件: %s", configFile)
	log.Printf("服务器启动在端口: %s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal("服务器启动失败:", err)
	}
}
