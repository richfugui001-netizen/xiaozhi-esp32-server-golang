package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"xiaozhi-esp32-server-golang/internal/app/server/auth"
	manager_client "xiaozhi-esp32-server-golang/internal/app/server/manager_client"
	redisdb "xiaozhi-esp32-server-golang/internal/db/redis"
	user_config "xiaozhi-esp32-server-golang/internal/domain/config"

	log "xiaozhi-esp32-server-golang/logger"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	logrus "github.com/sirupsen/logrus"

	"github.com/spf13/viper"
)

// 全局变量用于控制周期性更新
var (
	configUpdateTicker *time.Ticker
	configUpdateStop   chan struct{}
	configUpdateWg     sync.WaitGroup

	// WebSocket重试控制
	websocketRetryStop chan struct{}
	websocketRetryWg   sync.WaitGroup
)

func Init(configFile string) error {
	//init config
	err := initConfig(configFile)
	if err != nil {
		fmt.Printf("initConfig err: %+v", err)
		os.Exit(1)
		return err
	}

	//init log
	initLog()

	// 从接口获取配置并更新
	if err := updateConfigFromAPI(); err != nil {
		fmt.Printf("从接口获取配置失败，使用本地配置: %v\n", err)
	}

	// 如果配置类型为manager，连接WebSocket
	if err := initManagerWebSocket(); err != nil {
		fmt.Printf("初始化Manager WebSocket连接失败: %v\n", err)
	}

	// 启动周期性配置更新
	startPeriodicConfigUpdate()

	//init vad
	//initVad()

	//init redis
	initRedis()

	//init auth
	err = initAuthManager()
	if err != nil {
		fmt.Printf("initAuthManager err: %+v", err)
		os.Exit(1)
		return err
	}

	return nil
}

// startPeriodicConfigUpdate 启动周期性配置更新
func startPeriodicConfigUpdate() {
	// 从配置中获取更新间隔，默认5分钟
	updateInterval := viper.GetDuration("config_provider.update_interval")
	if updateInterval <= 0 {
		updateInterval = 30 * time.Second
	}

	// 检查是否启用周期性更新
	if !viper.GetBool("config_provider.enable_periodic_update") {
		log.Info("周期性配置更新已禁用")
		return
	}

	configUpdateStop = make(chan struct{})
	configUpdateTicker = time.NewTicker(updateInterval)

	configUpdateWg.Add(1)
	go func() {
		defer configUpdateWg.Done()
		defer configUpdateTicker.Stop()

		for {
			select {
			case <-configUpdateTicker.C:
				if err := updateConfigFromAPI(); err != nil {
					log.Warnf("周期性配置更新失败: %v", err)
				} else {
					log.Debug("周期性配置更新成功")
				}
			case <-configUpdateStop:
				log.Info("周期性配置更新已停止")
				return
			}
		}
	}()

	log.Infof("周期性配置更新已启动，更新间隔: %v", updateInterval)
}

// StopPeriodicConfigUpdate 停止周期性配置更新
func StopPeriodicConfigUpdate() {
	if configUpdateStop != nil {
		close(configUpdateStop)
		configUpdateWg.Wait()
		logrus.Info("周期性配置更新已停止")
	}
}

func initConfig(configFile string) error {
	viper.SetConfigFile(configFile)

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	return nil
}

// updateConfigFromAPI 从接口获取配置并更新viper配置
func updateConfigFromAPI() error {
	configProviderType := viper.GetString("config_provider.type")

	//fmt.Printf("获取系统配置, config_provider.type: %s\n", configProviderType)

	// 从配置文件获取后端管理系统地址
	configProvider, err := user_config.GetProvider(configProviderType)
	if err != nil {
		return fmt.Errorf("获取配置提供者失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取系统配置JSON字符串
	configJSON, err := configProvider.GetSystemConfig(ctx)
	if err != nil {
		return fmt.Errorf("获取系统配置失败: %v", err)
	}

	if configJSON == "" {
		return nil
	}

	// 解析JSON为map
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &configMap); err != nil {
		return fmt.Errorf("解析配置JSON失败: %v", err)
	}

	// 使用viper.MergeConfigMap设置到viper
	if err := viper.MergeConfigMap(configMap); err != nil {
		return fmt.Errorf("合并配置到viper失败: %v", err)
	}

	return nil
}

func initLog() error {
	// 输出到文件
	binPath, _ := os.Executable()
	baseDir := filepath.Dir(binPath)
	logPath := fmt.Sprintf("%s/%s%s", baseDir, viper.GetString("log.path"), viper.GetString("log.file"))
	/* 日志轮转相关函数
	`WithLinkName` 为最新的日志建立软连接
	`WithRotationTime` 设置日志分割的时间，隔多久分割一次
	WithMaxAge 和 WithRotationCount二者只能设置一个
		`WithMaxAge` 设置文件清理前的最长保存时间
		`WithRotationCount` 设置文件清理前最多保存的个数
	*/
	// 下面配置日志每隔 1 分钟轮转一个新文件，保留最近 3 分钟的日志文件，多余的自动清理掉。
	writer, err := rotatelogs.New(
		logPath+".%Y%m%d",
		rotatelogs.WithLinkName(logPath),
		rotatelogs.WithRotationCount(uint(viper.GetInt("log.max_age"))),
		rotatelogs.WithRotationTime(time.Duration(86400)*time.Second),
	)
	if err != nil {
		fmt.Printf("init log error: %v\n", err)
		os.Exit(1)
		return err
	}

	// 根据配置决定输出目标
	if viper.GetBool("log.stdout") {
		// 同时输出到文件和标准输出
		multiWriter := io.MultiWriter(writer, os.Stdout)
		logrus.SetOutput(multiWriter)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", //时间格式化，添加毫秒
			ForceColors:     true,                      // 标准输出启用颜色
		})
	} else {
		// 只输出到文件
		logrus.SetOutput(writer)
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000", //时间格式化，添加毫秒
			ForceColors:     false,                     // 文件输出不启用颜色
		})
	}

	// 禁用默认的调用者报告，使用自定义的caller字段
	logrus.SetReportCaller(false)
	logLevel, _ := logrus.ParseLevel(viper.GetString("log.level"))
	logrus.SetLevel(logLevel)

	return nil
}

/*
	func initVad() error {
		err := vad.InitVAD()
		if err != nil {
			fmt.Printf("initVad error: %v\n", err)
			os.Exit(1)
			return err
		}
		return nil
	}
*/
func initRedis() error {
	// 初始化我们的统一Redis模块
	redisConfig := &redisdb.Config{
		Host:     viper.GetString("redis.host"),
		Port:     viper.GetInt("redis.port"),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	}

	err := redisdb.Init(redisConfig)
	if err != nil {
		fmt.Printf("init redis error: %v\n", err)
		return err
	}

	return nil
}

// initManagerWebSocket 初始化Manager WebSocket连接
func initManagerWebSocket() error {
	configProviderType := viper.GetString("config_provider.type")

	// 只有当配置类型为manager时才连接WebSocket
	if configProviderType != "manager" {
		log.Infof("配置类型为 %s，跳过WebSocket连接", configProviderType)
		return nil
	}

	// 初始化WebSocket重试控制通道
	websocketRetryStop = make(chan struct{})

	// 启动WebSocket连接重试协程
	websocketRetryWg.Add(1)
	go retryManagerWebSocketConnection()

	log.Info("Manager WebSocket连接重试协程已启动")
	return nil
}

// retryManagerWebSocketConnection 使用退避算法重试WebSocket连接
func retryManagerWebSocketConnection() {
	defer websocketRetryWg.Done()

	// 硬编码的退避算法参数
	initialDelay := 3 * time.Second // 初始延迟3秒
	maxDelay := 1 * time.Minute     // 最大延迟1分钟
	backoffMultiplier := 2.0        // 退避倍数

	// 指数退避算法
	delay := initialDelay
	retryCount := 0

	for {
		select {
		case <-websocketRetryStop:
			log.Info("收到关闭信号，停止WebSocket重试")
			return
		default:
			// 尝试连接
			if err := tryConnectManagerWebSocket(); err != nil {
				retryCount++
				log.Warnf("Manager WebSocket连接失败 (第%d次): %v", retryCount, err)

				// 计算下一次延迟时间
				delay = time.Duration(float64(delay) * backoffMultiplier)
				if delay > maxDelay {
					delay = maxDelay
				}

				log.Infof("等待 %v 后重试连接...", delay)

				// 使用select实现可中断的延迟
				select {
				case <-websocketRetryStop:
					log.Info("收到关闭信号，停止WebSocket重试")
					return
				case <-time.After(delay):
					continue
				}
			}

			// 连接成功，重置重试计数和延迟
			log.Info("Manager WebSocket连接成功")
			retryCount = 0
			delay = initialDelay

			// 监控连接状态，如果连接断开则重新开始重试
			if err := monitorWebSocketConnection(); err != nil {
				log.Warnf("WebSocket连接监控检测到断开: %v", err)
				continue
			}
		}
	}
}

// monitorWebSocketConnection 监控WebSocket连接状态
func monitorWebSocketConnection() error {
	// 定期发送ping来检测连接状态
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 连续ping失败计数
	pingFailCount := 0
	maxPingFailCount := 3 // 允许连续失败3次

	for {
		select {
		case <-websocketRetryStop:
			return fmt.Errorf("收到关闭信号")
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := testManagerWebSocketConnection(ctx); err != nil {
				pingFailCount++
				log.Warnf("WebSocket连接状态检测失败 (第%d次): %v", pingFailCount, err)

				// 只有连续失败超过阈值才认为连接断开
				if pingFailCount >= maxPingFailCount {
					cancel()
					log.Warnf("连续ping失败%d次，认为WebSocket连接已断开", maxPingFailCount)
					return fmt.Errorf("连接状态检测失败: %v", err)
				}

				// 失败次数未达到阈值，继续监控
				cancel()
				continue
			}
			cancel()

			// ping成功，重置失败计数
			if pingFailCount > 0 {
				log.Infof("WebSocket连接状态检测恢复成功，重置失败计数")
				pingFailCount = 0
			}
		}
	}
}

// tryConnectManagerWebSocket 尝试连接Manager WebSocket
func tryConnectManagerWebSocket() error {
	// 创建上下文，设置较短的超时时间
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 连接到Manager WebSocket
	if err := manager_client.ConnectManagerWebSocket(ctx); err != nil {
		return fmt.Errorf("连接Manager WebSocket失败: %v", err)
	}

	// 测试连接
	if err := testManagerWebSocketConnection(ctx); err != nil {
		return fmt.Errorf("连接测试失败: %v", err)
	}

	return nil
}

// testManagerWebSocketConnection 测试Manager WebSocket连接
func testManagerWebSocketConnection(ctx context.Context) error {
	// 发送ping请求
	if err := manager_client.ManagerWebSocketPing(ctx); err != nil {
		return fmt.Errorf("ping测试失败: %v", err)
	}

	log.Infof("Manager WebSocket连接测试成功")
	return nil
}

func initAuthManager() error {
	return auth.Init()
}

// StopWebSocketRetry 优雅关闭WebSocket重试协程
func StopWebSocketRetry() {
	if websocketRetryStop != nil {
		close(websocketRetryStop)
		websocketRetryWg.Wait()
		log.Info("WebSocket重试协程已优雅关闭")
	}
}
