package chat

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/domain/play_music"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
	mcp_go "github.com/mark3labs/mcp-go/mcp"
)

type LLMResponseChannelItem struct {
	ctx                 context.Context
	requestEinoMessages []*schema.Message
	responseChan        chan llm_common.LLMResponseStruct
	onStartFunc         func(args ...any)
	onEndFunc           func(err error, args ...any)
}

type LLMManager struct {
	clientState     *ClientState
	serverTransport *ServerTransport
	ttsManager      *TTSManager

	einoTools []*schema.ToolInfo

	llmResponseQueue *util.Queue[LLMResponseChannelItem]
}

func NewLLMManager(clientState *ClientState, serverTransport *ServerTransport, ttsManager *TTSManager) *LLMManager {
	return &LLMManager{
		clientState:      clientState,
		serverTransport:  serverTransport,
		ttsManager:       ttsManager,
		llmResponseQueue: util.NewQueue[LLMResponseChannelItem](10),
	}
}

func (l *LLMManager) Start(ctx context.Context) {
	l.processLLMResponseQueue(ctx)
}

func (l *LLMManager) processLLMResponseQueue(ctx context.Context) {
	for {
		item, err := l.llmResponseQueue.Pop(ctx, 0) // 阻塞式
		if err != nil {
			if err == util.ErrQueueCtxDone {
				return
			}
			// 其他错误
			continue
		}

		log.Debugf("processLLMResponseQueue item: %+v", item)
		if item.onStartFunc != nil {
			item.onStartFunc()
		}
		_, err = l.handleLLMResponse(item.ctx, item.requestEinoMessages, item.responseChan)
		if item.onEndFunc != nil {
			item.onEndFunc(err)
		}
	}
}

func (l *LLMManager) ClearLLMResponseQueue() {
	l.llmResponseQueue.Clear()
}

func (l *LLMManager) AddTextToTTSQueue(text string) error {
	log.Debugf("AddTextToTTSQueue text: %s", text)
	msg := []*schema.Message{}
	llmResponseChan := make(chan llm_common.LLMResponseStruct, 10)
	llmResponseChan <- llm_common.LLMResponseStruct{
		IsStart: true,
		IsEnd:   true,
		Text:    text,
	}
	close(llmResponseChan)
	l.HandleLLMResponseChannelAsync(l.clientState.GetSessionCtx(), msg, llmResponseChan)

	return nil
}

func (l *LLMManager) HandleLLMResponseChannelAsync(ctx context.Context, requestEinoMessages []*schema.Message, responseChan chan llm_common.LLMResponseStruct) error {
	needSendTtsCmd := true
	val := ctx.Value("nest")
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if nest, ok := val.(int); ok {
		if nest > 1 {
			needSendTtsCmd = false
		}
	}

	var onStartFunc func(...any)
	var onEndFunc func(err error, args ...any)

	if needSendTtsCmd {
		onStartFunc = func(...any) {
			l.serverTransport.SendTtsStart()
		}
		onEndFunc = func(err error, args ...any) {
			l.serverTransport.SendTtsStop()
		}
	}

	item := LLMResponseChannelItem{
		ctx:                 ctx,
		requestEinoMessages: requestEinoMessages,
		responseChan:        responseChan,
		onStartFunc:         onStartFunc,
		onEndFunc:           onEndFunc,
	}
	err := l.llmResponseQueue.Push(item)
	if err != nil {
		log.Warnf("llmResponseQueue 已满或已关闭, 丢弃消息")
		return fmt.Errorf("llmResponseQueue 已满或已关闭, 丢弃消息")
	}
	return nil
}

func (l *LLMManager) HandleLLMResponseChannelSync(ctx context.Context, requestEinoMessages []*schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct, einoTools []*schema.ToolInfo) (bool, error) {
	needSendTtsCmd := true
	val := ctx.Value("nest")
	log.Debugf("AddLLMResponseChannel nest: %+v", val)
	if nest, ok := val.(int); ok {
		if nest > 1 {
			needSendTtsCmd = false
		}
	}
	if needSendTtsCmd {
		l.serverTransport.SendTtsStart()
	}
	ok, err := l.handleLLMResponse(ctx, requestEinoMessages, llmResponseChannel)
	if needSendTtsCmd {
		l.serverTransport.SendTtsStop()
	}

	return ok, err
}

// HandleLLMResponse 处理LLM响应
func (l *LLMManager) handleLLMResponse(ctx context.Context, requestEinoMessages []*schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct) (bool, error) {
	log.Debugf("handleLLMResponse start")
	defer log.Debugf("handleLLMResponse end")
	select {
	case <-ctx.Done():
		log.Debugf("handleLLMResponse ctx done, return")
		return false, nil
	default:
	}

	state := l.clientState
	var toolCalls []schema.ToolCall
	var fullText bytes.Buffer

	//var hasTextResponse bool
	for {
		select {
		case <-ctx.Done():
			// 上下文已取消，优先处理取消逻辑
			log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
			//sendTtsStartEndFunc(false)
			return false, nil
		default:
			// 非阻塞检查，如果ctx没有Done，继续处理LLM响应
			select {
			case llmResponse, ok := <-llmResponseChannel:
				if !ok {
					// 通道已关闭，退出协程
					log.Infof("LLM 响应通道已关闭，退出协程")
					return true, nil
				}

				log.Debugf("LLM 响应: %+v", llmResponse)

				if len(llmResponse.ToolCalls) > 0 {
					log.Debugf("获取到工具: %+v", llmResponse.ToolCalls)
					toolCalls = append(toolCalls, llmResponse.ToolCalls...)
				}

				if llmResponse.Text != "" {
					//hasTextResponse = true
					// 处理文本内容响应
					if err := l.ttsManager.handleTextResponse(ctx, llmResponse, true); err != nil {
						return true, err
					}
					fullText.WriteString(llmResponse.Text)
				}

				if llmResponse.IsEnd {
					//写到redis中
					if len(requestEinoMessages) > 0 {
						llm_memory.Get().AddMessage(ctx, state.DeviceID, schema.User, requestEinoMessages[len(requestEinoMessages)-1].Content)
					}
					strFullText := fullText.String()
					if strFullText != "" {
						llm_memory.Get().AddMessage(ctx, state.DeviceID, schema.Assistant, strFullText)
					}
					if len(toolCalls) > 0 {
						/*
							if !hasTextResponse {
								//有工具调用 && 没有文本响应，发送"查询中", 异步tts
								l.ttsManager.handleTextResponse(ctx, llm_common.LLMResponseStruct{
									Text: "查询中, 请稍候",
								}, false)
							}*/

						lctx := context.WithValue(ctx, "nest", 2)
						invokeToolSuccess, err := l.handleToolCallResponse(lctx, requestEinoMessages, toolCalls)
						if err != nil {
							log.Errorf("处理工具调用响应失败: %v", err)
							return true, fmt.Errorf("处理工具调用响应失败: %v", err)
						}
						if !invokeToolSuccess {
							//工具调用失败
							if err := l.ttsManager.handleTextResponse(ctx, llmResponse, false); err != nil {
								return true, err
							}
							fullText.WriteString(llmResponse.Text)
							//sendTtsStartEndFunc(false)
						}
					} else {
						//sendTtsStartEndFunc(false)
					}

					return ok, nil
				}
			case <-ctx.Done():
				// 上下文已取消，退出协程
				log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
				//sendTtsStartEndFunc(false)
				return false, nil
			}
		}
	}
}

// handleToolCallResponse 处理工具调用响应
func (l *LLMManager) handleToolCallResponse(ctx context.Context, requestEinoMessages []*schema.Message, tools []schema.ToolCall) (bool, error) {
	if len(tools) == 0 {
		return false, nil
	}

	state := l.clientState

	log.Infof("处理 %d 个工具调用", len(tools))

	var invokeToolSuccess bool
	msgList := make([]*schema.Message, 0)

	// 从 context 中获取 chat_session_operator（如果存在）
	// 如果不存在，说明没有需要 ChatSession 操作的工具，可以正常执行
	var toolCtx context.Context = ctx
	if chatSessionOperator, ok := ctx.Value("chat_session_operator").(ChatSessionOperator); ok {
		// 在 context 中传递 chat_session_operator，供 local mcp tool 使用
		toolCtx = context.WithValue(ctx, "chat_session_operator", chatSessionOperator)
	}

	var shouldStopLLMProcessing bool

	for _, toolCall := range tools {
		toolName := toolCall.Function.Name
		tool, ok := mcp.GetToolByName(state.DeviceID, toolName)
		if !ok || tool == nil {
			log.Errorf("未找到工具: %s", toolName)
			continue
		}
		log.Infof("进行工具调用请求: %s, 参数: %+v", toolName, toolCall.Function.Arguments)
		startTs := time.Now().UnixMilli()
		fcResult, err := tool.InvokableRun(toolCtx, toolCall.Function.Arguments)
		if err != nil {
			log.Errorf("工具调用失败: %v", err)
			continue
		}
		costTs := time.Now().UnixMilli() - startTs
		invokeToolSuccess = true
		if len(fcResult) > 2048 {
			log.Infof("工具调用结果 len: %d, 耗时: %dms", len(fcResult), costTs)
		} else {
			log.Infof("工具调用结果 %s, 耗时: %dms", fcResult, costTs)
		}

		var result string = fcResult
		var contentList []mcp_go.Content
		if mcpResp, ok := l.handleLocalToolResult(fcResult); ok {
			/*if mcpResp.IsTerminal() {
				log.Infof("工具调用结果: %s, 终止: %t", fcResult, mcpResp.IsTerminal())
				return invokeToolSuccess, nil
			}*/
			contentList = mcpResp.GetContent()
		} else if toolCallResult, ok := l.handleToolResult(fcResult); ok {
			if toolCallResult.IsError {
				log.Errorf("工具调用失败: %s, 错误: %s", fcResult, toolCallResult.IsError)
			}
			contentList = toolCallResult.Content
		}
		if len(contentList) > 0 {
			var mcpContent string
			//如果有audio数据, 则进行播放
			for _, content := range contentList {
				if audioContent, ok := content.(mcp_go.AudioContent); ok {
					log.Debugf("调用工具 %s 返回音频资源长度: %d", toolName, len(audioContent.Data))
					//播放音频资源,此时mcpContent是
					err := l.handleAudioContent(ctx, mcpContent, audioContent)
					if err != nil {
						log.Errorf("mcp播放音频资源失败: %v", err)
					}
					mcpContent = ""
					result = ""
					shouldStopLLMProcessing = true
					break
				} else if textContent, ok := content.(mcp_go.TextContent); ok {
					log.Debugf("调用工具 %s 返回文本资源长度: %s", toolName, textContent.Text)
					mcpContent += textContent.Text
				}
			}
			if mcpContent != "" {
				result = mcpContent
			}
		}

		if result != "" {
			msg := []*schema.Message{
				&schema.Message{
					Role:      schema.Assistant,
					ToolCalls: []schema.ToolCall{toolCall},
				},
				&schema.Message{
					Role:       schema.Tool,
					ToolCallID: toolCall.ID,
					Content:    result,
				},
			}
			msgList = append(msgList, msg...)
		}
	}

	// 如果工具调用成功且没有被标记为停止处理，则继续LLM调用
	if invokeToolSuccess && !shouldStopLLMProcessing {
		requestEinoMessages = append(requestEinoMessages, msgList...)
		l.DoLLmRequest(ctx, requestEinoMessages, l.einoTools, true)
	}

	return invokeToolSuccess, nil
}

func (l *LLMManager) handleAudioContent(ctx context.Context, realMusicName string, audioContent mcp_go.AudioContent) error {
	rawAudioData, err := base64.StdEncoding.DecodeString(audioContent.Data)
	if err != nil {
		log.Errorf("解码音频数据失败: %v", err)
		return fmt.Errorf("解码音频数据失败: %v", err)
	}
	audioFormat := util.GetAudioFormatByMimeType(audioContent.MIMEType)
	// 使用music_player播放音乐
	audioChan, err := play_music.PlayMusicFromAudioData(ctx, rawAudioData, l.clientState.OutputAudioFormat.SampleRate, l.clientState.OutputAudioFormat.FrameDuration, audioFormat)
	if err != nil {
		log.Errorf("播放音乐失败: %v", err)
		return fmt.Errorf("播放音乐失败: %v", err)
	}

	playText := fmt.Sprintf("正在播放音乐: %s", realMusicName)
	l.serverTransport.SendSentenceStart(playText)
	defer func() {
		l.serverTransport.SendSentenceEnd(playText)
		if l.serverTransport != nil {
			l.serverTransport.SendTtsStop()
		}
		log.Infof("音乐播放完成: %s", realMusicName)
	}()

	l.ttsManager.SendTTSAudio(ctx, audioChan, true)

	return nil
}

func (l *LLMManager) handleLocalToolResult(toolResult string) (MCPResponse, bool) {
	// 首先尝试解析新的结构化响应
	var response MCPResponse
	var err error
	if response, err = ParseMCPResponse(toolResult); err != nil {
		return nil, false
	}
	return response, true
}

func (l *LLMManager) handleToolResult(toolResultStr string) (mcp_go.CallToolResult, bool) {
	var toolResult mcp_go.CallToolResult
	if err := json.Unmarshal([]byte(toolResultStr), &toolResult); err != nil {
		log.Errorf("解析工具结果失败: %v", err)
		return toolResult, false
	}

	return toolResult, true
}

func (l *LLMManager) DoLLmRequest(ctx context.Context, requestEinoMessages []*schema.Message, einoTools []*schema.ToolInfo, isSync bool) error {
	log.Debugf("发送带工具的 LLM 请求, seesionID: %s, requestEinoMessages: %+v", l.clientState.SessionID, requestEinoMessages)
	clientState := l.clientState

	l.einoTools = einoTools

	clientState.SetStatus(ClientStatusLLMStart)
	responseSentences, err := llm.HandleLLMWithContextAndTools(
		ctx,
		clientState.LLMProvider,
		requestEinoMessages,
		einoTools,
		l.clientState.SessionID,
	)
	if err != nil {
		log.Errorf("发送带工具的 LLM 请求失败, seesionID: %s, error: %v", l.clientState.SessionID, err)
		return fmt.Errorf("发送带工具的 LLM 请求失败: %v", err)
	}

	log.Debugf("DoLLmRequest goroutine开始 - SessionID: %s, context状态: %v", l.clientState.SessionID, ctx.Err())

	if isSync {
		_, err := l.HandleLLMResponseChannelSync(ctx, requestEinoMessages, responseSentences, einoTools)
		if err != nil {
			log.Errorf("处理 LLM 响应失败, seesionID: %s, error: %v", l.clientState.SessionID, err)
			return err
		}
	} else {
		err = l.HandleLLMResponseChannelAsync(ctx, requestEinoMessages, responseSentences)
		if err != nil {
			log.Errorf("处理 LLM 响应失败, seesionID: %s, error: %v", l.clientState.SessionID, err)
		}
	}

	log.Debugf("DoLLmRequest 结束 - SessionID: %s", l.clientState.SessionID)

	return nil
}
