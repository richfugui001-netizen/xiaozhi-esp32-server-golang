package websocket

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	"xiaozhi-esp32-server-golang/internal/app/server/common"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"
)

// WebSocketServer 表示 WebSocket 服务器
type WebSocketServer struct {
	// 配置升级器
	upgrader websocket.Upgrader
	// 客户端状态，使用 sync.Map 实现并发安全
	clientStates sync.Map
	// 认证管理器
	authManager *auth.AuthManager
	// 端口
	port int
	// MCP管理器
	globalMCPManager *mcp.GlobalMCPManager
}

// NewWebSocketServer 创建新的 WebSocket 服务器
func NewWebSocketServer(port int) *WebSocketServer {
	return &WebSocketServer{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源的连接
			},
		},
		authManager:      auth.A(),
		port:             port,
		globalMCPManager: mcp.GetGlobalMCPManager(),
	}
}

// Start 启动 WebSocket 服务器
func (s *WebSocketServer) Start() error {
	// 启动MCP管理器
	if err := s.globalMCPManager.Start(); err != nil {
		log.Errorf("启动全局MCP管理器失败: %v", err)
		return err
	}

	// 启动会话清理
	go s.cleanupSessions()

	// 注册路由处理器
	http.HandleFunc("/xiaozhi/v1/", s.handleWebSocket)
	http.HandleFunc("/xiaozhi/ota/", s.handleOta)
	http.HandleFunc("/xiaozhi/mcp/", s.handleMCPWebSocket)
	http.HandleFunc("/xiaozhi/api/mcp/tools/", s.handleMCPAPI)
	http.HandleFunc("/xiaozhi/api/vision", s.handleVisionAPI) //图片识别API

	listenAddr := fmt.Sprintf("0.0.0.0:%d", s.port)
	log.Infof("WebSocket 服务器启动在 ws://%s/xiaozhi/v1/", listenAddr)
	log.Infof("MCP WebSocket 端点: ws://%s/xiaozhi/mcp/{deviceId}", listenAddr)
	log.Infof("MCP API 端点: http://%s/xiaozhi/api/mcp/tools/{deviceId}", listenAddr)

	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Log().Fatalf("WebSocket 服务器启动失败: %v", err)
		return err
	}
	return nil
}

// handleGetDeviceTools 获取设备的工具列表
func (s *WebSocketServer) handleGetDeviceTools(w http.ResponseWriter, r *http.Request, deviceID string) {

}

// cleanupSessions 定期清理过期会话
func (s *WebSocketServer) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.authManager.CleanupSessions(30 * time.Minute)
	}
}

// handleWebSocket 处理 WebSocket 连接
func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 验证请求头
	deviceID := r.Header.Get("Device-Id")
	if deviceID == "" {
		log.Warn("缺少 Device-Id 请求头")
		http.Error(w, "缺少 Device-Id 请求头", http.StatusBadRequest)
		return
	}

	isAuth := viper.GetBool("auth.enable")
	if isAuth {
		token := r.Header.Get("Authorization")
		if token == "" {
			log.Warn("缺少 Authorization 请求头")
			http.Error(w, "缺少 Authorization 请求头", http.StatusUnauthorized)
			return
		}

		// 验证令牌
		if !s.authManager.ValidateToken(token) {
			log.Warnf("无效的令牌: %s", token)
			http.Error(w, "无效的令牌", http.StatusUnauthorized)
			return
		}
	}

	// 升级 HTTP 连接为 WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("WebSocket 升级失败: %v", err)
		return
	}

	chatManager, err := common.NewChatManager(
		common.WithDeviceID(deviceID),
		common.WithWebSocketConn(conn),
	)
	if err != nil {
		log.Errorf("创建chatManager失败: %v", err)
		return
	}

	clientState := chatManager.GetClientState()

	s.clientStates.Store(clientState.Conn, clientState)

	// 连接关闭时从列表中移除
	defer func() {
		chatManager.OnClose()
		s.clientStates.Delete(clientState.Conn)
	}()

	// 处理消息
	for {
		// 每次收到消息都刷新超时时间, 空闲60秒就退出
		conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			// 这里会捕获到超时、断开等异常
			log.Warnf("WebSocket连接异常断开: %v", err)
			break
		}

		// 处理文本消息
		if messageType == websocket.TextMessage {
			log.Infof("收到文本消息: %s", string(message))
			if err := chatManager.HandleTextMessage(message); err != nil {
				log.Errorf("处理文本消息失败: %v", err)
				continue
			}
		} else if messageType == websocket.BinaryMessage {
			log.Infof("收到音频数据，大小: %d 字节", len(message))
			if clientState.GetClientVoiceStop() {
				//log.Debug("客户端停止说话, 跳过音频数据")
				continue
			}
			// 同时通过音频处理器处理
			if ok := chatManager.HandleAudioMessage(message); !ok {
				log.Errorf("音频缓冲区已满: %v", err)
			}
		} else if messageType == websocket.CloseMessage {
			log.Infof("收到关闭消息")
			break
		} else if messageType == websocket.PingMessage {
			// 响应 Ping 消息
			if err := conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				log.Errorf("发送 Pong 消息失败: %v", err)
				break
			}
		}
	}
}
