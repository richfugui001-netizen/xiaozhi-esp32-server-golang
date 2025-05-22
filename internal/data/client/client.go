package client

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"time"

	"sync"

	"xiaozhi-esp32-server-golang/internal/domain/asr"
	asr_types "xiaozhi-esp32-server-golang/internal/domain/asr/types"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/tts"
	userconfig "xiaozhi-esp32-server-golang/internal/domain/user_config"
	"xiaozhi-esp32-server-golang/internal/domain/vad"

	. "xiaozhi-esp32-server-golang/internal/data/audio"

	log "xiaozhi-esp32-server-golang/logger"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

// Dialogue 表示对话历史
type Dialogue struct {
	Messages []llm_common.Message
}

// *websocket.Conn  读: 不允许多个协程同时读   写: 不允许多个协程同时写   读写: 允许同时读写
type Conn struct {
	lock          sync.RWMutex
	connType      int // 0: websocket, 1: mqtt
	websocketConn *websocket.Conn
	MqttConn      *MqttConn //mqtt连接
}

func (c *Conn) WriteJSON(message interface{}) error {

	log.Debugf("WriteJSON 发送消息: %+v", message)
	if c.connType == 0 {
		c.lock.Lock()
		defer c.lock.Unlock()
		return c.websocketConn.WriteJSON(message)
	} else {
		return c.MqttConn.WriteJSON(message)
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

func GenWebsocketClientState(deviceID string, conn *websocket.Conn) (*ClientState, error) {
	deviceConfig, err := userconfig.U().GetUserConfig(context.Background(), deviceID)
	if err != nil {
		log.Errorf("获取 设备 %s 配置失败: %+v", deviceID, err)
		return nil, err
	}

	// 创建带取消功能的上下文
	ctx, cancel := context.WithCancel(context.Background())

	systemPrompt, _ := llm_memory.Get().GetSystemPrompt(ctx, deviceID)
	clientState := &ClientState{
		Dialogue:     &Dialogue{},
		Abort:        false,
		ListenMode:   "auto",
		DeviceID:     deviceID,
		Conn:         &Conn{websocketConn: conn, connType: 0},
		Ctx:          ctx,
		Cancel:       cancel,
		SystemPrompt: systemPrompt.Content,
		DeviceConfig: deviceConfig,
		OutputAudioFormat: AudioFormat{
			SampleRate:    SampleRate,
			Channels:      Channels,
			FrameDuration: FrameDuration,
			Format:        Format,
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
			SilenceThresholdTime: 500,
		},
		SessionCtx: Ctx{},
	}

	ttsType := getTTsType(deviceConfig.Tts)
	//如果使用 xiaozhi tts，则固定使用24000hz, 20ms帧长
	if ttsType == "xiaozhi" {
		clientState.OutputAudioFormat.SampleRate = 24000
		clientState.OutputAudioFormat.FrameDuration = 20
	}

	ttsProvider, err := getTTSProvider(deviceConfig.Tts)
	if err != nil {
		log.Errorf("创建 TTS 提供者失败: %v", err)
		return nil, err
	}
	clientState.TTSProvider = ttsProvider

	clientState.Init()

	return clientState, nil
}

func GenMqttUdpClientState(deviceID string, pubTopic string, mqttClient mqtt.Client, udpSession *UdpSession, clientMsg *ClientMessage) (*ClientState, error) {
	deviceConfig, err := userconfig.U().GetUserConfig(context.Background(), deviceID)
	if err != nil {
		log.Errorf("获取 设备 %s 配置失败: %+v", deviceID, err)
		return nil, err
	}

	// 创建带取消功能的上下文
	ctx, cancel := context.WithCancel(context.Background())

	mqttConn := &MqttConn{
		Conn:     mqttClient,
		PubTopic: pubTopic,
	}

	systemPrompt, _ := llm_memory.Get().GetSystemPrompt(ctx, deviceID)
	clientState := &ClientState{
		Dialogue:     &Dialogue{},
		Abort:        false,
		ListenMode:   "auto",
		DeviceID:     deviceID,
		Conn:         &Conn{MqttConn: mqttConn, connType: 1},
		Ctx:          ctx,
		Cancel:       cancel,
		SystemPrompt: systemPrompt.Content,
		DeviceConfig: deviceConfig,
		OutputAudioFormat: AudioFormat{
			SampleRate:    SampleRate,
			Channels:      Channels,
			FrameDuration: FrameDuration,
			Format:        Format,
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
			SilenceThresholdTime: 500,
		},
		SessionCtx: Ctx{},
		UdpInfo:    udpSession,
	}

	ttsType := getTTsType(deviceConfig.Tts)
	//如果使用 xiaozhi tts，则固定使用24000hz, 20ms帧长
	if ttsType == "xiaozhi" {
		clientState.OutputAudioFormat.SampleRate = 24000
		clientState.OutputAudioFormat.FrameDuration = 20
	}

	clientState.StartMqttUdpClient()
	return clientState, nil
}

// 在mqtt 收到type: listen, state: start后进行
func (c *ClientState) StartMqttUdpClient() error {
	ttsProvider, err := getTTSProvider(c.DeviceConfig.Tts)
	if err != nil {
		log.Errorf("创建 TTS 提供者失败: %v", err)
		return err
	}
	c.TTSProvider = ttsProvider

	c.Init()

	return nil
}

func getTTsType(ttsConfig userconfig.TtsConfig) string {
	ttsProviderName := viper.GetString("tts.provider")
	if ttsConfig.Type != "" {
		ttsProviderName = ttsConfig.Type
	}
	return ttsProviderName
}

func getTTSProvider(ttsConfig userconfig.TtsConfig) (tts.TTSProvider, error) {
	ttsProviderName := getTTsType(ttsConfig)
	providerConfig := viper.GetStringMap(fmt.Sprintf("tts.%s", ttsProviderName))

	ttsProvider, err := tts.GetTTSProvider(ttsProviderName, providerConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 TTS 提供者失败: %v", err)
	}
	return ttsProvider, nil
}

// ClientState 表示客户端状态
type ClientState struct {
	// 对话历史
	Dialogue *Dialogue
	// 打断状态
	Abort bool
	// 拾音模式
	ListenMode string
	// 设备ID
	DeviceID string
	// 会话ID
	SessionID string
	// 连接
	Conn *Conn

	//设备配置
	DeviceConfig userconfig.UConfig

	Vad
	Asr
	Llm

	// TTS 提供者
	TTSProvider tts.TTSProvider

	// 上下文控制
	Ctx    context.Context
	Cancel context.CancelFunc

	//prompt, 系统提示词
	SystemPrompt string

	InputAudioFormat  AudioFormat //输入音频格式
	OutputAudioFormat AudioFormat //输出音频格式

	// opus接收的音频数据缓冲区
	OpusAudioBuffer chan []byte

	// pcm接收的音频数据缓冲区
	AsrAudioBuffer *AsrAudioBuffer

	VoiceStatus
	SessionCtx Ctx

	UdpInfo   *UdpSession //客户端udp地址
	MqttInfo  *MqttConn   //mqtt连接
	Statistic Statistic   //耗时统计
}

func (c *ClientState) IsMqttUdp() bool {
	return c.Conn.connType == 1
}

type UdpInfo struct {
	UdpAddr *net.UDPAddr
	Nonce   []byte //16位随机数
}

func (s *ClientState) ResetSessionCtx() {
	s.SessionCtx.Lock()
	defer s.SessionCtx.Unlock()
	if s.SessionCtx.Ctx == nil {
		s.SessionCtx.Ctx, s.SessionCtx.Cancel = context.WithCancel(s.Ctx)
	}
}

func (s *ClientState) CancelSessionCtx() {
	s.SessionCtx.Lock()
	defer s.SessionCtx.Unlock()
	if s.SessionCtx.Ctx != nil {
		s.SessionCtx.Cancel()
		s.SessionCtx.Ctx = nil
	}
}

func (s *ClientState) GetSessionCtx() context.Context {
	s.SessionCtx.Lock()
	defer s.SessionCtx.Unlock()
	if s.SessionCtx.Ctx == nil {
		s.SessionCtx.Ctx, s.SessionCtx.Cancel = context.WithCancel(s.Ctx)
	}
	return s.SessionCtx.Ctx
}

type Ctx struct {
	sync.RWMutex
	Ctx    context.Context
	Cancel context.CancelFunc
}

func (s *ClientState) getLLMProvider() (llm.LLMProvider, error) {
	llmProviderName := viper.GetString("llm.provider")
	if s.DeviceConfig.Llm.Type != "" {
		llmProviderName = s.DeviceConfig.Llm.Type
	}
	configMap := viper.GetStringMap(fmt.Sprintf("llm.%s", llmProviderName))
	llmProvider, err := llm.GetLLMProvider(llmProviderName, configMap)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 提供者失败: %v", err)
	}
	return llmProvider, nil
}

func (s *ClientState) InitLlm() {
	ctx, cancel := context.WithCancel(s.Ctx)

	llmProvider, err := s.getLLMProvider()
	if err != nil {
		log.Errorf("创建 LLM 提供者失败: %v", err)
		return
	}

	s.Llm = Llm{
		Ctx:         ctx,
		Cancel:      cancel,
		LLMProvider: llmProvider,
	}
}

func (s *ClientState) InitAsr() {
	//初始化asr
	asrProvider, err := asr.NewAsrProvider(viper.GetString("asr.provider"), viper.GetStringMap(fmt.Sprintf("asr.%s", viper.GetString("asr.provider"))))
	if err != nil {
		log.Errorf("创建asr提供者失败: %v", err)
		return
	}
	ctx, cancel := context.WithCancel(s.Ctx)
	s.Asr = Asr{
		Ctx:             ctx,
		Cancel:          cancel,
		AsrProvider:     asrProvider,
		AsrAudioChannel: make(chan []float32, 100),
		AsrResult:       bytes.Buffer{},
	}
}

func (c *ClientState) Init() {
	c.InitLlm()
	c.InitAsr()
	c.SetAsrPcmFrameSize(c.InputAudioFormat.SampleRate, c.InputAudioFormat.Channels, c.InputAudioFormat.FrameDuration)
}

func (c *ClientState) Destroy() {
	c.Asr.Stop()
	c.Vad.Reset()
}

func (c *ClientState) SetAsrPcmFrameSize(sampleRate int, channels int, perFrameDuration int) {
	c.AsrAudioBuffer.PcmFrameSize = sampleRate * channels * perFrameDuration / 1000
}

func (state *ClientState) SendMsg(msg interface{}) error {
	return state.Conn.WriteJSON(msg)
}

func (state *ClientState) actionSendAudioData(audioData []byte) error {
	if state.IsMqttUdp() {
		select {
		case <-state.Ctx.Done():
			return fmt.Errorf("上下文已取消")
		default:
			select {
			case state.UdpInfo.SendChannel <- audioData:
				return nil
			default:
				return fmt.Errorf("udp 发送缓冲区已满")
			}
		}
	}

	return state.Conn.WriteMessage(websocket.BinaryMessage, audioData)
}

func (state *ClientState) SendTTSAudio(audioChan chan []byte, isStart bool) error {
	// 步骤1: 收集前三帧（或更少）
	preBuffer := make([][]byte, 0, 3)
	preBufferCount := 0

	totalFrames := preBufferCount // 跟踪已发送的总帧数

	isStatistic := true
	// 收集前三帧
	for totalFrames < 3 {
		select {
		case frame, ok := <-audioChan:
			if isStart && isStatistic {
				log.Debugf("从接收音频结束 asr->llm->tts首帧 整体 耗时: %d ms", state.GetAsrLlmTtsDuration())
				isStatistic = false
			}
			if !ok {
				// 通道已关闭，发送已收集的帧并返回
				for _, f := range preBuffer {
					if err := state.actionSendAudioData(f); err != nil {
						return fmt.Errorf("发送 TTS 音频 len: %d 失败: %v", len(f), err)
					}
				}
				return nil
			}
			if err := state.actionSendAudioData(frame); err != nil {
				return fmt.Errorf("发送 TTS 音频 len: %d 失败: %v", len(frame), err)
			}
			log.Debugf("发送 TTS 音频: %d 帧, len: %d", totalFrames, len(frame))
			totalFrames++
		case <-state.Ctx.Done():
			// 上下文已取消
			log.Infof("连接已关闭，终止音频发送，已发送 %d 帧", totalFrames)
			return nil
		}
	}

	// 步骤3: 设置定时器，每60ms发送一帧
	ticker := time.NewTicker(time.Duration(state.OutputAudioFormat.FrameDuration) * time.Millisecond)
	defer ticker.Stop()

	// 循环处理剩余帧
	for {
		select {
		case <-ticker.C:
			// 时间到，尝试获取并发送下一帧
			select {
			case frame, ok := <-audioChan:
				if !ok {
					// 通道已关闭，所有帧已处理完毕
					return nil
				}

				// 发送当前帧
				if err := state.actionSendAudioData(frame); err != nil {
					return fmt.Errorf("发送 TTS 音频 len: %d 失败: %v", len(frame), err)
				}
				totalFrames++
				//log.Debugf("发送 TTS 音频: %d 帧, len: %d", totalFrames, len(frame))

			default:
				// 没有帧可获取，等待下一个周期
			}

		case <-state.Ctx.Done():
			// 上下文已取消
			log.Infof("连接已关闭，终止音频发送，已发送 %d 帧", totalFrames)
			return nil
		}
	}
}

type Vad struct {
	lock sync.RWMutex
	// VAD 提供者
	VadProvider vad.VAD
}

func (v *Vad) Init() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	vadProvider, err := vad.AcquireVAD()
	if err != nil {
		return fmt.Errorf("创建 VAD 提供者失败: %v", err)
	}
	vadProvider.Reset()
	v.VadProvider = vadProvider
	return nil
}

func (v *Vad) Reset() error {
	v.lock.Lock()
	defer v.lock.Unlock()
	if v.VadProvider != nil {
		vad.ReleaseVAD(v.VadProvider)
		v.VadProvider = nil
	}
	return nil
}

type Llm struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	// LLM 提供者
	LLMProvider llm.LLMProvider
	//asr to text接收的通道
	LLmRecvChannel chan llm_common.LLMResponseStruct
}

type Asr struct {
	lock sync.RWMutex
	// ASR 提供者
	Ctx              context.Context
	Cancel           context.CancelFunc
	AsrProvider      asr.AsrProvider
	AsrAudioChannel  chan []float32                 //流式音频输入的channel
	AsrResultChannel chan asr_types.StreamingResult //流式输出asr识别到的结果片断
	AsrResult        bytes.Buffer                   //保存此次识别到的最终文本
}

func (a *Asr) Reset() {
	a.AsrResult.Reset()
}

func (a *Asr) RetireAsrResult(ctx context.Context) (string, error) {
	for {
		select {
		case <-ctx.Done():
			a.Reset()
			return "", fmt.Errorf("RetireAsrResult ctx Done")
		case result, ok := <-a.AsrResultChannel:
			log.Debugf("asr result: %s", result.Text)
			a.AsrResult.WriteString(result.Text)
			if result.IsFinal || !ok {
				text := a.AsrResult.String()
				a.Reset()
				return text, nil
			}
		}
	}
}

func (a *Asr) Stop() {
	a.lock.Lock()
	defer a.lock.Unlock()
	if a.AsrAudioChannel != nil {
		log.Debugf("停止asr")
		close(a.AsrAudioChannel)
		a.AsrAudioChannel = nil
	}
}

type AsrAudioBuffer struct {
	PcmData          []float32
	AudioBufferMutex sync.RWMutex
	PcmFrameSize     int
}

func (a *AsrAudioBuffer) AddAsrAudioData(pcmFrameData []float32) error {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = append(a.PcmData, pcmFrameData...)
	return nil
}

func (a *AsrAudioBuffer) GetAsrDataSize() int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	return len(a.PcmData)
}

func (a *AsrAudioBuffer) GetFrameCount() int {
	a.AudioBufferMutex.RLock()
	defer a.AudioBufferMutex.RUnlock()
	return len(a.PcmData) / a.PcmFrameSize
}

// 副本
func (a *AsrAudioBuffer) GetAsrData(frameCount int) []float32 {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	retSize := frameCount * a.PcmFrameSize
	pcmData := make([]float32, retSize)
	copy(pcmData, a.PcmData[:retSize])
	a.PcmData = a.PcmData[retSize:]
	return pcmData
}

func (a *AsrAudioBuffer) RemoveAsrAudioData(frameCount int) {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = a.PcmData[frameCount*a.PcmFrameSize:]
}

func (a *AsrAudioBuffer) ClearAsrAudioData() {
	a.AudioBufferMutex.Lock()
	defer a.AudioBufferMutex.Unlock()
	a.PcmData = nil
}

type VoiceStatus struct {
	HaveVoice            bool  //上次是否有说话
	HaveVoiceLastTime    int64 //最后说话时间
	VoiceStop            bool  //是否停止说话
	SilenceThresholdTime int64 //无声音持续时间阈值
}

func (v *VoiceStatus) Reset() {
	v.HaveVoice = false
	v.HaveVoiceLastTime = 0
	v.VoiceStop = false
}

func (v *VoiceStatus) IsSilence(diffMilli int64) bool {
	return diffMilli > v.SilenceThresholdTime
}

func (v *VoiceStatus) GetClientHaveVoice() bool {
	return v.HaveVoice
}

func (v *VoiceStatus) SetClientHaveVoice(haveVoice bool) {
	v.HaveVoice = haveVoice
}

func (v *VoiceStatus) GetClientHaveVoiceLastTime() int64 {
	return v.HaveVoiceLastTime
}

func (v *VoiceStatus) SetClientHaveVoiceLastTime(lastTime int64) {
	v.HaveVoiceLastTime = lastTime
}

func (v *VoiceStatus) GetClientVoiceStop() bool {
	return v.VoiceStop
}

func (v *VoiceStatus) SetClientVoiceStop(voiceStop bool) {
	v.VoiceStop = voiceStop
}

// ClientMessage 表示客户端消息
type ClientMessage struct {
	Type        string       `json:"type"`
	DeviceID    string       `json:"device_id,omitempty"`
	SessionID   string       `json:"session_id,omitempty"`
	Text        string       `json:"text,omitempty"`
	Mode        string       `json:"mode,omitempty"`
	State       string       `json:"state,omitempty"`
	Token       string       `json:"token,omitempty"`
	DeviceMac   string       `json:"device_mac,omitempty"`
	Version     int          `json:"version,omitempty"`
	Transport   string       `json:"transport,omitempty"`
	AudioParams *AudioFormat `json:"audio_params,omitempty"`
}
