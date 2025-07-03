package client

import (
	"encoding/json"
	"sync"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// *websocket.Conn  读: 不允许多个协程同时读   写: 不允许多个协程同时写   读写: 允许同时读写
type Conn struct {
	lock          sync.RWMutex
	connType      int // 0: websocket, 1: mqtt
	websocketConn *websocket.Conn
	MqttConn      *MqttConn //mqtt连接
}

func (c *Conn) WriteJSON(message interface{}) error {
	strMsg, _ := json.Marshal(message)
	log.Debugf("WriteJSON 发送消息: %+v", string(strMsg))
	if c.connType == 0 {
		c.lock.Lock()
		defer c.lock.Unlock()
		return c.websocketConn.WriteJSON(message)
	} else {
		return c.MqttConn.WriteJSON(message)
	}
}

func (c *Conn) ReadJSON(v interface{}) error {
	if c.connType == 0 {
		c.lock.Lock()
		defer c.lock.Unlock()
		return c.websocketConn.ReadJSON(v)
	} else {
		return c.MqttConn.ReadJSON(v)
	}
}

func (c *Conn) WriteMessage(messageType int, message []byte) error {

	if messageType == websocket.TextMessage {
		log.Debugf("WriteMessage 发送消息: %+v", string(message))
	} else {
		//log.Debugf("WriteMessage Binary 消息: %d", len(message))
	}
	if c.connType == 0 {
		c.lock.Lock()
		defer c.lock.Unlock()
		return c.websocketConn.WriteMessage(messageType, message)
	} else {
		return c.MqttConn.WriteMessage(messageType, message)
	}
}

func (c *Conn) ReadMessage() (messageType int, message []byte, err error) {
	if c.connType == 0 {
		return c.websocketConn.ReadMessage()
	} else {
		return c.MqttConn.ReadMessage()
	}
}

func (c *Conn) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.connType == 0 {
		return c.websocketConn.Close()
	}
	return nil
}
