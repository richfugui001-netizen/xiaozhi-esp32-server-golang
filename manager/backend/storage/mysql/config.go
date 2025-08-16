package mysql

import (
	"fmt"
	"xiaozhi/manager/backend/config"
)

// Config MySQL配置
type Config struct {
	Host            string `json:"host"`
	Port            string `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Database        string `json:"database"`
	MaxIdleConns    int    `json:"max_idle_conns"`
	MaxOpenConns    int    `json:"max_open_conns"`
	ConnMaxLifetime int    `json:"conn_max_lifetime"`
}

// NewConfigFromDatabase 从数据库配置创建MySQL配置
func NewConfigFromDatabase(dbConfig config.DatabaseConfig) *Config {
	return &Config{
		Host:            dbConfig.Host,
		Port:            dbConfig.Port,
		Username:        dbConfig.Username,
		Password:        dbConfig.Password,
		Database:        dbConfig.Database,
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: 3600,
	}
}

// DSN 生成数据源名称
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Username, c.Password, c.Host, c.Port, c.Database)
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("MySQL host is required")
	}
	if c.Port == "" {
		return fmt.Errorf("MySQL port is required")
	}
	if c.Username == "" {
		return fmt.Errorf("MySQL username is required")
	}
	if c.Database == "" {
		return fmt.Errorf("MySQL database name is required")
	}
	return nil
}

// ValidateConfig 验证MySQL配置
func ValidateConfig(config config.DatabaseConfig) error {
	if config.Host == "" {
		return fmt.Errorf("MySQL host is required")
	}
	if config.Port == "" {
		return fmt.Errorf("MySQL port is required")
	}
	if config.Username == "" {
		return fmt.Errorf("MySQL username is required")
	}
	if config.Database == "" {
		return fmt.Errorf("MySQL database name is required")
	}
	return nil
}