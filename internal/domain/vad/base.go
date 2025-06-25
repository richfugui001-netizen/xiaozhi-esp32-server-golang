package vad

import (
	"errors"
	"xiaozhi-esp32-server-golang/internal/domain/vad/inter"
	"xiaozhi-esp32-server-golang/internal/domain/vad/silero_vad"
	"xiaozhi-esp32-server-golang/internal/domain/vad/webrtc_vad"
)

func AcquireVAD(provider string, config map[string]interface{}) (inter.VAD, error) {
	switch provider {
	case "silero_vad":
		return silero_vad.AcquireVAD(config)
	case "webrtc_vad":
		return webrtc_vad.AcquireVAD(config)
	default:
		return nil, errors.New("invalid vad provider")
	}
}

func ReleaseVAD(vad inter.VAD) error {
	//根据vad的类型，调用对应的ReleaseVAD方法
	switch vad.(type) {
	case *webrtc_vad.WebRTCVAD:
		return webrtc_vad.ReleaseVAD(vad)
	case *silero_vad.SileroVAD:
		return silero_vad.ReleaseVAD(vad)
	default:
		return errors.New("invalid vad type")
	}
	return nil
}
