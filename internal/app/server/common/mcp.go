package common

import (
	"fmt"
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"
)

type McpTransport struct {
	Client *ClientState
}

func (c *McpTransport) SendMcpMsg(payload interface{}) error {
	serverMsg := ServerMessage{
		Type:      MessageTypeMcp,
		SessionID: c.Client.SessionID,
		PayLoad:   payload,
	}
	return c.Client.Conn.WriteJSON(serverMsg)
}

func (c *McpTransport) RecvMcpMsg(timeOut int) ([]byte, error) {
	select {
	case msg := <-c.Client.McpRecvMsgChan:
		return msg, nil
	case <-time.After(time.Duration(timeOut) * time.Millisecond):
		return nil, fmt.Errorf("mcp 接收消息超时")
	}
}

func initMcp(clientState *ClientState) {
	mcpClientSession := mcp.GetDeviceMcpClient(clientState.DeviceID)
	if mcpClientSession == nil {
		mcpClientSession = mcp.NewDeviceMCPSession(clientState.DeviceID)
		mcp.AddDeviceMcpClient(clientState.DeviceID, mcpClientSession)
	}

	// 创建IotOverMcp客户端
	mcpTransport := &McpTransport{
		Client: clientState,
	}
	iotOverMcpClient := mcp.NewIotOverMcpClient(clientState.DeviceID, mcpTransport)
	if iotOverMcpClient == nil {
		log.Errorf("创建IotOverMcp客户端失败")
		clientState.Conn.Close()
		return
	}
	mcpClientSession.SetIotOverMcp(iotOverMcpClient)
}
