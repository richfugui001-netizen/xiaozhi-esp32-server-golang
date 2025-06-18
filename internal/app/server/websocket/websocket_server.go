package websocket

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"

	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	"xiaozhi-esp32-server-golang/internal/app/server/common"
	"xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	userconfig "xiaozhi-esp32-server-golang/internal/domain/user_config"
	"xiaozhi-esp32-server-golang/internal/util"
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

// handleMCPWebSocket 处理MCP WebSocket连接
func (s *WebSocketServer) handleMCPWebSocket(w http.ResponseWriter, r *http.Request) {
	// 从URL路径中提取deviceId
	// URL格式: /xiaozhi/mcp/{deviceId}
	path := strings.TrimPrefix(r.URL.Path, "/xiaozhi/mcp/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "缺少设备ID参数", http.StatusBadRequest)
		return
	}

	deviceID := strings.TrimSuffix(path, "/")
	if deviceID == "" {
		http.Error(w, "设备ID不能为空", http.StatusBadRequest)
		return
	}

	log.Infof("收到MCP服务器的WebSocket连接请求，设备ID: %s", deviceID)

	// 验证认证（如果启用）
	isAuth := viper.GetBool("auth.enable")
	if isAuth {
		token := r.Header.Get("Authorization")
		if token == "" {
			log.Warn("缺少 Authorization 请求头")
			http.Error(w, "缺少 Authorization 请求头", http.StatusUnauthorized)
			return
		}

		if !s.authManager.ValidateToken(token) {
			log.Warnf("无效的令牌: %s", token)
			http.Error(w, "无效的令牌", http.StatusUnauthorized)
			return
		}
	}

	// 升级WebSocket连接
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorf("升级WebSocket连接失败: %v", err)
		return
	}

	mcpClientSession := mcp.GetDeviceMcpClient(deviceID)
	if mcpClientSession == nil {
		mcpClientSession = mcp.NewDeviceMCPSession(deviceID)
		mcp.AddDeviceMcpClient(deviceID, mcpClientSession)
	}

	// 创建MCP客户端
	mcpClient := mcp.NewWsEndPointMcpClient(mcpClientSession.Ctx, deviceID, conn)
	if mcpClient == nil {
		log.Errorf("创建MCP客户端失败")
		conn.Close()
		return
	}
	mcpClientSession.SetWsEndPointMcp(mcpClient)

	// 监听客户端断开连接
	go func() {
		<-mcpClientSession.Ctx.Done()
		mcp.RemoveDeviceMcpClient(deviceID)
		log.Infof("设备 %s 的MCP连接已断开", deviceID)
	}()

	log.Infof("设备 %s 的MCP连接已建立", deviceID)
}

// handleMCPAPI 处理MCP REST API请求
func (s *WebSocketServer) handleMCPAPI(w http.ResponseWriter, r *http.Request) {
	// 从URL路径中提取deviceId
	// URL格式: /xiaozhi/api/mcp/tools/{deviceId}
	path := strings.TrimPrefix(r.URL.Path, "/xiaozhi/api/mcp/tools/")
	if path == "" || path == r.URL.Path {
		http.Error(w, "缺少设备ID参数", http.StatusBadRequest)
		return
	}

	deviceID := strings.TrimSuffix(path, "/")
	if deviceID == "" {
		http.Error(w, "设备ID不能为空", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		s.handleGetDeviceTools(w, r, deviceID)
	default:
		http.Error(w, "不支持的HTTP方法", http.StatusMethodNotAllowed)
	}
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

func (s *WebSocketServer) SendMsg(conn *client.Conn, msg interface{}) error {
	log.Debugf("发送消息: %+v", msg)
	return conn.WriteJSON(msg)
}

func (s *WebSocketServer) SendBinaryMsg(conn *client.Conn, audio []byte) error {
	return conn.WriteMessage(websocket.BinaryMessage, audio)
}

// 获取设备配置
func (s *WebSocketServer) getUserConfig(deviceID string) (*userconfig.UConfig, error) {
	userConfig, err := userconfig.U().GetUserConfig(context.Background(), deviceID)
	if err != nil {
		return nil, fmt.Errorf("获取用户配置失败: %v", err)
	}
	return &userConfig, nil
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
	// 设置初始超时时间，比如60秒
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	// 初始化客户端状态
	clientState, err := client.GenWebsocketClientState(deviceID, conn)
	if err != nil {
		log.Errorf("初始化客户端状态失败: %v", err)
		return
	}

	s.clientStates.Store(clientState.Conn, clientState)

	// 连接关闭时从列表中移除
	defer func() {
		log.Infof("设备 %s 断开连接", deviceID)
		// 关闭done通道通知所有goroutine退出
		clientState.Cancel()
		clientState.Destroy()
		clientState.Conn.Close()
		s.clientStates.Delete(conn)
	}()

	// 处理消息
	for {
		// 每次收到消息都刷新超时时间, 空闲60秒就退出
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			// 这里会捕获到超时、断开等异常
			log.Warnf("WebSocket连接异常断开: %v", err)
			break
		}

		// 处理文本消息
		if messageType == websocket.TextMessage {
			log.Infof("收到文本消息: %s", string(message))
			if err := common.HandleTextMessage(clientState, message); err != nil {
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
			if ok := common.RecvAudio(clientState, message); !ok {
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

func (s *WebSocketServer) handleOta(w http.ResponseWriter, r *http.Request) {
	//获取客户端ip
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	userName := struct {
		Ip string `json:"ip"`
	}{
		Ip: ip,
	}
	userNameJson, err := json.Marshal(userName)
	if err != nil {
		log.Errorf("用户名序列化失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
	base64UserName := base64.StdEncoding.EncodeToString(userNameJson)

	//从header头部获取Device-Id和Client-Id
	deviceId := r.Header.Get("Device-Id")
	clientId := r.Header.Get("Client-Id")

	if deviceId == "" || clientId == "" {
		log.Errorf("缺少Device-Id或Client-Id")
		http.Error(w, "缺少Device-Id或Client-Id", http.StatusBadRequest)
		return
	}

	deviceId = strings.ReplaceAll(deviceId, ":", "_")

	mqttClientId := fmt.Sprintf("GID_test@@@%s@@@%s", deviceId, clientId)
	pwd := util.Sha256Digest([]byte(mqttClientId))

	//根据ip选择不同的配置
	clientIp := r.Header.Get("X-Real-IP")
	if clientIp == "" {
		clientIp = r.Header.Get("X-Forwarded-For")
	}
	if clientIp == "" {
		clientIp = r.RemoteAddr
	}

	otaConfigPrefix := "ota.external."
	//如果ip是192.168开头的，则选择test配置
	if strings.HasPrefix(clientIp, "192.168") || strings.HasPrefix(clientIp, "127.0.0.1") {
		otaConfigPrefix = "ota.test."
	} else {
		otaConfigPrefix = "ota.external."
	}

	//密码
	respData := &OtaResponse{
		Websocket: WebsocketInfo{
			Url:   viper.GetString(otaConfigPrefix + "websocket.url"),
			Token: viper.GetString(otaConfigPrefix + "websocket.token"),
		},
		Mqtt: MqttInfo{
			Endpoint:       viper.GetString(otaConfigPrefix + "mqtt.endpoint"),
			ClientId:       mqttClientId,
			Username:       base64UserName,
			Password:       pwd,
			PublishTopic:   client.DeviceMockPubTopicPrefix,
			SubscribeTopic: client.DeviceMockSubTopicPrefix,
		},
		ServerTime: ServerTimeInfo{
			Timestamp:      time.Now().UnixMilli(),
			TimezoneOffset: 480,
		},

		Firmware: FirmwareInfo{
			Version: "0.9.9",
			Url:     "",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(respData); err != nil {
		log.Errorf("OTA响应序列化失败: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
		return
	}
	return
}

// handleVisionAPI 处理图片识别API
func (s *WebSocketServer) handleVisionAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持POST请求", http.StatusMethodNotAllowed)
		return
	}

	//从header头部获取Device-Id和Client-Id
	deviceId := r.Header.Get("Device-Id")
	clientId := r.Header.Get("Client-Id")
	_ = clientId
	if deviceId == "" {
		log.Errorf("缺少Device-Id")
		http.Error(w, "缺少Device-Id", http.StatusBadRequest)
		return
	}

	if viper.GetBool("vision.enable_auth") {

		//从header Authorization中获取Bearer token
		authToken := r.Header.Get("Authorization")
		if authToken == "" {
			log.Errorf("缺少Authorization")
			http.Error(w, "缺少Authorization", http.StatusBadRequest)
			return
		}
		authToken = strings.TrimPrefix(authToken, "Bearer ")

		err := common.VisvionAuth(authToken)
		if err != nil {
			log.Errorf("图片识别认证失败: %v", err)
			http.Error(w, "图片识别认证失败", http.StatusUnauthorized)
			return
		}
	}

	// 解析 multipart 表单，最大 10MB
	question := r.FormValue("question")
	if question == "" {
		http.Error(w, "缺少question参数", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "缺少file参数或文件读取失败", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "文件读取失败", http.StatusInternalServerError)
		return
	}

	file.Close()

	result, err := common.HandleVllm(deviceId, fileBytes, question)
	if err != nil {
		log.Errorf("图片识别失败: %v", err)
		http.Error(w, "图片识别失败", http.StatusInternalServerError)
		return
	}

	// TODO: 调用llm进行图片识别，输出识别内容
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(result))
}
