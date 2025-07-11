package chat

import (
	"bytes"
	"context"
	"fmt"
	"time"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	"xiaozhi-esp32-server-golang/internal/domain/llm"
	llm_common "xiaozhi-esp32-server-golang/internal/domain/llm/common"
	llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"
	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
)

type LLMResponseChannelItem struct {
	requestEinoMessages []*schema.Message
	responseChan        chan llm_common.LLMResponseStruct
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
		sessionCtx := l.clientState.GetSessionCtx()
		l.HandleLLMResponse(sessionCtx, item.requestEinoMessages, item.responseChan)
	}
}

func (l *LLMManager) ClearLLMResponseQueue() {
	l.llmResponseQueue.Clear()
}

func (l *LLMManager) AddLLMResponseChannel(requestEinoMessages []*schema.Message, responseChan chan llm_common.LLMResponseStruct) error {
	err := l.llmResponseQueue.Push(LLMResponseChannelItem{requestEinoMessages: requestEinoMessages, responseChan: responseChan})
	if err != nil {
		log.Warnf("llmResponseQueue 已满或已关闭, 丢弃消息")
		return fmt.Errorf("llmResponseQueue 已满或已关闭, 丢弃消息")
	}
	return nil
}

// HandleLLMResponse 处理LLM响应
func (l *LLMManager) HandleLLMResponse(ctx context.Context, requestEinoMessages []*schema.Message, llmResponseChannel chan llm_common.LLMResponseStruct) (bool, error) {
	log.Debugf("HandleLLMResponse start")
	defer log.Debugf("HandleLLMResponse end")

	select {
	case <-ctx.Done():
		log.Debugf("HandleLLMResponse ctx done, return")
		return false, nil
	default:
	}

	state := l.clientState
	var toolCalls []schema.ToolCall
	var fullText bytes.Buffer

	sendTtsStartEndFunc := func(isStart bool) error {
		msgState := MessageStateStart
		if !isStart {
			msgState = MessageStateStop
		}

		if msgState == MessageStateStart {
			err := l.serverTransport.SendTtsStart()
			if err != nil {
				log.Errorf("发送tts start失败: %+v", err)
				return err
			}
			return nil
		} else if msgState == MessageStateStop {
			err := l.serverTransport.SendTtsStop()
			if err != nil {
				log.Errorf("发送tts stop失败: %+v", err)
				return err
			}
			return nil
		}

		return fmt.Errorf("msgState: %s", msgState)
	}

	if !state.GetTtsStart() {
		sendTtsStartEndFunc(true)
	}

	for {
		select {
		case <-ctx.Done():
			// 上下文已取消，优先处理取消逻辑
			log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
			sendTtsStartEndFunc(false)
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
					// 处理文本内容响应
					if err := l.ttsManager.handleTextResponse(ctx, llmResponse, &fullText); err != nil {
						return true, err
					}
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
						invokeToolSuccess, err := l.handleToolCallResponse(ctx, requestEinoMessages, toolCalls)
						if err != nil {
							log.Errorf("处理工具调用响应失败: %v", err)
							return true, fmt.Errorf("处理工具调用响应失败: %v", err)
						}
						if !invokeToolSuccess {
							//工具调用失败
							if err := l.ttsManager.handleTextResponse(ctx, llmResponse, &fullText); err != nil {
								return true, err
							}
							sendTtsStartEndFunc(false)
						}
					} else {
						sendTtsStartEndFunc(false)
					}

					return ok, nil
				}
			case <-ctx.Done():
				// 上下文已取消，退出协程
				log.Infof("%s 上下文已取消，停止处理LLM响应, context done, exit", state.DeviceID)
				sendTtsStartEndFunc(false)
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
	for _, toolCall := range tools {
		toolName := toolCall.Function.Name
		tool, ok := mcp.GetToolByName(state.DeviceID, toolName)
		if !ok || tool == nil {
			log.Errorf("未找到工具: %s", toolName)
			continue
		}
		log.Infof("进行工具调用请求: %s, 参数: %+v", toolName, toolCall.Function.Arguments)
		startTs := time.Now().UnixMilli()
		result, err := tool.InvokableRun(ctx, toolCall.Function.Arguments)
		if err != nil {
			log.Errorf("工具调用失败: %v", err)
			continue
		}
		costTs := time.Now().UnixMilli() - startTs
		invokeToolSuccess = true
		log.Infof("工具调用结果: %s, 耗时: %dms", result, costTs)
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

	if invokeToolSuccess {
		requestEinoMessages = append(requestEinoMessages, msgList...)
		//不需要带tool进行调用
		l.DoLLmRequest(ctx, requestEinoMessages, nil)
	}

	return invokeToolSuccess, nil
}

func (l *LLMManager) DoLLmRequest(ctx context.Context, requestEinoMessages []*schema.Message, einoTools []*schema.ToolInfo) error {
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
	err = l.AddLLMResponseChannel(requestEinoMessages, responseSentences)
	if err != nil {
		log.Errorf("处理 LLM 响应失败, seesionID: %s, error: %v", l.clientState.SessionID, err)
	}

	log.Debugf("DoLLmRequest 结束 - SessionID: %s", l.clientState.SessionID)

	return nil
}
