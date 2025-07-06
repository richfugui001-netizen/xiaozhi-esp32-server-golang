package common

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	types_audio "xiaozhi-esp32-server-golang/internal/data/audio"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"
)

type Protocol struct {
	clientState *ClientState
}

func NewProtocol(clientState *ClientState) *Protocol {
	return &Protocol{
		clientState: clientState,
	}
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

	if isMcp, ok := msg.Features["mcp"]; ok && isMcp {
		go initMcp(p.clientState)
	}

	p.clientState.InputAudioFormat = types_audio.AudioFormat{
		SampleRate:    msg.AudioParams.SampleRate,
		Channels:      msg.AudioParams.Channels,
		FrameDuration: msg.AudioParams.FrameDuration,
		Format:        msg.AudioParams.Format,
	}

	// 设置asr pcm帧大小, 输入音频格式, 给vad, asr使用
	p.clientState.SetAsrPcmFrameSize(p.clientState.InputAudioFormat.SampleRate, p.clientState.InputAudioFormat.Channels, p.clientState.InputAudioFormat.FrameDuration)

	ProcessVadAudio(p.clientState)

	// 发送 hello 响应
	return SendHello(p.clientState, "websocket", &p.clientState.OutputAudioFormat)
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
			if err := startChat(p.clientState.GetSessionCtx(), p.clientState, text); err != nil {
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

				err = startChat(ctx, p.clientState, text)
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
