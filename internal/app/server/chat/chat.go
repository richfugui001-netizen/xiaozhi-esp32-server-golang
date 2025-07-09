package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/viper"

	types_conn "xiaozhi-esp32-server-golang/internal/app/server/types"
	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	userconfig "xiaozhi-esp32-server-golang/internal/domain/config"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/tts"
	"xiaozhi-esp32-server-golang/internal/domain/vad/silero_vad"
	log "xiaozhi-esp32-server-golang/logger"
)

type ChatManager struct {
	DeviceID  string
	transport types_conn.IConn

	clientState *ClientState
	session     *ChatSession
	ctx         context.Context
}

type ChatManagerOption func(*ChatManager)

func NewChatManager(deviceID string, transport types_conn.IConn, options ...ChatManagerOption) (*ChatManager, error) {
	cm := &ChatManager{
		DeviceID:  deviceID,
		transport: transport,
		ctx:       context.Background(),
	}

	for _, option := range options {
		option(cm)
	}

	clientState, err := GenClientState(cm.ctx, cm.DeviceID)
	if err != nil {
		log.Errorf("初始化客户端状态失败: %v", err)
		return nil, err
	}
	cm.clientState = clientState

	serverTransport := NewServerTransport(cm.transport, clientState)

	asrManager := NewASRManager(clientState, serverTransport)
	ttsManager := NewTTSManager(clientState, serverTransport)
	llmManager := NewLLMManager(clientState, serverTransport, ttsManager)

	cm.session = NewChatSession(
		clientState,
		WithASRManager(asrManager),
		WithTTSManager(ttsManager),
		WithServerTransport(serverTransport),
		WithLLMManager(llmManager),
	)

	return cm, nil
}

func GenClientState(pctx context.Context, deviceID string) (*ClientState, error) {
	configProvider, err := userconfig.GetProvider()
	if err != nil {
		log.Errorf("获取 用户配置提供者失败: %+v", err)
		return nil, err
	}
	deviceConfig, err := configProvider.GetUserConfig(pctx, deviceID)
	if err != nil {
		log.Errorf("获取 设备 %s 配置失败: %+v", deviceID, err)
		return nil, err
	}

	if deviceConfig.Vad.Provider == "silero_vad" {
		silero_vad.InitVadPool(deviceConfig.Vad.Config)
	}

	// 创建带取消功能的上下文
	ctx, cancel := context.WithCancel(pctx)

	maxSilenceDuration := viper.GetInt64("chat.chat_max_silence_duration")
	if maxSilenceDuration == 0 {
		maxSilenceDuration = 200
	}

	systemPrompt, _ := llm_memory.Get().GetSystemPrompt(ctx, deviceID)

	clientState := &ClientState{
		Dialogue:     &Dialogue{},
		Abort:        false,
		ListenMode:   "auto",
		DeviceID:     deviceID,
		Ctx:          ctx,
		Cancel:       cancel,
		SystemPrompt: systemPrompt.Content,
		DeviceConfig: deviceConfig,
		OutputAudioFormat: types_audio.AudioFormat{
			SampleRate:    types_audio.SampleRate,
			Channels:      types_audio.Channels,
			FrameDuration: types_audio.FrameDuration,
			Format:        types_audio.Format,
		},
		OpusAudioBuffer: make(chan []byte, 100),
		AsrAudioBuffer: &AsrAudioBuffer{
			PcmData:          make([]float32, 0),
			AudioBufferMutex: sync.RWMutex{},
			PcmFrameSize:     0,
		},
		VoiceStatus: VoiceStatus{
			HaveVoice:            false,
			HaveVoiceLastTime:    0,
			VoiceStop:            false,
			SilenceThresholdTime: maxSilenceDuration,
		},
		SessionCtx: Ctx{},
	}

	ttsType := clientState.DeviceConfig.Tts.Provider
	//如果使用 xiaozhi tts，则固定使用24000hz, 20ms帧长
	if ttsType == "xiaozhi" || ttsType == "edge_offline" {
		clientState.OutputAudioFormat.SampleRate = 24000
		clientState.OutputAudioFormat.FrameDuration = 20
	}

	return clientState, nil
}

// 在mqtt 收到type: listen, state: start后进行
func (c *ChatManager) InitAsrLlmTts() error {
	ttsConfig := c.clientState.DeviceConfig.Tts
	ttsProvider, err := tts.GetTTSProvider(ttsConfig.Provider, ttsConfig.Config)
	if err != nil {
		return fmt.Errorf("创建 TTS 提供者失败: %v", err)
	}
	c.clientState.TTSProvider = ttsProvider

	if err := c.clientState.InitLlm(); err != nil {
		return fmt.Errorf("初始化LLM失败: %v", err)
	}
	if err := c.clientState.InitAsr(); err != nil {
		return fmt.Errorf("初始化ASR失败: %v", err)
	}
	c.clientState.SetAsrPcmFrameSize(c.clientState.InputAudioFormat.SampleRate, c.clientState.InputAudioFormat.Channels, c.clientState.InputAudioFormat.FrameDuration)

	return nil
}

func (c *ChatManager) Start() error {
	go func() error {
		err := c.InitAsrLlmTts()
		if err != nil {
			log.Errorf("初始化ASR/LLM/TTS失败: %v", err)
			return err
		}

		go c.CmdMessageLoop()
		go c.AudioMessageLoop()
		return nil
	}()

	return nil
}

func (c *ChatManager) CmdMessageLoop() {
	for {
		select {
		case <-c.ctx.Done():
			log.Infof("设备 %s recvCmd context cancel", c.clientState.DeviceID)
			return
		default:
			message, err := c.transport.RecvCmd(120)
			if err != nil {
				log.Errorf("recv cmd error: %v", err)
				return
			}
			log.Infof("收到文本消息: %s", string(message))
			if err := c.HandleTextMessage(message); err != nil {
				log.Errorf("处理文本消息失败: %v", err)
				continue
			}
		}
	}
}

func (c *ChatManager) AudioMessageLoop() {
	for {
		select {
		case <-c.ctx.Done():
			log.Debugf("设备 %s recvCmd context cancel", c.clientState.DeviceID)
			return
		default:
			message, err := c.transport.RecvAudio(300)
			if err != nil {
				log.Errorf("recv audio error: %v", err)
				return
			}
			log.Debugf("收到音频数据，大小: %d 字节", len(message))
			if c.clientState.GetClientVoiceStop() {
				//log.Debug("客户端停止说话, 跳过音频数据")
				continue
			}
			// 同时通过音频处理器处理
			if ok := c.HandleAudioMessage(message); !ok {
				log.Errorf("音频缓冲区已满: %v", err)
			}
		}
	}
}

// 主动关闭断开连接
func (c *ChatManager) Close() error {
	log.Infof("主动关闭断开连接, 设备 %s", c.clientState.DeviceID)
	StopSpeaking(c.session.serverTransport, true)
	c.clientState.Destroy()
	return nil
}

func (c *ChatManager) OnClose() error {
	log.Infof("设备 %s 断开连接", c.clientState.DeviceID)
	// 关闭done通道通知所有goroutine退出
	c.clientState.Cancel()
	c.clientState.Destroy()
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
		return c.session.HandleHelloMessage(&clientMsg)
	case MessageTypeListen:
		return c.session.HandleListenMessage(&clientMsg)
	case MessageTypeAbort:
		return c.session.HandleAbortMessage(&clientMsg)
	case MessageTypeIot:
		return c.session.HandleIoTMessage(&clientMsg)
	case MessageTypeMcp:
		return c.session.HandleMcpMessage(&clientMsg)
	case MessageTypeGoodBye:
		return c.session.HandleGoodByeMessage(&clientMsg)
	default:
		// 未知消息类型，直接回显
		return fmt.Errorf("未知消息类型: %s", clientMsg.Type)
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
