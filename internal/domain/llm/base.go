package llm

import (
	"context"
	"fmt"

	"xiaozhi-esp32-server-golang/internal/domain/llm/ollama"
	"xiaozhi-esp32-server-golang/internal/domain/llm/openai"
)

// LLMProvider 大语言模型提供者接口
// 所有LLM实现必须遵循此接口
type LLMProvider interface {
	// Response 生成文本响应，返回一个字符串通道
	// sessionID: 会话标识符，用于跟踪请求
	// dialogue: 对话历史，包含用户和模型的消息
	Response(sessionID string, dialogue []interface{}) chan string

	// ResponseWithFunctions 生成带工具调用的响应，返回一个接口通道
	// sessionID: 会话标识符，用于跟踪请求
	// dialogue: 对话历史，包含用户和模型的消息
	// functions: 可用的工具/函数定义
	ResponseWithFunctions(sessionID string, dialogue []interface{}, functions interface{}) chan interface{}

	// ResponseWithContext 带有上下文控制的响应，支持取消操作
	// ctx: 上下文，可用于取消长时间运行的请求
	// sessionID: 会话标识符
	// dialogue: 对话历史
	ResponseWithContext(ctx context.Context, sessionID string, dialogue []interface{}) chan string

	// GetModelInfo 获取模型信息
	// 返回模型名称和其他元数据
	GetModelInfo() map[string]interface{}
}

// LLMFactory 大语言模型工厂接口
// 用于创建不同类型的LLM提供者
type LLMFactory interface {
	// CreateProvider 根据配置创建LLM提供者
	CreateProvider(config map[string]interface{}) (LLMProvider, error)
}

func GetLLMProvider(providerName string, config map[string]interface{}) (LLMProvider, error) {
	llmType := config["type"].(string)
	switch llmType {
	case "openai":
		return openai.NewOpenAIProvider(config), nil
	case "ollama":
		return ollama.NewOllamaProvider(config), nil
	}
	return nil, fmt.Errorf("不支持的LLM提供者: %s", providerName)
}

// Config LLM配置结构
type Config struct {
	ModelName  string                 `json:"model_name"`
	APIKey     string                 `json:"api_key"`
	BaseURL    string                 `json:"base_url"`
	MaxTokens  int                    `json:"max_tokens"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// TextMessage 文本消息结构
type TextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// FunctionCall 函数调用结构
type FunctionCall struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

// NewTextMessage 创建新的文本消息
func NewTextMessage(role, content string) TextMessage {
	return TextMessage{
		Role:    role,
		Content: content,
	}
}

// NewUserMessage 创建用户消息
func NewUserMessage(content string) TextMessage {
	return NewTextMessage("user", content)
}

// NewAssistantMessage 创建助手消息
func NewAssistantMessage(content string) TextMessage {
	return NewTextMessage("assistant", content)
}

// NewSystemMessage 创建系统消息
func NewSystemMessage(content string) TextMessage {
	return NewTextMessage("system", content)
}
