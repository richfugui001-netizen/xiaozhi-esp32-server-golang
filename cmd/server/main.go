package main

import (
	"flag"
	"fmt"
	"xiaozhi-esp32-server-golang/internal/app/server"
	log "xiaozhi-esp32-server-golang/logger"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 解析命令行参数
	configFile := flag.String("c", "config/config.json", "配置文件路径")
	flag.Parse()

	if *configFile == "" {
		fmt.Println("配置文件路径不能为空")
		return
	}

	err := Init(*configFile)
	if err != nil {
		return
	}

	// 创建服务器
	err = server.InitServer()
	if err != nil {
		log.Fatalf("初始化服务器失败: %v", err)
		return
	}

	// 阻塞监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Info("服务器已启动，按 Ctrl+C 退出")
	<-quit

	log.Info("正在关闭服务器...")
	// TODO: 在这里添加清理资源的代码
	log.Info("服务器已关闭")
}
