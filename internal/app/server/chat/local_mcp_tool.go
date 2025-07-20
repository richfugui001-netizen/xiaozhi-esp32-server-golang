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
		"结束当前对话会话",
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

	log.Info("聊天相关的本地MCP工具初始化完成")
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

	// 构造返回结果
	result := map[string]interface{}{
		"success":     true,
		"timestamp":   now.Unix(),
		"datetime":    now.Format("2006-01-02 15:04:05"),
		"date":        now.Format("2006-01-02"),
		"time":        now.Format("15:04:05"),
		"timezone":    now.Location().String(),
		"year":        now.Year(),
		"month":       int(now.Month()),
		"day":         now.Day(),
		"hour":        now.Hour(),
		"minute":      now.Minute(),
		"second":      now.Second(),
		"weekday":     now.Weekday().String(),
		"yearday":     now.YearDay(),
		"week_number": getWeekNumber(now),
		"formatted": map[string]string{
			"rfc3339": now.Format(time.RFC3339),
			"rfc822":  now.Format(time.RFC822),
			"kitchen": now.Format(time.Kitchen),
			"stamp":   now.Format(time.Stamp),
			"chinese": formatChineseDateTime(now),
		},
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return `{"success": false, "error": "序列化结果失败"}`, err
	}

	log.Infof("获取当前时间日期成功: %s", now.Format("2006-01-02 15:04:05"))
	return string(resultBytes), nil
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

	// 构造返回结果
	result := map[string]interface{}{
		"success":   true,
		"action":    "exit_conversation",
		"reason":    reason,
		"timestamp": time.Now().Unix(),
		"message":   "对话即将结束，感谢您的使用！",
		"exit_code": 0,
		"farewell": map[string]string{
			"chinese": "再见！期待下次与您交流。",
			"english": "Goodbye! Looking forward to our next conversation.",
		},
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return `{"success": false, "error": "序列化结果失败"}`, err
	}

	log.Infof("退出对话处理完成，原因: %s", reason)

	// 从context中获取ChatSession并调用Close方法
	if chatSessionValue := ctx.Value("chat_session"); chatSessionValue != nil {
		if chatSession, ok := chatSessionValue.(*ChatSession); ok {
			log.Info("找到ChatSession，正在调用Close方法关闭会话")
			defer chatSession.Close()
		} else {
			log.Warn("从context中获取的chat_session不是*ChatSession类型")
		}
	} else {
		log.Warn("从context中未找到chat_session")
	}

	return string(resultBytes), nil
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

// RegisterChatMCPTools 公共函数，供外部调用注册聊天MCP工具
func RegisterChatMCPTools() {
	InitChatLocalMCPTools()
}

// GetRegisteredChatTools 获取已注册的聊天工具列表
func GetRegisteredChatTools() []string {
	return []string{
		"get_current_datetime",
		"exit_conversation",
	}
}
