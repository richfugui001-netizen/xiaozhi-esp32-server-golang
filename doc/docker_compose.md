# Docker Compose 部署指南

## 概述

本项目使用 Docker Compose 进行容器化部署，包含以下核心服务：

- **MySQL 数据库服务** - 数据存储
- **主程序服务** - 核心业务逻辑
- **后端管理服务** - API 接口服务
- **前端管理服务** - Web 管理界面

## 服务架构

### 1. MySQL 数据库服务 (xiaozhi-mysql)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/mysql:8.0`
- 端口映射：`23306:3306`
- 数据库名：`xiaozhi_admin`
- 用户名：`root`
- 密码：`password`

**特性：**
- 使用 MySQL 8.0 版本
- 配置了健康检查机制
- 数据持久化存储
- 支持 MySQL 原生密码认证

### 2. 主程序服务 (xiaozhi-main-server)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/hackers365/xiaozhi_golang:latest`
- 端口映射：
  - `18989:8989` - 主服务端口
  - `12883:2883` - MQTT 服务端口
  - `8888:8888/udp` - UDP 服务端口

**依赖关系：**
- 依赖 MySQL 服务健康状态
- 依赖后端服务启动完成

**环境变量：**
- 数据库连接配置
- 后端服务地址配置

### 3. 后端管理服务 (xiaozhi-backend)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/hackers365/xiaozhi_manager_backend:latest`
- 端口映射：`28080:8080`

**依赖关系：**
- 依赖 MySQL 服务健康状态

**功能：**
- 提供 RESTful API 接口
- 设备管理功能
- 用户管理功能

### 4. 前端管理服务 (xiaozhi-frontend)

**配置信息：**
- 镜像：`docker.jsdelivr.fyi/hackers365/xiaozhi_manager_frontend:latest`
- 端口映射：`18080:80`

**依赖关系：**
- 依赖后端服务

**功能：**
- Web 管理界面
- 设备状态监控
- 系统配置管理

## 部署流程

### 1. 环境准备

确保系统已安装 Docker 和 Docker Compose：

```bash
# 检查 Docker 版本
docker --version

# 检查 Docker Compose 版本
docker-compose --version
```

### 2. 配置文件准备

确保以下目录和文件存在：

```
xiaozhi-esp32-server-golang/
├── docker/docker-composer/
│   └── docker-compose.yml
├── config/
│   └── (配置文件)
├── logs/
│   └── (日志目录)
└── manager/backend/config/
    └── (后端配置)
```

### 3. 启动服务

在 `docker/docker-composer/` 目录下执行：

```bash
# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看服务日志
docker-compose logs -f
```

### 4. 服务访问

启动成功后，可通过以下地址访问各服务：

- **前端管理界面**：http://localhost:18080
- **后端 API**：http://localhost:28080
- **主服务**：http://localhost:18989
- **MQTT 服务**：localhost:12883
- **MySQL 数据库**：localhost:23306

## 常用操作

### 查看服务状态

```bash
# 查看所有服务状态
docker-compose ps

# 查看特定服务状态
docker-compose ps main-server
```

### 查看服务日志

```bash
# 查看所有服务日志
docker-compose logs

# 查看特定服务日志
docker-compose logs main-server

# 实时查看日志
docker-compose logs -f main-server
```

### 重启服务

```bash
# 重启所有服务
docker-compose restart

# 重启特定服务
docker-compose restart main-server
```

### 停止服务

```bash
# 停止所有服务
docker-compose down

# 停止并删除数据卷
docker-compose down -v
```

### 更新服务

```bash
# 拉取最新镜像并重启服务
docker-compose pull
docker-compose up -d
```

## 网络配置

项目使用自定义网络 `xiaozhi-network`，所有服务都在同一网络下，可以通过服务名进行内部通信：

- MySQL 服务：`mysql:3306`
- 后端服务：`backend:8080`
- 前端服务：`frontend:80`

## 数据持久化

### MySQL 数据

MySQL 数据通过 Docker 卷 `mysql_data` 进行持久化存储，数据不会因容器重启而丢失。

### 配置文件

主程序和后端服务的配置文件通过卷挂载方式映射到容器内：

- 主程序配置：`../../config:/workspace/config`
- 后端配置：`../../manager/backend/config:/root/config`

### 日志文件

日志文件通过卷挂载方式映射到容器内：

- 主程序日志：`../../logs:/workspace/logs`

## 健康检查

MySQL 服务配置了健康检查机制：

```yaml
healthcheck:
  test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-ppassword"]
  timeout: 20s
  retries: 10
  interval: 10s
  start_period: 30s
```

其他服务依赖 MySQL 的健康状态，确保数据库完全启动后才启动相关服务。

## 故障排除

### 1. 服务启动失败

```bash
# 查看详细错误信息
docker-compose logs [服务名]

# 检查端口占用
netstat -tulpn | grep [端口号]
```

### 2. 数据库连接失败

```bash
# 检查 MySQL 服务状态
docker-compose ps mysql

# 查看 MySQL 日志
docker-compose logs mysql

# 进入 MySQL 容器检查
docker-compose exec mysql mysql -u root -ppassword
```

### 3. 网络连接问题

```bash
# 检查网络配置
docker network ls
docker network inspect xiaozhi-network

# 测试服务间通信
docker-compose exec main-server ping mysql
```

## 性能优化建议

1. **资源限制**：在生产环境中，建议为各服务设置资源限制
2. **日志轮转**：配置日志轮转避免日志文件过大
3. **备份策略**：定期备份 MySQL 数据
4. **监控**：集成监控系统监控服务状态

## 安全注意事项

1. **密码安全**：生产环境中请修改默认密码
2. **端口暴露**：根据实际需要调整端口映射
3. **网络安全**：配置防火墙规则
4. **镜像安全**：使用官方或可信的镜像源
