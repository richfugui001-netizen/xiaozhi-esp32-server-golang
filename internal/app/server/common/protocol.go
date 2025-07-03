package common

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"
	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

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
	case MessageTypeGoodBye:
		return handleGoodByeMessage(clientState, &clientMsg)
	default:
		// 未知消息类型，直接回显
		return clientState.Conn.WriteMessage(websocket.TextMessage, message)
	}
}

// HandleAudioMessage 处理音频消息
func HandleAudioMessage(clientState *ClientState, data []byte) bool {
	select {
	case clientState.OpusAudioBuffer <- data:
		return true
	default:
		log.Warnf("音频缓冲区已满, 丢弃音频数据")
	}
	return false
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
	return SendHello(clientState, "websocket", &clientState.OutputAudioFormat)
}

// handleListenMessage 处理监听消息
func handleListenMessage(clientState *ClientState, msg *ClientMessage) error {
	// 根据状态处理
	switch msg.State {
	case MessageStateStart:
		handleListenStart(clientState, msg)
	case MessageStateStop:
		handleListenStop(clientState)
	case MessageStateDetect:
		// 唤醒词检测
		StopSpeaking(clientState, false)

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

// handleAbortMessage 处理中止消息
func handleAbortMessage(clientState *ClientState, msg *ClientMessage) error {
	// 设置打断状态
	clientState.Abort = true
	clientState.Dialogue.Messages = nil // 清空对话历史

	StopSpeaking(clientState, true)

	// 记录日志
	log.Infof("设备 %s abort 会话", msg.DeviceID)
	return nil
}

// handleIoTMessage 处理物联网消息
func handleIoTMessage(clientState *ClientState, msg *ClientMessage) error {
	// 获取客户端状态
	//sessionID := clientState.SessionID

	// 验证设备ID
	/*
		if _, err := s.authManager.GetSession(msg.DeviceID); err != nil {
			return fmt.Errorf("会话验证失败: %v", err)
		}*/

	// 发送 IoT 响应
	err := SendIot(clientState, msg)
	if err != nil {
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

func handleGoodByeMessage(clientState *ClientState, msg *ClientMessage) error {
	clientState.Conn.Close()
	return nil
}

func handleListenStart(state *ClientState, msg *ClientMessage) error {
	// 处理拾音模式
	if msg.Mode != "" {
		state.ListenMode = msg.Mode
		log.Infof("设备 %s 拾音模式: %s", msg.DeviceID, msg.Mode)
	}
	if state.ListenMode == "manual" {
		StopSpeaking(state, false)
	}
	state.SetStatus(ClientStatusListening)

	return OnListenStart(state)
}

func handleListenStop(state *ClientState) error {
	if state.ListenMode == "auto" {
		state.CancelSessionCtx()
	}

	//调用
	state.OnManualStop()

	return nil
}

func OnListenStart(state *ClientState) error {
	log.Debugf("OnListenStart start")
	defer log.Debugf("OnListenStart end")

	select {
	case <-state.Ctx.Done():
		log.Debugf("OnListenStart Ctx done, return")
		return nil
	default:
	}

	state.Destroy()

	ctx := state.GetSessionCtx()

	//初始化asr相关
	if state.ListenMode == "manual" {
		state.VoiceStatus.SetClientHaveVoice(true)
	}

	// 启动asr流式识别，复用 restartAsrRecognition 函数
	err := restartAsrRecognition(ctx, state)
	if err != nil {
		log.Errorf("asr流式识别失败: %v", err)
		state.Conn.Close()
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

			text, err := state.RetireAsrResult(ctx)
			if err != nil {
				log.Errorf("处理asr结果失败: %v", err)
				return
			}

			//统计asr耗时
			log.Debugf("处理asr结果: %s, 耗时: %d ms", text, state.GetAsrDuration())

			if text != "" {
				// 重置重试计数器
				startIdleTime = 0

				//当获取到asr结果时, 结束语音输入
				state.OnVoiceSilence()

				//发送asr消息
				err = SendAsrResult(state, text)
				if err != nil {
					log.Errorf("发送asr消息失败: %v", err)
					return
				}

				err = startChat(ctx, state, text)
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
				log.Debugf("ready Restart Asr, state.Status: %s", state.Status)
				if state.Status == ClientStatusListening || state.Status == ClientStatusListenStop {
					// text 为空，检查是否需要重新启动ASR
					diffTs := time.Now().Unix() - startIdleTime
					if startIdleTime > 0 && diffTs <= maxIdleTime {
						log.Warnf("ASR识别结果为空，尝试重启ASR识别, diff ts: %s", diffTs)
						if restartErr := restartAsrRecognition(ctx, state); restartErr != nil {
							log.Errorf("重启ASR识别失败: %v", restartErr)
							return
						}
						continue
					} else {
						log.Warnf("ASR识别结果为空，已达到最大空闲时间: %d", maxIdleTime)
						state.Conn.Close()
						return
					}
				}
			}
			return
		}
	}()
	return nil
}
