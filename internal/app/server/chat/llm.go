package chat

import (
	"bytes"
	"context"
	"fmt"
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
)

type LLMResponseChannelItem struct {
	ctx                 context.Context
	requestEinoMessages []*schema.Message
	responseChan        chan llm_common.LLMResponseStruct
	onStartFunc         func()
	onEndFunc           func(err error)
}

type LLMManager struct {
	clientState     *ClientState
	serverTransport *ServerTransport
	ttsManager      *TTSManager

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

	var onStartFunc func()
	var onEndFunc func(err error)

	if needSendTtsCmd {
		onStartFunc = func() {
			l.serverTransport.SendTtsStart()
		}
		onEndFunc = func(err error) {
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

func (l *LLMManager) HandleLLMResponseChannelSync(ctx context.Context, requestEinoMessages []*schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct) (bool, error) {
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
		defer l.serverTransport.SendTtsStop()
	}

	return l.handleLLMResponse(ctx, requestEinoMessages, llmResponseChannel)
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

	var hasTextResponse bool
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
					hasTextResponse = true
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
	var shouldStopLLMProcessing bool

	// 从 context 中获取 chat_session_operator（如果存在）
	// 如果不存在，说明没有需要 ChatSession 操作的工具，可以正常执行
	var toolCtx context.Context = ctx
	if chatSessionOperator, ok := ctx.Value("chat_session_operator").(ChatSessionOperator); ok {
		// 在 context 中传递 chat_session_operator，供 local mcp tool 使用
		toolCtx = context.WithValue(ctx, "chat_session_operator", chatSessionOperator)
	}

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
		log.Infof("工具调用结果: %s, 耗时: %dms", fcResult, costTs)

		var result string = fcResult
		// 首先尝试解析新的结构化响应
		if response, err := ParseMCPResponse(fcResult); err == nil {
			log.Debugf("解析到结构化MCP响应，类型: %s", response.GetType())
			if response.IsTerminal() {
				log.Infof("工具响应类型: %s, 成功: %t, 终止: %t", response.GetType(), response.GetSuccess(), response.IsTerminal())
				return invokeToolSuccess, nil
			}
			result = response.GetContent()
		}

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

	// 如果工具调用成功且没有被标记为停止处理，则继续LLM调用
	if invokeToolSuccess && !shouldStopLLMProcessing {
		requestEinoMessages = append(requestEinoMessages, msgList...)
		//不需要带tool进行调用
		l.DoLLmRequest(ctx, requestEinoMessages, nil, true)
	} else if shouldStopLLMProcessing {
		log.Infof("根据工具返回标志，跳过后续LLM处理")
	}

	return invokeToolSuccess, nil
}

func (l *LLMManager) DoLLmRequest(ctx context.Context, requestEinoMessages []*schema.Message, einoTools []*schema.ToolInfo, isSync bool) error {
	log.Debugf("发送带工具的 LLM 请求, seesionID: %s, requestEinoMessages: %+v", l.clientState.SessionID, requestEinoMessages)
	clientState := l.clientState

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
		_, err := l.HandleLLMResponseChannelSync(ctx, requestEinoMessages, responseSentences)
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
