package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	log "xiaozhi-esp32-server-golang/logger"
)

const (
	// DefaultRequestTimeout 默认请求超时时间
	DefaultRequestTimeout = 30 * time.Second
	// DefaultCloseTimeout 默认关闭超时时间
	DefaultCloseTimeout = 5 * time.Second
)

/**
// Interface for the transport layer.
type Interface interface {
	// Start the connection. Start should only be called once.
	Start(ctx context.Context) error

	// SendRequest sends a json RPC request and returns the response synchronously.
	SendRequest(ctx context.Context, request JSONRPCRequest) (*JSONRPCResponse, error)

	// SendNotification sends a json RPC Notification to the server.
	SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error

	// SetNotificationHandler sets the handler for notifications.
	// Any notification before the handler is set will be discarded.
	SetNotificationHandler(handler func(notification mcp.JSONRPCNotification))

	// Close the connection.
	Close() error
}
*/

type WebsocketTransport struct {
	url  string
	conn *websocket.Conn

	notifyHandler func(notification mcp.JSONRPCNotification)
	// 添加关闭回调
	onCloseHandler func(reason string)

	// 响应通道管理
	respChans    map[string]chan *transport.JSONRPCResponse
	respChansMux sync.RWMutex

	// 消息监听控制
	readDone chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc

	// 连接状态
	closed    bool
	closedMux sync.RWMutex

	// WebSocket写入锁，防止并发写入
	writeMux sync.Mutex

	// 超时配置
	requestTimeout time.Duration
	closeTimeout   time.Duration
}

func (t *WebsocketTransport) Send(ctx context.Context, msg []byte) error {
	// 检查连接状态
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// 发送消息（使用互斥锁保护写入操作）
	t.writeMux.Lock()
	err := t.conn.WriteMessage(websocket.TextMessage, msg)
	t.writeMux.Unlock()
	return err
}

func NewWebsocketTransport(conn *websocket.Conn) (*WebsocketTransport, error) {
	ctx, cancel := context.WithCancel(context.Background())

	wst := &WebsocketTransport{
		conn:           conn,
		respChans:      make(map[string]chan *transport.JSONRPCResponse),
		readDone:       make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
		requestTimeout: DefaultRequestTimeout,
		closeTimeout:   DefaultCloseTimeout,
	}
	// 启动消息监听协程
	go wst.readMessages()

	return wst, nil
}

// 实现 Interface 接口
func (t *WebsocketTransport) Start(ctx context.Context) error {
	return nil
}

// readMessages 持续监听 WebSocket 消息
func (t *WebsocketTransport) readMessages() {
	defer close(t.readDone)

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			// 使用 Go 语言级别的超时控制
			_, message, err := t.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Errorf("WebSocket read error: %v", err)
				}

				// 连接关闭时通知client层
				if t.onCloseHandler != nil {
					reason := "connection_closed"
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						reason = "normal_closure"
					} else if websocket.IsUnexpectedCloseError(err) {
						reason = "unexpected_closure"
					}
					t.onCloseHandler(reason)
				}

				return
			}

			// 处理接收到的消息
			t.handleMessage(message)
		}
	}
}

// handleMessage 处理接收到的消息
func (t *WebsocketTransport) handleMessage(message []byte) {
	// 尝试解析为 JSON-RPC 响应
	var response transport.JSONRPCResponse
	if err := json.Unmarshal(message, &response); err == nil {
		// 这是一个 JSON-RPC 响应
		t.handleResponse(&response)
		return
	}

	// 尝试解析为 JSON-RPC 通知
	var notification mcp.JSONRPCNotification
	if err := json.Unmarshal(message, &notification); err == nil && notification.Method != "" {
		// 这是一个 JSON-RPC 通知
		t.handleNotification(&notification)
		return
	}

	// 无法识别的消息格式
	log.Warnf("Received unrecognized message: %s", string(message))
}

// handleResponse 处理 JSON-RPC 响应
func (t *WebsocketTransport) handleResponse(response *transport.JSONRPCResponse) {
	respByte, _ := json.Marshal(response)
	// 将 ID 转换为字符串作为键
	idStr := response.ID.String()

	t.respChansMux.RLock()
	respChan, exists := t.respChans[idStr]
	t.respChansMux.RUnlock()

	if exists {
		// 发送响应到对应的通道
		select {
		case respChan <- response:
			// 响应已发送，清理通道
			t.respChansMux.Lock()
			delete(t.respChans, idStr)
			t.respChansMux.Unlock()
			close(respChan)
		case <-time.After(t.requestTimeout):
			log.Warnf("websocket mcp handleResponse timeout for ID: %s, response: %+v", idStr, string(respByte))
		}
	} else {
		log.Warnf("No response channel found for ID: %s, response: %+v", idStr, string(respByte))
	}
}

// handleNotification 处理 JSON-RPC 通知
func (t *WebsocketTransport) handleNotification(notification *mcp.JSONRPCNotification) {
	if t.notifyHandler != nil {
		t.notifyHandler(*notification)
	}
}

func (t *WebsocketTransport) SendRequest(ctx context.Context, request transport.JSONRPCRequest) (*transport.JSONRPCResponse, error) {
	// 检查连接状态
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return nil, fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// 创建响应通道
	idStr := request.ID.String()

	respChan := make(chan *transport.JSONRPCResponse, 1)

	// 注册响应通道
	t.respChansMux.Lock()
	t.respChans[idStr] = respChan
	t.respChansMux.Unlock()

	// 发送请求（使用互斥锁保护写入操作）
	t.writeMux.Lock()
	err := t.conn.WriteJSON(request)
	t.writeMux.Unlock()
	if err != nil {
		// 发送失败，清理通道
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 使用 Go 语言级别的超时控制等待响应
	select {
	case response := <-respChan:
		return response, nil
	case <-ctx.Done():
		// 上下文取消，清理通道
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, ctx.Err()
	case <-time.After(t.requestTimeout):
		// Go 语言级别的超时控制
		t.respChansMux.Lock()
		delete(t.respChans, idStr)
		t.respChansMux.Unlock()
		close(respChan)
		return nil, fmt.Errorf("request timeout")
	}
}

func (t *WebsocketTransport) SendNotification(ctx context.Context, notification mcp.JSONRPCNotification) error {
	// 检查连接状态
	t.closedMux.RLock()
	if t.closed {
		t.closedMux.RUnlock()
		return fmt.Errorf("connection is closed")
	}
	t.closedMux.RUnlock()

	// 发送通知消息（使用互斥锁保护写入操作）
	t.writeMux.Lock()
	err := t.conn.WriteJSON(notification)
	t.writeMux.Unlock()
	return err
}

func (t *WebsocketTransport) SetNotificationHandler(handler func(notification mcp.JSONRPCNotification)) {
	t.notifyHandler = handler
}

// SetOnCloseHandler 设置连接关闭回调
func (t *WebsocketTransport) SetOnCloseHandler(handler func(reason string)) {
	t.onCloseHandler = handler
}

func (t *WebsocketTransport) Close() error {
	// 标记连接已关闭
	t.closedMux.Lock()
	t.closed = true
	t.closedMux.Unlock()

	// 通知client层连接即将关闭
	if t.onCloseHandler != nil {
		t.onCloseHandler("manual_close")
	}

	// 取消上下文
	t.cancel()

	// 等待读取协程结束
	select {
	case <-t.readDone:
	case <-time.After(t.closeTimeout):
		log.Warnf("Timeout waiting for read goroutine to finish")
	}

	// 清理所有响应通道
	t.respChansMux.Lock()
	for idStr, respChan := range t.respChans {
		close(respChan)
		delete(t.respChans, idStr)
	}
	t.respChansMux.Unlock()

	// 关闭 WebSocket 连接
	return t.conn.Close()
}

func (t *WebsocketTransport) GetSessionId() string {
	return t.conn.RemoteAddr().String()
}

// IsClosed 检查连接是否已关闭
func (t *WebsocketTransport) IsClosed() bool {
	t.closedMux.RLock()
	defer t.closedMux.RUnlock()
	return t.closed
}

// GetActiveRequests 获取当前活跃的请求数量
func (t *WebsocketTransport) GetActiveRequests() int {
	t.respChansMux.RLock()
	defer t.respChansMux.RUnlock()
	return len(t.respChans)
}

// SetRequestTimeout 设置请求超时时间
func (t *WebsocketTransport) SetRequestTimeout(timeout time.Duration) {
	t.requestTimeout = timeout
}

// SetCloseTimeout 设置关闭超时时间
func (t *WebsocketTransport) SetCloseTimeout(timeout time.Duration) {
	t.closeTimeout = timeout
}

// GetRequestTimeout 获取当前请求超时时间
func (t *WebsocketTransport) GetRequestTimeout() time.Duration {
	return t.requestTimeout
}

// GetCloseTimeout 获取当前关闭超时时间
func (t *WebsocketTransport) GetCloseTimeout() time.Duration {
	return t.closeTimeout
}
