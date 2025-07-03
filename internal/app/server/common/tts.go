package common

import (
	"bytes"
	"context"
	"fmt"
	"time"
	. "xiaozhi-esp32-server-golang/internal/data/client"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	log "xiaozhi-esp32-server-golang/logger"
)

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

	if err := SendSentenceStart(state, llmResponse.Text); err != nil {
		log.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
		return fmt.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
	}

	fullText.WriteString(llmResponse.Text)

	// 发送音频帧
	if err := SendTTSAudio(ctx, state, outputChan, llmResponse.IsStart); err != nil {
		log.Errorf("发送 TTS 音频失败: %s, %v", llmResponse.Text, err)
		return fmt.Errorf("发送 TTS 音频失败: %s, %v", llmResponse.Text, err)
	}

	if err := SendSentenceEnd(state, llmResponse.Text); err != nil {
		log.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
		return fmt.Errorf("发送 TTS 文本失败: %s, %v", llmResponse.Text, err)
	}

	return nil
}

func SendTTSAudio(ctx context.Context, clientState *ClientState, audioChan chan []byte, isStart bool) error {
	// 步骤1: 收集前三帧（或更少）
	preBuffer := make([][]byte, 0, 3)
	preBufferCount := 0

	totalFrames := preBufferCount // 跟踪已发送的总帧数

	isStatistic := true
	//首次发送180ms音频, 根据outputAudioFormat.FrameDuration计算
	firstFrameCount := 60 / clientState.OutputAudioFormat.FrameDuration
	if firstFrameCount > 20 || firstFrameCount < 3 {
		firstFrameCount = 5
	}
	// 收集前180ms音频
	for totalFrames < firstFrameCount {
		select {
		case <-ctx.Done():
			log.Debugf("SendTTSAudio context done, exit, totalFrames: %d", totalFrames)
			return nil
		default:
			select {
			case frame, ok := <-audioChan:
				if isStart && isStatistic {
					log.Debugf("从接收音频结束 asr->llm->tts首帧 整体 耗时: %d ms", clientState.GetAsrLlmTtsDuration())
					isStatistic = false
				}
				if !ok {
					// 通道已关闭，发送已收集的帧并返回
					for _, f := range preBuffer {
						if err := clientState.ActionSendAudioData(f); err != nil {
							return fmt.Errorf("发送 TTS 音频 len: %d 失败: %v", len(f), err)
						}
					}
					return nil
				}
				select {
				case <-ctx.Done():
					log.Debugf("SendTTSAudio context done, exit, totalFrames: %d", totalFrames)
					return nil
				default:
					if err := clientState.ActionSendAudioData(frame); err != nil {
						return fmt.Errorf("发送 TTS 音频 len: %d 失败: %v", len(frame), err)
					}
					log.Debugf("发送 TTS 音频: %d 帧, len: %d", totalFrames, len(frame))
					totalFrames++
				}
			case <-ctx.Done():
				// 上下文已取消
				log.Infof("SendTTSAudio context done, exit, totalFrames: %d", totalFrames)
				return nil
			}
		}
	}

	// 步骤3: 设置定时器，每60ms发送一帧
	ticker := time.NewTicker(time.Duration(clientState.OutputAudioFormat.FrameDuration) * time.Millisecond)
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

				select {
				case <-ctx.Done():
					log.Debugf("SendTTSAudio context done, exit")
					return nil
				default:
					// 发送当前帧
					if err := clientState.ActionSendAudioData(frame); err != nil {
						return fmt.Errorf("发送 TTS 音频 len: %d 失败: %v", len(frame), err)
					}
					totalFrames++
					//log.Debugf("发送 TTS 音频: %d 帧, len: %d", totalFrames, len(frame))
				}
			default:
				// 没有帧可获取，等待下一个周期
			}
		case <-ctx.Done():
			// 上下文已取消
			log.Infof("SendTTSAudio context done, exit, totalFrames: %d", totalFrames)
			return nil
		}
	}
}
