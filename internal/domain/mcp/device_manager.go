package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/mcp"
)

// DeviceMCPClient MCP客户端，接收MCP服务器的WebSocket连接
type DeviceMCPClient struct {
	deviceID  string
	conn      *websocket.Conn
	tools     map[string]tool.InvokableTool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	connected bool
	lastPing  time.Time
}

// NewDeviceMCPClient 创建新的MCP客户端
func NewDeviceMCPClient(deviceID string, conn *websocket.Conn) *DeviceMCPClient {
	ctx, cancel := context.WithCancel(context.Background())
	client := &DeviceMCPClient{
		deviceID:  deviceID,
		conn:      conn,
		tools:     make(map[string]tool.InvokableTool),
		ctx:       ctx,
		cancel:    cancel,
		connected: true,
		lastPing:  time.Now(),
	}

	// 启动消息处理
	go client.handleMessages()
	// 启动心跳检测
	go client.keepAlive()
	// 启动工具列表刷新
	client.startToolsListRefresh()

	// 发送初始化请求
	if err := client.sendInitializeRequest(); err != nil {
		logger.Errorf("发送初始化请求失败: %v", err)
		client.closeConnection()
		return nil
	}

	return client
}

// sendInitializeRequest 发送初始化请求
func (dc *DeviceMCPClient) sendInitializeRequest() error {
	initRequest := mcp.InitializeRequest{
		Request: mcp.Request{
			Method: string(mcp.MethodInitialize),
		},
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "xiaozhi-esp32-client",
				Version: "1.0.0",
			},
			Capabilities: mcp.ClientCapabilities{
				Experimental: make(map[string]any),
			},
		},
	}

	jsonRPCRequest := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(1),
		Request: initRequest.Request,
		Params:  initRequest.Params,
	}

	return dc.conn.WriteJSON(jsonRPCRequest)
}

// handleMessages 处理来自MCP服务器的消息
func (dc *DeviceMCPClient) handleMessages() {
	defer dc.closeConnection()

	for {
		var msg mcp.JSONRPCMessage
		err := dc.conn.ReadJSON(&msg)
		if err != nil {
			logger.Errorf("读取MCP消息失败: %v", err)
			return
		}

		if err := dc.handleJSONRPCMessage(msg); err != nil {
			logger.Errorf("处理MCP消息失败: %v", err)
			continue
		}
	}
}

// handleJSONRPCMessage 处理JSON-RPC消息
func (dc *DeviceMCPClient) handleJSONRPCMessage(msg mcp.JSONRPCMessage) error {
	switch typedMsg := msg.(type) {
	case mcp.JSONRPCRequest:
		return dc.handleJSONRPCRequest(typedMsg)
	case mcp.JSONRPCResponse:
		return dc.handleJSONRPCResponse(typedMsg)
	case mcp.JSONRPCNotification:
		return dc.handleJSONRPCNotification(typedMsg)
	case mcp.JSONRPCError:
		return dc.handleJSONRPCError(typedMsg)
	default:
		return fmt.Errorf("未知的消息类型")
	}
}

// handleJSONRPCRequest 处理来自MCP服务器的JSON-RPC请求
func (dc *DeviceMCPClient) handleJSONRPCRequest(req mcp.JSONRPCRequest) error {
	switch req.Method {
	case string(mcp.MethodToolsList):
		return dc.handleToolsListRequest(req)
	case string(mcp.MethodToolsCall):
		return dc.handleToolsCallRequest(req)
	default:
		return dc.sendJSONRPCError(req.ID, mcp.METHOD_NOT_FOUND, "方法未找到")
	}
}

// handleToolsListRequest 处理工具列表请求
func (dc *DeviceMCPClient) handleToolsListRequest(req mcp.JSONRPCRequest) error {
	dc.mu.RLock()
	tools := make([]mcp.Tool, 0, len(dc.tools))
	for _, invokableTool := range dc.tools {
		toolInfo, err := invokableTool.Info(context.Background())
		if err != nil {
			continue
		}
		tool := mcp.NewTool(
			toolInfo.Name,
			mcp.WithDescription(toolInfo.Desc),
		)
		tools = append(tools, tool)
	}
	dc.mu.RUnlock()

	result := mcp.NewListToolsResult(tools, "")
	response := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      req.ID,
		Result:  result,
	}

	return dc.conn.WriteJSON(response)
}

// handleToolsCallRequest 处理工具调用请求
func (dc *DeviceMCPClient) handleToolsCallRequest(req mcp.JSONRPCRequest) error {
	var callToolReq mcp.CallToolRequest
	if paramsBytes, err := json.Marshal(req.Params); err == nil {
		if err := json.Unmarshal(paramsBytes, &callToolReq.Params); err != nil {
			return dc.sendJSONRPCError(req.ID, mcp.INVALID_PARAMS, "无效的参数")
		}
	} else {
		return dc.sendJSONRPCError(req.ID, mcp.INVALID_PARAMS, "无效的参数")
	}

	toolName := callToolReq.Params.Name
	if toolName == "" {
		return dc.sendJSONRPCError(req.ID, mcp.INVALID_PARAMS, "缺少工具名称")
	}

	dc.mu.RLock()
	tool, exists := dc.tools[toolName]
	dc.mu.RUnlock()

	if !exists {
		return dc.sendJSONRPCError(req.ID, mcp.METHOD_NOT_FOUND, "工具未找到")
	}

	var arguments string
	if args, ok := callToolReq.Params.Arguments.(string); ok {
		arguments = args
	}

	result, err := tool.InvokableRun(context.Background(), arguments)
	if err != nil {
		return dc.sendJSONRPCError(req.ID, mcp.INTERNAL_ERROR, fmt.Sprintf("工具执行失败: %v", err))
	}

	toolResult := mcp.NewToolResultText(result)
	response := mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      req.ID,
		Result:  toolResult,
	}

	return dc.conn.WriteJSON(response)
}

// handleJSONRPCResponse 处理JSON-RPC响应
func (dc *DeviceMCPClient) handleJSONRPCResponse(resp mcp.JSONRPCResponse) error {
	logger.Infof("收到MCP服务器响应: %+v", resp)
	return nil
}

// handleJSONRPCNotification 处理JSON-RPC通知
func (dc *DeviceMCPClient) handleJSONRPCNotification(notif mcp.JSONRPCNotification) error {
	logger.Infof("收到MCP服务器通知: %s", notif.Method)
	return nil
}

// handleJSONRPCError 处理JSON-RPC错误
func (dc *DeviceMCPClient) handleJSONRPCError(errMsg mcp.JSONRPCError) error {
	logger.Errorf("收到MCP服务器错误: %+v", errMsg.Error)
	return nil
}

// sendJSONRPCError 发送JSON-RPC错误响应
func (dc *DeviceMCPClient) sendJSONRPCError(id mcp.RequestId, code int, message string) error {
	errorResponse := mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Error: struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    any    `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
		},
	}
	return dc.conn.WriteJSON(errorResponse)
}

// keepAlive 保持连接活跃
func (dc *DeviceMCPClient) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dc.ctx.Done():
			return
		case <-ticker.C:
			dc.mu.RLock()
			if !dc.connected {
				dc.mu.RUnlock()
				continue
			}
			dc.mu.RUnlock()

			if err := dc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logger.Errorf("发送ping消息失败: %v", err)
				dc.closeConnection()
				return
			}
		}
	}
}

// closeConnection 关闭连接
func (dc *DeviceMCPClient) closeConnection() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if dc.conn != nil {
		dc.conn.Close()
		dc.conn = nil
	}
	dc.connected = false
}

// RegisterTool 注册工具
func (dc *DeviceMCPClient) RegisterTool(name string, tool tool.InvokableTool) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.tools[name] = tool
}

// UnregisterTool 注销工具
func (dc *DeviceMCPClient) UnregisterTool(name string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	delete(dc.tools, name)
}

// Stop 停止MCP客户端
func (dc *DeviceMCPClient) Stop() error {
	dc.cancel()
	dc.closeConnection()
	logger.Info("MCP客户端已停止")
	return nil
}

// GetTools 获取工具列表
func (dc *DeviceMCPClient) GetTools() map[string]tool.InvokableTool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	tools := make(map[string]tool.InvokableTool)
	for name, tool := range dc.tools {
		tools[name] = tool
	}
	return tools
}

// Context 获取客户端的上下文
func (dc *DeviceMCPClient) Context() context.Context {
	return dc.ctx
}

// IsConnected 检查是否已连接
func (dc *DeviceMCPClient) IsConnected() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.connected
}

// sendToolsListRequest 发送工具列表请求
func (dc *DeviceMCPClient) sendToolsListRequest() error {
	request := mcp.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      mcp.NewRequestId(1),
		Request: mcp.Request{
			Method: string(mcp.MethodToolsList),
		},
		Params: struct{}{},
	}
	return dc.conn.WriteJSON(request)
}

// startToolsListRefresh 启动定期刷新工具列表
func (dc *DeviceMCPClient) startToolsListRefresh() {
	go func() {
		ticker := time.NewTicker(30 * time.Second) // 每30秒刷新一次
		defer ticker.Stop()

		// 立即发送一次请求
		if err := dc.sendToolsListRequest(); err != nil {
			logger.Errorf("初始工具列表请求失败: %v", err)
		}

		for {
			select {
			case <-dc.ctx.Done():
				return
			case <-ticker.C:
				if err := dc.sendToolsListRequest(); err != nil {
					logger.Errorf("刷新工具列表失败: %v", err)
				} else {
					logger.Debugf("已发送工具列表请求")
				}
			}
		}
	}()
}
