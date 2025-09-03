package controllers

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebSocketControllerInterface 定义WebSocket控制器的接口
type WebSocketControllerInterface interface {
	RequestMcpToolsFromClient(ctx context.Context, agentID string) ([]string, error)
}

// GetAgentMcpToolsCommon 获取智能体MCP工具列表的公共函数
// 这个函数可以被管理员和普通用户控制器共同使用
func GetAgentMcpToolsCommon(
	c *gin.Context,
	agentID string,
	webSocketController WebSocketControllerInterface,
	agentValidator func(agentID string) error, // 验证智能体权限的函数
) {
	log.Printf("GetAgentMcpToolsCommon 开始执行，agentID: %s", agentID)

	if agentID == "" {
		log.Printf("错误: agent_id参数为空")
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent_id parameter is required"})
		return
	}

	// 验证智能体权限（由调用方提供验证逻辑）
	if err := agentValidator(agentID); err != nil {
		log.Printf("智能体验证失败: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("智能体验证成功，开始检查WebSocket控制器")

	// 检查WebSocket控制器是否存在
	if webSocketController == nil {
		// 当WebSocket控制器不存在时，返回空列表而不是错误
		log.Printf("WebSocket控制器未初始化，返回空工具列表")
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	log.Printf("WebSocket控制器存在，开始请求MCP工具列表")

	// 创建上下文
	ctx := context.Background()

	// 调用RequestMcpToolsFromClient获取工具列表
	toolNames, err := webSocketController.RequestMcpToolsFromClient(ctx, agentID)
	if err != nil {
		log.Printf("获取MCP工具列表失败: %v", err)
		// 如果获取失败，返回空列表而不是错误
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": []interface{}{}}})
		return
	}

	log.Printf("成功获取MCP工具列表: %v", toolNames)

	// 将工具名称转换为前端期望的格式
	var tools []gin.H
	for _, toolName := range toolNames {
		tools = append(tools, gin.H{
			"name":        toolName,
			"description": fmt.Sprintf("MCP工具: %s", toolName),
			"schema":      true,
		})
	}

	log.Printf("转换后的工具列表: %+v", tools)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"tools": tools}})
}
