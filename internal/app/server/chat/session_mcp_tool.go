package chat

//此文件处理 local mcp tool 与 session绑定 的工具调用

//关闭会话
func (c *ChatSession) LocalMcpCloseChat() error {
	c.Close()
	return nil
}
