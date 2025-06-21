package user_config

import (
	"context"
)

type UserConfig interface {
	//GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)
	GetAsrConfig(ctx context.Context, userID string) (map[string]interface{}, error)
	GetTtsConfig(ctx context.Context, userID string) (map[string]interface{}, error)
	GetLlmConfig(ctx context.Context, userID string) (map[string]interface{}, error)
}
