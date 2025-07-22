package chat

import (
	"context"
	"fmt"

	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/play_music"
	log "xiaozhi-esp32-server-golang/logger"
)

//此文件处理 local mcp tool 与 session绑定 的工具调用

// 关闭会话
func (c *ChatManager) LocalMcpCloseChat() error {
	c.Close()
	return nil
}

// 清空历史对话
func (c *ChatManager) LocalMcpClearHistory() error {
	llm_memory.Get().ResetMemory(c.ctx, c.DeviceID)
	return nil
}

// 播放音乐
func (c *ChatManager) LocalMcpPlayMusic(ctx context.Context, musicName string) error {
	log.Infof("开始播放音乐: %s", musicName)

	// 这里可以根据音乐名称获取音乐URL
	// 目前简化实现，假设musicName就是URL或者从配置中获取
	musicURL := c.getMusicURL(musicName)
	if musicURL == "" {
		return fmt.Errorf("未找到音乐: %s", musicName)
	}

	// 发送音乐播放开始消息
	if c.session != nil && c.session.serverTransport != nil {
		c.session.serverTransport.SendTtsStart()
	}

	// 使用music_player播放音乐
	audioChan, err := play_music.PlayMusicStream(ctx, musicURL, c.clientState.OutputAudioFormat.SampleRate, c.clientState.OutputAudioFormat.FrameDuration, "mp3")
	if err != nil {
		log.Errorf("播放音乐失败: %v", err)
		return fmt.Errorf("播放音乐失败: %v", err)
	}

	// 发送音频流到客户端
	go func() {
		defer func() {
			if c.session != nil && c.session.serverTransport != nil {
				c.session.serverTransport.SendTtsStop()
			}
			log.Infof("音乐播放完成: %s", musicName)
		}()

		for {
			select {
			case <-ctx.Done():
				log.Debugf("音乐播放上下文取消: %s", musicName)
				return
			case audioFrame, ok := <-audioChan:
				if !ok {
					// 音频流结束
					log.Debugf("音乐播放结束: %s", musicName)
					return
				}

				// 发送音频帧到客户端
				if c.session != nil && c.session.serverTransport != nil {
					if err := c.session.serverTransport.SendAudio(audioFrame); err != nil {
						log.Errorf("发送音乐音频帧失败: %v", err)
						return
					}
				}
			}
		}
	}()

	return nil
}

// getMusicURL 根据音乐名称获取URL
func (c *ChatManager) getMusicURL(musicName string) string {
	return "https://freetyst.nf.migu.cn/public%2Fproduct9th%2Fproduct46%2F2024%2F08%2F2317%2F2016%E5%B9%B401%E6%9C%8820%E6%97%A511%E7%82%B929%E5%88%86%E5%86%85%E5%AE%B9%E5%87%86%E5%85%A5%E6%AD%A3%E4%B8%9C537%E9%A6%96%2F%E5%85%A8%E6%9B%B2%E8%AF%95%E5%90%AC%2FMp3_64_22_16%2F6005660FVS8174331.mp3?Key=6c2bc6dd7b5361dc&Tim=1753089480488&channelid=01&msisdn=5291d20681e24976a7e9c761e3e149ba"
	// 简化实现：可以从配置文件或数据库中获取音乐URL映射
	// 目前直接假设musicName就是URL
	if musicName == "" {
		return ""
	}

	// 如果包含http://或https://，直接作为URL使用
	if len(musicName) > 7 && (musicName[:7] == "http://" || musicName[:8] == "https://") {
		return musicName
	}

	// 简化示例：预定义一些音乐URL映射
	musicURLMap := map[string]string{
		"测试音乐": "http://music.163.com/song/media/outer/url?id=123456",
		"轻音乐":  "http://music.163.com/song/media/outer/url?id=789012",
		"示例音乐": "http://example.com/music.mp3",
	}

	if url, exists := musicURLMap[musicName]; exists {
		return url
	}

	// 如果没有找到映射，可以返回默认URL或空字符串
	log.Warnf("未找到音乐URL映射: %s", musicName)
	return ""
}
