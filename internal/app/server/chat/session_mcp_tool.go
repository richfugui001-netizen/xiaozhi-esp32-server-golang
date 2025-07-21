package chat

import llm_memory "xiaozhi-esp32-server-golang/internal/domain/llm/memory"

//此文件处理 local mcp tool 与 session绑定 的工具调用

//关闭会话
func (c *ChatManager) LocalMcpCloseChat() error {
	c.Close()
	return nil
}

//清空历史对话
func (c *ChatManager) LocalMcpClearHistory() error {
	llm_memory.Get().ResetMemory(c.ctx, c.DeviceID)
	return nil
}
