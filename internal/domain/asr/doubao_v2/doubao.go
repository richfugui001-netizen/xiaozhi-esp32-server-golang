package doubao_v2

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"xiaozhi-esp32-server-golang/internal/common/logger"
	"xiaozhi-esp32-server-golang/internal/common/utils"
	"xiaozhi-esp32-server-golang/internal/domain/asr/types"

	"github.com/gorilla/websocket"
)

// Protocol constants
const (
	clientFullRequest   = 0x1
	clientAudioRequest  = 0x2
	serverFullResponse  = 0x9
	serverAck           = 0xB
	serverErrorResponse = 0xF
)

// Sequence types
const (
	noSequence  = 0x0
	negSequence = 0x2
)

// Serialization methods
const (
	noSerialization = 0x0
	jsonFormat      = 0x1
	gzipCompression = 0x1
)

// DoubaoV2ASR 豆包ASR实现
type DoubaoV2ASR struct {
	config      DoubaoV2Config
	conn        *websocket.Conn
	connMutex   sync.Mutex
	isStreaming bool
	reqID       string
	connectID   string
	logger      logger.Logger

	// 流式识别相关字段
	result      string
	err         error
	sendDataCnt int
}

// NewDoubaoV2ASR 创建一个新的豆包ASR实例
func NewDoubaoV2ASR(config DoubaoV2Config, logger logger.Logger) (*DoubaoV2ASR, error) {
	logger.Info("创建豆包ASR实例")
	logger.Info(fmt.Sprintf("配置: %+v", config))

	if config.AppID == "" {
		logger.Error("缺少appid配置")
		return nil, fmt.Errorf("缺少appid配置")
	}
	if config.AccessToken == "" {
		logger.Error("缺少access_token配置")
		return nil, fmt.Errorf("缺少access_token配置")
	}

	// 使用默认配置填充缺失的字段
	if config.Host == "" {
		config.Host = DefaultConfig.Host
	}
	if config.WsURL == "" {
		config.WsURL = DefaultConfig.WsURL
	}
	if config.ModelName == "" {
		config.ModelName = DefaultConfig.ModelName
	}
	if config.EndWindowSize == 0 {
		config.EndWindowSize = DefaultConfig.EndWindowSize
	}
	if config.ChunkDuration == 0 {
		config.ChunkDuration = DefaultConfig.ChunkDuration
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultConfig.Timeout
	}

	connectID := fmt.Sprintf("%d", time.Now().UnixNano())

	return &DoubaoV2ASR{
		config:    config,
		connectID: connectID,
		logger:    logger,
	}, nil
}

// StreamingRecognize 实现流式识别接口
func (d *DoubaoV2ASR) StreamingRecognize(ctx context.Context, audioStream <-chan []float32) (chan types.StreamingResult, error) {
	// 建立连接
	if err := d.connect(); err != nil {
		return nil, err
	}

	// 发送初始请求
	if err := d.sendInitialRequest(); err != nil {
		d.disconnect()
		return nil, err
	}

	// 创建结果通道
	resultChan := make(chan types.StreamingResult, 20)

	// 启动音频发送goroutine
	go d.forwardStreamAudio(ctx, audioStream, resultChan)

	// 启动结果接收goroutine
	go d.receiveStreamResults(ctx, resultChan)

	return resultChan, nil
}

// connect 建立WebSocket连接
func (d *DoubaoV2ASR) connect() error {
	d.logger.Info("开始建立WebSocket连接")
	d.logger.Info(fmt.Sprintf("连接URL: %s", d.config.WsURL))

	d.connMutex.Lock()
	defer d.connMutex.Unlock()

	if d.conn != nil {
		d.logger.Info("WebSocket连接已存在，跳过连接")
		return nil
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	headers := map[string][]string{
		"X-Api-App-Key":     {d.config.AppID},
		"X-Api-Access-Key":  {d.config.AccessToken},
		"X-Api-Resource-Id": {"volc.bigasr.sauc.duration"},
		"X-Api-Connect-Id":  {d.connectID},
	}

	// 打印掩码后的请求头信息，便于排查鉴权问题
	masked := func(token string) string {
		if len(token) <= 8 {
			return "****"
		}
		return token[:4] + "****" + token[len(token)-4:]
	}
	d.logger.Debug("WS请求头: app=%s, access=%s, connect=%s", d.config.AppID, masked(d.config.AccessToken), d.connectID)

	conn, _, err := dialer.Dial(d.config.WsURL, headers)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %v", err)
	}

	d.conn = conn
	d.isStreaming = true
	d.reqID = fmt.Sprintf("%d", time.Now().UnixNano())

	d.logger.Debug("豆包ASR连接建立成功, connectID=%s, reqID=%s", d.connectID, d.reqID)
	return nil
}

// disconnect 断开WebSocket连接
func (d *DoubaoV2ASR) disconnect() {
	d.connMutex.Lock()
	defer d.connMutex.Unlock()

	if d.conn != nil {
		d.conn.Close()
		d.conn = nil
	}
	d.isStreaming = false
}

// sendInitialRequest 发送初始请求
func (d *DoubaoV2ASR) sendInitialRequest() error {
	request := d.constructRequest()
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("序列化请求数据失败: %v", err)
	}

	// 打印未压缩的请求JSON
	d.logger.Debug("初始请求JSON: %s", string(requestBytes))

	// 压缩请求数据
	compressedRequest, err := utils.GzipCompress(requestBytes)
	if err != nil {
		return fmt.Errorf("压缩请求数据失败: %v", err)
	}

	d.logger.Debug("初始请求大小: 原始=%d bytes, 压缩后=%d bytes", len(requestBytes), len(compressedRequest))

	header := utils.GenerateHeader(clientFullRequest, noSequence, jsonFormat)

	// 构造完整请求
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(compressedRequest)))
	fullRequest := append(header, size...)
	fullRequest = append(fullRequest, compressedRequest...)

	// 发送请求
	if err := d.conn.WriteMessage(websocket.BinaryMessage, fullRequest); err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}

	// 读取响应
	_, response, err := d.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	result, err := d.parseResponse(response)
	if err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查初始响应状态
	if msg, ok := result["payload_msg"].(map[string]interface{}); ok {
		if code, ok := msg["code"].(float64); ok && int(code) != 20000000 {
			return fmt.Errorf("ASR初始化错误: %v", msg)
		}
	}

	d.logger.Debug("豆包ASR初始化成功, 首包解析: %v", result)
	return nil
}

// constructRequest 构造请求数据
func (d *DoubaoV2ASR) constructRequest() map[string]interface{} {
	return map[string]interface{}{
		"user": map[string]interface{}{
			"uid": d.reqID,
		},
		"audio": map[string]interface{}{
			"format":   "pcm",
			"rate":     16000,
			"bits":     16,
			"channel":  1,
			"language": "zh-CN",
		},
		"request": map[string]interface{}{
			"model_name":      d.config.ModelName,
			"end_window_size": d.config.EndWindowSize,
			"enable_punc":     d.config.EnablePunc,
			"enable_itn":      d.config.EnableITN,
			"enable_ddc":      d.config.EnableDDC,
			"result_type":     "single",
			"show_utterances": false,
		},
	}
}

// parseResponse 解析响应数据
func (d *DoubaoV2ASR) parseResponse(data []byte) (map[string]interface{}, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("响应数据太短")
	}

	// 解析头部
	_ = data[0] >> 4 // protocol version
	headerSize := data[0] & 0x0f
	messageType := data[1] >> 4
	flags := data[1] & 0x0f
	serializationMethod := data[2] >> 4
	compressionMethod := data[2] & 0x0f

	// 跳过头部获取payload
	payload := data[headerSize*4:]
	result := make(map[string]interface{})

	var payloadMsg []byte
	var payloadSize int32

	switch messageType {
	case serverFullResponse:
		if len(payload) < 8 {
			return nil, fmt.Errorf("serverFullResponse payload too short")
		}
		seq := binary.BigEndian.Uint32(payload[0:4])
		result["seq"] = seq
		payloadSize = int32(binary.BigEndian.Uint32(payload[4:8]))
		if len(payload) < 8+int(payloadSize) {
			return nil, fmt.Errorf("serverFullResponse payload too short")
		}
		payloadMsg = payload[8:]
	case serverAck:
		if len(payload) < 4 {
			return nil, fmt.Errorf("serverAck payload too short")
		}
		seq := binary.BigEndian.Uint32(payload[0:4])
		result["seq"] = seq
		if len(payload) >= 8 {
			payloadSize = int32(binary.BigEndian.Uint32(payload[4:8]))
			if len(payload) < 8+int(payloadSize) {
				return nil, fmt.Errorf("serverAck payload too short")
			}
			payloadMsg = payload[8:]
		} else {
			payloadSize = 0
			payloadMsg = nil
		}
	case serverErrorResponse:
		code := uint32(binary.BigEndian.Uint32(payload[:4]))
		result["code"] = code
		payloadSize = int32(binary.BigEndian.Uint32(payload[4:8]))
		payloadMsg = payload[8:]
	}

	if payloadMsg != nil {
		if compressionMethod == gzipCompression {
			reader, err := gzip.NewReader(bytes.NewReader(payloadMsg))
			if err != nil {
				return nil, fmt.Errorf("解压响应数据失败: %v", err)
			}
			defer reader.Close()

			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(reader); err != nil {
				return nil, fmt.Errorf("读取解压数据失败: %v", err)
			}
			payloadMsg = buf.Bytes()
		}

		if serializationMethod == jsonFormat {
			var jsonData map[string]interface{}
			if err := json.Unmarshal(payloadMsg, &jsonData); err != nil {
				return nil, fmt.Errorf("解析JSON响应失败: %v", err)
			}
			d.logger.Debug("parseResponse: hdr(version=%d, headerSize=%d, msgType=0x%X, flags=0x%X, serial=%d, compress=%d, payloadSize=%d)", data[0]>>4, headerSize, messageType, flags, serializationMethod, compressionMethod, payloadSize)
			d.logger.Debug("parseResponse: JSON解析成功, 数据=%v", jsonData)
			result["payload_msg"] = jsonData
		} else if serializationMethod != noSerialization {
			result["payload_msg"] = string(payloadMsg)
		}
	}

	result["payload_size"] = payloadSize
	return result, nil
}

// forwardStreamAudio 转发流式音频数据
func (d *DoubaoV2ASR) forwardStreamAudio(ctx context.Context, audioStream <-chan []float32, resultChan chan types.StreamingResult) {
	defer func() {
		// 发送结束消息
		d.sendEndMessage()
		close(resultChan)
	}()

	for {
		select {
		case <-ctx.Done():
			d.logger.Debug("forwardStreamAudio 上下文已取消")
			return
		case pcmChunk, ok := <-audioStream:
			if !ok {
				d.logger.Debug("forwardStreamAudio 音频流已关闭")
				return
			}

			// 转换PCM数据为字节
			audioBytes := utils.Float32SliceToBytes(pcmChunk)

			d.logger.Debug("forwardStreamAudio 发送音频数据, pcmChunk len: %v, audioBytes len: %v", len(pcmChunk), len(audioBytes))

			// 发送音频数据
			if err := d.sendAudioData(audioBytes, false); err != nil {
				d.logger.Error("forwardStreamAudio 发送音频数据失败: %v", err)
				return
			}
		}
	}
}

// receiveStreamResults 接收流式识别结果
func (d *DoubaoV2ASR) receiveStreamResults(ctx context.Context, resultChan chan types.StreamingResult) {
	defer func() {
		d.disconnect()
	}()

	for {
		select {
		case <-ctx.Done():
			d.logger.Debug("receiveStreamResults 上下文已取消")
			return
		default:
		}

		d.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		msgType, response, err := d.conn.ReadMessage()
		if err != nil {
			d.logger.Error("receiveStreamResults 读取识别结果失败: %v", err)
			return
		}
		d.logger.Debug("receiveStreamResults: 收到消息, type=%d, bytes=%d", msgType, len(response))

		result, err := d.parseResponse(response)
		if err != nil {
			d.logger.Error("receiveStreamResults 解析识别结果失败: %v", err)
			continue
		}

		// 处理正常响应
		if payloadMsg, ok := result["payload_msg"].(map[string]interface{}); ok {
			if resultData, hasResult := payloadMsg["result"].(map[string]interface{}); hasResult {
				text := ""
				if textData, hasText := resultData["text"].(string); hasText {
					text = textData
				}

				d.logger.Debug("流式识别: 识别成功, 文本='%s'", text)

				// 发送识别结果
				select {
				case <-ctx.Done():
					return
				case resultChan <- types.StreamingResult{
					Text:    text,
					IsFinal: false, // 流式识别中，先设为false
				}:
				}

				// 如果文本为空且超过空闲时间，发送结束信号
				if text == "" {
					select {
					case <-ctx.Done():
						return
					case resultChan <- types.StreamingResult{
						Text:    "",
						IsFinal: true,
					}:
						return
					}
				}
			} else if errorData, hasError := payloadMsg["error"]; hasError {
				d.logger.Error("ASR响应错误: %v", errorData)
				return
			}
		}
	}
}

// sendAudioData 发送音频数据
func (d *DoubaoV2ASR) sendAudioData(data []byte, isLast bool) error {
	d.logger.Debug("sendAudioData: 数据长度=%d, isLast=%t", len(data), isLast)

	if d.conn == nil {
		return fmt.Errorf("WebSocket连接不存在")
	}

	// 压缩音频数据
	compressedAudio, err := utils.GzipCompress(data)
	if err != nil {
		return fmt.Errorf("压缩音频数据失败: %v", err)
	}
	if len(data) > 0 {
		d.logger.Debug("sendAudioData: 压缩比=%.2f", float64(len(compressedAudio))/float64(len(data)))
	}

	flags := uint8(0)
	if isLast {
		flags = negSequence
	}

	header := utils.GenerateHeader(clientAudioRequest, flags, noSerialization)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(compressedAudio)))

	audioMessage := append(header, size...)
	audioMessage = append(audioMessage, compressedAudio...)

	if err := d.conn.WriteMessage(websocket.BinaryMessage, audioMessage); err != nil {
		return fmt.Errorf("发送音频数据失败: %v", err)
	}

	return nil
}

// sendEndMessage 发送结束消息
func (d *DoubaoV2ASR) sendEndMessage() {
	if d.conn == nil {
		return
	}

	endMessage := map[string]interface{}{
		"is_speaking": false,
	}

	endMessageBytes, err := json.Marshal(endMessage)
	if err != nil {
		d.logger.Error("序列化结束消息失败: %v", err)
		return
	}

	compressedEndMessage, err := utils.GzipCompress(endMessageBytes)
	if err != nil {
		d.logger.Error("压缩结束消息失败: %v", err)
		return
	}

	header := utils.GenerateHeader(clientFullRequest, noSequence, jsonFormat)
	size := make([]byte, 4)
	binary.BigEndian.PutUint32(size, uint32(len(compressedEndMessage)))

	fullEndMessage := append(header, size...)
	fullEndMessage = append(fullEndMessage, compressedEndMessage...)

	if err := d.conn.WriteMessage(websocket.BinaryMessage, fullEndMessage); err != nil {
		d.logger.Error("发送结束消息失败: %v", err)
	}
}

// readFinalResult 读取最终识别结果
func (d *DoubaoV2ASR) readFinalResult() (string, error) {
	if d.conn == nil {
		return "", fmt.Errorf("WebSocket连接不存在")
	}

	// 设置读取超时
	d.conn.SetReadDeadline(time.Now().Add(time.Duration(d.config.Timeout) * time.Second))

	// 读取结果
	var result string
	for {
		_, response, err := d.conn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("读取响应失败: %v", err)
		}

		parsedResult, err := d.parseResponse(response)
		if err != nil {
			d.logger.Error("解析响应失败: %v", err)
			continue
		}

		// 处理正常响应
		if payloadMsg, ok := parsedResult["payload_msg"].(map[string]interface{}); ok {
			if resultData, hasResult := payloadMsg["result"].(map[string]interface{}); hasResult {
				if textData, hasText := resultData["text"].(string); hasText {
					result = textData
					break
				}
			} else if errorData, hasError := payloadMsg["error"]; hasError {
				return "", fmt.Errorf("ASR错误: %v", errorData)
			}
		}
	}

	return result, nil
}

// Reset 重置ASR状态
func (d *DoubaoV2ASR) Reset() error {
	d.connMutex.Lock()
	defer d.connMutex.Unlock()

	d.isStreaming = false
	d.disconnect()

	d.reqID = ""
	d.result = ""
	d.err = nil

	d.logger.Info("ASR状态已重置")
	return nil
}
