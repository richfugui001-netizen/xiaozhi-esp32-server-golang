package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/orcaman/concurrent-map/v2"
)

type McpClientPool struct {
	device2McpClient cmap.ConcurrentMap[string, *DeviceMCPClient]
}

var mcpClientPool *McpClientPool

func init() {
	mcpClientPool = &McpClientPool{
		device2McpClient: cmap.New[*DeviceMCPClient](),
	}
	go mcpClientPool.checkOffline()
}

func (p *McpClientPool) GetMcpClient(deviceID string) *DeviceMCPClient {
	client, ok := p.device2McpClient.Get(deviceID)
	if !ok {
		return nil
	}
	return client
}

func (p *McpClientPool) RemoveMcpClient(deviceID string) {
	p.device2McpClient.Remove(deviceID)
}

func (p *McpClientPool) AddMcpClient(deviceID string, client *DeviceMCPClient) {
	p.device2McpClient.Set(deviceID, client)
}

func (p *McpClientPool) GetToolByDeviceId(deviceId string, toolsName string) (tool.InvokableTool, bool) {
	client := p.GetMcpClient(deviceId)
	if client == nil {
		return nil, false
	}
	return client.GetToolByName(toolsName)
}

func (p *McpClientPool) GetAllToolsByDeviceId(deviceId string) (map[string]tool.InvokableTool, error) {
	client := p.GetMcpClient(deviceId)
	if client == nil {
		return nil, fmt.Errorf("client not found")
	}
	return client.GetTools(), nil
}

func (p *McpClientPool) checkOffline() {
	for _, client := range p.device2McpClient.Items() {
		if time.Since(client.lastPing) > 2*time.Minute {
			client.connected = false
			client.cancel()
			p.RemoveMcpClient(client.deviceID)
		}
	}
}

// DeviceMCPClient MCP客户端，接收MCP服务器的WebSocket连接
type DeviceMCPClient struct {
	deviceID   string
	mcpClient  *client.Client
	tools      map[string]tool.InvokableTool
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	connected  bool
	serverInfo *mcp.InitializeResult
	lastPing   time.Time
}

// NewDeviceMCPClient 创建新的MCP客户端
func NewDeviceMCPClient(deviceID string, conn *websocket.Conn) *DeviceMCPClient {
	ctx, cancel := context.WithCancel(context.Background())

	deviceMcpClient := &DeviceMCPClient{
		deviceID:  deviceID,
		tools:     make(map[string]tool.InvokableTool),
		ctx:       ctx,
		cancel:    cancel,
		connected: true,
		lastPing:  time.Now(),
	}

	wsTransport, err := NewWebsocketTransport(conn)
	if err != nil {
		logger.Errorf("创建MCP客户端失败: %v", err)
		return nil
	}

	mcpClient := client.NewClient(wsTransport)
	deviceMcpClient.mcpClient = mcpClient

	wsTransport.SetNotificationHandler(deviceMcpClient.handleJSONRPCNotification)

	err = deviceMcpClient.sendInitlize(ctx)
	if err != nil {
		logger.Errorf("初始化MCP客户端失败: %v", err)
		return nil
	}

	err = deviceMcpClient.mcpClient.Start(ctx)
	if err != nil {
		logger.Errorf("启动MCP客户端失败: %v", err)
		return nil
	}

	go deviceMcpClient.refreshToolsAndPing()

	return deviceMcpClient
}

func (dc *DeviceMCPClient) refreshToolsAndPing() {
	tick := time.NewTicker(60 * time.Second)
	defer tick.Stop()

	pingTick := time.NewTicker(30 * time.Second)
	defer pingTick.Stop()

	findTools := func() {
		tools, err := dc.mcpClient.ListTools(dc.ctx, mcp.ListToolsRequest{})
		if err != nil {
			logger.Errorf("获取工具列表失败: %v", err)
			return
		}
		dc.tools = ConvertMcpToolListToInvokableToolList(tools.Tools, dc.deviceID, dc.mcpClient)
		logger.Infof("设备 %s 获取工具列表成功: %v", dc.deviceID, dc.tools)
	}

	findTools()
	for {
		select {
		case <-dc.ctx.Done():
			return
		case <-tick.C:
			findTools()
		case <-pingTick.C:
			err := dc.mcpClient.Ping(dc.ctx)
			if err == nil {
				dc.lastPing = time.Now()
			}
		}
	}
}

func (dc *DeviceMCPClient) sendInitlize(ctx context.Context) error {
	initRequest := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "mcp-go",
				Version: "0.1.0",
			},
			Capabilities: mcp.ClientCapabilities{},
		},
	}

	serverInfo, err := dc.mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		fmt.Println("Failed to initialize: %v", err)
		return err
	}
	dc.serverInfo = serverInfo
	return nil

}

// handleJSONRPCNotification 处理JSON-RPC通知
func (dc *DeviceMCPClient) handleJSONRPCNotification(notif mcp.JSONRPCNotification) {
	logger.Infof("收到MCP服务器通知: %s", notif.Method)
	return
}

// handleJSONRPCError 处理JSON-RPC错误
func (dc *DeviceMCPClient) handleJSONRPCError(errMsg mcp.JSONRPCError) error {
	logger.Errorf("收到MCP服务器错误: %+v", errMsg.Error)
	return nil
}

// GetTools 获取工具列表
func (dc *DeviceMCPClient) GetTools() map[string]tool.InvokableTool {
	return dc.tools
}

func (dc *DeviceMCPClient) GetToolByName(toolName string) (tool.InvokableTool, bool) {
	if tool, ok := dc.tools[toolName]; ok {
		return tool, true
	}
	return nil, false
}

// Context 获取客户端的上下文
func (dc *DeviceMCPClient) Context() context.Context {
	return dc.ctx
}
