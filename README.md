# xiaozhi-esp32-server-golang
# 项目简介
此项目是 虾哥 小智ai 后端的golang版本，实现了asr输入, llm输出, tts输入输出 全流式处理，依托于golang的高性能可以支撑大并发的请求

# 特性
- 全流式数据处理
- 实现了小智ai websocket
- 实现了mqtt和udp服务器
- 外部资源连接池复用，减少耗时
- 支持 全局mcp、websocket接入点mcp、端侧mcp
- 支持 视觉mcp

# 快速开始
[docker 快速开始](doc/docker.md)

# 延迟测试
[延迟测试结果](doc/delay_test.md)
# 模块简介
## VAD
实现了silvero vad声音检测
## ASR
对接了funasr的websocket服务接口，部署地址 https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md
## LLM
实现了 openai 格式兼容的接口
## TTS
已实现
- doubao websocket
- edgetts   
- cosyvoice
    
# 快速开始

### 安装onnx running依赖
### 部署funasr的服务
https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md
### 编译mqtt服务
go build cmd/mqtt/*.go
### 配置文件 config.json
### 编译xiaozhi-esp32-server-golang
### 启动

# TODO
### 用户认证
### 接入更多云厂商asr服务
### 用户界面
### llm记忆体
### docker化部署


![群二维码_0618(1)](https://github.com/user-attachments/assets/9b444c38-52a3-4f00-95e4-137b20d1c4fe)




个人微信：hackers365

![个人二维码_0618(1)](https://github.com/user-attachments/assets/ac866b29-8cd3-4227-9c53-bf3e9738a056)

