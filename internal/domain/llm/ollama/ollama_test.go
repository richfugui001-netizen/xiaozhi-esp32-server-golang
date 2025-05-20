package ollama

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/llm/common"
)

// 测试Ollama提供者配置
func TestOllamaProvider(t *testing.T) {
	// 测试用例
	testCases := []struct {
		name            string
		config          map[string]interface{}
		expectBaseURL   string
		expectModelName string
	}{
		{
			name: "基本配置",
			config: map[string]interface{}{
				"model_name": "llama2",
				"base_url":   "http://example.com:11434",
			},
			expectBaseURL:   "http://example.com:11434/v1",
			expectModelName: "llama2",
		},
		{
			name: "已带v1的URL",
			config: map[string]interface{}{
				"model_name": "mistral",
				"base_url":   "http://example.com:11434/v1",
			},
			expectBaseURL:   "http://example.com:11434/v1",
			expectModelName: "mistral",
		},
		{
			name: "默认URL",
			config: map[string]interface{}{
				"model_name": "llama2",
			},
			expectBaseURL:   "http://localhost:11434/v1",
			expectModelName: "llama2",
		},
	}

	// 执行测试
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewOllamaProvider(tc.config)

			// 验证结果
			if provider.BaseURL != tc.expectBaseURL {
				t.Errorf("BaseURL = %s, 期望 %s", provider.BaseURL, tc.expectBaseURL)
			}
			if provider.ModelName != tc.expectModelName {
				t.Errorf("ModelName = %s, 期望 %s", provider.ModelName, tc.expectModelName)
			}
			if provider.HTTPClient == nil {
				t.Error("HTTPClient 不应为 nil")
			}
		})
	}
}

// 测试创建工厂
func TestOllamaFactory(t *testing.T) {
	factory := NewOllamaFactory()

	config := map[string]interface{}{
		"model_name": "llama2",
		"base_url":   "http://example.com:11434",
	}

	provider, err := factory.CreateProvider(config)
	if err != nil {
		t.Fatalf("创建提供者失败: %v", err)
	}

	// 验证返回的提供者
	ollamaProvider, ok := provider.(*OllamaProvider)
	if !ok {
		t.Fatalf("期望返回OllamaProvider类型")
	}

	if ollamaProvider.ModelName != "llama2" {
		t.Errorf("ModelName = %s, 期望 llama2", ollamaProvider.ModelName)
	}

	if ollamaProvider.BaseURL != "http://example.com:11434/v1" {
		t.Errorf("BaseURL = %s, 期望 http://example.com:11434/v1", ollamaProvider.BaseURL)
	}
}

// 模拟Ollama API的SSE响应
func mockOllamaStreamHandler(w http.ResponseWriter, r *http.Request) {
	// 验证请求方法
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var req common.LLMRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 模拟流式输出
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 发送多个SSE消息
	responses := []string{
		"我是",
		"一个由",
		"Ollama",
		"运行的",
		"大语言模型",
		"<think>这是思考内容</think>",
		"我可以",
		"帮助你解答问题",
	}

	for i, text := range responses {
		// 构造OpenAI响应
		resp := common.LLMResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", i),
			Object:  "chat.completion.chunk",
			Created: time.Now().Unix(),
			Choices: []common.Choice{
				{
					Index: 0,
					Delta: common.Delta{
						Content: text,
					},
					FinishReason: nil,
				},
			},
		}

		// 序列化为JSON
		jsonData, _ := json.Marshal(resp)

		// 发送SSE格式的消息
		fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
		flusher.Flush()

		// 适当延迟模拟真实API响应
		time.Sleep(50 * time.Millisecond)
	}

	// 发送结束标志
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// 模拟带工具调用的API响应
func mockOllamaFunctionCallHandler(w http.ResponseWriter, r *http.Request) {
	// 验证请求方法
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 验证Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// 首先发送一些常规文本
	textResp := common.LLMResponse{
		ID:      "chatcmpl-text",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Choices: []common.Choice{
			{
				Index: 0,
				Delta: common.Delta{
					Content: "我将为您查询天气信息",
				},
				FinishReason: nil,
			},
		},
	}
	jsonData, _ := json.Marshal(textResp)
	fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
	flusher.Flush()
	time.Sleep(50 * time.Millisecond)

	// 然后发送工具调用
	toolArgs := json.RawMessage(`{"location":"北京","date":"today"}`)
	toolResp := common.LLMResponse{
		ID:      "chatcmpl-tool",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Choices: []common.Choice{
			{
				Index: 0,
				Delta: common.Delta{
					ToolCalls: []common.ToolCall{
						{
							Index: 0,
							ID:    "call_123456",
							Type:  "function",
							Function: common.Function{
								Name:      "get_weather",
								Arguments: toolArgs,
							},
						},
					},
				},
				FinishReason: nil,
			},
		},
	}
	jsonData, _ = json.Marshal(toolResp)
	fmt.Fprintf(w, "data: %s\n\n", string(jsonData))
	flusher.Flush()

	// 发送结束标志
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// 测试基本响应功能
func TestResponse(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(mockOllamaStreamHandler))
	defer server.Close()

	// 创建测试用的Provider
	config := map[string]interface{}{
		"model_name": "llama2",
		"base_url":   server.URL, // 使用测试服务器URL，会自动添加/v1
	}
	provider := NewOllamaProvider(config)

	// 准备测试数据
	dialogue := []interface{}{
		map[string]string{"role": "user", "content": "介绍一下你自己"},
	}

	// 调用Response方法
	responseChan := provider.Response("test-llama", dialogue)

	// 收集响应
	var responses []string
	for text := range responseChan {
		responses = append(responses, text)
	}

	// 验证结果
	if len(responses) < 3 {
		t.Errorf("响应数量 = %d, 期望至少 3", len(responses))
	}

	// 检查思考内容是否被过滤
	for _, resp := range responses {
		if strings.Contains(resp, "<think>") || strings.Contains(resp, "</think>") {
			t.Errorf("响应中包含未过滤的think标签: %s", resp)
		}
	}

	// 组合完整响应
	fullResponse := strings.Join(responses, "")

	// 验证是否包含Ollama相关内容
	if !strings.Contains(fullResponse, "Ollama") {
		t.Errorf("响应应包含Ollama相关内容，实际为: %s", fullResponse)
	}
}

// 测试工具调用功能
func TestResponseWithFunctions(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(mockOllamaFunctionCallHandler))
	defer server.Close()

	// 创建测试用的Provider
	config := map[string]interface{}{
		"model_name": "llama2",
		"base_url":   server.URL,
	}
	provider := NewOllamaProvider(config)

	// 准备测试数据
	dialogue := []interface{}{
		map[string]string{"role": "user", "content": "北京今天天气怎么样？"},
	}
	functions := []interface{}{
		map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_weather",
				"description": "获取指定城市的天气信息",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "城市名称",
						},
						"date": map[string]interface{}{
							"type":        "string",
							"description": "日期",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	// 调用ResponseWithFunctions方法
	responseChan := provider.ResponseWithFunctions("test-llama", dialogue, functions)

	// 收集响应
	var textResponses []string
	var toolCalls []map[string]interface{}

	for resp := range responseChan {
		respMap, ok := resp.(map[string]string)
		if ok && respMap["type"] == common.ResponseTypeContent {
			textResponses = append(textResponses, respMap["content"])
		}

		toolMap, ok := resp.(map[string]interface{})
		if ok && toolMap["type"] == common.ResponseTypeToolCalls {
			toolCalls = append(toolCalls, toolMap)
		}
	}

	// 验证结果
	if len(textResponses) == 0 {
		t.Error("没有收到文本响应")
	}

	if len(toolCalls) == 0 {
		t.Error("没有收到工具调用")
	}
}

// 仅当明确指示时才运行的实际API测试
func TestRealOllamaConnection(t *testing.T) {
	// 默认跳过此测试，除非环境变量RUN_REAL_API_TEST设置为true
	if os.Getenv("RUN_REAL_API_TEST") != "true" {
		t.Skip("跳过实际API测试。设置RUN_REAL_API_TEST=true以启用")
	}

	// 使用实际API配置
	config := map[string]interface{}{
		"model_name": "llama2",                 // 使用Ollama提供的默认模型
		"base_url":   "http://localhost:11434", // 默认Ollama地址
	}
	provider := NewOllamaProvider(config)

	// 准备测试数据
	dialogue := []interface{}{
		map[string]string{"role": "user", "content": "你好，请用一句话介绍一下自己"},
	}

	// 调用Response方法
	responseChan := provider.Response("test-llama", dialogue)

	// 收集响应
	var fullResponse string
	for text := range responseChan {
		fullResponse += text
		t.Logf("收到响应片段: %s", text)
	}

	// 检查响应
	if fullResponse == "" {
		t.Error("没有收到任何响应")
	} else {
		t.Logf("完整响应: %s", fullResponse)
	}
}
