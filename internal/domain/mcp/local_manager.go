package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "xiaozhi-esp32-server-golang/logger"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// LocalToolHandler 本地工具处理函数类型
type LocalToolHandler func(ctx context.Context, argumentsInJSON string) (string, error)

// LocalTool 本地工具定义
type LocalTool struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	InputSchema *schema.ParamsOneOf `json:"input_schema,omitempty"`
	Handler     LocalToolHandler    `json:"-"` // 不序列化处理函数
}

// LocalMCPManager 本地MCP工具管理器
type LocalMCPManager struct {
	tools map[string]*LocalTool // 工具名称 -> 工具定义
	mu    sync.RWMutex          // 读写锁保护并发访问
}

var (
	localManager *LocalMCPManager
	localOnce    sync.Once
)

// GetLocalMCPManager 获取本地MCP管理器单例
func GetLocalMCPManager() *LocalMCPManager {
	localOnce.Do(func() {
		localManager = &LocalMCPManager{
			tools: make(map[string]*LocalTool),
		}
		// 初始化默认的本地工具
		localManager.initDefaultTools()
	})
	return localManager
}

// initDefaultTools 初始化默认的本地工具
func (l *LocalMCPManager) initDefaultTools() {
	// 示例：系统信息工具
	systemInfoTool := &LocalTool{
		Name:        "get_system_info",
		Description: "获取系统基本信息",
		InputSchema: &schema.ParamsOneOf{},
		Handler: func(ctx context.Context, argumentsInJSON string) (string, error) {
			timestamp := "unknown"
			if ts := ctx.Value("timestamp"); ts != nil {
				timestamp = fmt.Sprintf("%v", ts)
			} else {
				timestamp = fmt.Sprintf("%d", time.Now().Unix())
			}
			return `{"status": "running", "version": "1.0.0", "timestamp": "` + timestamp + `"}`, nil
		},
	}
	l.RegisterTool(systemInfoTool)

	// 示例：健康检查工具
	healthCheckTool := &LocalTool{
		Name:        "health_check",
		Description: "系统健康检查",
		InputSchema: &schema.ParamsOneOf{},
		Handler: func(ctx context.Context, argumentsInJSON string) (string, error) {
			return `{"status": "healthy", "message": "服务运行正常"}`, nil
		},
	}
	l.RegisterTool(healthCheckTool)

	log.Info("本地MCP管理器默认工具初始化完成")
}

// RegisterTool 注册本地工具
func (l *LocalMCPManager) RegisterTool(tool *LocalTool) error {
	if tool == nil {
		return fmt.Errorf("工具不能为空")
	}

	if tool.Name == "" {
		return fmt.Errorf("工具名称不能为空")
	}

	if tool.Handler == nil {
		return fmt.Errorf("工具处理函数不能为空")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查工具是否已存在
	if _, exists := l.tools[tool.Name]; exists {
		log.Warnf("本地工具 %s 已存在，将被覆盖", tool.Name)
	}

	l.tools[tool.Name] = tool
	log.Infof("成功注册本地工具: %s - %s", tool.Name, tool.Description)
	return nil
}

// RegisterToolFunc 注册工具函数（简化版本）
func (l *LocalMCPManager) RegisterToolFunc(name, description string, handler LocalToolHandler, inputSchema *schema.ParamsOneOf) error {
	tool := &LocalTool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		Handler:     handler,
	}
	return l.RegisterTool(tool)
}

// UnregisterTool 注销工具
func (l *LocalMCPManager) UnregisterTool(name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, exists := l.tools[name]; !exists {
		return fmt.Errorf("工具 %s 不存在", name)
	}

	delete(l.tools, name)
	log.Infof("成功注销本地工具: %s", name)
	return nil
}

// GetAllTools 获取所有本地工具，返回Eino工具接口格式
func (l *LocalMCPManager) GetAllTools() map[string]tool.InvokableTool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make(map[string]tool.InvokableTool)
	for name, localTool := range l.tools {
		result[name] = &LocalToolWrapper{localTool: localTool}
	}
	return result
}

// GetToolByName 根据名称获取工具
func (l *LocalMCPManager) GetToolByName(name string) (tool.InvokableTool, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	localTool, exists := l.tools[name]
	if !exists {
		return nil, false
	}

	return &LocalToolWrapper{localTool: localTool}, true
}

// GetToolNames 获取所有工具名称列表
func (l *LocalMCPManager) GetToolNames() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	names := make([]string, 0, len(l.tools))
	for name := range l.tools {
		names = append(names, name)
	}
	return names
}

// GetToolCount 获取工具数量
func (l *LocalMCPManager) GetToolCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.tools)
}

// LocalToolWrapper Eino工具接口的本地工具包装器
type LocalToolWrapper struct {
	localTool *LocalTool
}

// Info 获取工具信息，实现tool.BaseTool接口
func (w *LocalToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        w.localTool.Name,
		Desc:        w.localTool.Description,
		ParamsOneOf: w.localTool.InputSchema,
	}, nil
}

// InvokableRun 执行工具，实现tool.InvokableTool接口
func (w *LocalToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	if w.localTool.Handler == nil {
		return "", fmt.Errorf("本地工具 %s 的处理函数未定义", w.localTool.Name)
	}

	log.Infof("执行本地工具: %s, 参数: %s", w.localTool.Name, argumentsInJSON)

	result, err := w.localTool.Handler(ctx, argumentsInJSON)
	if err != nil {
		log.Errorf("本地工具 %s 执行失败: %v", w.localTool.Name, err)
		return "", fmt.Errorf("本地工具执行失败: %v", err)
	}

	log.Infof("本地工具 %s 执行成功，结果: %s", w.localTool.Name, result)
	return result, nil
}

// Start 启动本地管理器（预留接口）
func (l *LocalMCPManager) Start() error {
	log.Info("本地MCP管理器已启动")
	return nil
}

// Stop 停止本地管理器（预留接口）
func (l *LocalMCPManager) Stop() error {
	// 注意：我们不清空工具，因为本地管理器的工具应该在整个应用程序生命周期内保持可用
	// 如果需要清空工具，应该显式调用UnregisterTool方法
	log.Info("本地MCP管理器已停止")
	return nil
}
