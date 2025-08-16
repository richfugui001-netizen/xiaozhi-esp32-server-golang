package main

import (
	"fmt"
	"log"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 加载配置
	cfg, err := config.LoadConfig("config/config.json")
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}

	// 连接数据库
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.Username, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("数据库连接失败:", err)
	}

	// 查询configs表数据
	var totalCount int64
	db.Model(&models.Config{}).Count(&totalCount)
	fmt.Printf("configs表总记录数: %d\n", totalCount)

	// 按类型统计
	type TypeCount struct {
		Type  string
		Count int64
	}
	var typeCounts []TypeCount
	db.Model(&models.Config{}).Select("type, COUNT(*) as count").Group("type").Scan(&typeCounts)

	fmt.Println("\n按类型统计:")
	for _, tc := range typeCounts {
		fmt.Printf("类型: %s, 数量: %d\n", tc.Type, tc.Count)
	}

	// 显示所有配置记录
	var configs []models.Config
	db.Find(&configs)
	fmt.Printf("\n所有配置记录 (共%d条):\n", len(configs))
	for _, cfg := range configs {
		fmt.Printf("ID: %d, 类型: %s, 名称: %s, 启用: %t\n", cfg.ID, cfg.Type, cfg.Name, cfg.Enabled)
	}
}