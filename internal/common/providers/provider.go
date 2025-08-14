package providers

// Provider 基础提供者接口
type Provider interface {
	Initialize() error
	Cleanup() error
}

// AsrEventListener ASR事件监听器接口
type AsrEventListener interface {
	OnAsrResult(text string) bool
}
