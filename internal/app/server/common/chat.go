package common

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	log "xiaozhi-esp32-server-golang/logger"
)

func handleListenStart(state *ClientState, msg *ClientMessage) error {
	// 处理拾音模式
	if msg.Mode != "" {
		state.ListenMode = msg.Mode
		log.Infof("设备 %s 拾音模式: %s", msg.DeviceID, msg.Mode)
	}

	state.CancelSessionCtx()
	state.SetStatus(ClientStatusListening)

	return Restart(state)
}

func handleListenStop(state *ClientState) error {
	if state.ListenMode == "auto" {
		state.CancelSessionCtx()
	}

	//调用
	state.OnManualStop()

	return nil
}

func Restart(state *ClientState) error {
	//记录下调用栈
	//log.Debugf("重启拾音, 调用栈: %s", string(debug.Stack()))

	select {
	case <-state.Ctx.Done():
		log.Debugf("Restart Ctx done, return")
		return nil
	default:
	}

	log.Debugf("Restart start")
	defer log.Debugf("Restart end")

	state.Destroy()

	ctx := state.GetSessionCtx()

	//初始化asr相关
	if state.ListenMode != "auto" {
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
				response := ServerMessage{
					Type:      ServerMessageTypeStt,
					SessionID: state.SessionID,
					Text:      text,
				}
				if err := state.Conn.WriteJSON(response); err != nil {
					log.Errorf("发送asr消息失败: %v", err)
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

func handleContinueChat(state *ClientState) error {
	log.Debugf("handleContainueChat start")
	defer log.Debugf("handleContainueChat end")

	select {
	case <-state.Ctx.Done():
		log.Debugf("handleContinueChat Ctx done, return")
		return nil
	default:
	}
	return Restart(state)
}

// restartAsrRecognition 重启ASR识别
func restartAsrRecognition(ctx context.Context, state *ClientState) error {
	log.Debugf("重启ASR识别开始")

	// 取消当前ASR上下文
	if state.Asr.Cancel != nil {
		state.Asr.Cancel()
	}

	state.VoiceStatus.Reset()
	state.AsrAudioBuffer.ClearAsrAudioData()

	// 等待一小段时间让资源清理
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 重新创建ASR上下文和通道
	state.Asr.Ctx, state.Asr.Cancel = context.WithCancel(ctx)
	state.Asr.AsrAudioChannel = make(chan []float32, 100)

	// 重新启动流式识别
	asrResultChannel, err := state.AsrProvider.StreamingRecognize(state.Asr.Ctx, state.Asr.AsrAudioChannel)
	if err != nil {
		log.Errorf("重启ASR流式识别失败: %v", err)
		return fmt.Errorf("重启ASR流式识别失败: %v", err)
	}

	state.AsrResultChannel = asrResultChannel
	log.Debugf("重启ASR识别成功")
	return nil
}
