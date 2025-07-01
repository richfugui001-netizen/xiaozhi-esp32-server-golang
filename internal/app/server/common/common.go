package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
	log "xiaozhi-esp32-server-golang/logger"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"

	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/audio"

	. "xiaozhi-esp32-server-golang/internal/data/msg"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

// ServerMessage 表示服务器消息
type ServerMessage struct {
	Type        string                   `json:"type"`
	Text        string                   `json:"text,omitempty"`
	SessionID   string                   `json:"session_id,omitempty"`
	Version     int                      `json:"version"`
	State       string                   `json:"state,omitempty"`
	Transport   string                   `json:"transport,omitempty"`
	AudioFormat *types_audio.AudioFormat `json:"audio_params,omitempty"`
	Emotion     string                   `json:"emotion,omitempty"`
	PayLoad     json.RawMessage          `json:"payload,omitempty"`
}

// HandleLLMResponse 处理LLM响应
func HandleLLMResponse(ctx context.Context, state *ClientState, requestEinoMessages []*schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct) (bool, error) {
	log.Debugf("HandleLLMResponse start")
	defer log.Debugf("HandleLLMResponse end")

	var toolCalls []schema.ToolCall
	var fullText bytes.Buffer

	sendTtsStartEndFunc := func(isStart bool) error {
		msgState := MessageStateStart
		if !isStart {
			msgState = MessageStateStop
		}
		// 发送结束消息
		response := ServerMessage{
			Type:      ServerMessageTypeTts,
			State:     msgState,
			SessionID: state.SessionID,
		}
		if err := state.SendMsg(response); err != nil {
			log.Errorf("发送 TTS 文本失败: stop, %v", err)
			return fmt.Errorf("发送 TTS 文本失败: stop")
		}

		if isStart {
			state.SetTtsStart(true)
		}
		return nil
	}

	if !state.GetTtsStart() {
		sendTtsStartEndFunc(true)
	}

	for {
		select {
		case <-ctx.Done():
			// 上下文已取消，优先处理取消逻辑
			log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
			sendTtsStartEndFunc(false)
			return false, nil
		default:
			// 非阻塞检查，如果ctx没有Done，继续处理LLM响应
			select {
			case llmResponse, ok := <-llmResponseChannel:
				if !ok {
					// 通道已关闭，退出协程
					log.Infof("LLM 响应通道已关闭，退出协程")
					return true, nil
				}

				log.Debugf("LLM 响应: %+v", llmResponse)

				if len(llmResponse.ToolCalls) > 0 {
					log.Debugf("获取到工具: %+v", llmResponse.ToolCalls)
					toolCalls = append(toolCalls, llmResponse.ToolCalls...)
				}

				if llmResponse.Text != "" {
					// 处理文本内容响应
					if err := handleTextResponse(ctx, state, llmResponse, &fullText); err != nil {
						return true, err
					}
				}

				if llmResponse.IsEnd {
					//延迟50ms毫秒再发stop
					//time.Sleep(50 * time.Millisecond)
					//写到redis中
					if len(requestEinoMessages) > 0 {
						llm_memory.Get().AddMessage(ctx, state.DeviceID, schema.User, requestEinoMessages[len(requestEinoMessages)-1].Content)
					}
					strFullText := fullText.String()
					if strFullText != "" {
						llm_memory.Get().AddMessage(ctx, state.DeviceID, schema.Assistant, strFullText)
					}
					if len(toolCalls) > 0 {
						invokeToolSuccess, err := handleToolCallResponse(ctx, state, requestEinoMessages, toolCalls)
						if err != nil {
							log.Errorf("处理工具调用响应失败: %v", err)
							return true, fmt.Errorf("处理工具调用响应失败: %v", err)
						}
						if !invokeToolSuccess {
							//工具调用失败
							if err := handleTextResponse(ctx, state, llmResponse, &fullText); err != nil {
								return true, err
							}
							sendTtsStartEndFunc(false)
						}
					} else {
						sendTtsStartEndFunc(false)
					}

					return ok, nil
				}
			case <-ctx.Done():
				// 上下文已取消，退出协程
				log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
				sendTtsStartEndFunc(false)
				return false, nil
			}
		}
	}
}

// handleTextResponse 处理文本内容响应
func handleTextResponse(ctx context.Context, state *ClientState, llmResponse llm_common.LLMResponseStruct, fullText *bytes.Buffer) error {
	if llmResponse.Text == "" {
		return nil
	}

	// 使用带上下文的TTS处理
	outputChan, err := state.TTSProvider.TextToSpeechStream(ctx, llmResponse.Text, state.OutputAudioFormat.SampleRate, state.OutputAudioFormat.Channels, state.OutputAudioFormat.FrameDuration)
	if err != nil {
		log.Errorf("生成 TTS 音频失败: %v", err)
		return fmt.Errorf("生成 TTS 音频失败: %v", err)
	}

	// 先发送文本
	response := ServerMessage{
		Type:      ServerMessageTypeTts,
		State:     MessageStateSentenceStart,
		Text:      llmResponse.Text,
		SessionID: state.SessionID,
	}
	if err := state.SendMsg(response); err != nil {
		log.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
		return fmt.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
	}

	state.SetStatus(ClientStatusTTSStart)

	fullText.WriteString(llmResponse.Text)

	// 发送音频帧
	if err := state.SendTTSAudio(ctx, outputChan, llmResponse.IsStart); err != nil {
		log.Errorf("发送 TTS 音频失败: %s, %v", llmResponse.Text, err)
		return fmt.Errorf("发送 TTS 音频失败: %s, %v", llmResponse.Text, err)
	}

	// 先发送文本
	response = ServerMessage{
		Type:      ServerMessageTypeTts,
		State:     MessageStateSentenceEnd,
		Text:      llmResponse.Text,
		SessionID: state.SessionID,
	}
	if err := state.SendMsg(response); err != nil {
		log.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
		return fmt.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
	}

	return nil
}

// handleToolCallResponse 处理工具调用响应
func handleToolCallResponse(ctx context.Context, state *ClientState, requestEinoMessages []*schema.Message, tools []schema.ToolCall) (bool, error) {
	if len(tools) == 0 {
		return false, nil
	}

	log.Infof("处理 %d 个工具调用", len(tools))

	var invokeToolSuccess bool
	msgList := make([]*schema.Message, 0)
	for _, toolCall := range tools {
		toolName := toolCall.Function.Name
		tool, ok := mcp.GetToolByName(state.DeviceID, toolName)
		if !ok || tool == nil {
			log.Errorf("未找到工具: %s", toolName)
			continue
		}
		log.Infof("进行工具调用请求: %s, 参数: %+v", toolName, toolCall.Function.Arguments)
		startTs := time.Now().UnixMilli()
		result, err := tool.InvokableRun(ctx, toolCall.Function.Arguments)
		if err != nil {
			log.Errorf("工具调用失败: %v", err)
			continue
		}
		costTs := time.Now().UnixMilli() - startTs
		invokeToolSuccess = true
		log.Infof("工具调用结果: %s, 耗时: %dms", result, costTs)
		msg := []*schema.Message{
			&schema.Message{
				Role:      schema.Assistant,
				ToolCalls: []schema.ToolCall{toolCall},
			},
			&schema.Message{
				Role:       schema.Tool,
				ToolCallID: toolCall.ID,
				Content:    result,
			},
		}
		msgList = append(msgList, msg...)
	}

	if invokeToolSuccess {
		requestEinoMessages = append(requestEinoMessages, msgList...)
		//不需要带tool进行调用
		DoLLmRequest(ctx, state, requestEinoMessages, state.SessionID, nil)
	}

	return invokeToolSuccess, nil
}

func ProcessVadAudio(state *ClientState) {
	go func() {
		audioFormat := state.InputAudioFormat
		audioProcesser, err := audio.GetAudioProcesser(audioFormat.SampleRate, audioFormat.Channels, audioFormat.FrameDuration)
		if err != nil {
			log.Errorf("获取解码器失败: %v", err)
			return
		}
		frameSize := state.AsrAudioBuffer.PcmFrameSize
		pcmFrame := make([]float32, frameSize)

		vadNeedGetCount := 1
		if state.DeviceConfig.Vad.Provider == "silero_vad" {
			vadNeedGetCount = 60 / audioFormat.FrameDuration
		}

		for {
			//sessionCtx := state.GetSessionCtx()
			select {
			case opusFrame, ok := <-state.OpusAudioBuffer:
				log.Debugf("processAsrAudio 收到音频数据, len: %d", len(opusFrame))
				if !ok {
					log.Debugf("processAsrAudio 音频通道已关闭")
					return
				}

				var skipVad bool
				var haveVoice bool
				clientHaveVoice := state.GetClientHaveVoice()
				if state.Asr.AutoEnd || state.ListenMode == "manual" {
					skipVad = true         //跳过vad
					clientHaveVoice = true //之前有声音
					haveVoice = true       //本次有声音
				}

				if state.GetClientVoiceStop() { //已停止 说话 则不接收音频数据
					//log.Infof("客户端停止说话, 跳过音频数据")
					continue
				}

				log.Debugf("clientVoiceStop: %+v, asrDataSize: %d, listenMode: %s, isSkipVad: %v\n", state.GetClientVoiceStop(), state.AsrAudioBuffer.GetAsrDataSize(), state.ListenMode, skipVad)

				n, err := audioProcesser.DecoderFloat32(opusFrame, pcmFrame)
				if err != nil {
					log.Errorf("解码失败: %v", err)
					continue
				}

				var vadPcmData []float32
				pcmData := pcmFrame[:n]
				if !skipVad {
					//如果已经检测到语音, 则不进行vad检测, 直接将pcmData传给asr
					if state.VadProvider == nil {
						// 初始化vad
						err = state.Vad.Init(state.DeviceConfig.Vad.Provider, state.DeviceConfig.Vad.Config)
						if err != nil {
							log.Errorf("初始化vad失败: %v", err)
							continue
						}
					}
					//decode opus to pcm
					state.AsrAudioBuffer.AddAsrAudioData(pcmData)

					if state.AsrAudioBuffer.GetAsrDataSize() >= vadNeedGetCount*state.AsrAudioBuffer.PcmFrameSize {
						//如果要进行vad, 至少要取60ms的音频数据
						vadPcmData = state.AsrAudioBuffer.GetAsrData(vadNeedGetCount)
						state.VadProvider.Reset()
						haveVoice, err = state.VadProvider.IsVADExt(vadPcmData, audioFormat.SampleRate, frameSize)

						if err != nil {
							log.Errorf("processAsrAudio VAD检测失败: %v", err)
							//删除
							continue
						}
						//首次触发识别到语音时,为了语音数据完整性 将vadPcmData赋值给pcmData, 之后的音频数据全部进入asr
						if haveVoice && !clientHaveVoice {
							//首次获取全部pcm数据送入asr
							pcmData = state.AsrAudioBuffer.GetAndClearAllData()
						}
					}
					log.Debugf("isVad, pcmData len: %d, vadPcmData len: %d, haveVoice: %v", len(pcmData), len(vadPcmData), haveVoice)
				}

				if haveVoice {
					log.Infof("检测到语音, len: %d", len(pcmData))
					state.SetClientHaveVoice(true)
					state.SetClientHaveVoiceLastTime(time.Now().UnixMilli())
					state.Vad.ResetIdleDuration()
				} else {
					state.Vad.AddIdleDuration(int64(audioFormat.FrameDuration))
					idleDuration := state.Vad.GetIdleDuration()
					log.Infof("空闲时间: %dms", idleDuration)
					if idleDuration > state.GetMaxIdleDuration() {
						log.Infof("超出空闲时长: %dms, 断开连接", idleDuration)
						//断开连接
						state.Conn.Close()
						return
					}
					//如果之前没有语音, 本次也没有语音, 则从缓存中删除
					if !clientHaveVoice {
						//保留近10帧
						if state.AsrAudioBuffer.GetFrameCount() > vadNeedGetCount*3 {
							state.AsrAudioBuffer.RemoveAsrAudioData(1)
						}
						continue
					}
				}

				if clientHaveVoice {
					//vad识别成功, 往asr音频通道里发送数据
					log.Infof("vad识别成功, 往asr音频通道里发送数据, len: %d", len(pcmData))
					if state.AsrAudioChannel != nil {
						state.AsrAudioChannel <- pcmData
					}
				}

				//已经有语音了, 但本次没有检测到语音, 则需要判断是否已经停止说话
				lastHaveVoiceTime := state.GetClientHaveVoiceLastTime()

				if clientHaveVoice && lastHaveVoiceTime > 0 && !haveVoice {
					idleDuration := state.Vad.GetIdleDuration()
					if state.IsSilence(idleDuration) { //从有声音到 静默的判断
						state.OnVoiceSilence()
						continue
					}
				}

			case <-state.Ctx.Done():
				return
			}
		}
	}()
}

// handleTextMessage 处理文本消息
func HandleTextMessage(clientState *ClientState, message []byte) error {
	var clientMsg ClientMessage
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		log.Errorf("解析消息失败: %v", err)
		return fmt.Errorf("解析消息失败: %v", err)
	}

	// 处理不同类型的消息
	switch clientMsg.Type {
	case MessageTypeHello:
		return handleHelloMessage(clientState, &clientMsg)
	case MessageTypeListen:
		return handleListenMessage(clientState, &clientMsg)
	case MessageTypeAbort:
		return handleAbortMessage(clientState, &clientMsg)
	case MessageTypeIot:
		return handleIoTMessage(clientState, &clientMsg)
	case MessageTypeMcp:
		return handleMcpMessage(clientState, &clientMsg)
	default:
		// 未知消息类型，直接回显
		return clientState.Conn.WriteMessage(websocket.TextMessage, message)
	}
}

// handleHelloMessage 处理 hello 消息
func handleHelloMessage(clientState *ClientState, msg *ClientMessage) error {
	// 创建新会话
	session, err := auth.A().CreateSession(msg.DeviceID)
	if err != nil {
		return fmt.Errorf("创建会话失败: %v", err)
	}

	// 更新客户端状态
	clientState.SessionID = session.ID

	if isMcp, ok := msg.Features["mcp"]; ok && isMcp {
		go initMcp(clientState)
	}

	clientState.InputAudioFormat = types_audio.AudioFormat{
		SampleRate:    msg.AudioParams.SampleRate,
		Channels:      msg.AudioParams.Channels,
		FrameDuration: msg.AudioParams.FrameDuration,
		Format:        msg.AudioParams.Format,
	}

	// 设置asr pcm帧大小, 输入音频格式, 给vad, asr使用
	clientState.SetAsrPcmFrameSize(clientState.InputAudioFormat.SampleRate, clientState.InputAudioFormat.Channels, clientState.InputAudioFormat.FrameDuration)

	ProcessVadAudio(clientState)

	// 发送 hello 响应
	response := ServerMessage{
		Type:        MessageTypeHello,
		Text:        "欢迎连接到小智服务器",
		SessionID:   session.ID,
		Transport:   "websocket",
		AudioFormat: &clientState.OutputAudioFormat,
	}

	return clientState.SendMsg(response)
}

func RecvAudio(clientState *ClientState, data []byte) bool {
	select {
	case clientState.OpusAudioBuffer <- data:
		return true
	default:
		log.Warnf("音频缓冲区已满, 丢弃音频数据")
	}
	return false
}

// handleListenMessage 处理监听消息
func handleListenMessage(clientState *ClientState, msg *ClientMessage) error {

	//sessionID := clientState.SessionID

	// 根据状态处理
	switch msg.State {
	case MessageStateStart:
		handleListenStart(clientState, msg)
	case MessageStateStop:
		handleListenStop(clientState)
	case MessageStateDetect:
		// 唤醒词检测
		clientState.SetClientHaveVoice(false)

		clientState.CancelSessionCtx()

		// 如果有文本，处理唤醒词
		if msg.Text != "" {
			text := msg.Text
			// 移除标点符号和处理长度
			text = removePunctuation(text)

			// 检查是否是唤醒词
			isWakeupWord := isWakeupWord(text)
			//enableGreeting := viper.GetBool("enable_greeting") // 从配置获取

			if isWakeupWord {
				// 如果是唤醒词，且关闭了唤醒词回复，发送 STT 消息后停止 TTS
				/*sttResponse := ServerMessage{
					Type:      ServerMessageTypeStt,
					Text:      text,
					SessionID: sessionID,
				}
				if err := clientState.SendMsg(sttResponse); err != nil {
					return fmt.Errorf("发送 STT 消息失败: %v", err)
				}*/
				log.Infof("唤醒词: %s", text)
			} else {
				// 否则开始对话
				/*if err := startChat(clientState.GetSessionCtx(), clientState, text); err != nil {
					log.Errorf("开始对话失败: %v", err)
				}*/
			}
		}
	}

	// 记录日志
	log.Infof("设备 %s 更新音频监听状态: %s", msg.DeviceID, msg.State)
	return nil
}

// removePunctuation 移除文本中的标点符号
func removePunctuation(text string) string {
	// 创建一个字符串构建器
	var builder strings.Builder
	builder.Grow(len(text))

	for _, r := range text {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// isWakeupWord 检查文本是否是唤醒词
func isWakeupWord(text string) bool {
	wakeupWords := viper.GetStringSlice("wakeup_words")
	for _, word := range wakeupWords {
		if text == word {
			return true
		}
	}
	return false
}

// handleAbortMessage 处理中止消息
func handleAbortMessage(clientState *ClientState, msg *ClientMessage) error {
	sessionID := clientState.SessionID

	// 设置打断状态
	clientState.Abort = true
	clientState.Dialogue.Messages = nil // 清空对话历史
	clientState.CancelSessionCtx()

	Restart(clientState)

	// 发送中止响应
	response := ServerMessage{
		Type:      MessageTypeAbort,
		State:     MessageStateSuccess,
		SessionID: sessionID,
		Text:      "会话已中止",
	}

	// 发送响应
	if err := clientState.SendMsg(response); err != nil {
		return fmt.Errorf("发送响应失败: %v", err)
	}

	// 记录日志
	log.Infof("设备 %s 中止会话", msg.DeviceID)
	return nil
}

// handleIoTMessage 处理物联网消息
func handleIoTMessage(clientState *ClientState, msg *ClientMessage) error {
	// 获取客户端状态
	sessionID := clientState.SessionID

	// 验证设备ID
	/*
		if _, err := s.authManager.GetSession(msg.DeviceID); err != nil {
			return fmt.Errorf("会话验证失败: %v", err)
		}*/

	// 发送 IoT 响应
	response := ServerMessage{
		Type:      ServerMessageTypeIot,
		Text:      msg.Text,
		SessionID: sessionID,
		State:     MessageStateSuccess,
	}

	// 发送响应
	if err := clientState.SendMsg(response); err != nil {
		return fmt.Errorf("发送响应失败: %v", err)
	}

	// 记录日志
	log.Infof("设备 %s 物联网指令: %s", msg.DeviceID, msg.Text)
	return nil
}

func handleMcpMessage(clientState *ClientState, msg *ClientMessage) error {
	mcpSession := mcp.GetDeviceMcpClient(clientState.DeviceID)
	if mcpSession != nil {
		select {
		case clientState.McpRecvMsgChan <- msg.PayLoad:
		default:
			log.Warnf("mcp 接收消息通道已满, 丢弃消息")
		}
	}
	return nil
}

func initIotOverMcp(clientState *ClientState) error {
	mcpSession := mcp.GetDeviceMcpClient(clientState.DeviceID)
	if mcpSession == nil {
		mcpSession = mcp.NewDeviceMCPSession(clientState.DeviceID)
		mcp.AddDeviceMcpClient(clientState.DeviceID, mcpSession)
	}

	mcpTransport := &McpTransport{
		Client: clientState,
	}

	iotOverMcp := mcp.NewIotOverMcpClient(clientState.DeviceID, mcpTransport)
	mcpSession.SetIotOverMcp(iotOverMcp)

	return nil
}

// startChat 开始对话
func startChat(ctx context.Context, clientState *ClientState, text string) error {
	// 获取客户端状态

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

	err = DoLLmRequest(ctx, clientState, requestEinoMessages, sessionID, einoTools)
	if err != nil {
		log.Errorf("发送带工具的 LLM 请求失败, seesionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("发送带工具的 LLM 请求失败: %v", err)
	}

	return nil
}

func DoLLmRequest(ctx context.Context, clientState *ClientState, requestEinoMessages []*schema.Message, sessionID string, einoTools []*schema.ToolInfo) error {
	log.Debugf("发送带工具的 LLM 请求, seesionID: %s, requestEinoMessages: %+v", sessionID, requestEinoMessages)
	clientState.SetStatus(ClientStatusLLMStart)
	responseSentences, err := llm.HandleLLMWithContextAndTools(
		ctx,
		clientState.LLMProvider,
		requestEinoMessages,
		einoTools,
		sessionID,
	)
	if err != nil {
		log.Errorf("发送带工具的 LLM 请求失败, seesionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("发送带工具的 LLM 请求失败: %v", err)
	}

	go func() {
		log.Debugf("DoLLmRequest goroutine开始 - SessionID: %s, context状态: %v", sessionID, ctx.Err())
		ok, err := HandleLLMResponse(ctx, clientState, requestEinoMessages, responseSentences)
		if err != nil {
			log.Errorf("处理 LLM 响应失败, seesionID: %s, error: %v", sessionID, err)
			clientState.CancelSessionCtx()
		}

		log.Debugf("DoLLmRequest goroutine结束 - SessionID: %s, ok: %v", sessionID, ok)
		_ = ok
		/*
			if ok {
				s.handleContinueChat(state)
			}*/
	}()

	return nil
}
