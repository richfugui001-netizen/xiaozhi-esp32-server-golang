package eino_llm

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	log "xiaozhi-esp32-server-golang/logger"
)

// EinoLLMProvider 基于Eino框架的LLM提供者
// 直接使用Eino的ChatModel接口和类型，支持openai和ollama
type EinoLLMProvider struct {
	chatModel    model.ToolCallingChatModel
	modelName    string
	maxTokens    int
	streamable   bool
	config       map[string]interface{}
	providerType string // "openai" 或 "ollama"
}

// EinoConfig Eino LLM配置
type EinoConfig struct {
	Type       string                 `json:"type"` // "openai" 或 "ollama"
	ModelName  string                 `json:"model_name"`
	APIKey     string                 `json:"api_key"`
	BaseURL    string                 `json:"base_url"`
	MaxTokens  int                    `json:"max_tokens"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Streamable bool                   `json:"streamable,omitempty"`
}

// 连接池配置
const (
	maxIdleConns        = 100
	maxIdleConnsPerHost = 10
	idleConnTimeout     = 90 * time.Second
	requestTimeout      = 30 * time.Second
)

// 全局HTTP客户端，用于所有OpenAI请求
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
			MaxIdleConns:        maxIdleConns,
			MaxIdleConnsPerHost: maxIdleConnsPerHost,
			IdleConnTimeout:     idleConnTimeout,
			TLSHandshakeTimeout: 10 * time.Second,
			//ExpectContinueTimeout: 1 * time.Second,
			DisableKeepAlives: false,
		}

		httpClient = &http.Client{
			Transport: transport,
			Timeout:   requestTimeout,
		}
	})

	return httpClient
}

// NewEinoLLMProvider 创建新的Eino LLM提供者，根据type支持openai和ollama
func NewEinoLLMProvider(config map[string]interface{}) (*EinoLLMProvider, error) {
	providerType, _ := config["type"].(string)
	if providerType == "" {
		return nil, fmt.Errorf("type不能为空，必须是 'openai' 或 'ollama'")
	}

	modelName, _ := config["model_name"].(string)
	if modelName == "" {
		return nil, fmt.Errorf("model_name不能为空")
	}

	maxTokens := 500
	if mt, ok := config["max_tokens"].(int); ok {
		maxTokens = mt
	}

	streamable := true
	if s, ok := config["streamable"].(bool); ok {
		streamable = s
	}

	var chatModel model.ToolCallingChatModel
	var err error

	// 根据类型创建不同的ChatModel实现
	switch providerType {
	case "openai":
		chatModel, err = createOpenAIChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("创建OpenAI ChatModel失败: %v", err)
		}
	case "ollama":
		chatModel, err = createOllamaChatModel(config)
		if err != nil {
			return nil, fmt.Errorf("创建Ollama ChatModel失败: %v", err)
		}
	default:
		return nil, fmt.Errorf("不支持的模型类型: %s", providerType)
	}

	provider := &EinoLLMProvider{
		chatModel:    chatModel,
		modelName:    modelName,
		maxTokens:    maxTokens,
		streamable:   streamable,
		config:       config,
		providerType: providerType,
	}

	return provider, nil
}

// createOpenAIChatModel 创建OpenAI的ChatModel实现
func createOpenAIChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	modelName, _ := config["model_name"].(string)
	if modelName == "" {
		modelName = "gpt-3.5-turbo"
	}

	apiKey, _ := config["api_key"].(string)
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	baseURL, _ := config["base_url"].(string)

	// 创建OpenAI ChatModel配置
	openaiConfig := &openai.ChatModelConfig{
		Model:  modelName,
		APIKey: apiKey,
	}

	if baseURL != "" {
		openaiConfig.BaseURL = baseURL
	}

	// 使用eino-ext官方OpenAI实现
	chatModel, err := openai.NewChatModel(ctx, openaiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建OpenAI ChatModel失败: %v", err)
	}

	log.Infof("成功创建OpenAI ChatModel，模型: %s", modelName)
	return chatModel, nil
}

// createOllamaChatModel 创建Ollama的ChatModel实现
func createOllamaChatModel(config map[string]interface{}) (model.ToolCallingChatModel, error) {
	ctx := context.Background()

	modelName, _ := config["model_name"].(string)
	baseURL, _ := config["base_url"].(string)

	if modelName == "" || baseURL == "" {
		log.Warnf("model_name和base_url不能为空，使用默认模型: %s", modelName)
		return nil, fmt.Errorf("model_name和base_url不能为空")
	}

	// 创建Ollama ChatModel配置
	ollamaConfig := &ollama.ChatModelConfig{
		BaseURL: baseURL,
		Model:   modelName,
	}

	// 使用eino-ext官方Ollama实现
	chatModel, err := ollama.NewChatModel(ctx, ollamaConfig)
	if err != nil {
		return nil, fmt.Errorf("创建Ollama ChatModel失败: %v", err)
	}

	log.Infof("成功创建Ollama ChatModel，模型: %s", modelName)
	return chatModel, nil
}

// GetModelInfo 获取模型信息
func (p *EinoLLMProvider) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"model_name":      p.modelName,
		"max_tokens":      p.maxTokens,
		"streamable":      p.streamable,
		"type":            "eino",
		"provider_type":   p.providerType,
		"framework":       "eino",
		"adapter_version": "3.0.0",
		"base_url":        p.config["base_url"],
	}
}

// Response 生成响应 - 使用Eino原生消息类型
func (p *EinoLLMProvider) Response(sessionID string, dialogue []*schema.Message) chan string {
	return p.ResponseWithContext(context.Background(), sessionID, dialogue)
}

// ResponseWithContext 带有上下文控制的响应，直接调用Eino函数
func (p *EinoLLMProvider) ResponseWithContext(ctx context.Context, sessionID string, dialogue []*schema.Message) chan string {
	responseChan := make(chan string, 20)

	go func() {
		defer close(responseChan)

		log.Infof("[Eino-LLM] 开始处理请求 - SessionID: %s, MessageCount: %d, Type: %s", sessionID, len(dialogue), p.providerType)

		if p.streamable {
			// 直接使用Eino的Stream方法
			streamReader, err := p.chatModel.Stream(ctx, dialogue, model.WithMaxTokens(p.maxTokens))
			if err != nil {
				log.Errorf("Eino流式调用失败: %v", err)
				// 对于mock实现，如果Stream失败，回退到Generate
				message, genErr := p.chatModel.Generate(ctx, dialogue, model.WithMaxTokens(p.maxTokens))
				if genErr != nil {
					log.Errorf("Eino生成响应失败: %v", genErr)
					return
				}
				if message != nil && message.Content != "" {
					responseChan <- message.Content
				}
				return
			}

			if streamReader != nil {
				defer streamReader.Close()

				// 处理流式响应
				for {
					message, err := streamReader.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						log.Errorf("接收流式响应失败: %v", err)
						break
					}

					if message != nil && message.Content != "" {
						responseChan <- message.Content
					}
				}
			}
		} else {
			// 直接使用Eino的Generate方法
			message, err := p.chatModel.Generate(ctx, dialogue, model.WithMaxTokens(p.maxTokens))
			if err != nil {
				log.Errorf("Eino生成响应失败: %v", err)
				return
			}

			if message != nil && message.Content != "" {
				responseChan <- message.Content
			}
		}

		log.Infof("[Eino-LLM] 请求处理完成 - SessionID: %s", sessionID)
	}()

	return responseChan
}

// ResponseWithFunctions 带函数调用的响应，使用Eino原生工具类型，直接调用EinoResponseWithTools
func (p *EinoLLMProvider) ResponseWithFunctions(sessionID string, dialogue []*schema.Message, functions []*schema.ToolInfo) chan interface{} {
	responseChan := make(chan interface{})

	go func() {
		defer close(responseChan)

		log.Infof("[Eino-LLM] 开始处理带工具的请求 - SessionID: %s, Type: %s", sessionID, p.providerType)

		ctx := context.Background()

		// 直接调用EinoResponseWithTools获取Eino原生响应
		einoResponseChan := p.EinoResponseWithTools(ctx, sessionID, dialogue, functions)

		// 将Eino原生响应转换为接口格式
		for message := range einoResponseChan {
			if message != nil {
				// 处理消息内容
				if message.Content != "" {
					responseChan <- map[string]string{
						"type":    "content",
						"content": message.Content,
					}
				}

				// 处理工具调用（如果存在）
				if len(message.ToolCalls) > 0 {
					responseChan <- map[string]interface{}{
						"type":       "tool_calls",
						"tool_calls": message.ToolCalls,
					}
				}
			}
		}

		log.Infof("[Eino-LLM] 工具调用请求处理完成 - SessionID: %s", sessionID)
	}()

	return responseChan
}

// EinoResponse 直接使用Eino消息类型的响应
func (p *EinoLLMProvider) EinoResponse(ctx context.Context, sessionID string, messages []*schema.Message) chan string {
	return p.ResponseWithContext(ctx, sessionID, messages)
}

// EinoResponseWithTools 直接使用Eino类型的带工具响应
func (p *EinoLLMProvider) EinoResponseWithTools(ctx context.Context, sessionID string, messages []*schema.Message, tools []*schema.ToolInfo) chan *schema.Message {
	responseChan := make(chan *schema.Message)

	go func() {
		defer close(responseChan)

		log.Infof("[Eino-LLM] 开始处理Eino工具请求 - SessionID: %s", sessionID)

		// 如果有工具，需要绑定工具到ChatModel
		if len(tools) > 0 {
			_, err := p.chatModel.WithTools(tools)
			if err != nil {
				log.Errorf("绑定工具失败: %v", err)
				return
			}
		}

		if p.streamable {
			// 直接使用Eino的Stream方法
			streamReader, err := p.chatModel.Stream(ctx, messages, model.WithMaxTokens(p.maxTokens))
			if err != nil {
				log.Errorf("Eino工具流式调用失败: %v", err)
				// 对于mock实现，如果Stream失败，回退到Generate
				message, genErr := p.chatModel.Generate(ctx, messages, model.WithMaxTokens(p.maxTokens))
				if genErr != nil {
					log.Errorf("Eino工具生成响应失败: %v", genErr)
					return
				}
				if message != nil {
					responseChan <- message
				}
				return
			}

			if streamReader != nil {
				defer streamReader.Close()

				// 处理流式响应
				for {
					message, err := streamReader.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						log.Errorf("接收流式响应失败: %v", err)
						break
					}

					if message != nil {
						responseChan <- message
					}
				}
			}
		} else {
			// 直接使用Eino的Generate方法
			message, err := p.chatModel.Generate(ctx, messages, model.WithMaxTokens(p.maxTokens))
			if err != nil {
				log.Errorf("Eino工具生成响应失败: %v", err)
				return
			}

			if message != nil {
				responseChan <- message
			}
		}

		log.Infof("[Eino-LLM] Eino工具请求处理完成 - SessionID: %s", sessionID)
	}()

	return responseChan
}

// GetChatModel 获取底层的Eino ChatModel
func (p *EinoLLMProvider) GetChatModel() model.ToolCallingChatModel {
	return p.chatModel
}

// GetProviderType 获取提供者类型
func (p *EinoLLMProvider) GetProviderType() string {
	return p.providerType
}

// WithMaxTokens 设置最大令牌数
func (p *EinoLLMProvider) WithMaxTokens(maxTokens int) *EinoLLMProvider {
	newProvider := *p
	newProvider.maxTokens = maxTokens
	return &newProvider
}

// WithStreamable 设置是否支持流式
func (p *EinoLLMProvider) WithStreamable(streamable bool) *EinoLLMProvider {
	newProvider := *p
	newProvider.streamable = streamable
	return &newProvider
}
