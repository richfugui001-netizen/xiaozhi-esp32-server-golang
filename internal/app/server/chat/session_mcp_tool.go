package chat

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/play_music"
	log "xiaozhi-esp32-server-golang/logger"
)

//此文件处理 local mcp tool 与 session绑定 的工具调用

// 音乐搜索API响应结构
type MusicSearchResponse struct {
	Data  []MusicItem `json:"data"`
	Code  int         `json:"code"`
	Error string      `json:"error"`
}

type MusicItem struct {
	Type   string `json:"type"`
	Link   string `json:"link"`
	SongID string `json:"songid"`
	Title  string `json:"title"`
	Author string `json:"author"`
	LRC    bool   `json:"lrc"`
	URL    string `json:"url"`
	Pic    string `json:"pic"`
}

// 全局HTTP客户端
var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// 获取配置了连接池的HTTP客户端
func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		httpClient = &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
	})
	return httpClient
}

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
	log.Infof("搜索音乐: %s 中", musicName)

	// 这里可以根据音乐名称获取音乐URL
	// 目前简化实现，假设musicName就是URL或者从配置中获取
	musicURL, realMusicName, err := c.getMusicURL(musicName)
	if err != nil {
		return fmt.Errorf("获取音乐URL失败: %v", err)
	}
	if musicURL == "" {
		return fmt.Errorf("未找到音乐: %s", musicName)
	}
	log.Infof("找到音乐: %s, URL: %s", realMusicName, musicURL)

	// 使用music_player播放音乐
	audioChan, err := play_music.PlayMusicStream(ctx, musicURL, c.clientState.OutputAudioFormat.SampleRate, c.clientState.OutputAudioFormat.FrameDuration, "mp3")
	if err != nil {
		log.Errorf("播放音乐失败: %v", err)
		return fmt.Errorf("播放音乐失败: %v", err)
	}

	// 发送音频流到客户端
	go func() {
		playText := fmt.Sprintf("正在播放音乐: %s", realMusicName)
		c.session.serverTransport.SendSentenceStart(playText)
		defer func() {
			c.session.serverTransport.SendSentenceEnd(playText)
			if c.session != nil && c.session.serverTransport != nil {
				c.session.serverTransport.SendTtsStop()
			}
			log.Infof("音乐播放完成: %s", realMusicName)
		}()

		for {
			select {
			case <-ctx.Done():
				log.Debugf("音乐播放上下文取消: %s", realMusicName)
				return
			case audioFrame, ok := <-audioChan:
				if !ok {
					// 音频流结束
					log.Debugf("音乐播放结束: %s", realMusicName)
					return
				}

				// 发送音频帧到客户端
				if c.session != nil && c.session.serverTransport != nil {
					if err := c.session.serverTransport.SendAudio(audioFrame); err != nil {
						log.Errorf("发送音乐音频帧失败: %v", err)
						return
					}
					time.Sleep(time.Duration(c.clientState.OutputAudioFormat.FrameDuration) * time.Millisecond)
				}
			}
		}
	}()

	return nil
}

// getMusicURL 根据音乐名称获取URL
func (c *ChatManager) getMusicURL(musicName string) (string, string, error) {

	musicURL := "https://freetyst.nf.migu.cn/public%2Fproduct9th%2Fproduct46%2F2024%2F08%2F2317%2F2016%E5%B9%B401%E6%9C%8820%E6%97%A511%E7%82%B929%E5%88%86%E5%86%85%E5%AE%B9%E5%87%86%E5%85%A5%E6%AD%A3%E4%B8%9C537%E9%A6%96%2F%E5%85%A8%E6%9B%B2%E8%AF%95%E5%90%AC%2FMp3_64_22_16%2F6005660FVS8174331.mp3?Key=d3b04946ff6297ec&Tim=1753430018842&channelid=01&msisdn=06c3638197af48a98d0c7d91200c8279"
	return musicURL, musicName, nil
}
