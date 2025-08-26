package database

import (
	"fmt"
	"log"
	"xiaozhi/manager/backend/config"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func Init(cfg config.DatabaseConfig) *gorm.DB {
	var db *gorm.DB
	var err error

	if cfg.Database == "sqlite" {
		// SQLite 数据库连接
		log.Println("使用SQLite数据库:", cfg.Host)
		db, err = gorm.Open(sqlite.Open(cfg.Host), &gorm.Config{})
	} else {
		// MySQL 数据库连接
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}

	if err != nil {
		log.Println("数据库连接失败:", err)
		log.Println("将使用fallback模式运行（硬编码用户验证）")
		return nil
	}

	log.Println("数据库连接成功")

	// 注意：不再自动创建表结构和默认管理员用户
	// 这些操作现在由引导页面通过API接口来处理
	log.Println("数据库连接成功，等待引导页面初始化...")

	return db
}

func Close(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		log.Println("获取数据库连接失败:", err)
		return
	}
	sqlDB.Close()
}
