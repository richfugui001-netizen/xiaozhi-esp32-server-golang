package common

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"
)

type Protocol struct {
	clientState *ClientState

	chatTextQueue chan string
}

func NewProtocol(clientState *ClientState) *Protocol {
	p := &Protocol{
		clientState:   clientState,
		chatTextQueue: make(chan string, 10),
	}
	p.processChatText()
	return p
}

func (p *Protocol) HandleMqttHelloMessage(msg *ClientMessage, strAesKey string, strFullNonce string) error {
	p.HandleCommonHelloMessage(msg)

	clientState := p.clientState

	udpExternalHost := viper.GetString("udp.external_host")
	udpExternalPort := viper.GetInt("udp.external_port")

	udpConfig := &UdpConfig{
		Server: udpExternalHost,
		Port:   udpExternalPort,
		Key:    strAesKey,
		Nonce:  strFullNonce,
	}

	// 发送响应
	return SendHello(clientState, "udp", &clientState.OutputAudioFormat, udpConfig)
}

func (p *Protocol) HandleCommonHelloMessage(msg *ClientMessage) error {
	if isMcp, ok := msg.Features["mcp"]; ok && isMcp {
		go initMcp(p.clientState)
	}

	clientState := p.clientState

	clientState.InputAudioFormat = *msg.AudioParams
	clientState.SetAsrPcmFrameSize(clientState.InputAudioFormat.SampleRate, clientState.InputAudioFormat.Channels, clientState.InputAudioFormat.FrameDuration)

	ProcessVadAudio(clientState)

	return nil
}

// handleHelloMessage 处理 hello 消息
func (p *Protocol) HandleHelloMessage(msg *ClientMessage) error {
	// 创建新会话
	session, err := auth.A().CreateSession(msg.DeviceID)
	if err != nil {
		return fmt.Errorf("创建会话失败: %v", err)
	}

	// 更新客户端状态
	p.clientState.SessionID = session.ID

	p.HandleCommonHelloMessage(msg)

	// 发送 hello 响应
	return SendHello(p.clientState, "websocket", &p.clientState.OutputAudioFormat, nil)
}

// handleListenMessage 处理监听消息
func (p *Protocol) HandleListenMessage(msg *ClientMessage) error {
	// 根据状态处理
	switch msg.State {
	case MessageStateStart:
		p.HandleListenStart(msg)
	case MessageStateStop:
		p.HandleListenStop()
	case MessageStateDetect:
		p.HandleListenDetect(msg)
	}

	// 记录日志
	log.Infof("设备 %s 更新音频监听状态: %s", msg.DeviceID, msg.State)
	return nil
}

func (p *Protocol) HandleListenDetect(msg *ClientMessage) error {
	// 唤醒词检测
	StopSpeaking(p.clientState, false)

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
			if err := p.startChat(text); err != nil {
				log.Errorf("开始对话失败: %v", err)
			}
		}
	}
	return nil
}

// handleAbortMessage 处理中止消息
func (p *Protocol) HandleAbortMessage(msg *ClientMessage) error {
	// 设置打断状态
	p.clientState.Abort = true
	p.clientState.Dialogue.Messages = nil // 清空对话历史

	StopSpeaking(p.clientState, true)

	// 记录日志
	log.Infof("设备 %s abort 会话", msg.DeviceID)
	return nil
}

// handleIoTMessage 处理物联网消息
func (p *Protocol) HandleIoTMessage(msg *ClientMessage) error {
	// 获取客户端状态
	//sessionID := clientState.SessionID

	// 验证设备ID
	/*
		if _, err := s.authManager.GetSession(msg.DeviceID); err != nil {
			return fmt.Errorf("会话验证失败: %v", err)
		}*/

	// 发送 IoT 响应
	err := SendIot(p.clientState, msg)
	if err != nil {
		return fmt.Errorf("发送响应失败: %v", err)
	}

	// 记录日志
	log.Infof("设备 %s 物联网指令: %s", msg.DeviceID, msg.Text)
	return nil
}

func (p *Protocol) HandleMcpMessage(msg *ClientMessage) error {
	mcpSession := mcp.GetDeviceMcpClient(p.clientState.DeviceID)
	if mcpSession != nil {
		select {
		case p.clientState.McpRecvMsgChan <- msg.PayLoad:
		default:
			log.Warnf("mcp 接收消息通道已满, 丢弃消息")
		}
	}
	return nil
}

func (p *Protocol) HandleGoodByeMessage(msg *ClientMessage) error {
	p.clientState.Conn.Close()
	return nil
}

func (p *Protocol) HandleListenStart(msg *ClientMessage) error {
	// 处理拾音模式
	if msg.Mode != "" {
		p.clientState.ListenMode = msg.Mode
		log.Infof("设备 %s 拾音模式: %s", msg.DeviceID, msg.Mode)
	}
	if p.clientState.ListenMode == "manual" {
		StopSpeaking(p.clientState, false)
	}
	p.clientState.SetStatus(ClientStatusListening)

	return p.OnListenStart()
}

func (p *Protocol) HandleListenStop() error {
	if p.clientState.ListenMode == "auto" {
		p.clientState.CancelSessionCtx()
	}

	//调用
	p.clientState.OnManualStop()

	return nil
}

func (p *Protocol) OnListenStart() error {
	log.Debugf("OnListenStart start")
	defer log.Debugf("OnListenStart end")

	select {
	case <-p.clientState.Ctx.Done():
		log.Debugf("OnListenStart Ctx done, return")
		return nil
	default:
	}

	p.clientState.Destroy()

	ctx := p.clientState.GetSessionCtx()

	//初始化asr相关
	if p.clientState.ListenMode == "manual" {
		p.clientState.VoiceStatus.SetClientHaveVoice(true)
	}

	// 启动asr流式识别，复用 restartAsrRecognition 函数
	err := restartAsrRecognition(ctx, p.clientState)
	if err != nil {
		log.Errorf("asr流式识别失败: %v", err)
		p.clientState.Conn.Close()
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

			text, err := p.clientState.RetireAsrResult(ctx)
			if err != nil {
				log.Errorf("处理asr结果失败: %v", err)
				return
			}

			//统计asr耗时
			log.Debugf("处理asr结果: %s, 耗时: %d ms", text, p.clientState.GetAsrDuration())

			if text != "" {
				// 重置重试计数器
				startIdleTime = 0

				//当获取到asr结果时, 结束语音输入
				p.clientState.OnVoiceSilence()

				//发送asr消息
				err = SendAsrResult(p.clientState, text)
				if err != nil {
					log.Errorf("发送asr消息失败: %v", err)
					return
				}

				err = p.startChat(text)
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
				log.Debugf("ready Restart Asr, p.clientState.Status: %s", p.clientState.Status)
				if p.clientState.Status == ClientStatusListening || p.clientState.Status == ClientStatusListenStop {
					// text 为空，检查是否需要重新启动ASR
					diffTs := time.Now().Unix() - startIdleTime
					if startIdleTime > 0 && diffTs <= maxIdleTime {
						log.Warnf("ASR识别结果为空，尝试重启ASR识别, diff ts: %s", diffTs)
						if restartErr := restartAsrRecognition(ctx, p.clientState); restartErr != nil {
							log.Errorf("重启ASR识别失败: %v", restartErr)
							return
						}
						continue
					} else {
						log.Warnf("ASR识别结果为空，已达到最大空闲时间: %d", maxIdleTime)
						p.clientState.Conn.Close()
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
func (p *Protocol) startChat(text string) error {
	select {
	case p.chatTextQueue <- text:
	default:
		log.Warnf("chatTextQueue 已满, 丢弃消息")
	}
	return nil
}

func (p *Protocol) processChatText() {
	log.Debugf("processChatText start")
	defer log.Debugf("processChatText end")

	go func() {
		for {
			select {
			case <-p.clientState.Ctx.Done():
				return
			default:
				select {
				case text := <-p.chatTextQueue:
					err := p.actionDoChat(text)
					if err != nil {
						log.Errorf("处理对话失败: %v", err)
						return
					}
				default:
				}
			}
		}
	}()
}

func (p *Protocol) actionDoChat(text string) error {
	ctx := p.clientState.GetSessionCtx()
	clientState := p.clientState

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

	llmManager := NewLLMManager(ctx, clientState)

	err = llmManager.DoLLmRequest(requestEinoMessages, einoTools)
	if err != nil {
		log.Errorf("发送带工具的 LLM 请求失败, seesionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("发送带工具的 LLM 请求失败: %v", err)
	}
	return nil
}
