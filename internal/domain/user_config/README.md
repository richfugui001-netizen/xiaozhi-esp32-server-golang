# 用户配置 Provider 模式

## 概述

本模块实现了用户配置的provider模式，支持多种存储后端（Redis、内存、文件等），提供统一的接口来管理用户配置数据。

## 架构设计

### 接口层次

```
UserConfig (原有接口)           - 只读接口，向后兼容
    ↑
UserConfigProvider (新接口)     - 完整的CRUD接口
    ↑
具体实现 (Redis, Memory, File)   - 不同存储后端的实现
```

### 核心接口

#### 1. UserConfig (interface.go)
原有的接口，保持向后兼容：
```go
type UserConfig interface {
    GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)
}
```

#### 2. UserConfigProvider (base.go)
扩展的provider接口，支持完整的CRUD操作：
```go
type UserConfigProvider interface {
    GetUserConfig(ctx context.Context, userID string) (types.UConfig, error)
    SetUserConfig(ctx context.Context, userID string, config types.UConfig) error
    DeleteUserConfig(ctx context.Context, userID string) error
    Close() error
}
```

## 支持的存储类型

### 1. Redis Provider
- **类型**: `"redis"`
- **配置参数**:
  - `host`: Redis主机地址 (默认: "localhost")
  - `port`: Redis端口 (默认: 6379)
  - `password`: Redis密码 (默认: "")
  - `db`: 数据库编号 (默认: 0)
  - `prefix`: 键前缀 (默认: "xiaozhi")

```go
config := map[string]interface{}{
    "host":     "localhost",
    "port":     6379,
    "password": "",
    "db":       0,
    "prefix":   "xiaozhi",
}
provider, err := GetUserConfigProvider("redis", config)
```

### 2. Memory Provider
- **类型**: `"memory"`
- **配置参数**:
  - `max_entries`: 最大存储条目数 (默认: 1000)
- **注意**: 重启后数据丢失，适用于测试或临时存储

```go
config := map[string]interface{}{
    "max_entries": 500,
}
provider, err := GetUserConfigProvider("memory", config)
```

### 3. File Provider (TODO)
- **类型**: `"file"`
- **状态**: 暂未实现

## 使用方法

### 基本使用

```go
import (
    "context"
    "xiaozhi-esp32-server-golang/internal/domain/user_config"
    "xiaozhi-esp32-server-golang/internal/domain/user_config/types"
)

func main() {
    ctx := context.Background()
    
    // 1. 创建provider
    config := map[string]interface{}{
        "host": "localhost",
        "port": 6379,
    }
    provider, err := user_config.GetUserConfigProvider("redis", config)
    if err != nil {
        log.Fatal(err)
    }
    defer provider.Close()
    
    // 2. 设置用户配置
    userConfig := types.UConfig{
        SystemPrompt: "你是一个有用的AI助手",
        Llm: types.LlmConfig{Type: "openai"},
        Tts: types.TtsConfig{Type: "edge"},
        Asr: types.AsrConfig{Type: "funasr"},
    }
    
    err = provider.SetUserConfig(ctx, "user123", userConfig)
    if err != nil {
        log.Fatal(err)
    }
    
    // 3. 获取用户配置
    config, err := provider.GetUserConfig(ctx, "user123")
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. 删除用户配置
    err = provider.DeleteUserConfig(ctx, "user123")
    if err != nil {
        log.Fatal(err)
    }
}
```

### 使用默认配置

```go
// 使用默认配置创建provider
provider, err := user_config.GetProviderWithDefaultConfig("redis")
```

### 配置验证

```go
// 验证配置参数
config := map[string]interface{}{
    "host": "localhost",
    "port": 6379,
}

if err := user_config.ValidateConfig("redis", config); err != nil {
    log.Fatal("配置验证失败:", err)
}

provider, err := user_config.GetUserConfigProvider("redis", config)
```

### 适配器模式（向后兼容）

如果需要将新的provider接口适配为原有的UserConfig接口：

```go
provider, err := user_config.GetUserConfigProvider("memory", nil)
if err != nil {
    log.Fatal(err)
}

// 适配为原有接口
userConfig := user_config.NewUserConfigAdapter(provider)

// 只能使用GetUserConfig方法
config, err := userConfig.GetUserConfig(ctx, "user123")
```

## 扩展新的存储类型

要添加新的存储类型（如文件存储），需要：

1. **创建实现包**（如 `internal/domain/user_config/file/`）
2. **实现UserConfigProvider接口**
3. **在base.go中注册**：

```go
// 在GetUserConfigProvider函数中添加
case "file":
    provider, err := userconfig_file.NewFileUserConfigProvider(config)
    if err != nil {
        return nil, fmt.Errorf("创建文件用户配置提供者失败: %v", err)
    }
    return provider, nil
```

4. **添加配置验证和默认配置**

## 测试

运行测试：
```bash
go test ./internal/domain/user_config/ -v
```

测试覆盖了：
- Memory provider的CRUD操作
- 适配器模式
- 默认配置获取
- 配置验证

## 特性

✅ **多存储后端**: 支持Redis、内存等多种存储方式  
✅ **向后兼容**: 保持原有UserConfig接口不变  
✅ **配置验证**: 提供配置参数验证功能  
✅ **默认配置**: 每种存储类型都有合理的默认配置  
✅ **适配器模式**: 新老接口无缝切换  
✅ **完整测试**: 包含单元测试和集成测试  
✅ **易于扩展**: 简单的接口设计，容易添加新的存储类型  

## 注意事项

1. **Redis连接**: Redis provider会在创建时测试连接，确保Redis服务可用
2. **内存限制**: Memory provider有最大条目数限制，防止内存溢出
3. **资源清理**: 使用完成后记得调用Close()方法释放资源
4. **错误处理**: 所有方法都返回详细的错误信息，便于调试
5. **线程安全**: Memory provider使用读写锁保证并发安全 