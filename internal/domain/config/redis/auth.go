package redis_config

import (
	"context"
	"xiaozhi-esp32-server-golang/internal/domain/config/types"
)

func (r *UserConfig) IsDeviceActivated(ctx context.Context, deviceId string, clientId string) (bool, error) {
	return false, nil
}

// code, challenge, msg, timeoutMs
func (r *UserConfig) GetActivationInfo(ctx context.Context, deviceId string, clientId string) (int, string, string, int) {
	return 1234, "challenge", "msg", 300
}

func (r *UserConfig) VerifyChallenge(ctx context.Context, deviceId string, clientId string, activationPayload types.ActivationPayload) (bool, error) {
	return true, nil
}
