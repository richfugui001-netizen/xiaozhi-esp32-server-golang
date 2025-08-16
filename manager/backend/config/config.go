package config

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	JWT      JWTConfig      `json:"jwt"`
}

type ServerConfig struct {
	Port string `json:"port"`
	Mode string `json:"mode"`
}

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type JWTConfig struct {
	Secret     string `json:"secret"`
	ExpireHour int    `json:"expire_hour"`
}

func Load() *Config {
	return LoadFromFile("manager/backend/config/config.json")
}

func LoadFromFile(configPath string) *Config {
	file, err := os.Open(configPath)
	if err != nil {
		log.Fatalf("无法打开配置文件 %s: %v", configPath, err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		log.Fatalf("解析配置文件失败 %s: %v", configPath, err)
	}

	return &config
}
