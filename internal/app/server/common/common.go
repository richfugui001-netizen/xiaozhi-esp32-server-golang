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

	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/audio"

	. "xiaozhi-esp32-server-golang/internal/data/msg"

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
}

func HandleLLMResponse(ctx context.Context, state *ClientState, llmResponseChannel chan llm_common.LLMResponseStruct) (bool, error) {
	log.Debugf("HandleLLMResponse start")
	defer log.Debugf("HandleLLMResponse end")

	var fullText bytes.Buffer
	for {
		select {
		case llmResponse, ok := <-llmResponseChannel:
			if !ok {
				// 通道已关闭，退出协程
				log.Infof("LLM 响应通道已关闭，退出协程")
				return true, nil
			}

			log.Debugf("LLM 响应: %+v", llmResponse)

			// 使用带上下文的TTS处理
			outputChan, err := state.TTSProvider.TextToSpeechStream(state.Ctx, llmResponse.Text, state.OutputAudioFormat.SampleRate, state.OutputAudioFormat.Channels, state.OutputAudioFormat.FrameDuration)
			if err != nil {
				log.Errorf("生成 TTS 音频失败: %v", err)
				return true, fmt.Errorf("生成 TTS 音频失败: %v", err)
			}

			if llmResponse.IsStart {
				// 先发送文本
				response := ServerMessage{
					Type:      ServerMessageTypeTts,
					State:     MessageStateStart,
					SessionID: state.SessionID,
				}
				if err := state.SendMsg(response); err != nil {
					log.Errorf("发送 TTS Start 失败: %v", err)
					return true, fmt.Errorf("发送 TTS Start 失败: %v", err)
				}
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
				return true, fmt.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
			}

			fullText.WriteString(llmResponse.Text)

			// 发送音频帧
			if err := state.SendTTSAudio(ctx, outputChan, llmResponse.IsStart); err != nil {
				log.Errorf("发送 TTS 音频失败: %s, %v", llmResponse.Text, err)
				return true, fmt.Errorf("发送 TTS 音频失败: %s, %v", llmResponse.Text, err)
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
				return true, fmt.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
			}

			if llmResponse.IsEnd {
				//延迟50ms毫秒再发stop
				//time.Sleep(50 * time.Millisecond)
				//写到redis中
				llm_memory.Get().AddMessage(ctx, state.DeviceID, "assistant", fullText.String())
				// 发送结束消息
				response := ServerMessage{
					Type:      ServerMessageTypeTts,
					State:     MessageStateStop,
					SessionID: state.SessionID,
				}
				if err := state.SendMsg(response); err != nil {
					log.Errorf("发送 TTS 文本失败: stop, %v", err)
					return false, fmt.Errorf("发送 TTS 文本失败: stop")
				}

				return ok, nil
			}
		case <-ctx.Done():
			// 上下文已取消，退出协程
			log.Infof("设备 %s 连接已关闭，停止处理LLM响应", state.DeviceID)
			return false, nil
		}
	}

}

func ProcessVadAudio(state *ClientState) {
	go func() {
		audioFormat := state.InputAudioFormat
		audioProcesser, err := audio.GetAudioProcesser(audioFormat.SampleRate, audioFormat.Channels, audioFormat.FrameDuration)
		if err != nil {
			log.Errorf("获取解码器失败: %v", err)
			return
		}
		pcmFrame := make([]float32, state.AsrAudioBuffer.PcmFrameSize)

		vadNeedGetCount := 60 / audioFormat.FrameDuration

		var skipVad bool
		for {
			//sessionCtx := state.GetSessionCtx()
			select {
			case opusFrame, ok := <-state.OpusAudioBuffer:
				if state.GetClientVoiceStop() { //已停止 说话 则不接收音频数据
					//log.Infof("客户端停止说话, 跳过音频数据")
					continue
				}
				log.Debugf("processAsrAudio 收到音频数据, len: %d", len(opusFrame))
				if !ok {
					log.Debugf("processAsrAudio 音频通道已关闭")
					return
				}
				log.Debugf("clientVoiceStop: %+v, asrDataSize: %d\n", state.GetClientVoiceStop(), state.AsrAudioBuffer.GetAsrDataSize())
				clientHaveVoice := state.GetClientHaveVoice()
				var haveVoice bool
				if state.ListenMode != "auto" {
					haveVoice = true
					clientHaveVoice = true
					skipVad = true
				}

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
						err = state.Vad.Init()
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
						haveVoice, err = state.VadProvider.IsVAD(vadPcmData)
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
				} else {
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
					state.AsrAudioChannel <- pcmData
				}

				//已经有语音了, 但本次没有检测到语音, 则需要判断是否已经停止说话
				lastHaveVoiceTime := state.GetClientHaveVoiceLastTime()

				if clientHaveVoice && lastHaveVoiceTime > 0 && !haveVoice {
					diffMilli := time.Now().UnixMilli() - lastHaveVoiceTime
					if state.IsSilence(diffMilli) {
						state.SetClientVoiceStop(true)
						//客户端停止说话
						state.Asr.Stop()
						//释放vad
						state.Vad.Reset()
						//asr统计
						state.SetStartAsrTs()
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

	sessionID := clientState.SessionID

	// 根据状态处理
	switch msg.State {
	case MessageStateStart:
		handleListenStart(clientState, msg)
	case MessageStateStop:
		handleListenStop(clientState)
	case MessageStateDetect:
		// 唤醒词检测
		clientState.SetClientHaveVoice(false)

		// 如果有文本，处理唤醒词
		if msg.Text != "" {
			text := msg.Text
			// 移除标点符号和处理长度
			text = removePunctuation(text)

			// 检查是否是唤醒词
			isWakeupWord := isWakeupWord(text)
			enableGreeting := viper.GetBool("enable_greeting") // 从配置获取

			if isWakeupWord && !enableGreeting {
				// 如果是唤醒词，且关闭了唤醒词回复，发送 STT 消息后停止 TTS
				sttResponse := ServerMessage{
					Type:      ServerMessageTypeStt,
					Text:      text,
					SessionID: sessionID,
				}
				if err := clientState.SendMsg(sttResponse); err != nil {
					return fmt.Errorf("发送 STT 消息失败: %v", err)
				}
			} else {
				// 否则开始对话
				if err := startChat(clientState.GetSessionCtx(), clientState, text); err != nil {
					log.Errorf("开始对话失败: %v", err)
				}
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

// startChat 开始对话
func startChat(ctx context.Context, clientState *ClientState, text string) error {
	// 获取客户端状态

	sessionID := clientState.SessionID

	requestMessages, err := llm_memory.Get().GetMessagesForLLM(ctx, clientState.DeviceID, 10)
	if err != nil {
		log.Errorf("获取对话历史失败: %v", err)
	}

	requestMessages = append(requestMessages, llm_common.Message{
		Role:    "user",
		Content: text,
	})

	// 添加用户消息到对话历史
	llm_memory.Get().AddMessage(ctx, clientState.DeviceID, "user", text)

	ctx, cancel := context.WithCancel(ctx)
	_ = cancel

	// 发送 LLM 请求
	responseSentences, err := llm.HandleLLMWithContext(
		ctx,
		clientState.LLMProvider,
		messagesToInterfaces(requestMessages),
		sessionID,
	)
	if err != nil {
		log.Errorf("发送 LLM 请求失败, seesionID: %s, error: %v", sessionID, err)
		return fmt.Errorf("发送 LLM 请求失败: %v", err)
	}

	go func() {
		ok, err := HandleLLMResponse(ctx, clientState, responseSentences)
		if err != nil {
			cancel()
		}

		_ = ok
		/*
			if ok {
				s.handleContinueChat(state)
			}*/
	}()

	return nil
}

// 添加一个转换函数
func messagesToInterfaces(msgs []llm_common.Message) []interface{} {
	result := make([]interface{}, len(msgs))
	for i, msg := range msgs {
		result[i] = msg
	}
	return result
}
