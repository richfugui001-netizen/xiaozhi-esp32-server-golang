package user_config

import (
	"context"
	"fmt"

	userconfig_memory "xiaozhi-esp32-server-golang/internal/domain/user_config/memory"
	userconfig_redis "xiaozhi-esp32-server-golang/internal/domain/user_config/redis"
	"xiaozhi-esp32-server-golang/internal/domain/user_config/types"
)

// UserConfigProvider 用户配置提供者接口
// 这是一个扩展的接口，支持更多操作，区别于原有的UserConfig接口
type UserConfigProvider interface {
	// GetUserConfig 获取用户配置（兼容原有接口）
	GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)
}

// UserConfigFactory 用户配置工厂接口
type UserConfigFactory interface {
	// CreateProvider 根据配置创建用户配置提供者
	CreateProvider(config map[string]interface{}) (UserConfigProvider, error)
}

// Config 用户配置提供者配置结构
type Config struct {
	Type       string                 `json:"type"`       // 存储类型: "redis", "memory", "file"
	Parameters map[string]interface{} `json:"parameters"` // 存储相关配置参数
}

func GetProvider() (UserConfigProvider, error) {
	provider, err := GetUserConfigProvider("redis", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	return provider, nil
}

// GetUserConfigProvider 创建用户配置提供者
// 根据传入的存储类型和配置参数创建对应的提供者实例
// providerType: 提供者类型，支持 "redis", "memory", "file"
// config: 提供者配置参数
// 返回UserConfigProvider接口，支持完整的CRUD操作
func GetUserConfigProvider(providerType string, config map[string]interface{}) (UserConfigProvider, error) {
	if config == nil {
		config = make(map[string]interface{})
	}

	switch providerType {
	case "redis":
		// 创建Redis用户配置提供者
		provider, err := userconfig_redis.NewRedisUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("创建Redis用户配置提供者失败: %v", err)
		}
		return provider, nil
	case "memory":
		// 创建内存用户配置提供者
		provider, err := userconfig_memory.NewMemoryUserConfigProvider(config)
		if err != nil {
			return nil, fmt.Errorf("创建内存用户配置提供者失败: %v", err)
		}
		return provider, nil
	case "file":
		// TODO: 创建文件用户配置提供者
		return nil, fmt.Errorf("文件用户配置提供者暂未实现")
	default:
		return nil, fmt.Errorf("不支持的用户配置提供者: %s", providerType)
	}
}

// ValidateConfig 验证配置参数
func ValidateConfig(providerType string, config map[string]interface{}) error {
	switch providerType {
	case "redis":
		// Redis 配置验证
		if config["host"] == nil {
			return fmt.Errorf("Redis配置缺少host参数")
		}
		if config["port"] == nil {
			return fmt.Errorf("Redis配置缺少port参数")
		}
	case "memory":
		// 内存配置无需特殊验证
		return nil
	case "file":
		// 文件配置验证
		if config["path"] == nil {
			return fmt.Errorf("文件配置缺少path参数")
		}
	default:
		return fmt.Errorf("不支持的提供者类型: %s", providerType)
	}
	return nil
}

// DefaultConfig 获取默认配置
func DefaultConfig(providerType string) map[string]interface{} {
	switch providerType {
	case "redis":
		return map[string]interface{}{
			"host":     "localhost",
			"port":     6379,
			"password": "",
			"db":       0,
			"prefix":   "xiaozhi",
		}
	case "memory":
		return map[string]interface{}{
			"max_entries": 1000,
		}
	case "file":
		return map[string]interface{}{
			"path":   "./data/user_config.json",
			"format": "json",
		}
	default:
		return make(map[string]interface{})
	}
}

// NewUserConfigAdapter 创建适配器，将UserConfigProvider适配为原有的UserConfig接口
// 这样可以保持向后兼容性
func NewUserConfigAdapter(provider UserConfigProvider) UserConfig {
	return &userConfigAdapter{provider: provider}
}

// userConfigAdapter 适配器实现
type userConfigAdapter struct {
	provider UserConfigProvider
}

// GetUserConfig 实现原有UserConfig接口
func (a *userConfigAdapter) GetUserConfig(ctx context.Context, userID string) (types.UConfig, error) {
	return a.provider.GetUserConfig(ctx, userID)
}
