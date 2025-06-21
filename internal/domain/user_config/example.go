package user_config

import (
	"context"
	"fmt"
	"log"

	"xiaozhi-esp32-server-golang/internal/domain/user_config/types"
)

// ExampleUsage 展示如何使用用户配置provider
func ExampleUsage() {
	ctx := context.Background()

	// 示例1: 使用Redis provider
	fmt.Println("=== Redis Provider 示例 ===")
	redisConfig := map[string]interface{}{
		"host":     "localhost",
		"port":     6379,
		"password": "",
		"db":       0,
		"prefix":   "xiaozhi",
	}

	redisProvider, err := GetUserConfigProvider("redis", redisConfig)
	if err != nil {
		log.Printf("创建Redis provider失败: %v", err)
	} else {
		// 测试配置操作
		testProvider(ctx, redisProvider, "redis_user_123")
		redisProvider.Close()
	}

	// 示例2: 使用Memory provider
	fmt.Println("\n=== Memory Provider 示例 ===")
	memoryConfig := map[string]interface{}{
		"max_entries": 500,
	}

	memoryProvider, err := GetUserConfigProvider("memory", memoryConfig)
	if err != nil {
		log.Printf("创建Memory provider失败: %v", err)
	} else {
		// 测试配置操作
		testProvider(ctx, memoryProvider, "memory_user_456")
		memoryProvider.Close()
	}

	// 示例3: 使用适配器保持向后兼容
	fmt.Println("\n=== 适配器模式示例 ===")
	if memoryProvider != nil {
		// 将Provider适配为原有的UserConfig接口
		userConfig := NewUserConfigAdapter(memoryProvider)
		config, err := userConfig.GetUserConfig(ctx, "memory_user_456")
		if err != nil {
			log.Printf("通过适配器获取配置失败: %v", err)
		} else {
			fmt.Printf("通过适配器获取的配置: %+v\n", config)
		}
	}
}

// testProvider 测试provider的基本功能
func testProvider(ctx context.Context, provider UserConfigProvider, userID string) {
	// 1. 设置用户配置
	config := types.UConfig{
		SystemPrompt: "你是一个有用的AI助手",
		Llm: types.LlmConfig{
			Type: "openai",
		},
		Tts: types.TtsConfig{
			Type: "edge",
		},
		Asr: types.AsrConfig{
			Type: "funasr",
		},
	}

	err := provider.SetUserConfig(ctx, userID, config)
	if err != nil {
		log.Printf("设置用户配置失败: %v", err)
		return
	}
	fmt.Printf("✓ 用户 %s 配置设置成功\n", userID)

	// 2. 获取用户配置
	retrievedConfig, err := provider.GetUserConfig(ctx, userID)
	if err != nil {
		log.Printf("获取用户配置失败: %v", err)
		return
	}
	fmt.Printf("✓ 获取到用户配置: %+v\n", retrievedConfig)

	// 3. 删除用户配置
	err = provider.DeleteUserConfig(ctx, userID)
	if err != nil {
		log.Printf("删除用户配置失败: %v", err)
		return
	}
	fmt.Printf("✓ 用户 %s 配置删除成功\n", userID)
}

// GetProviderWithDefaultConfig 使用默认配置创建provider的便捷方法
func GetProviderWithDefaultConfig(providerType string) (UserConfigProvider, error) {
	defaultConfig := DefaultConfig(providerType)
	return GetUserConfigProvider(providerType, defaultConfig)
}

// ValidateAndCreateProvider 验证配置并创建provider
func ValidateAndCreateProvider(providerType string, config map[string]interface{}) (UserConfigProvider, error) {
	// 首先验证配置
	if err := ValidateConfig(providerType, config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	// 创建provider
	return GetUserConfigProvider(providerType, config)
}
