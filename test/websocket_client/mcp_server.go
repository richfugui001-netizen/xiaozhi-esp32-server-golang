package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type McpInterface interface {
	SendMcpMsg(payload json.RawMessage) error
	RecvMcpMsg(timeOut int) ([]byte, error)
}

type McpTransport struct {
	SendMsgChan chan []byte
	RecvMsgChan chan []byte
}

func (c *McpTransport) SendMcpMsg(payload json.RawMessage) error {
	serverMsg := ServerMessage{
		Type:    MessageTypeMcp,
		PayLoad: payload,
	}
	serverBytes, err := json.Marshal(serverMsg)
	if err != nil {
		return err
	}
	select {
	case c.SendMsgChan <- serverBytes:
		return nil
	case <-time.After(time.Duration(2000) * time.Millisecond):
		return fmt.Errorf("mcp 发送消息超时")
	}
}

func (c *McpTransport) RecvMcpMsg(timeOut int) ([]byte, error) {
	select {
	case msg := <-c.RecvMsgChan:
		return msg, nil
	case <-time.After(time.Duration(timeOut) * time.Millisecond):
		return nil, fmt.Errorf("mcp 接收消息超时")
	}
}

func NewMcpServer(sendMsgChan chan []byte, recvMsgChan chan []byte) {
	/*
		hooks := &server.Hooks{}

		hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
			result.ServerInfo.Name = "taiji-pi-s3"
			result.ServerInfo.Version = "1.0.0"
			fmt.Printf("afterInitialize: %v, %v, %v\n", id, message, result)
		})*/

	s := server.NewMCPServer("taiji-pi-s3", "1.0.0")

	// Add tool
	tool := mcp.NewTool("hello_world",
		mcp.WithDescription("Say hello to someone"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the person to greet"),
		),
	)

	// 新增查询天气工具
	weatherTool := mcp.NewTool("query_weather",
		mcp.WithDescription("查询大连天气"),
	)

	// 新增生成随机数工具（参数类型为 string，handler 内部转换）
	randomNumberTool := mcp.NewTool("random_number",
		mcp.WithDescription("生成指定范围的随机整数"),
		mcp.WithNumber("min",
			mcp.Required(),
			mcp.Description("最小值"),
		),
		mcp.WithNumber("max",
			mcp.Required(),
			mcp.Description("最大值"),
		),
	)

	// Add tool handler
	s.AddTool(tool, helloHandler)
	// 注册查询天气工具
	s.AddTool(weatherTool, queryWeatherHandler)
	// 注册生成随机数工具
	s.AddTool(randomNumberTool, randomNumberHandler)

	mcpHandle := &McpTransport{
		SendMsgChan: sendMsgChan,
		RecvMsgChan: recvMsgChan,
	}

	transport, err := NewWebSocketServerTransport(mcpHandle, WithWebSocketServerOptionMcpServer(s))
	if err != nil {
		log.Fatalf("Failed to create websocket server transport: %v", err)
	}
	transport.Run()
}

func helloHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Hello, %s!", name)), nil
}

// 查询天气 handler
func queryWeatherHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("天气晴朗 20度 北风3级"), nil
}

// 生成随机数 handler
func randomNumberHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	//重新实现
	min := request.GetInt("min", 0)
	max := request.GetInt("max", 100)

	if min > max {
		return mcp.NewToolResultError("min 不能大于 max"), nil
	}
	rnd := min
	if max > min {
		rnd = min + int(time.Now().UnixNano()%int64(max-min+1))
	}
	return mcp.NewToolResultText(fmt.Sprintf("随机数：%d", rnd)), nil
}
