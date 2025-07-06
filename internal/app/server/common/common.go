package common

import (
	"context"
	"fmt"
	log "xiaozhi-esp32-server-golang/logger"

	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"

	. "xiaozhi-esp32-server-golang/internal/data/client"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func StopSpeaking(clientState *ClientState, isSendTtsStop bool) {
	clientState.CancelSessionCtx()
	if isSendTtsStop {
		SendTtsStop(clientState)
	}
}

// startChat 开始对话
func startChat(ctx context.Context, clientState *ClientState, text string) error {
	// 获取客户端状态

	sessionID := clientState.SessionID

	requestMessages, err := llm_memory.Get().GetMessagesForLLM(ctx, clientState.DeviceID, 10)
	if err != nil {
		log.Errorf("获取对话历史失败: %v", err)
	}

	// 直接创建Eino原生消息
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: text,
	}
	requestMessages = append(requestMessages, *userMessage)

	// 添加用户消息到对话历史
	//llm_memory.Get().AddMessage(ctx, clientState.DeviceID, schema.User, text)

	// 直接传递Eino原生消息，无需转换
	requestEinoMessages := make([]*schema.Message, len(requestMessages))
	for i, msg := range requestMessages {
		requestEinoMessages[i] = &msg
	}

	// 获取全局MCP工具列表
	mcpTools, err := mcp.GetToolsByDeviceId(clientState.DeviceID)
	if err != nil {
		log.Errorf("获取设备 %s 的工具失败: %v", clientState.DeviceID, err)
		mcpTools = make(map[string]tool.InvokableTool)
	}

	// 将MCP工具转换为接口格式以便传递给转换函数
	mcpToolsInterface := make(map[string]interface{})
	for name, tool := range mcpTools {
		mcpToolsInterface[name] = tool
	}

	// 转换MCP工具为Eino ToolInfo格式
	einoTools, err := llm.ConvertMCPToolsToEinoTools(ctx, mcpToolsInterface)
	if err != nil {
		log.Errorf("转换MCP工具失败: %v", err)
		einoTools = nil
	}

	toolNameList := make([]string, 0)
	for _, tool := range einoTools {
		toolNameList = append(toolNameList, tool.Name)
	}

	// 发送带工具的LLM请求
	log.Infof("使用 %d 个MCP工具发送LLM请求, tools: %+v", len(einoTools), toolNameList)

	llmManager := NewLLMManager(ctx, clientState)

	err = llmManager.DoLLmRequest(requestEinoMessages, einoTools)
	if err != nil {
		log.Errorf("发送带工具的 LLM 请求失败, seesionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("发送带工具的 LLM 请求失败: %v", err)
	}

	return nil
}
