package common

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	log "xiaozhi-esp32-server-golang/logger"
)

type ChatManager struct {
	clientState *ClientState
	protocol    *Protocol
}

func NewChatManager(clientState *ClientState) *ChatManager {
	return &ChatManager{
		clientState: clientState,
		protocol:    NewProtocol(clientState),
	}
}

func (c *ChatManager) OnClose() error {
	log.Infof("设备 %s 断开连接", c.clientState.DeviceID)
	// 关闭done通道通知所有goroutine退出
	c.clientState.Cancel()
	c.clientState.Destroy()
	c.clientState.Conn.Close()
	return nil
}

// handleTextMessage 处理文本消息
func (c *ChatManager) HandleTextMessage(message []byte) error {
	var clientMsg ClientMessage
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		log.Errorf("解析消息失败: %v", err)
		return fmt.Errorf("解析消息失败: %v", err)
	}

	// 处理不同类型的消息
	switch clientMsg.Type {
	case MessageTypeHello:
		return c.protocol.HandleHelloMessage(&clientMsg)
	case MessageTypeListen:
		return c.protocol.HandleListenMessage(&clientMsg)
	case MessageTypeAbort:
		return c.protocol.HandleAbortMessage(&clientMsg)
	case MessageTypeIot:
		return c.protocol.HandleIoTMessage(&clientMsg)
	case MessageTypeMcp:
		return c.protocol.HandleMcpMessage(&clientMsg)
	case MessageTypeGoodBye:
		return c.protocol.HandleGoodByeMessage(&clientMsg)
	default:
		// 未知消息类型，直接回显
		return c.clientState.Conn.WriteMessage(websocket.TextMessage, message)
	}
}

// HandleAudioMessage 处理音频消息
func (c *ChatManager) HandleAudioMessage(data []byte) bool {
	select {
	case c.clientState.OpusAudioBuffer <- data:
		return true
	default:
		log.Warnf("音频缓冲区已满, 丢弃音频数据")
	}
	return false
}

func (c *ChatManager) GetClientState() *ClientState {
	return c.clientState
}

func (c *ChatManager) GetDeviceId() string {
	return c.clientState.DeviceID
}
