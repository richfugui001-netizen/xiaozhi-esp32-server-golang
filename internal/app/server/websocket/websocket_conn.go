package websocket

import (
	"context"
	"errors"
	"sync"
	"time"
	"xiaozhi-esp32-server-golang/internal/app/server/types"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/gorilla/websocket"
)

// WebSocketConn 实现 types.IConn 接口，适配 WebSocket 连接
type WebSocketConn struct {
	ctx    context.Context
	cancel context.CancelFunc

	onCloseCb func()

	conn     *websocket.Conn
	deviceID string

	recvCmdChan   chan []byte
	recvAudioChan chan []byte
	sync.RWMutex
}

// NewWebSocketConn 创建一个新的 WebSocketConn 实例
func NewWebSocketConn(conn *websocket.Conn, deviceID string) *WebSocketConn {
	ctx, cancel := context.WithCancel(context.Background())
	instance := &WebSocketConn{
		ctx:           ctx,
		cancel:        cancel,
		conn:          conn,
		deviceID:      deviceID,
		recvCmdChan:   make(chan []byte, 100),
		recvAudioChan: make(chan []byte, 100),
	}

	go func() {
		for {
			select {
			case <-instance.ctx.Done():
				return
			default:
				instance.conn.SetReadDeadline(time.Now().Add(120 * time.Second))
				msgType, audio, err := instance.conn.ReadMessage()
				if err != nil {
					log.Errorf("read message error: %v", err)
					instance.onCloseCb() //通知chatManager进行退出
					return
				}

				if msgType == websocket.TextMessage {
					select {
					case instance.recvCmdChan <- audio:
					default:
						log.Errorf("recv cmd channel is full")
					}
				} else if msgType == websocket.BinaryMessage {
					select {
					case instance.recvAudioChan <- audio:
					default:
						log.Errorf("recv audio channel is full")
					}
				}
			}
		}
	}()

	return instance
}

func (w *WebSocketConn) SendCmd(msg []byte) error {
	w.Lock()
	defer w.Unlock()
	err := w.conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Errorf("send cmd error: %v", err)
		return err
	}
	return nil
}

func (w *WebSocketConn) SendAudio(audio []byte) error {
	w.Lock()
	defer w.Unlock()
	err := w.conn.WriteMessage(websocket.BinaryMessage, audio)
	if err != nil {
		log.Errorf("send audio error: %v", err)
		return err
	}
	return nil
}

func (w *WebSocketConn) RecvCmd(timeout int) ([]byte, error) {
	for {
		select {
		case msg := <-w.recvCmdChan:
			return msg, nil
		case <-time.After(time.Duration(timeout) * time.Second):
			return nil, errors.New("timeout")
		}
	}
}

func (w *WebSocketConn) RecvAudio(timeout int) ([]byte, error) {
	for {
		select {
		case audio := <-w.recvAudioChan:
			return audio, nil
		case <-time.After(time.Duration(timeout) * time.Second):
			return nil, errors.New("timeout")
		}
	}
}

func (w *WebSocketConn) Close() error {
	w.cancel()
	w.conn.Close()
	close(w.recvCmdChan)
	close(w.recvAudioChan)
	return nil
}

func (w *WebSocketConn) OnClose(cb func()) {
	w.onCloseCb = cb
}

func (w *WebSocketConn) GetDeviceID() string {
	return w.deviceID
}

func (w *WebSocketConn) GetTransportType() string {
	return types.TransportTypeWebsocket
}

func (w *WebSocketConn) GetData(key string) (interface{}, error) {
	return nil, errors.New("not implemented")
}
