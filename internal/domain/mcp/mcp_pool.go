package mcp

import (
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	cmap "github.com/orcaman/concurrent-map/v2"
)

type McpClientPool struct {
	device2McpClient cmap.ConcurrentMap[string, *DeviceMcpSession]
}

var mcpClientPool *McpClientPool

func init() {
	mcpClientPool = &McpClientPool{
		device2McpClient: cmap.New[*DeviceMcpSession](),
	}
	go mcpClientPool.checkOffline()
}

func (p *McpClientPool) GetMcpClient(deviceID string) *DeviceMcpSession {
	client, ok := p.device2McpClient.Get(deviceID)
	if !ok {
		return nil
	}
	return client
}

func (p *McpClientPool) RemoveMcpClient(deviceID string) {
	p.device2McpClient.Remove(deviceID)
}

func (p *McpClientPool) AddMcpClient(deviceID string, client *DeviceMcpSession) {
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
		if time.Since(client.wsEndPointMcp.lastPing) > 2*time.Minute {
			client.wsEndPointMcp.connected = false
			client.wsEndPointMcp.cancel()
		}
		if time.Since(client.iotOverMcp.lastPing) > 2*time.Minute {
			client.iotOverMcp.connected = false
			client.iotOverMcp.cancel()
		}
		if !client.wsEndPointMcp.connected && !client.iotOverMcp.connected {
			p.RemoveMcpClient(client.deviceID)
		}
	}
}
