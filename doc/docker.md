
# 运行环境

部署funasr

参见 [funasr docker部署文档](https://github.com/modelscope/FunASR/blob/main/runtime/docs/SDK_advanced_guide_online_zh.md)

克隆代码
>git clone 'https://github.com/hackers365/xiaozhi-esp32-server-golang'

配置config/config.json
>参见 [config配置说明](config.md)

启动docker并挂载config目录和端口(http/websocket:8989, 其它端口按需映射)

```
docker run -itd --name xiaozhi_server -v config:/workspace/config -p 8989:8989 hackers365/xiaozhi_server:0.1

国内连不上的话，使用如下源

docker run -itd --name xiaozhi_server -v config:/workspace/config -p 8989:8989 docker.jsdelivr.fyi/hackers365/xiaozhi_server:0.1
```

现在应该可以连上 
>ws://机器ip:8989/xiaozhi/v1/ 

进行聊天了


# 开发环境
>docker run -itd --name xiaozhi_server_golang -v config:/workspace/config -p 8989:8989 hackers365/xiaozhi_golang:0.1

