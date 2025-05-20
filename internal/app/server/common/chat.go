package common

import (
	"context"
	"fmt"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	log "xiaozhi-esp32-server-golang/logger"
	"runtime/debug"
)

func handleListenStart(state *ClientState, msg *ClientMessage) error {
	// 处理拾音模式
	if msg.Mode != "" {
		state.ListenMode = msg.Mode
		log.Infof("设备 %s 拾音模式: %s", msg.DeviceID, msg.Mode)
	}
	return Restart(state)
}

func handleListenStop(state *ClientState) error {
	// 停止录音
	state.SetClientHaveVoice(true)
	state.SetClientVoiceStop(true)
	state.SetClientHaveVoiceLastTime(0)
	state.Destroy()
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

	log.Debugf("Restart start: %+s", debug.Stack())
	defer log.Debugf("Restart end")

	state.Destroy()
	state.ResetSessionCtx()
	ctx := state.GetSessionCtx()

	//初始化asr相关
	state.VoiceStatus.Reset()
	state.AsrAudioBuffer.ClearAsrAudioData()

	if state.ListenMode != "auto" {
		state.VoiceStatus.SetClientHaveVoice(true)
	}

	// 启动asr流式识别
	state.Asr.Ctx, state.Asr.Cancel = context.WithCancel(ctx)
	state.Asr.AsrAudioChannel = make(chan []float32, 100)
	asrResultChannel, err := state.AsrProvider.StreamingRecognize(state.Asr.Ctx, state.Asr.AsrAudioChannel)
	if err != nil {
		log.Errorf("启动asr流式识别失败: %v", err)
		return fmt.Errorf("启动asr流式识别失败: %v", err)
	}
	state.AsrResultChannel = asrResultChannel

	// 启动一个goroutine处理asr结果
	go func() {
		text, err := state.RetireAsrResult(ctx)
		if err != nil {
			log.Errorf("处理asr结果失败: %v", err)
			return
		}
		log.Debugf("处理asr结果: %s", text)
		if text != "" {
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
		} else {
			//如果asr结果为空，则继续对话
			//handleContinueChat(state)
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
