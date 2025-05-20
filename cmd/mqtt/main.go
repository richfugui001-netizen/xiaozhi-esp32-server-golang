package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	mqttServer "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/sirupsen/logrus"

	log "xiaozhi-esp32-server-golang/logger"
)

var Server *mqttServer.Server

// 配置结构体
type Config struct {
	Mqtt struct {
		Port int    `json:"port"`
		Host string `json:"host"`
	} `json:"mqtt"`
	Log struct {
		Level    string `json:"level"`
		Filename string `json:"filename"`
	} `json:"log"`
	TLS struct {
		Enable bool   `json:"enable"`
		Pem    string `json:"pem"`
		Key    string `json:"key"`
	} `json:"tls"`
}

// 全局配置
var config Config

// 初始化函数
func Init(configFile string) error {
	// 初始化日志目录
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
		return err
	}

	// 读取配置文件
	fmt.Println("使用配置文件:", configFile)

	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		// 配置文件读取失败时使用默认配置
		fmt.Printf("读取配置文件失败: %v，将使用默认配置\n", err)

		// 设置默认配置
		config.Mqtt.Port = 1883
		config.Mqtt.Host = ""
		config.Log.Level = "info"
		config.Log.Filename = "logs/mqtt_server.log"
	} else {
		// 解析配置文件
		err = json.Unmarshal(data, &config)
		if err != nil {
			fmt.Printf("解析配置文件失败: %v\n", err)
			return err
		}

		// 如果配置中没有设置端口，使用默认值
		if config.Mqtt.Port == 0 {
			config.Mqtt.Port = 1883
		}

		// 如果配置中没有设置日志级别，使用默认值
		if config.Log.Level == "" {
			config.Log.Level = "info"
		}

		// 如果配置中没有设置日志文件名，使用默认值
		if config.Log.Filename == "" {
			config.Log.Filename = "logs/mqtt_server.log"
		}
	}

	// 配置logrus
	logrus.SetFormatter(log.Formatter(false))

	// 设置日志级别
	level, err := logrus.ParseLevel(config.Log.Level)
	if err != nil {
		fmt.Printf("解析日志级别失败: %v，将使用info级别\n", err)
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	// 同时输出到控制台和文件
	logFile, err := os.OpenFile(config.Log.Filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("打开日志文件失败: %v\n", err)
		return err
	}

	// 创建多输出
	mw := io.MultiWriter(os.Stdout, logFile)
	logrus.SetOutput(mw)

	log.Info("配置初始化完成")
	log.Infof("MQTT配置: 主机=%s, 端口=%d", config.Mqtt.Host, config.Mqtt.Port)
	log.Infof("日志配置: 级别=%s, 文件=%s", config.Log.Level, config.Log.Filename)

	return nil
}

func main() {
	// 解析命令行参数
	configFile := flag.String("c", "config/mqtt_config.json", "配置文件路径")
	flag.Parse()

	if *configFile == "" {
		fmt.Println("配置文件路径不能为空")
		return
	}

	// 初始化配置和日志
	err := Init(*configFile)
	if err != nil {
		fmt.Printf("初始化失败: %v\n", err)
		return
	}

	// 启动MQTT服务器
	startMqttServer()

	// 阻塞监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Info("MQTT服务器已启动，按 Ctrl+C 退出")
	<-quit

	log.Info("正在关闭MQTT服务器...")
	if Server != nil {
		Server.Close()
	}
	log.Info("MQTT服务器已关闭")
}

func startMqttServer() {

	Server = mqttServer.New(&mqttServer.Options{
		InlineClient: true,
	})

	err := Server.AddHook(&AuthHook{}, nil)
	if err != nil {
		log.Fatalf("添加 AuthHook 失败: %v", err)
		os.Exit(1)
		return
	}

	// 添加设备钩子
	deviceHook := &DeviceHook{server: Server}
	err = Server.AddHook(deviceHook, nil)
	if err != nil {
		log.Fatalf("添加 DeviceHook 失败: %v", err)
		os.Exit(1)
		return
	}

	// 启动周期性打印订阅主题的任务（每10秒打印一次）
	//deviceHook.StartPeriodicSubscriptionPrinter(10 * time.Second)
	if config.TLS.Enable {
		pemFile := config.TLS.Pem
		keyFile := config.TLS.Key
		cert, err := tls.LoadX509KeyPair(pemFile, keyFile)

		if err != nil {
			log.Fatalf("加载证书失败: %v", err)
			os.Exit(1)
			return
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		ssltcp := listeners.NewTCP(listeners.Config{
			ID:        "ssl",
			Address:   ":8883",
			TLSConfig: tlsConfig,
		})
		err = Server.AddListener(ssltcp)
		if err != nil {
			log.Fatal(err)
		}
	}

	// 使用配置中的端口号
	address := fmt.Sprintf("%s:%d", config.Mqtt.Host, config.Mqtt.Port)
	tcp := listeners.NewTCP(listeners.Config{
		Type:    "tcp",
		ID:      "t1",
		Address: address,
	})
	err = Server.AddListener(tcp)
	if err != nil {
		log.Fatalf("添加 TCP 监听失败: %v", err)
	}

	log.Infof("MQTT 服务器启动，监听 %s 端口...", address)
	err = Server.Serve()
	if err != nil {
		log.Fatalf("MQTT 服务器启动失败: %v", err)
	}
}
