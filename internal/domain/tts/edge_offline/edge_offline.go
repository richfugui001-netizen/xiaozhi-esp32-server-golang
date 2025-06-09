package edge_offline

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/tts/common"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gopxl/beep"
	"github.com/gorilla/websocket"
)

const (
	// WebSocket缓冲区大小
	wsBufferSize = 1024 * 1024 // 1MB
)

var (
	// 全局连接池
	wsConnPool     = make(map[*websocket.Conn]*wsConnWrapper)
	wsConnPoolLock sync.RWMutex

	// 全局 WebSocket 配置
	wsDialer = websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
)

// wsConnWrapper WebSocket 连接包装器
type wsConnWrapper struct {
	conn         *websocket.Conn
	InUse        bool
	lastActiveAt time.Time
}

// EdgeOfflineTTSProvider WebSocket TTS 提供者
type EdgeOfflineTTSProvider struct {
	ServerURL string
	Timeout   time.Duration
}

// NewEdgeOfflineTTSProvider 创建新的 Edge Offline TTS 提供者
func NewEdgeOfflineTTSProvider(config map[string]interface{}) *EdgeOfflineTTSProvider {
	serverURL, _ := config["server_url"].(string)
	timeout, _ := config["timeout"].(float64)

	// 设置默认值
	if serverURL == "" {
		serverURL = "ws://localhost:8080/tts"
	}
	if timeout == 0 {
		timeout = 30 // 默认30秒超时
	}

	return &EdgeOfflineTTSProvider{
		ServerURL: serverURL,
		Timeout:   time.Duration(timeout) * time.Second,
	}
}

// getConnection 从连接池获取或创建新的连接
func (p *EdgeOfflineTTSProvider) getConnection(ctx context.Context) (*wsConnWrapper, error) {
	// 创建新连接
	conn, _, err := wsDialer.DialContext(ctx, p.ServerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("WebSocket连接失败: %v", err)
	}

	wrapper := &wsConnWrapper{
		conn:         conn,
		lastActiveAt: time.Now(),
	}
	return wrapper, nil
	/*
		wsConnPoolLock.Lock()
		defer wsConnPoolLock.Unlock()

		// 查找可用的连接
		for conn, wrapper := range wsConnPool {
			if !wrapper.InUse && time.Since(wrapper.lastActiveAt) < 30*time.Second {
				wrapper.lastActiveAt = time.Now()
				wrapper.InUse = true
				return wrapper, nil
			}
			// 移除过期或正在使用的连接
			conn.Close()
			delete(wsConnPool, conn)
		}

		// 创建新连接
		conn, _, err := wsDialer.DialContext(ctx, p.ServerURL, nil)
		if err != nil {
			return nil, fmt.Errorf("WebSocket连接失败: %v", err)
		}

		wrapper := &wsConnWrapper{
			conn:         conn,
			lastActiveAt: time.Now(),
		}
		wsConnPool[conn] = wrapper
		wrapper.InUse = true
		return wrapper, nil
	*/
}

// removeConnection 从连接池中移除连接
func (p *EdgeOfflineTTSProvider) removeConnection(conn *websocket.Conn) {
	if conn == nil {
		return
	}

	wsConnPoolLock.Lock()
	defer wsConnPoolLock.Unlock()

	if wrapper, exists := wsConnPool[conn]; exists {
		wrapper.conn.Close()
		delete(wsConnPool, conn)
	}
}

// ReleaseConn 释放连接回连接池
func (p *EdgeOfflineTTSProvider) ReleaseConn(wsConn *wsConnWrapper) {
	wsConnPoolLock.Lock()
	defer wsConnPoolLock.Unlock()

	wsConn.InUse = false
	wsConn.lastActiveAt = time.Now()
}

// TextToSpeech 将文本转换为语音，返回音频帧数据
func (p *EdgeOfflineTTSProvider) TextToSpeech(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) ([][]byte, error) {
	var frames [][]byte

	// 创建一个新的上下文，带有超时
	ctxWithTimeout, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	// 获取连接
	wrapper, err := p.getConnection(ctxWithTimeout)
	if err != nil {
		return nil, err
	}

	// 发送文本
	err = wrapper.conn.WriteMessage(websocket.TextMessage, []byte(text))
	if err != nil {
		p.removeConnection(wrapper.conn)
		return nil, fmt.Errorf("发送文本失败: %v", err)
	}

	// 创建管道用于音频数据传输
	pipeReader, pipeWriter := io.Pipe()
	outputChan := make(chan []byte, 1000)
	startTs := time.Now().UnixMilli()

	// 创建音频解码器
	audioDecoder, err := common.CreateAudioDecoder(ctxWithTimeout, pipeReader, outputChan, frameDuration, "mp3")
	if err != nil {
		pipeReader.Close()
		return nil, fmt.Errorf("创建音频解码器失败: %v", err)
	}

	// 启动解码器
	go func() {
		if err := audioDecoder.Run(startTs); err != nil {
			log.Errorf("音频解码失败: %v", err)
		}
	}()

	// 接收WebSocket数据并写入管道
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer pipeWriter.Close()

		for {
			messageType, data, err := wrapper.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				log.Errorf("读取WebSocket消息失败: %v", err)
				return
			}

			if messageType == websocket.BinaryMessage {
				if _, err := pipeWriter.Write(data); err != nil {
					log.Errorf("写入音频数据失败: %v", err)
					return
				}
			}
		}
	}()

	// 收集所有的Opus帧
	go func() {
		for frame := range outputChan {
			frames = append(frames, frame)
		}
	}()

	// 等待完成或超时
	select {
	case <-ctxWithTimeout.Done():
		p.removeConnection(wrapper.conn)
		return nil, fmt.Errorf("TTS合成超时或被取消")
	case <-done:
		wrapper.lastActiveAt = time.Now()
		close(outputChan)
		return frames, nil
	}
}

// TextToSpeechStream 流式语音合成
func (p *EdgeOfflineTTSProvider) TextToSpeechStream(ctx context.Context, text string, sampleRate int, channels int, frameDuration int) (chan []byte, error) {
	outputChan := make(chan []byte, 100)

	go func() {

		// 创建一个新的上下文，带有超时
		ctxWithTimeout, cancel := context.WithTimeout(ctx, p.Timeout)
		defer cancel()

		// 获取连接
		wrapper, err := p.getConnection(ctxWithTimeout)
		if err != nil {
			log.Errorf("获取WebSocket连接失败: %v", err)
			return
		}
		defer p.ReleaseConn(wrapper)

		// 发送文本
		err = wrapper.conn.WriteMessage(websocket.TextMessage, []byte(text))
		if err != nil {
			p.removeConnection(wrapper.conn)
			log.Errorf("发送文本失败: %v", err)
			return
		}

		// 创建管道用于音频数据传输
		pipeReader, pipeWriter := io.Pipe()

		defer func() {
			pipeReader.Close()
			pipeWriter.Close()
		}()

		// 启动解码器
		go func() {
			startTs := time.Now().UnixMilli()
			// 创建音频解码器
			audioDecoder, err := common.CreateAudioDecoder(ctxWithTimeout, pipeReader, outputChan, frameDuration, "pcm")
			if err != nil {
				pipeReader.Close()
				log.Errorf("创建音频解码器失败: %v", err)
				return
			}

			audioDecoder.WithFormat(beep.Format{
				SampleRate:  beep.SampleRate(sampleRate),
				NumChannels: channels,
				Precision:   2,
			})

			if err := audioDecoder.Run(startTs); err != nil {
				log.Errorf("音频解码失败: %v", err)
			}
		}()

		// 接收WebSocket数据并写入管道
		for {
			select {
			case <-ctxWithTimeout.Done():
				p.removeConnection(wrapper.conn)
				return
			default:
				messageType, data, err := wrapper.conn.ReadMessage()
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						return
					}
					log.Errorf("读取WebSocket消息失败: %v", err)
					p.removeConnection(wrapper.conn)
					return
				}

				if messageType == websocket.BinaryMessage {
					if _, err := pipeWriter.Write(data); err != nil {
						log.Errorf("写入音频数据失败: %v", err)
						p.removeConnection(wrapper.conn)
						return
					}
					return
				}
			}
		}
	}()

	return outputChan, nil
}
