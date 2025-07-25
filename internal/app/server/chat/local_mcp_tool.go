package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"xiaozhi-esp32-server-golang/internal/domain/mcp"
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/schema"
)

// InitChatLocalMCPTools 初始化聊天相关的本地MCP工具
func InitChatLocalMCPTools() {
	manager := mcp.GetLocalMCPManager()

	log.Info("初始化聊天相关的本地MCP工具...")

	// 注册当前时间和日期工具
	err := manager.RegisterToolFunc(
		"get_current_datetime",
		"获取当前时间和日期信息",
		getCurrentDateTimeHandler,
		&schema.ParamsOneOf{
			// 可以接受一个可选的timezone参数
		},
	)
	if err != nil {
		log.Errorf("注册当前时间日期工具失败: %v", err)
	} else {
		log.Info("成功注册工具: get_current_datetime")
	}

	// 注册退出工具
	err = manager.RegisterToolFunc(
		"exit_conversation",
		"当用户明确表示要结束对话、退出系统或告别时使用，用于优雅地关闭当前聊天会话",
		exitConversationHandler,
		&schema.ParamsOneOf{
			// 可以接受一个可选的reason参数
		},
	)
	if err != nil {
		log.Errorf("注册退出对话工具失败: %v", err)
	} else {
		log.Info("成功注册工具: exit_conversation")
	}

	// 注册清空历史对话工具
	err = manager.RegisterToolFunc(
		"clear_conversation_history",
		"当用户要求清空、清除或重置历史对话记录时使用，用于清空当前会话的所有历史对话内容",
		clearConversationHistoryHandler,
		&schema.ParamsOneOf{
			// 可以接受一个可选的reason参数
		},
	)
	if err != nil {
		log.Errorf("注册清空历史对话工具失败: %v", err)
	} else {
		log.Info("成功注册工具: clear_conversation_history")
	}

	// 注册播放音乐工具
	err = manager.RegisterToolFunc(
		"play_music",
		"播放指定名称的音乐。参数格式: {\"name\": \"音乐名称\"}",
		playMusicHandler,
		&schema.ParamsOneOf{
			// 只接受音乐名称参数
		},
	)
	if err != nil {
		log.Errorf("注册播放音乐工具失败: %v", err)
	} else {
		log.Info("成功注册工具: play_music")
	}

	log.Info("聊天相关的本地MCP工具初始化完成")
}

// playMusicHandler 播放音乐的处理函数
func playMusicHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("执行播放音乐工具")

	// 解析参数
	var params map[string]interface{}
	musicName := "" // 音乐名称

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
			response := NewErrorResponse("play_music", "参数解析失败", "PARSE_ERROR", "请检查参数格式是否正确")
			return response.ToJSON()
		}
		if name, ok := params["name"].(string); ok && name != "" {
			musicName = name
		}
	}

	if musicName == "" {
		response := NewErrorResponse("play_music", "缺少必需的参数: name", "MISSING_PARAM", "请提供音乐名称参数")
		return response.ToJSON()
	}

	// 从context中获取ChatSessionOperator并调用LocalMcpPlayMusic方法
	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			log.Infof("找到ChatSessionOperator，正在调用LocalMcpPlayMusic方法播放音乐: %s", musicName)
			if err := chatSessionOperator.LocalMcpPlayMusic(ctx, musicName); err != nil {
				log.Errorf("播放音乐失败: %v", err)
				response := NewErrorResponse("play_music", fmt.Sprintf("播放音乐失败: %v", err), "PLAYBACK_ERROR", "请检查音乐名称或网络连接")
				return response.ToJSON()
			} else {
				// 成功播放 - 动作类响应，终止后续处理
				response := NewAudioResponse("play_music", "play_music", fmt.Sprintf("开始播放音乐: %s", musicName), "playing", true)
				response.UserState = "listening_music"
				response.Instruction = "音乐已开始播放，请保持安静，不要生成额外的文本回复"
				response.Metadata = map[string]string{
					"music_name": musicName,
				}
				log.Infof("开始播放音乐: %s", musicName)
				return response.ToJSON()
			}
		} else {
			log.Warn("从context中获取的chat_session_operator不是ChatSessionOperator类型")
			response := NewErrorResponse("play_music", "无法找到有效的会话操作接口", "INTERFACE_ERROR", "系统内部错误，请重试")
			return response.ToJSON()
		}
	} else {
		log.Warn("从context中未找到chat_session_operator")
		response := NewErrorResponse("play_music", "未找到会话操作接口", "NO_INTERFACE", "系统内部错误，请重试")
		return response.ToJSON()
	}
}

// getCurrentDateTimeHandler 获取当前时间和日期的处理函数
func getCurrentDateTimeHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("执行获取当前时间日期工具")

	// 解析参数
	var params map[string]interface{}
	timezone := "Local" // 默认时区

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if tz, ok := params["timezone"].(string); ok && tz != "" {
				timezone = tz
			}
		}
	}

	now := time.Now()

	// 尝试解析指定的时区
	if timezone != "Local" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			now = now.In(loc)
		} else {
			log.Warnf("无法加载时区 %s，使用本地时区", timezone)
		}
	}

	// 构造返回数据
	data := map[string]interface{}{
		"datetime": map[string]interface{}{
			"formatted":     now.Format("2006-01-02 15:04:05"),
			"iso8601":       now.Format(time.RFC3339),
			"chinese":       formatChineseDateTime(now),
			"unix":          now.Unix(),
			"year":          now.Year(),
			"month":         int(now.Month()),
			"day":           now.Day(),
			"hour":          now.Hour(),
			"minute":        now.Minute(),
			"second":        now.Second(),
			"weekday":       now.Weekday().String(),
			"weekday_zh":    getWeekdayChinese(now.Weekday()),
			"week_number":   getWeekNumber(now),
			"timezone":      timezone,
			"timezone_name": now.Location().String(),
		},
	}

	// 创建内容类响应
	response := NewContentResponse("get_current_datetime", data, fmt.Sprintf("当前时间：%s", formatChineseDateTime(now)))
	response.Format = "datetime"
	response.DisplayHint = "可用于显示当前日期时间信息"

	log.Infof("获取当前时间日期成功: %s", now.Format("2006-01-02 15:04:05"))
	return response.ToJSON()
}

// exitConversationHandler 退出对话的处理函数
func exitConversationHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("执行退出对话工具")

	// 解析参数
	var params map[string]interface{}
	reason := "用户主动退出" // 默认原因

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if r, ok := params["reason"].(string); ok && r != "" {
				reason = r
			}
		}
	}

	// 创建动作类响应 - 终止性操作
	response := NewActionResponse("exit_conversation", "exit_conversation", "对话即将结束，感谢您的使用！", "exiting", true)
	response.UserState = "conversation_ended"
	response.Instruction = "对话已结束，请不要生成额外的文本回复"
	response.Metadata = map[string]string{
		"reason":           reason,
		"exit_code":        "0",
		"farewell_chinese": "再见！期待下次与您交流。",
		"farewell_english": "Goodbye! Looking forward to our next conversation.",
	}

	log.Infof("退出对话处理完成，原因: %s", reason)

	// 从context中获取ChatSessionOperator并调用Close方法
	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			log.Info("找到ChatSessionOperator，正在调用Close方法关闭会话")
			defer chatSessionOperator.LocalMcpCloseChat()
		} else {
			log.Warn("从context中获取的chat_session_operator不是ChatSessionOperator类型")
		}
	} else {
		log.Warn("从context中未找到chat_session_operator")
	}

	return response.ToJSON()
}

// clearConversationHistoryHandler 清空历史对话的处理函数
func clearConversationHistoryHandler(ctx context.Context, argumentsInJSON string) (string, error) {
	log.Info("执行清空历史对话工具")

	// 解析参数
	var params map[string]interface{}
	reason := "用户主动清空历史" // 默认原因

	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &params); err == nil {
			if r, ok := params["reason"].(string); ok && r != "" {
				reason = r
			}
		}
	}

	// 从context中获取ChatSessionOperator并调用LocalMcpClearHistory方法
	if chatSessionOperatorValue := ctx.Value("chat_session_operator"); chatSessionOperatorValue != nil {
		if chatSessionOperator, ok := chatSessionOperatorValue.(ChatSessionOperator); ok {
			log.Info("找到ChatSessionOperator，正在调用LocalMcpClearHistory方法清空历史")
			if err := chatSessionOperator.LocalMcpClearHistory(); err != nil {
				log.Errorf("清空历史对话失败: %v", err)
				return NewErrorResponse("clear_conversation_history",
					fmt.Sprintf("清空历史失败: %v", err),
					"CLEAR_FAILED",
					"请重试清空操作").ToJSON()
			} else {
				// 成功清空 - 动作类响应，但不终止对话
				response := NewActionResponse("clear_conversation_history", "clear_history", "历史对话已成功清空，您可以开始全新的对话。", "completed", false)
				response.Metadata = map[string]string{
					"reason": reason,
					"status": "cleared",
				}
				log.Info("历史对话清空成功")
				return response.ToJSON()
			}
		} else {
			log.Warn("从context中获取的chat_session_operator不是ChatSessionOperator类型")
			return NewErrorResponse("clear_conversation_history",
				"无法找到有效的会话操作接口",
				"INTERFACE_ERROR",
				"系统内部错误，请重试").ToJSON()
		}
	} else {
		log.Warn("从context中未找到chat_session_operator")
		return NewErrorResponse("clear_conversation_history",
			"未找到会话操作接口",
			"NO_INTERFACE",
			"系统内部错误，请重试").ToJSON()
	}
}

// getWeekNumber 获取周数
func getWeekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

// formatChineseDateTime 格式化中文日期时间
func formatChineseDateTime(t time.Time) string {
	weekdays := map[time.Weekday]string{
		time.Sunday:    "星期日",
		time.Monday:    "星期一",
		time.Tuesday:   "星期二",
		time.Wednesday: "星期三",
		time.Thursday:  "星期四",
		time.Friday:    "星期五",
		time.Saturday:  "星期六",
	}

	return fmt.Sprintf("%d年%d月%d日 %s %02d:%02d:%02d",
		t.Year(), int(t.Month()), t.Day(),
		weekdays[t.Weekday()],
		t.Hour(), t.Minute(), t.Second(),
	)
}

// getWeekdayChinese 获取中文星期几
func getWeekdayChinese(weekday time.Weekday) string {
	weekdays := map[time.Weekday]string{
		time.Sunday:    "星期日",
		time.Monday:    "星期一",
		time.Tuesday:   "星期二",
		time.Wednesday: "星期三",
		time.Thursday:  "星期四",
		time.Friday:    "星期五",
		time.Saturday:  "星期六",
	}
	return weekdays[weekday]
}

// RegisterChatMCPTools 公共函数，供外部调用注册聊天MCP工具
func RegisterChatMCPTools() {
	InitChatLocalMCPTools()
}

// GetRegisteredChatTools 获取已注册的聊天工具列表
func GetRegisteredChatTools() []string {
	return []string{
		"get_current_datetime",
		"exit_conversation",
		"clear_conversation_history",
		"play_music",
	}
}
