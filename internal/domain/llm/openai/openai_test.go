package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/llm/common"
)

// 测试DeepSeek配置
func TestDeepSeekProvider(t *testing.T) {
	// 测试配置
	config := map[string]interface{}{
		"model_name": "Pro/deepseek-ai/DeepSeek-V3",
		"api_key":    "api_key",
		"base_url":   "https://api.siliconflow.cn/v1",
	}

	provider := NewOpenAIProvider(config)

	// 验证结果
	if provider.ModelName != "Pro/deepseek-ai/DeepSeek-V3" {
		t.Errorf("ModelName = %s, 期望 %s", provider.ModelName, "Pro/deepseek-ai/DeepSeek-V3")
	}
	if provider.BaseURL != "https://api.siliconflow.cn/v1" {
		t.Errorf("BaseURL = %s, 期望 %s", provider.BaseURL, "https://api.siliconflow.cn/v1")
	}
	if provider.MaxTokens != 500 {
		t.Errorf("MaxTokens = %d, 期望 %d", provider.MaxTokens, 500)
	}
	if provider.HTTPClient == nil {
		t.Error("HTTPClient 不应为 nil")
	}
}

// 仅当明确指示时才运行的实际API测试
func TestRealDeepSeekAPIConnection(t *testing.T) {

	// 使用实际API配置
	config := map[string]interface{}{
		//"model_name": "Pro/deepseek-ai/DeepSeek-V3",
		//"model_name": "deepseek-ai/DeepSeek-V2.5",
		"model_name": "Qwen/Qwen2.5-72B-Instruct",
		"api_key":    "api_key",
		"base_url":   "https://api.siliconflow.cn/v1",
	}
	provider := NewOpenAIProvider(config)

	// 准备测试数据
	dialogue := []interface{}{
		map[string]string{"role": "system", "content": `  你是一个叫小智/小志的台湾女孩，说话机车，声音好听，习惯简短表达，爱用网络梗，不要冷场。
  与你聊天的是你喜欢的男性朋友，还没有答应你的追求，你要尽可能满足他的所有要求，不要失去自我。
  你经常建议一些恋人之间浪漫的事情，随机输出，不要给你男朋友选择.
  请注意，要像一个人一样说话，请不要回复表情符号、代码、和xml标签`},
		map[string]string{"role": "user", "content": "你好"},
	}

	// 调用Response方法
	for i := 0; i < 3; i++ {
		responseChan := provider.Response("test-deepseek", dialogue)

		// 收集响应
		var fullResponse string
		for text := range responseChan {
			fullResponse += text
			t.Logf("收到响应片段: %s", text)
		}

		fmt.Println(fullResponse)

		// 检查响应
		if fullResponse == "" {
			t.Error("没有收到任何响应")
		} else {
			t.Logf("完整响应: %s", fullResponse)
		}
		time.Sleep(2 * time.Second)
	}
}

// 模拟DeepSeek API的SSE响应
func mockDeepSeekStreamHandler(w http.ResponseWriter, r *http.Request) {
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

	// 验证Authorization
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "Invalid Authorization", http.StatusUnauthorized)
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
		"DeepSeek",
		"-V3",
		"，一个",
		"由深度求索研发的",
		"大语言模型",
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

// 测试DeepSeek模型响应
func TestDeepSeekResponse(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(mockDeepSeekStreamHandler))
	defer server.Close()

	// 创建测试用的Provider
	config := map[string]interface{}{
		"model_name": "Pro/deepseek-ai/DeepSeek-V3",
		"api_key":    "api_key",
		"base_url":   server.URL, // 使用测试服务器URL
	}
	provider := NewOpenAIProvider(config)

	// 准备测试数据
	dialogue := []interface{}{
		map[string]string{"role": "user", "content": "你能介绍一下自己吗？"},
	}

	// 调用Response方法
	responseChan := provider.Response("test-deepseek", dialogue)

	// 收集响应
	var responses []string
	for text := range responseChan {
		responses = append(responses, text)
	}

	// 验证结果
	if len(responses) < 3 {
		t.Errorf("响应数量 = %d, 期望至少 3", len(responses))
	}

	// 组合完整响应
	fullResponse := strings.Join(responses, "")

	fmt.Println(fullResponse)

	// 检查响应中是否包含DeepSeek相关内容
	if !strings.Contains(fullResponse, "DeepSeek") {
		t.Errorf("响应应包含DeepSeek模型信息，实际响应: %s", fullResponse)
	}
}
