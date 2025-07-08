package mqtt_udp

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"xiaozhi-esp32-server-golang/internal/app/server/common"
	"xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	. "xiaozhi-esp32-server-golang/logger"
)

type MqttConfig struct {
	Broker   string
	Type     string
	Port     int
	ClientID string
	Username string
	Password string
}

// MqttSession 表示一个MQTT会话
type MqttSession struct {
	ID        string
	ClientID  string
	Key       string
	Nonce     string
	CreatedAt time.Time
}

// MqttServer MQTT服务器结构
type MqttServer struct {
	client              mqtt.Client
	udpServer           *UdpServer
	mqttConfig          *MqttConfig
	deviceId2UdpSession *sync.Map
	sync.RWMutex
}

// NewMqttServer 创建新的MQTT服务器
func NewMqttServer(config *MqttConfig, udpServer *UdpServer) *MqttServer {
	return &MqttServer{
		udpServer:           udpServer,
		mqttConfig:          config,
		deviceId2UdpSession: &sync.Map{},
	}
}

// Start 启动MQTT服务器
func (s *MqttServer) Start() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("%s://%s:%d", s.mqttConfig.Type, s.mqttConfig.Broker, s.mqttConfig.Port))
	opts.SetClientID(s.mqttConfig.ClientID)
	opts.SetUsername(s.mqttConfig.Username)
	opts.SetPassword(s.mqttConfig.Password)

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		Errorf("MQTT连接丢失: %v", err)
	})

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		Info("MQTT已连接")
		// 订阅客户端消息主题
		topic := ServerSubTopicPrefix // 默认主题前缀
		if token := client.Subscribe(topic, 0, s.handleMessage); token.Wait() && token.Error() != nil {
			Errorf("订阅主题失败: %v", token.Error())
		}
	})

	s.client = mqtt.NewClient(opts)
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("连接MQTT服务器失败: %v", token.Error())
	}

	err := s.checkClientActive()
	if err != nil {
		Errorf("检查客户端活跃失败: %v", err)
		return err
	}

	return nil
}

func (s *MqttServer) checkClientActive() error {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.deviceId2UdpSession.Range(func(key, value interface{}) bool {
					udpSession := value.(*UdpSession)
					clientState := udpSession.ChatManager.GetClientState()
					if !clientState.IsActive() {
						Infof("clientState is not active, clear deviceId: %s", clientState.DeviceID)
						//主动关闭断开连接
						udpSession.ChatManager.Close()

						//解除udp会话
						s.udpServer.CloseSession(udpSession.ConnId)
						//删除mqtt关联关系
						s.deviceId2UdpSession.Delete(key)
					}
					return true
				})
			}
		}
	}()
	return nil
}

func (s *MqttServer) SetUdpSession(udpSession *UdpSession) {
	Debugf("SetUdpSession, deviceId: %s", udpSession.DeviceId)
	s.deviceId2UdpSession.Store(udpSession.DeviceId, udpSession)
}

func (s *MqttServer) getUdpSession(deviceId string) *UdpSession {
	Debugf("getUdpSession, deviceId: %s", deviceId)
	if udpSession, ok := s.deviceId2UdpSession.Load(deviceId); ok {
		return udpSession.(*UdpSession)
	}
	return nil
}

// handleMessage 处理MQTT消息
func (s *MqttServer) handleMessage(client mqtt.Client, msg mqtt.Message) {
	Debugf("mqtt handleMessage, topic: %s, payload: %s", msg.Topic(), string(msg.Payload()))
	var clientMsg ClientMessage
	if err := json.Unmarshal(msg.Payload(), &clientMsg); err != nil {
		Errorf("解析JSON失败: %v", err)
		return
	}

	if clientMsg.Type == MessageTypeHello {
		s.handleHello(msg, clientMsg)
		return
	}

	_, deviceId := s.getDeviceIdByTopic(msg.Topic())
	if deviceId == "" {
		Errorf("deviceId is empty, msg: %+v", msg)
		return
	}

	udpSession := s.getUdpSession(deviceId)
	if udpSession == nil {
		Warnf("udpSession is nil, msg: %+v", msg)
		return
	}

	chatManager := udpSession.ChatManager

	chatManager.GetClientState().UpdateLastActiveTs()
	chatManager.HandleTextMessage(msg.Payload())
}

func (s *MqttServer) getDeviceIdByTopic(topic string) (string, string) {
	var topicMacAddr, deviceId string
	//根据topic(/p2p/device_public/mac_addr)解析出来mac_addr
	strList := strings.Split(topic, "/")
	if len(strList) == 4 {
		topicMacAddr = strList[3]
		deviceId = strings.ReplaceAll(topicMacAddr, "_", ":")
	}
	return topicMacAddr, deviceId
}

// handleHello 处理hello消息
func (s *MqttServer) handleHello(msg mqtt.Message, clientMsg client.ClientMessage) {
	// 检查传输协议
	if clientMsg.Transport != "udp" {
		Warnf("不支持的传输协议: %v", clientMsg.Transport)
		return
	}

	topicMacAddr, deviceId := s.getDeviceIdByTopic(msg.Topic())
	if deviceId == "" {
		Errorf("mac_addr解析失败: %v", msg.Topic())
		return
	}

	// 从UDP服务端获取会话信息
	session := s.udpServer.CreateSession(deviceId, "")
	if session == nil {
		Error("创建会话失败")
		return
	}

	publicTopic := fmt.Sprintf("%s%s", client.ServerPubTopicPrefix, topicMacAddr)

	mqttConn := &MqttConn{
		Conn:     s.client,
		PubTopic: publicTopic,
	}

	sendAudioFunc := func(audioData []byte) error {
		select {
		case session.SendChannel <- audioData:
			return nil
		default:
			return fmt.Errorf("发送音频数据失败, 通道已满")
		}
	}

	chatManager, err := common.NewChatManager(
		common.WithDeviceID(deviceId),
		common.WithMqttConn(mqttConn),
		common.WithUdpSendAudioData(sendAudioFunc),
	)
	if err != nil {
		Errorf("创建chatManager失败: %v", err)
		return
	}

	//赋值给session
	session.ChatManager = chatManager

	//保存至deviceId2UdpSession
	s.SetUdpSession(session)

	strAesKey, strFullNonce := s.getAesKeyAndNonce(session)
	//调用handleMqttHelloMessage处理hello消息通用逻辑
	chatManager.HandleMqttHelloMessage(&clientMsg, strAesKey, strFullNonce)

}

func (s *MqttServer) getAesKeyAndNonce(session *UdpSession) (string, string) {
	//处理
	strAesKey := hex.EncodeToString(session.AesKey[:])

	// 构造 fullNonce: 前缀2字节0100 + 长度2字节0000 + 真实nonce(8字节) + seq(4字节00000000)
	prefix := []byte{0x01, 0x00}
	length := []byte{0x00, 0x00}
	seq := []byte{0x00, 0x00, 0x00, 0x00}
	fullNonce := append(append(append(prefix, length...), session.Nonce[:]...), seq...)
	strFullNonce := hex.EncodeToString(fullNonce)

	return strAesKey, strFullNonce
}

// handleGoodbye 处理goodbye消息
func (s *MqttServer) handleGoodbye(msg mqtt.Message, clientMsg client.ClientMessage) {
	/*sessionID, ok := clientMsg.SessionID
	if !ok {
		Warn("会话ID无效")
		return
	}

	s.udpServer.CloseSession(sessionID)
	Infof("会话已关闭: %s", sessionID)*/
}
