# 数据库重置指南

## 问题描述

当遇到以下错误时：
```
Error 1170 (42000): BLOB/TEXT column 'username' used in key specification without a key length
```

这是因为GORM在MySQL中为`longtext`类型的字段创建唯一索引时没有指定长度。

## 解决方案

### 方案一：使用重置参数（推荐）

使用 `-reset-db` 参数启动服务，这会删除所有现有表并重新创建：

```bash
# 使用默认配置重置数据库
go run main.go -reset-db

# 使用指定配置文件重置数据库
go run main.go -config=config/config.dev.json -reset-db
```

**警告：这会删除所有现有数据！**

### 方案二：手动删除数据库

1. 连接到MySQL数据库
2. 执行重置脚本：
```bash
mysql -u root -p < reset_db.sql
```

或者手动执行：
```sql
DROP DATABASE IF EXISTS xiaozhi_admin;
CREATE DATABASE xiaozhi_admin CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

### 方案三：删除特定表

如果你只想重新创建有问题的表：

```sql
USE xiaozhi_admin;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS agents;
-- 删除其他表...
```

然后重新启动服务让GORM自动创建表。

## 修复内容

我们已经修复了以下模型的字段类型定义：

- `User.Username`: `varchar(50)`
- `User.Password`: `varchar(255)`  
- `User.Email`: `varchar(100)`
- `User.Role`: `varchar(20)`
- `Device.DeviceCode`: `varchar(100)`
- `Device.DeviceName`: `varchar(100)`
- `Device.Status`: `varchar(20)`
- `Agent.Name`: `varchar(100)`
- `Agent.Status`: `varchar(20)`
- 所有配置模型的 `Name`: `varchar(100)`
- 所有配置模型的 `Provider`: `varchar(50)`

## 验证修复

重置数据库后，你应该能看到：

1. 所有表成功创建
2. 默认管理员用户创建成功
3. 服务正常启动

## 启动命令示例

```bash
# 开发环境重置
go run main.go -config=config/config.dev.json -reset-db

# 生产环境（谨慎使用）
go run main.go -config=config/config.prod.json -reset-db

# 正常启动（修复后）
go run main.go -config=config/config.dev.json
```

## 注意事项

1. **数据备份**：重置前请备份重要数据
2. **生产环境**：生产环境请谨慎使用重置功能
3. **权限检查**：确保数据库用户有删除和创建表的权限
4. **配置检查**：确保配置文件中的数据库信息正确