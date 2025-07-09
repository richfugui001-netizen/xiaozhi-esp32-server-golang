package chat

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	types_conn "xiaozhi-esp32-server-golang/internal/app/server/types"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"
)

type ChatSession struct {
	clientState     *ClientState
	asrManager      *ASRManager
	ttsManager      *TTSManager
	llmManager      *LLMManager
	serverTransport *ServerTransport

	chatTextQueue chan string
}

type ChatSessionOption func(*ChatSession)

func WithASRManager(asr *ASRManager) ChatSessionOption {
	return func(s *ChatSession) {
		s.asrManager = asr
	}
}

func WithTTSManager(tts *TTSManager) ChatSessionOption {
	return func(s *ChatSession) {
		s.ttsManager = tts
	}
}

func WithServerTransport(serverTransport *ServerTransport) ChatSessionOption {
	return func(s *ChatSession) {
		s.serverTransport = serverTransport
	}
}

func WithLLMManager(llm *LLMManager) ChatSessionOption {
	return func(s *ChatSession) {
		s.llmManager = llm
	}
}

func NewChatSession(clientState *ClientState, opts ...ChatSessionOption) *ChatSession {
	s := &ChatSession{
		clientState:   clientState,
		chatTextQueue: make(chan string, 10),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.processChatText()
	return s
}

// handleHelloMessage 处理 hello 消息
func (s *ChatSession) HandleHelloMessage(msg *ClientMessage) error {
	if msg.Transport == types_conn.TransportTypeWebsocket {
		return s.HandleWebsocketHelloMessage(msg)
	} else if msg.Transport == types_conn.TransportTypeMqttUdp {
		return s.HandleMqttHelloMessage(msg)
	}
	return fmt.Errorf("不支持的传输类型: %s", msg.Transport)
}

func (s *ChatSession) HandleMqttHelloMessage(msg *ClientMessage) error {
	s.HandleCommonHelloMessage(msg)

	clientState := s.clientState

	udpExternalHost := viper.GetString("udp.external_host")
	udpExternalPort := viper.GetInt("udp.external_port")

	aesKey, err := s.serverTransport.GetData("aes_key")
	if err != nil {
		return fmt.Errorf("获取aes_key失败: %v", err)
	}
	fullNonce, err := s.serverTransport.GetData("full_nonce")
	if err != nil {
		return fmt.Errorf("获取full_nonce失败: %v", err)
	}

	strAesKey, ok := aesKey.(string)
	if !ok {
		return fmt.Errorf("aes_key不是字符串")
	}
	strFullNonce, ok := fullNonce.(string)
	if !ok {
		return fmt.Errorf("full_nonce不是字符串")
	}

	udpConfig := &UdpConfig{
		Server: udpExternalHost,
		Port:   udpExternalPort,
		Key:    strAesKey,
		Nonce:  strFullNonce,
	}

	// 发送响应
	return s.serverTransport.SendHello("udp", &clientState.OutputAudioFormat, udpConfig)
}

func (s *ChatSession) HandleCommonHelloMessage(msg *ClientMessage) error {
	// 创建新会话
	session, err := auth.A().CreateSession(msg.DeviceID)
	if err != nil {
		return fmt.Errorf("创建会话失败: %v", err)
	}

	// 更新客户端状态
	s.clientState.SessionID = session.ID

	if isMcp, ok := msg.Features["mcp"]; ok && isMcp {
		go initMcp(s.clientState, s.serverTransport)
	}

	clientState := s.clientState

	clientState.InputAudioFormat = *msg.AudioParams
	clientState.SetAsrPcmFrameSize(clientState.InputAudioFormat.SampleRate, clientState.InputAudioFormat.Channels, clientState.InputAudioFormat.FrameDuration)

	s.asrManager.ProcessVadAudio(clientState.Ctx)

	return nil
}

func (s *ChatSession) HandleWebsocketHelloMessage(msg *ClientMessage) error {
	err := s.HandleCommonHelloMessage(msg)
	if err != nil {
		return err
	}

	return s.serverTransport.SendHello("websocket", &s.clientState.OutputAudioFormat, nil)
}

// handleListenMessage 处理监听消息
func (s *ChatSession) HandleListenMessage(msg *ClientMessage) error {
	// 根据状态处理
	switch msg.State {
	case MessageStateStart:
		s.HandleListenStart(msg)
	case MessageStateStop:
		s.HandleListenStop()
	case MessageStateDetect:
		s.HandleListenDetect(msg)
	}

	// 记录日志
	log.Infof("设备 %s 更新音频监听状态: %s", msg.DeviceID, msg.State)
	return nil
}

func (s *ChatSession) HandleListenDetect(msg *ClientMessage) error {
	// 唤醒词检测
	StopSpeaking(s.serverTransport, false)

	// 如果有文本，处理唤醒词
	if msg.Text != "" {
		text := msg.Text
		// 移除标点符号和处理长度
		text = removePunctuation(text)

		// 检查是否是唤醒词
		isWakeupWord := isWakeupWord(text)
		enableGreeting := viper.GetBool("enable_greeting") // 从配置获取

		var needStartChat bool
		if isWakeupWord && enableGreeting {
			needStartChat = true
		}
		if !isWakeupWord {
			needStartChat = true
		}
		if needStartChat {
			// 否则开始对话
			if err := s.startChat(text); err != nil {
				log.Errorf("开始对话失败: %v", err)
			}
		}
	}
	return nil
}

// handleAbortMessage 处理中止消息
func (s *ChatSession) HandleAbortMessage(msg *ClientMessage) error {
	// 设置打断状态
	s.clientState.Abort = true
	s.clientState.Dialogue.Messages = nil // 清空对话历史

	StopSpeaking(s.serverTransport, true)

	// 记录日志
	log.Infof("设备 %s abort 会话", msg.DeviceID)
	return nil
}

// handleIoTMessage 处理物联网消息
func (s *ChatSession) HandleIoTMessage(msg *ClientMessage) error {
	// 获取客户端状态
	//sessionID := clientState.SessionID

	// 验证设备ID
	/*
		if _, err := s.authManager.GetSession(msg.DeviceID); err != nil {
			return fmt.Errorf("会话验证失败: %v", err)
		}*/

	// 发送 IoT 响应
	err := s.serverTransport.SendIot(msg)
	if err != nil {
		return fmt.Errorf("发送响应失败: %v", err)
	}

	// 记录日志
	log.Infof("设备 %s 物联网指令: %s", msg.DeviceID, msg.Text)
	return nil
}

func (s *ChatSession) HandleMcpMessage(msg *ClientMessage) error {
	mcpSession := mcp.GetDeviceMcpClient(s.clientState.DeviceID)
	if mcpSession != nil {
		select {
		case s.serverTransport.McpRecvMsgChan <- msg.PayLoad:
		default:
			log.Warnf("mcp 接收消息通道已满, 丢弃消息")
		}
	}
	return nil
}

func (s *ChatSession) HandleGoodByeMessage(msg *ClientMessage) error {
	s.serverTransport.Close()
	return nil
}

func (s *ChatSession) HandleListenStart(msg *ClientMessage) error {
	// 处理拾音模式
	if msg.Mode != "" {
		s.clientState.ListenMode = msg.Mode
		log.Infof("设备 %s 拾音模式: %s", msg.DeviceID, msg.Mode)
	}
	if s.clientState.ListenMode == "manual" {
		StopSpeaking(s.serverTransport, false)
	}
	s.clientState.SetStatus(ClientStatusListening)

	return s.OnListenStart()
}

func (s *ChatSession) HandleListenStop() error {
	if s.clientState.ListenMode == "auto" {
		s.clientState.CancelSessionCtx()
	}

	//调用
	s.clientState.OnManualStop()

	return nil
}

func (s *ChatSession) OnListenStart() error {
	log.Debugf("OnListenStart start")
	defer log.Debugf("OnListenStart end")

	select {
	case <-s.clientState.Ctx.Done():
		log.Debugf("OnListenStart Ctx done, return")
		return nil
	default:
	}

	s.clientState.Destroy()

	ctx := s.clientState.GetSessionCtx()

	//初始化asr相关
	if s.clientState.ListenMode == "manual" {
		s.clientState.VoiceStatus.SetClientHaveVoice(true)
	}

	// 启动asr流式识别，复用 restartAsrRecognition 函数
	err := s.asrManager.RestartAsrRecognition(ctx)
	if err != nil {
		log.Errorf("asr流式识别失败: %v", err)
		s.serverTransport.Close()
		return err
	}

	// 启动一个goroutine处理asr结果
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("asr结果处理goroutine panic: %v, stack: %s", r, string(debug.Stack()))
			}
		}()

		//最大空闲 60s

		var startIdleTime, maxIdleTime int64
		startIdleTime = time.Now().Unix()
		maxIdleTime = 60

		for {
			select {
			case <-ctx.Done():
				log.Debugf("asr ctx done")
				return
			default:
			}

			text, err := s.clientState.RetireAsrResult(ctx)
			if err != nil {
				log.Errorf("处理asr结果失败: %v", err)
				return
			}

			//统计asr耗时
			log.Debugf("处理asr结果: %s, 耗时: %d ms", text, s.clientState.GetAsrDuration())

			if text != "" {
				// 重置重试计数器
				startIdleTime = 0

				//当获取到asr结果时, 结束语音输入
				s.clientState.OnVoiceSilence()

				//发送asr消息
				err = s.serverTransport.SendAsrResult(text)
				if err != nil {
					log.Errorf("发送asr消息失败: %v", err)
					return
				}

				err = s.startChat(text)
				if err != nil {
					log.Errorf("开始对话失败: %v", err)
					return
				}
				return
			} else {
				select {
				case <-ctx.Done():
					log.Debugf("asr ctx done")
					return
				default:
				}
				log.Debugf("ready Restart Asr, s.clientState.Status: %s", s.clientState.Status)
				if s.clientState.Status == ClientStatusListening || s.clientState.Status == ClientStatusListenStop {
					// text 为空，检查是否需要重新启动ASR
					diffTs := time.Now().Unix() - startIdleTime
					if startIdleTime > 0 && diffTs <= maxIdleTime {
						log.Warnf("ASR识别结果为空，尝试重启ASR识别, diff ts: %s", diffTs)
						if restartErr := s.asrManager.RestartAsrRecognition(ctx); restartErr != nil {
							log.Errorf("重启ASR识别失败: %v", restartErr)
							return
						}
						continue
					} else {
						log.Warnf("ASR识别结果为空，已达到最大空闲时间: %d", maxIdleTime)
						s.serverTransport.Close()
						return
					}
				}
			}
			return
		}
	}()
	return nil
}

// startChat 开始对话
func (s *ChatSession) startChat(text string) error {
	select {
	case s.chatTextQueue <- text:
	default:
		log.Warnf("chatTextQueue 已满, 丢弃消息")
	}
	return nil
}

func (s *ChatSession) processChatText() {
	log.Debugf("processChatText start")
	defer log.Debugf("processChatText end")

	go func() {
		for {
			select {
			case <-s.clientState.Ctx.Done():
				return
			case <-s.clientState.GetSessionCtx().Done():
				//清空chatTextQueue, 然后continue
				s.chatTextQueue = make(chan string, 10)
				continue
			default:
				select {
				case text := <-s.chatTextQueue:
					err := s.actionDoChat(text)
					if err != nil {
						log.Errorf("处理对话失败: %v", err)
						continue
					}
				}
			}
		}
	}()
}

func (s *ChatSession) actionDoChat(text string) error {
	ctx := s.clientState.GetSessionCtx()
	clientState := s.clientState

	sessionID := clientState.SessionID

	requestMessages, err := llm_memory.Get().GetMessagesForLLM(ctx, clientState.DeviceID, 10)
	if err != nil {
		log.Errorf("获取对话历史失败: %v", err)
	}

	// 直接创建Eino原生消息
	userMessage := &schema.Message{
		Role:    schema.User,
		Content: text,
	}
	requestMessages = append(requestMessages, *userMessage)

	// 添加用户消息到对话历史
	//llm_memory.Get().AddMessage(ctx, clientState.DeviceID, schema.User, text)

	// 直接传递Eino原生消息，无需转换
	requestEinoMessages := make([]*schema.Message, len(requestMessages))
	for i, msg := range requestMessages {
		requestEinoMessages[i] = &msg
	}

	// 获取全局MCP工具列表
	mcpTools, err := mcp.GetToolsByDeviceId(clientState.DeviceID)
	if err != nil {
		log.Errorf("获取设备 %s 的工具失败: %v", clientState.DeviceID, err)
		mcpTools = make(map[string]tool.InvokableTool)
	}

	// 将MCP工具转换为接口格式以便传递给转换函数
	mcpToolsInterface := make(map[string]interface{})
	for name, tool := range mcpTools {
		mcpToolsInterface[name] = tool
	}

	// 转换MCP工具为Eino ToolInfo格式
	einoTools, err := llm.ConvertMCPToolsToEinoTools(ctx, mcpToolsInterface)
	if err != nil {
		log.Errorf("转换MCP工具失败: %v", err)
		einoTools = nil
	}

	toolNameList := make([]string, 0)
	for _, tool := range einoTools {
		toolNameList = append(toolNameList, tool.Name)
	}

	// 发送带工具的LLM请求
	log.Infof("使用 %d 个MCP工具发送LLM请求, tools: %+v", len(einoTools), toolNameList)

	err = s.llmManager.DoLLmRequest(ctx, requestEinoMessages, einoTools)
	if err != nil {
		log.Errorf("发送带工具的 LLM 请求失败, seesionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("发送带工具的 LLM 请求失败: %v", err)
	}
	return nil
}
