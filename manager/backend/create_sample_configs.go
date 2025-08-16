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

	// 创建示例配置数据
	sampleConfigs := []models.Config{
		// VAD配置
		{
			Type:     "vad",
			Name:     "默认VAD配置",
			Provider: "webrtc",
			JsonData: `{"sensitivity": 0.5, "frame_duration": 30}`,
			Enabled:  true,
			IsDefault: true,
		},
		// ASR配置
		{
			Type:     "asr",
			Name:     "默认ASR配置",
			Provider: "whisper",
			JsonData: `{"model": "base", "language": "zh"}`,
			Enabled:  true,
			IsDefault: true,
		},
		// LLM配置
		{
			Type:     "llm",
			Name:     "默认LLM配置",
			Provider: "openai",
			JsonData: `{"model": "gpt-3.5-turbo", "temperature": 0.7}`,
			Enabled:  true,
			IsDefault: true,
		},
		// TTS配置
		{
			Type:     "tts",
			Name:     "默认TTS配置",
			Provider: "azure",
			JsonData: `{"voice": "zh-CN-XiaoxiaoNeural", "speed": 1.0}`,
			Enabled:  true,
			IsDefault: true,
		},
		// VLLM配置
		{
			Type:     "vllm",
			Name:     "默认VLLM配置",
			Provider: "vllm",
			JsonData: `{"model": "llama2-7b", "max_tokens": 2048}`,
			Enabled:  true,
			IsDefault: true,
		},
		// OTA配置
		{
			Type:     "ota",
			Name:     "默认OTA配置",
			Provider: "http",
			JsonData: `{"server_url": "http://localhost:8080/ota", "check_interval": 3600}`,
			Enabled:  true,
			IsDefault: true,
		},
		// MQTT配置
		{
			Type:     "mqtt",
			Name:     "默认MQTT配置",
			Provider: "mqtt",
			JsonData: `{"enable": true, "broker": "localhost", "type": "tcp", "port": 1883, "client_id": "xiaozhi_client", "username": "", "password": ""}`,
			Enabled:  true,
			IsDefault: true,
		},
		// MQTT Server配置
		{
			Type:     "mqtt_server",
			Name:     "默认MQTT服务器配置",
			Provider: "emqx",
			JsonData: `{"port": 1883, "max_connections": 1000}`,
			Enabled:  true,
			IsDefault: true,
		},
		// UDP配置
		{
			Type:     "udp",
			Name:     "默认UDP配置",
			Provider: "udp",
			JsonData: `{"port": 8888, "buffer_size": 1024}`,
			Enabled:  true,
			IsDefault: true,
		},
	}

	// 插入示例配置数据
	for _, config := range sampleConfigs {
		if err := db.Create(&config).Error; err != nil {
			log.Printf("Failed to create %s config: %v", config.Type, err)
		} else {
			log.Printf("Created %s config: %s", config.Type, config.Name)
		}
	}

	fmt.Println("示例配置数据创建完成！")
}