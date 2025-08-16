package main

import (
	"fmt"
	"log"
	"strconv"
	"xiaozhi/manager/backend/config"
	"xiaozhi/manager/backend/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// 读取配置文件
	cfg := config.LoadFromFile("config/config.json")

	// 连接数据库
	port, _ := strconv.Atoi(cfg.Database.Port)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		port,
		cfg.Database.Database)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 删除所有旧的MQTT配置
	result := db.Where("type = ?", "mqtt").Delete(&models.Config{})
	if result.Error != nil {
		log.Printf("Failed to delete old mqtt configs: %v", result.Error)
	} else {
		log.Printf("Deleted %d old mqtt configs", result.RowsAffected)
	}

	// 创建新的MQTT配置
	mqttConfig := models.Config{
		Type:     "mqtt",
		Name:     "默认MQTT配置",
		Provider: "mqtt",
		JsonData: `{"enable": true, "broker": "localhost", "type": "tcp", "port": 1883, "client_id": "xiaozhi_client", "username": "", "password": ""}`,
		Enabled:  true,
		IsDefault: true,
	}

	if err := db.Create(&mqttConfig).Error; err != nil {
		log.Printf("Failed to create mqtt config: %v", err)
	} else {
		log.Printf("Created new mqtt config: %s", mqttConfig.Name)
	}

	fmt.Println("MQTT配置更新完成！")
}