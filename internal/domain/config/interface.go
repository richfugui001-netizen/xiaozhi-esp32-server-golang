package user_config

import (
	"context"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
)

// UserConfigProvider 用户配置提供者接口
// 这是一个扩展的接口，支持更多操作，区别于原有的UserConfig接口
type UserConfigProvider interface {
	//auth

	//llm memory

	// GetUserConfig 获取用户配置（兼容原有接口）
	GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)
}
