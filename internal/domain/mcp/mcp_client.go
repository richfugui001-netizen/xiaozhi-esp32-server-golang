package mcp

import (
	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
)

func GetToolByName(deviceId string, toolName string) (tool.InvokableTool, bool) {
	tool, ok := globalManager.GetToolByName(toolName)
	if ok {
		return tool, ok
	}

	tool, ok = mcpClientPool.GetToolByDeviceId(deviceId, toolName)
	if !ok {
		return nil, false
	}
	return tool, true
}

func AddDeviceMcpClient(deviceId string, mcpClient *DeviceMCPClient) error {
	mcpClientPool.AddMcpClient(deviceId, mcpClient)
	return nil
}

func RemoveDeviceMcpClient(deviceId string) error {
	mcpClientPool.RemoveMcpClient(deviceId)
	return nil
}

func GetToolsByDeviceId(deviceId string) (map[string]tool.InvokableTool, error) {
	retTools := make(map[string]tool.InvokableTool)
	//从全局管理器获取
	globalTools := globalManager.GetAllTools()
	for toolName, tool := range globalTools {
		retTools[toolName] = tool
	}

	//从MCP客户端池获取
	deviceTools, err := mcpClientPool.GetAllToolsByDeviceId(deviceId)
	if err != nil {
		log.Errorf("获取设备 %s 的工具失败: %v", deviceId, err)
		return retTools, nil
	}
	for toolName, tool := range deviceTools {
		retTools[toolName] = tool
	}
	return retTools, nil
}
