package userconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

var (
	userConfigInstance *UserConfig
	once               sync.Once
)

type UserConfig struct {
	redisInstance *redis.Client
	prefix        string
}

func InitUserConfig(redisOptions *redis.Options, prefix string) error {
	var initErr error
	once.Do(func() {
		if redisOptions == nil {
			initErr = fmt.Errorf("redis options cannot be nil")
			return
		}

		client := redis.NewClient(redisOptions)
		// 测试 Redis 连接
		if err := client.Ping(context.Background()).Err(); err != nil {
			initErr = fmt.Errorf("failed to connect to redis: %w", err)
			return
		}

		userConfigInstance = &UserConfig{
			redisInstance: client,
			prefix:        prefix,
		}
	})

	return initErr
}

func U() *UserConfig {
	return userConfigInstance
}

type AsrConfig struct {
	Type string `json:"type"`
}

type TtsConfig struct {
	Type string `json:"type"`
}

type LlmConfig struct {
	Type string `json:"type"`
}

type UConfig struct {
	SystemPrompt string    `json:"system_prompt"`
	Asr          AsrConfig `json:"asr"`
	Tts          TtsConfig `json:"tts"`
	Llm          LlmConfig `json:"llm"`
}

func (u *UConfig) getTTsType() string {
	ttsType := u.Tts.Type
	if ttsType == "" {
		ttsType = "local"
	}
	return ttsType
}

func (u *UserConfig) GetUserConfig(ctx context.Context, userID string) (UConfig, error) {
	key := u.GetUserConfigKey(userID)
	//hgetall 拿到所有的
	userConfig, err := u.redisInstance.HGetAll(ctx, key).Result()
	if err != nil {
		return UConfig{}, err
	}

	ret := UConfig{}
	//将UserConfig转换成UConfig结构
	for k, v := range userConfig {
		if k == "llm" {
			llmConfig := LlmConfig{}
			err = json.Unmarshal([]byte(v), &llmConfig)
			if err != nil {
				return UConfig{}, err
			}
			ret.Llm = llmConfig
		} else if k == "tts" {
			ttsConfig := TtsConfig{}
			err = json.Unmarshal([]byte(v), &ttsConfig)
			if err != nil {
				return UConfig{}, err
			}
			ret.Tts = ttsConfig
		} else if k == "asr" {
			asrConfig := AsrConfig{}
			err = json.Unmarshal([]byte(v), &asrConfig)
			if err != nil {
				return UConfig{}, err
			}
			ret.Asr = asrConfig
		}
	}
	return ret, nil
}

func (u *UserConfig) GetUserConfigKey(deviceId string) string {
	return fmt.Sprintf("%s:userconfig:%s", u.prefix, deviceId)
}
