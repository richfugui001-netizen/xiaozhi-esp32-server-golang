package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/llm/common"
	log "xiaozhi-esp32-server-golang/logger"
)

// 连接池配置
const (
	maxIdleConns        = 100
	maxIdleConnsPerHost = 10
	idleConnTimeout     = 90 * time.Second
	requestTimeout      = 30 * time.Second
)

// 全局HTTP客户端，用于所有Ollama请求
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// getHTTPClient 返回配置了连接池的HTTP客户端
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConnsPerHost,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}

		httpClient = &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
		}
	})

	return httpClient
}

// OllamaProvider Ollama大语言模型提供者
type OllamaProvider struct {
	ModelName  string
	BaseURL    string
	HTTPClient *http.Client
}

// NewOllamaProvider 创建新的Ollama提供者
func NewOllamaProvider(config map[string]interface{}) *OllamaProvider {
	modelName, _ := config["model_name"].(string)

	// 默认使用本地Ollama服务
	baseURL := "http://localhost:11434"
	if url, ok := config["base_url"].(string); ok && url != "" {
		baseURL = url
	}

	// 如果没有v1，增加v1
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = fmt.Sprintf("%s/v1", baseURL)
	}

	log.Infof("创建Ollama提供者: 模型=%s, URL=%s", modelName, baseURL)

	return &OllamaProvider{
		ModelName:  modelName,
		BaseURL:    baseURL,
		HTTPClient: getHTTPClient(), // 使用连接池
	}
}

// GetModelInfo 获取模型信息
func (p *OllamaProvider) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"model_name": p.ModelName,
		"base_url":   p.BaseURL,
		"type":       "ollama",
	}
}

// Response 生成响应
func (p *OllamaProvider) Response(sessionID string, dialogue []interface{}) chan string {
	return p.ResponseWithContext(context.Background(), sessionID, dialogue)
}

// ResponseWithContext 带有上下文控制的响应，支持取消操作
func (p *OllamaProvider) ResponseWithContext(ctx context.Context, sessionID string, dialogue []interface{}) chan string {
	responseChan := make(chan string)

	go func() {
		defer close(responseChan)

		reqBody := common.LLMRequest{
			Model:    p.ModelName,
			Messages: dialogue,
			Stream:   true,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			log.Errorf("序列化请求失败: %v", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Errorf("创建请求失败: %v", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		// Ollama不需要API密钥，但使用OpenAI兼容接口时，需要提供一个占位符
		req.Header.Set("Authorization", "Bearer ollama")
		// 添加追踪标识
		req.Header.Set("X-Session-ID", sessionID)

		startTime := time.Now()
		resp, err := p.HTTPClient.Do(req)
		if err != nil {
			log.Errorf("发送请求失败: %v", err)
			return
		}
		defer resp.Body.Close()

		log.Debugf("Ollama API请求耗时: %v", time.Since(startTime))

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Ollama API响应错误: %d %s", resp.StatusCode, string(body))
			return
		}

		reader := bufio.NewReader(resp.Body)
		isActive := true

		for {
			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				log.Warnf("请求已取消: %v", ctx.Err())
				return
			default:
				// 继续处理
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Errorf("读取流失败: %v", err)
				}
				break
			}

			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk common.LLMResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.Errorf("解析响应失败: %v", err)
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			content := chunk.Choices[0].Delta.Content
			if content == "" {
				continue
			}

			// 处理标签跨多个chunk的情况
			if strings.Contains(content, "<think>") {
				isActive = false
				parts := strings.Split(content, "<think>")
				if parts[0] != "" && isActive {
					responseChan <- parts[0]
				}
			} else if strings.Contains(content, "</think>") {
				isActive = true
				parts := strings.Split(content, "</think>")
				if len(parts) > 1 && parts[1] != "" {
					responseChan <- parts[1]
				}
			} else if isActive {
				responseChan <- content
			}
		}
	}()

	return responseChan
}

// ResponseWithFunctions 带函数调用的响应
func (p *OllamaProvider) ResponseWithFunctions(sessionID string, dialogue []interface{}, functions interface{}) chan interface{} {
	responseChan := make(chan interface{})

	go func() {
		defer close(responseChan)

		reqBody := common.LLMRequest{
			Model:    p.ModelName,
			Messages: dialogue,
			Stream:   true,
			Tools:    functions.([]interface{}),
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			log.Errorf("序列化请求失败: %v", err)
			responseChan <- map[string]string{
				"type":    common.ResponseTypeContent,
				"content": fmt.Sprintf("【Ollama服务请求异常: %v】", err),
			}
			return
		}

		ctx := context.Background()
		req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Errorf("创建请求失败: %v", err)
			responseChan <- map[string]string{
				"type":    common.ResponseTypeContent,
				"content": fmt.Sprintf("【Ollama服务请求异常: %v】", err),
			}
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer ollama")
		req.Header.Set("X-Session-ID", sessionID)

		startTime := time.Now()
		resp, err := p.HTTPClient.Do(req)
		if err != nil {
			log.Errorf("发送请求失败: %v", err)
			responseChan <- map[string]string{
				"type":    common.ResponseTypeContent,
				"content": fmt.Sprintf("【Ollama服务请求异常: %v】", err),
			}
			return
		}
		defer resp.Body.Close()

		log.Debugf("Ollama函数调用API请求耗时: %v", time.Since(startTime))

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Errorf("Ollama API响应错误: %d %s", resp.StatusCode, string(body))
			responseChan <- map[string]string{
				"type":    common.ResponseTypeContent,
				"content": fmt.Sprintf("【Ollama服务响应异常: 状态码 %d】", resp.StatusCode),
			}
			return
		}

		reader := bufio.NewReader(resp.Body)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					log.Errorf("读取流失败: %v", err)
				}
				break
			}

			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk common.LLMResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				log.Errorf("解析响应失败: %v", err)
				continue
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta

			// 发送内容或工具调用
			if delta.Content != "" {
				responseChan <- map[string]string{
					"type":    common.ResponseTypeContent,
					"content": delta.Content,
				}
			}

			if len(delta.ToolCalls) > 0 {
				responseChan <- map[string]interface{}{
					"type":       common.ResponseTypeToolCalls,
					"tool_calls": delta.ToolCalls,
				}
			}
		}
	}()

	return responseChan
}
