package common

import (
	"encoding/json"
)

// 请求与响应结构体
// Message 表示对话消息
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// LLMRequest 通用的大语言模型请求体
type LLMRequest struct {
	Model          string        `json:"model"`
	Messages       []interface{} `json:"messages"`
	Stream         bool          `json:"stream"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	Tools          []interface{} `json:"tools,omitempty"`
	Temperature    float64       `json:"temperature,omitempty"`
	EnableThinking bool          `json:"enable_thinking,omitempty"`
}

// LLMResponse 通用的大语言模型响应体
type LLMResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Choices []Choice `json:"choices"`
}

// Choice 选择
type Choice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// Delta 增量内容
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolCall 工具调用
type ToolCall struct {
	Index    int      `json:"index"`
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function 函数
type Function struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// 响应类型常量
const (
	ResponseTypeContent   = "content"
	ResponseTypeToolCalls = "tool_calls"
)

type LLMResponseStruct struct {
	Text    string
	IsStart bool
	IsEnd   bool
}
