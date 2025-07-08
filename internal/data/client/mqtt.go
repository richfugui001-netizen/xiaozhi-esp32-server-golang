package client

import (
	"encoding/json"

	msg "xiaozhi-esp32-server-golang/internal/data/msg"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	DeviceMockPubTopicPrefix = msg.MDeviceMockPubTopicPrefix
	DeviceMockSubTopicPrefix = msg.MDeviceMockSubTopicPrefix
	DeviceSubTopicPrefix     = msg.MDeviceSubTopicPrefix
	DevicePubTopicPrefix     = msg.MDevicePubTopicPrefix
	ServerSubTopicPrefix     = msg.MServerSubTopicPrefix
	ServerPubTopicPrefix     = msg.MServerPubTopicPrefix
)

const (
	ClientActiveTs = 120
)

type MqttConn struct {
	Conn     mqtt.Client
	PubTopic string
}

func (c *MqttConn) WriteJSON(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	token := c.Conn.Publish(c.PubTopic, 0, false, data)
	token.Wait()
	return token.Error()
}

func (c *MqttConn) ReadJSON(v interface{}) error {
	return nil
}

func (c *MqttConn) WriteMessage(messageType int, message []byte) error {
	token := c.Conn.Publish(c.PubTopic, byte(0), false, message)
	token.Wait()
	return token.Error()
}

func (c *MqttConn) ReadMessage() (messageType int, message []byte, err error) {
	// MQTT 客户端不支持直接读取消息，需要通过订阅回调处理
	// 这里返回一个空消息，实际的消息处理应该在订阅回调中完成
	return 0, nil, nil
}
